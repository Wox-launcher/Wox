//go:build linux

package wallpaper

import (
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func getSystemWallpaperPath() (string, error) {
	for _, key := range []string{"picture-uri-dark", "picture-uri"} {
		output, err := exec.Command("gsettings", "get", "org.gnome.desktop.background", key).Output()
		if err != nil {
			continue
		}
		if value := resolveWallpaperPath(strings.TrimSpace(string(output)), key == "picture-uri-dark"); value != "" {
			return value, nil
		}
	}
	return "", errors.New("desktop wallpaper is unavailable")
}

func resolveWallpaperPath(rawValue string, preferDark bool) string {
	value := strings.Trim(rawValue, "'\"")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "file://") {
		if parsed, parseErr := url.Parse(value); parseErr == nil {
			if decoded, decodeErr := url.PathUnescape(parsed.Path); decodeErr == nil {
				value = decoded
			} else {
				value = parsed.Path
			}
		}
	}
	return existingWallpaperImagePath(value, preferDark)
}

func existingWallpaperImagePath(path string, preferDark bool) string {
	if path == "" {
		return ""
	}
	if isSupportedWallpaperImagePath(path) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if strings.HasSuffix(strings.ToLower(path), ".xml") {
		return resolveWallpaperXMLPath(path, preferDark)
	}
	return ""
}

func resolveWallpaperXMLPath(path string, preferDark bool) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	var metadata struct {
		Filename     string `xml:"filename"`
		FilenameDark string `xml:"filename-dark"`
		File         string `xml:"file"`
	}
	if decodeErr := xml.NewDecoder(file).Decode(&metadata); decodeErr != nil && !errors.Is(decodeErr, io.EOF) {
		return ""
	}

	candidates := make([]string, 0, 3)
	if preferDark {
		candidates = append(candidates, metadata.FilenameDark)
	}
	candidates = append(candidates, metadata.Filename, metadata.File)
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(filepath.Dir(path), candidate)
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func isSupportedWallpaperImagePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp", ".tif", ".tiff", ".jxl":
		return true
	default:
		return false
	}
}
