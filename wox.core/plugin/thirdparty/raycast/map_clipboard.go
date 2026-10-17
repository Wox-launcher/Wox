package raycast

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/disintegration/imaging"

	"wox/util/clipboard"
)

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
