package raycast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"wox/util/clipboard"
)

// clipboardParts reads one history entry. A file list wins over plain text because
// Wox stores copied files separately. One existing image file stays an image.
func clipboardParts(entry ClipboardEntry) (text, imagePath string, files []string, ocr string, imageMissing bool) {
	var image string
	var sawImage bool
	for _, item := range entry.Items {
		for _, representation := range item.Representations {
			mime := strings.ToLower(representation.MimeType)
			if text == "" && strings.HasPrefix(mime, "text/plain") && representation.Content != "" {
				text = representation.Content
			}
			if ocr == "" && mime == "application/vnd.raycast.ocr" && strings.TrimSpace(representation.Content) != "" {
				ocr = strings.TrimSpace(representation.Content)
			}
			if !sawImage && strings.HasPrefix(mime, "image/") && representation.ContentType == "url" && representation.Content != "" {
				image = representation.Content
				sawImage = true
			}
			if mime == "text/uri-list" {
				files = append(files, clipboardURIList(representation.Content)...)
			}
		}
	}
	parsedFiles := files
	files = existingClipboardFiles(parsedFiles)
	if len(parsedFiles) > 0 && len(files) == 0 {
		return "", parsedFiles[0], nil, ocr, true
	}
	if len(files) == 1 && clipboardImageExt(files[0]) && regularLocalFile(files[0]) {
		return "", files[0], nil, ocr, false
	}
	if len(files) > 0 {
		return "", "", files, ocr, false
	}
	if text != "" {
		return text, "", nil, ocr, false
	}
	if image == "" {
		return "", "", nil, ocr, false
	}
	if !regularLocalFile(image) {
		return "", image, nil, ocr, true
	}
	return "", image, nil, ocr, false
}

func clipboardURIList(raw string) []string {
	var paths []string
	seen := map[string]struct{}{}
	for _, line := range strings.FieldsFunc(raw, func(char rune) bool { return char == '\n' || char == '\r' }) {
		path, ok := quicklinkLocalPath(strings.TrimSpace(line))
		if !ok {
			continue
		}
		key := strings.ToLower(path)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func existingClipboardFiles(paths []string) []string {
	var existing []string
	for _, path := range paths {
		if regularLocalFile(path) {
			existing = append(existing, path)
		}
	}
	return existing
}

func clipboardImageExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tif", ".tiff", ".ico":
		return true
	default:
		return false
	}
}

func clipboardTextAndImage(entry ClipboardEntry) (text, imagePath string, imageMissing bool) {
	var image string
	var sawImage bool
	for _, item := range entry.Items {
		for _, representation := range item.Representations {
			mime := strings.ToLower(representation.MimeType)
			if text == "" && strings.HasPrefix(mime, "text/plain") && representation.Content != "" {
				text = representation.Content
			}
			if !sawImage && strings.HasPrefix(mime, "image/") && representation.ContentType == "url" && representation.Content != "" {
				image = representation.Content
				sawImage = true
			}
		}
	}
	if text != "" {
		return text, "", false
	}
	if image == "" {
		return "", "", false
	}
	if !regularLocalFile(image) {
		return "", image, true
	}
	return "", image, false
}

func clipboardPinned(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false
	}
	return value
}

func parseRaycastTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func localImageHash(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > DefaultLimits().MaxImageBytes {
		return ""
	}
	img, err := imaging.Open(path)
	if err != nil || img.Bounds().Dx() > 8192 || img.Bounds().Dy() > 8192 {
		return ""
	}
	return clipboard.ImageHash(img)
}

func regularLocalFile(path string) bool {
	if path == "" || strings.Contains(path, "://") {
		return false
	}
	info, err := statApp(path)
	return err == nil && !info.isDir
}
