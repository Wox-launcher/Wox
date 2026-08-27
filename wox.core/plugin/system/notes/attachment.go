package notes

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	"wox/common"
	"wox/util"
)

const notesImageRefPrefix = "notes-image:"

// ImportNoteImage copies a source image into the local notes attachments directory.
// Bytes stay on disk so Cloud Sync only transports the attachment id in note JSON.
func ImportNoteImage(src string) (common.NoteImage, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return common.NoteImage{}, fmt.Errorf("image path is required")
	}
	source, err := os.Open(src)
	if err != nil {
		return common.NoteImage{}, fmt.Errorf("open image: %w", err)
	}
	defer source.Close()

	directory := util.GetLocation().GetNotesAttachmentsDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		return common.NoteImage{}, err
	}

	id := uuid.NewString() + noteImageExtension(src)
	destPath := filepath.Join(directory, id)
	dest, err := os.Create(destPath)
	if err != nil {
		return common.NoteImage{}, fmt.Errorf("create attachment: %w", err)
	}
	if _, err := io.Copy(dest, source); err != nil {
		dest.Close()
		_ = os.Remove(destPath)
		return common.NoteImage{}, fmt.Errorf("copy attachment: %w", err)
	}
	if err := dest.Close(); err != nil {
		_ = os.Remove(destPath)
		return common.NoteImage{}, err
	}

	imported := common.NoteImage{ID: id, FileName: filepath.Base(src)}
	if config, _, err := decodeNoteImageConfig(destPath); err == nil {
		imported.Width = config.Width
		imported.Height = config.Height
	}
	return imported, nil
}

// RemoveNoteAttachment deletes one local attachment file. Missing files are ignored.
func RemoveNoteAttachment(id string) error {
	path := ResolveNoteImagePath(common.NoteImage{ID: id})
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// NoteAttachmentIDs returns the sanitized attachment ids referenced by one document.
func NoteAttachmentIDs(document common.NoteDocument) []string {
	ids := make([]string, 0)
	seen := map[string]struct{}{}
	for _, block := range document.Blocks {
		if block.Type != common.NoteBlockImage || block.Image == nil {
			continue
		}
		id := SanitizeNoteImageID(block.Image.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

// ResolveNoteImagePath maps an attachment id onto the local notes attachments directory.
func ResolveNoteImagePath(image common.NoteImage) string {
	id := SanitizeNoteImageID(image.ID)
	if id == "" {
		return ""
	}
	return filepath.Join(util.GetLocation().GetNotesAttachmentsDirectory(), id)
}

// SanitizeNoteImageID keeps attachment ids as a single filename so notes cannot escape the attachments directory.
func SanitizeNoteImageID(id string) string {
	id = filepath.Base(strings.TrimSpace(id))
	if id == "." || id == ".." || id == string(filepath.Separator) || strings.ContainsAny(id, `/\`) {
		return ""
	}
	return id
}

// ParseNoteImageRef accepts the portable notes-image: id used in Markdown and HTML export.
func ParseNoteImageRef(dest string) string {
	return parseNoteImageDestination(dest).ID
}

// parsedNoteImageRef is the portable attachment pointer stored in Markdown and HTML.
type parsedNoteImageRef struct {
	ID     string
	Scale  int
	Width  int
	Height int
}

func parseNoteImageDestination(dest string) parsedNoteImageRef {
	dest = strings.TrimSpace(dest)
	dest, rawQuery, _ := strings.Cut(dest, "?")
	id := ""
	if value, ok := strings.CutPrefix(dest, notesImageRefPrefix); ok {
		id = SanitizeNoteImageID(value)
	} else if value, ok := strings.CutPrefix(dest, "attachments/"); ok {
		id = SanitizeNoteImageID(value)
	}
	return parseNoteImageQuery(id, rawQuery)
}

// parseNoteImageQuery reads optional scale and intrinsic pixel size from a notes-image URL.
func parseNoteImageQuery(id, query string) parsedNoteImageRef {
	ref := parsedNoteImageRef{ID: id}
	for _, part := range strings.Split(query, "&") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			continue
		}
		switch key {
		case "scale":
			ref.Scale = ClampNoteImageScale(parsed)
		case "width":
			ref.Width = parsed
		case "height":
			ref.Height = parsed
		}
	}
	return ref
}

// HydrateNoteImageDimensions fills missing pixel size from the local attachment file.
func HydrateNoteImageDimensions(document common.NoteDocument) {
	for index := range document.Blocks {
		image := document.Blocks[index].Image
		if image == nil || image.Width > 0 && image.Height > 0 {
			continue
		}
		path := ResolveNoteImagePath(*image)
		if path == "" {
			continue
		}
		config, _, err := decodeNoteImageConfig(path)
		if err != nil {
			continue
		}
		image.Width = config.Width
		image.Height = config.Height
	}
}

// ClampNoteImageScale keeps a persisted display scale in the 20-100 percent range.
func ClampNoteImageScale(scale int) int {
	if scale <= 0 {
		return 0
	}
	if scale < 20 {
		return 20
	}
	if scale > 100 {
		return 100
	}
	return scale
}

// NoteImageScaleOrDefault treats 0 as full width.
func NoteImageScaleOrDefault(scale int) int {
	if scale <= 0 {
		return 100
	}
	return ClampNoteImageScale(scale)
}

// AdjustNoteImageScale steps the display scale while keeping the aspect ratio.
func AdjustNoteImageScale(scale, delta int) int {
	next := ClampNoteImageScale(NoteImageScaleOrDefault(scale) + delta)
	if next == 100 {
		return 0
	}
	return next
}

func attachmentDocument(ids []string) common.NoteDocument {
	blocks := make([]common.NoteBlock, 0, len(ids))
	for _, id := range ids {
		blocks = append(blocks, common.NoteBlock{Type: common.NoteBlockImage, Image: &common.NoteImage{ID: id}})
	}
	return common.NoteDocument{Blocks: blocks}
}

func attachmentIDsMissing(previous, next []string) []string {
	keep := map[string]struct{}{}
	for _, id := range next {
		keep[id] = struct{}{}
	}
	missing := make([]string, 0)
	for _, id := range previous {
		if _, ok := keep[id]; ok {
			continue
		}
		missing = append(missing, id)
	}
	return missing
}

func noteImageExtension(path string) string {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff", ".heic", ".heif":
		return ext
	default:
		return ".png"
	}
}

func decodeNoteImageConfig(path string) (image.Config, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return image.Config{}, "", err
	}
	defer file.Close()
	return image.DecodeConfig(file)
}
