package notes

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"wox/common"
)

// documentFromPluginCommand builds a note from optional title, text, and a filesystem path.
func documentFromPluginCommand(title string, text string, path string) (common.NoteDocument, error) {
	title = strings.TrimSpace(title)
	text = strings.TrimRight(text, "\n")
	path = strings.TrimSpace(path)

	if title == "" && strings.TrimSpace(text) == "" && path == "" {
		return common.NoteDocument{}, fmt.Errorf("note content is required")
	}

	if path == "" {
		document := ParseClipboard(text)
		if title != "" {
			document.Blocks = append([]common.NoteBlock{{
				ID:   uuid.NewString(),
				Type: common.NoteBlockHeading1,
				Text: title,
			}}, document.Blocks...)
		}
		return NormalizeDocument(document), nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return common.NoteDocument{}, fmt.Errorf("invalid path: %s", path)
	}

	if isLikelyNoteImagePath(path) && !info.IsDir() {
		return documentFromImagePath(title, path)
	}

	if title == "" {
		title = filepath.Base(path)
	}

	var markdown strings.Builder
	if title != "" {
		markdown.WriteString("# ")
		markdown.WriteString(title)
		markdown.WriteString("\n\n")
	}
	markdown.WriteString("[")
	markdown.WriteString(path)
	markdown.WriteString("](")
	markdown.WriteString(path)
	markdown.WriteString(")\n")

	if !info.IsDir() {
		if fileText, ok := readNoteSourceFile(path); ok && strings.TrimSpace(fileText) != "" {
			markdown.WriteString("\n")
			markdown.WriteString(fileText)
			if !strings.HasSuffix(fileText, "\n") {
				markdown.WriteString("\n")
			}
		}
	}

	if strings.TrimSpace(text) != "" {
		markdown.WriteString("\n")
		markdown.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			markdown.WriteString("\n")
		}
	}

	return ParseMarkdown(markdown.String()), nil
}

// documentFromImagePath copies the file into notes attachments and stores only an image block.
// OCR and other caller-supplied text are ignored so a screenshot note is the picture itself.
func documentFromImagePath(title string, path string) (common.NoteDocument, error) {
	imported, err := ImportNoteImage(path)
	if err != nil {
		return common.NoteDocument{}, err
	}
	return documentFromImportedImage(title, imported), nil
}

// DocumentFromClipboardImage stores a pasted bitmap as a local notes attachment.
func DocumentFromClipboardImage(img image.Image, fileName string) (common.NoteDocument, error) {
	imported, err := ImportNoteImageFromImage(img, fileName)
	if err != nil {
		return common.NoteDocument{}, err
	}
	return documentFromImportedImage("", imported), nil
}

// DocumentFromClipboardFiles turns copied filesystem paths into image blocks or path links.
func DocumentFromClipboardFiles(paths []string) common.NoteDocument {
	var blocks []common.NoteBlock
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if isLikelyNoteImagePath(path) {
			imported, err := ImportNoteImage(path)
			if err == nil {
				blocks = append(blocks, common.NoteBlock{Type: common.NoteBlockImage, Image: &imported})
				continue
			}
		}
		name := filepath.Base(path)
		blocks = append(blocks, ParseMarkdown(fmt.Sprintf("[%s](%s)", name, path)).Blocks...)
	}
	if len(blocks) == 0 {
		return common.NoteDocument{}
	}
	return NormalizeDocument(common.NoteDocument{Version: documentVersion, Blocks: blocks})
}

func documentFromImportedImage(title string, imported common.NoteImage) common.NoteDocument {
	blocks := []common.NoteBlock{
		{ID: uuid.NewString(), Type: common.NoteBlockParagraph},
		{ID: uuid.NewString(), Type: common.NoteBlockImage, Image: &imported},
		{ID: uuid.NewString(), Type: common.NoteBlockParagraph},
	}
	if title != "" && title != imported.FileName && title != filepath.Base(imported.FileName) {
		blocks = append([]common.NoteBlock{{
			ID: uuid.NewString(), Type: common.NoteBlockHeading1, Text: title,
		}}, blocks...)
	}
	return NormalizeDocument(common.NoteDocument{Version: documentVersion, Blocks: blocks})
}

// readNoteSourceFile imports a small UTF-8 text file so binary and huge files stay path-only.
func readNoteSourceFile(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() <= 0 || info.Size() > createNoteMaxFileBytes {
		return "", false
	}
	if isLikelyNoteImagePath(path) {
		return "", false
	}

	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 {
		return "", false
	}
	return string(data), true
}

// isLikelyNoteImagePath treats common image extensions as attachment imports instead of text files.
func isLikelyNoteImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".heic", ".heif":
		return true
	default:
		return false
	}
}
