package fileicon

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"wox/common"
)

// getFileTypeIconImpl tries to resolve a themed icon PNG for the file's MIME type and cache it.
// Best-effort: look up MIME via extension, then search common icon-theme locations (hicolor/Adwaita) for 48px PNG.
func getFileTypeIconImpl(ctx context.Context, ext string) (common.WoxImage, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)
	if _, err := os.Stat(cachePath); err == nil {
		return common.NewWoxImageAbsolutePath(cachePath), nil
	}

	mime := mimeFromExt(ext)
	iconNames := []string{
		strings.ReplaceAll(mime, "/", "-"), // e.g. image-png
		genericFromMime(mime),              // e.g. image-x-generic, text-x-generic
		"application-octet-stream",         // fallback
	}

	// Common icon theme roots and sizes
	roots := []string{
		path.Join(os.Getenv("HOME"), ".local/share/icons"),
		path.Join(os.Getenv("HOME"), ".icons"),
		"/usr/share/icons/Adwaita",
		"/usr/share/icons/hicolor",
		"/usr/share/pixmaps",
	}
	sizes := []string{"48x48", "64x64", "32x32", "128x128"}

	// Search PNG first
	for _, root := range roots {
		for _, sizeDir := range sizes {
			for _, name := range iconNames {
				p := filepath.Join(root, sizeDir, "mimetypes", name+".png")
				if _, err := os.Stat(p); err == nil {
					// Copy to cache to avoid scanning next time
					if copyFile(p, cachePath) == nil {
						return common.NewWoxImageAbsolutePath(cachePath), nil
					}
					return common.NewWoxImageAbsolutePath(p), nil
				}
			}
		}
		// Some themes put png directly under mimetypes without size
		for _, name := range iconNames {
			p := filepath.Join(root, "mimetypes", name+".png")
			if _, err := os.Stat(p); err == nil {
				if copyFile(p, cachePath) == nil {
					return common.NewWoxImageAbsolutePath(cachePath), nil
				}
				return common.NewWoxImageAbsolutePath(p), nil
			}
		}
	}

	// Try scalable SVG; UI supports inline SVG, but we prefer caching as-is to avoid rasterization here
	for _, root := range roots {
		for _, name := range iconNames {
			p := filepath.Join(root, "scalable", "mimetypes", name+".svg")
			if _, err := os.Stat(p); err == nil {
				// We can't reliably rasterize here without GTK/cairo; return absolute path directly
				// If the UI can't load path to SVG, this will be ignored by caller's fallback
				return common.NewWoxImageAbsolutePath(p), nil
			}
		}
	}

	return common.WoxImage{}, errors.New("no themed icon found")
}

func mimeFromExt(ext string) string {
	e := strings.TrimPrefix(strings.ToLower(ext), ".")
	if e == "" || e == "__unknown" {
		return "application/octet-stream"
	}
	// Simple map for common types; avoid pulling net/http for mime.TypeByExtension to keep dependency small
	switch e {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "svg":
		return "image/svg+xml"
	case "pdf":
		return "application/pdf"
	case "txt", "log":
		return "text/plain"
	case "md":
		return "text/markdown"
	case "json":
		return "application/json"
	case "zip":
		return "application/zip"
	case "rar":
		return "application/vnd.rar"
	case "7z":
		return "application/x-7z-compressed"
	case "doc", "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xls", "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "ppt", "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}

func genericFromMime(mime string) string {
	if strings.HasPrefix(mime, "image/") {
		return "image-x-generic"
	}
	if strings.HasPrefix(mime, "text/") {
		return "text-x-generic"
	}
	if strings.HasPrefix(mime, "audio/") {
		return "audio-x-generic"
	}
	if strings.HasPrefix(mime, "video/") {
		return "video-x-generic"
	}
	return "application-octet-stream"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), fs.ModePerm); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return nil
}
