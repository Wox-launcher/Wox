package filesearch

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	pdf "github.com/ledongthuc/pdf"
)

const openXMLMaxEntryReadBytes int64 = 16 * 1024 * 1024

// extractContentText returns plain text suitable for content indexing. Text
// files are read directly, while supported document containers are parsed
// through format-specific extractors so binary bytes are never indexed as text.
func extractContentText(path string, maxBytes int64) (string, error) {
	switch contentNormalizeExtension(path) {
	case "docx":
		return extractOpenXMLText(path, maxBytes, openXMLWordTextFile)
	case "pptx":
		return extractOpenXMLText(path, maxBytes, openXMLPowerPointTextFile)
	case "xlsx":
		return extractOpenXMLText(path, maxBytes, openXMLSpreadsheetTextFile)
	case "pdf":
		return extractPDFText(path, maxBytes)
	default:
		return readPlainContentFile(path, maxBytes)
	}
}

// contentExtractionMaxBytes keeps text files bounded by their file size, while
// compressed document containers use the configured extracted-text cap because
// their XML text can legitimately be larger than the archive file itself.
func contentExtractionMaxBytes(path string, fileSize int64, maxBytes int64) int64 {
	if maxBytes <= 0 {
		return 0
	}
	if isStructuredContentExtension(contentNormalizeExtension(path)) {
		return maxBytes
	}
	if fileSize < maxBytes {
		return fileSize
	}
	return maxBytes
}

// isStructuredContentExtension reports formats that need a parser instead of a
// direct byte-to-string read.
func isStructuredContentExtension(extension string) bool {
	switch extension {
	case "docx", "pptx", "xlsx", "pdf":
		return true
	default:
		return false
	}
}

// readPlainContentFile reads the leading bytes of a regular text-like file.
func readPlainContentFile(path string, maxBytes int64) (string, error) {
	if maxBytes <= 0 {
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(f, maxBytes)
	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(limited, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	return string(buf[:n]), nil
}

// extractPDFText indexes the embedded text layer only. Scanned image-only PDFs
// need OCR and intentionally stay outside the lightweight content index path.
func extractPDFText(path string, maxBytes int64) (string, error) {
	f, reader, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	builder := newContentTextBuilder(maxBytes)
	fonts := map[string]*pdf.Font{}
	for pageNum := 1; pageNum <= reader.NumPage() && !builder.Done(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(fonts)
		if err != nil {
			return "", fmt.Errorf("extract pdf page %d: %w", pageNum, err)
		}
		builder.AppendText(text)
		builder.AppendSeparator()
	}
	return builder.String(), nil
}

type openXMLTextFileMatcher func(name string) bool

// openXMLWordTextFile selects user-authored Word document text parts.
func openXMLWordTextFile(name string) bool {
	if name == "word/document.xml" || name == "word/footnotes.xml" || name == "word/endnotes.xml" || name == "word/comments.xml" {
		return true
	}
	return strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") ||
		strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml")
}

// openXMLPowerPointTextFile selects slide, notes, and comment text parts.
func openXMLPowerPointTextFile(name string) bool {
	return strings.HasPrefix(name, "ppt/slides/slide") && strings.HasSuffix(name, ".xml") ||
		strings.HasPrefix(name, "ppt/notesSlides/notesSlide") && strings.HasSuffix(name, ".xml") ||
		strings.HasPrefix(name, "ppt/comments/comment") && strings.HasSuffix(name, ".xml")
}

// openXMLSpreadsheetTextFile selects shared strings plus inline worksheet text.
func openXMLSpreadsheetTextFile(name string) bool {
	return name == "xl/sharedStrings.xml" ||
		strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml")
}

// extractOpenXMLText streams selected XML entries from an Office OpenXML zip
// package and collects text nodes without loading the whole document in memory.
func extractOpenXMLText(path string, maxBytes int64, match openXMLTextFileMatcher) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	files := make([]*zip.File, 0)
	for _, file := range reader.File {
		name := strings.TrimPrefix(file.Name, "/")
		if match(name) {
			files = append(files, file)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	builder := newContentTextBuilder(maxBytes)
	for _, file := range files {
		if builder.Done() {
			break
		}
		if err := appendOpenXMLFileText(file, builder); err != nil {
			return "", err
		}
	}
	return builder.String(), nil
}

// appendOpenXMLFileText appends character data from a single XML part.
func appendOpenXMLFileText(file *zip.File, builder *contentTextBuilder) error {
	readCloser, err := file.Open()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	decoder := xml.NewDecoder(io.LimitReader(readCloser, openXMLMaxEntryReadBytes))
	for !builder.Done() {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if isXMLUnexpectedEOF(err) {
				return nil
			}
			return fmt.Errorf("parse %s: %w", file.Name, err)
		}
		if charData, ok := token.(xml.CharData); ok {
			builder.AppendText(string(charData))
			builder.AppendSeparator()
		}
	}
	return nil
}

func isXMLUnexpectedEOF(err error) bool {
	if syntaxErr, ok := err.(*xml.SyntaxError); ok {
		return strings.Contains(strings.ToLower(syntaxErr.Msg), "unexpected eof")
	}
	return strings.Contains(strings.ToLower(err.Error()), "unexpected eof")
}

type contentTextBuilder struct {
	builder   strings.Builder
	maxBytes  int
	lastSpace bool
}

func newContentTextBuilder(maxBytes int64) *contentTextBuilder {
	if maxBytes < 0 {
		maxBytes = 0
	}
	return &contentTextBuilder{maxBytes: int(maxBytes), lastSpace: true}
}

func (b *contentTextBuilder) AppendText(text string) {
	for _, r := range text {
		if b.Done() {
			return
		}
		if unicode.IsSpace(r) {
			b.AppendSeparator()
			continue
		}
		b.appendRune(r)
		b.lastSpace = false
	}
}

func (b *contentTextBuilder) AppendSeparator() {
	if b.Done() || b.lastSpace || b.builder.Len() == 0 {
		return
	}
	b.appendRune(' ')
	b.lastSpace = true
}

func (b *contentTextBuilder) Done() bool {
	return b.builder.Len() >= b.maxBytes
}

func (b *contentTextBuilder) String() string {
	return strings.TrimSpace(b.builder.String())
}

func (b *contentTextBuilder) appendRune(r rune) {
	if b.maxBytes <= 0 {
		return
	}
	if b.builder.Len()+len(string(r)) > b.maxBytes {
		return
	}
	b.builder.WriteRune(r)
}
