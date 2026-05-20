package fileicon

import (
	"context"
	"errors"
	"io/fs"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// getFileTypeIconImpl resolves a Linux theme icon path for a file type.
// The previous implementation only checked a tiny hard-coded MIME map and a
// few icon directories, which missed common names such as
// application-x-executable and left raw fileicon values unresolved in the UI.
// Prefer the desktop MIME database first, then fall back to the local map.
func getFileTypeIconImpl(ctx context.Context, ext string, size int) (string, error) {
	cachePath := buildCachePath(ext, size)
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	return lookupIconPathForNames(iconNamesForExtension(ext), size, cachePath)
}

// getFileIconImpl resolves Linux file icons from the desktop's own icon-name
// association first. The old implementation returned "not supported", which
// meant every fileicon caller fell back to fragile extension heuristics.
func getFileIconImpl(ctx context.Context, filePath string, size int) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("empty path")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	cachePath := buildPathCachePath(filePath, size, info.ModTime().UnixNano())
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	iconNames := getFileIconNames(ctx, filePath, info)
	if len(iconNames) == 0 {
		return "", errors.New("no themed icon found")
	}

	return lookupIconPathForNames(iconNames, size, cachePath)
}

func getFileIconNames(ctx context.Context, filePath string, info fs.FileInfo) []string {
	if iconNames, err := getGioIconNames(ctx, filePath); err == nil && len(iconNames) > 0 {
		return iconNames
	}

	if info.IsDir() {
		return []string{"inode-directory"}
	}

	iconNames := make([]string, 0, 4)
	if info.Mode()&0o111 != 0 {
		iconNames = append(iconNames, "application-x-executable")
	}
	iconNames = append(iconNames, iconNamesForExtension(filepath.Ext(filePath))...)
	return dedupeIconNames(iconNames)
}

func getGioIconNames(ctx context.Context, filePath string) ([]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	output, err := exec.CommandContext(ctx, "gio", "info", "--attributes=standard::icon", filePath).Output()
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "standard::icon:") {
			continue
		}

		rawNames := strings.TrimSpace(strings.TrimPrefix(line, "standard::icon:"))
		if rawNames == "" {
			return nil, errors.New("empty gio icon names")
		}

		parts := strings.Split(rawNames, ",")
		names := make([]string, 0, len(parts))
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name != "" {
				names = append(names, name)
			}
		}
		return dedupeIconNames(names), nil
	}

	return nil, errors.New("gio icon names not found")
}

func iconNamesForExtension(ext string) []string {
	resolvedMime := strings.TrimSpace(mime.TypeByExtension(ext))
	if resolvedMime == "" {
		resolvedMime = mimeFromExt(ext)
	}
	return iconNamesForMime(resolvedMime)
}

func iconNamesForMime(mimeType string) []string {
	mimeType = strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	iconNames := []string{strings.ReplaceAll(mimeType, "/", "-")}
	if generic := genericFromMime(mimeType); generic != "" {
		iconNames = append(iconNames, generic)
	}
	iconNames = append(iconNames, "application-octet-stream")
	return dedupeIconNames(iconNames)
}

func lookupIconPathForNames(iconNames []string, size int, cachePath string) (string, error) {
	for _, root := range iconThemeRoots() {
		for _, sizeDir := range iconLookupSizeDirs(size) {
			for _, category := range []string{"mimetypes"} {
				for _, iconName := range iconNames {
					if iconPath := firstExistingIconPath(filepath.Join(root, sizeDir, category), iconName); iconPath != "" {
						if strings.EqualFold(filepath.Ext(iconPath), ".svg") {
							return iconPath, nil
						}
						if copyFile(iconPath, cachePath) == nil {
							return cachePath, nil
						}
						return iconPath, nil
					}
				}
			}
		}

		for _, category := range []string{"mimetypes", filepath.Join("scalable", "mimetypes"), filepath.Join("symbolic", "mimetypes")} {
			for _, iconName := range iconNames {
				if iconPath := firstExistingIconPath(filepath.Join(root, category), iconName); iconPath != "" {
					if strings.EqualFold(filepath.Ext(iconPath), ".svg") {
						return iconPath, nil
					}
					if copyFile(iconPath, cachePath) == nil {
						return cachePath, nil
					}
					return iconPath, nil
				}
			}
		}
	}

	return "", errors.New("no themed icon found")
}

func firstExistingIconPath(baseDir string, iconName string) string {
	for _, ext := range []string{".png", ".svg", ".xpm"} {
		iconPath := filepath.Join(baseDir, iconName+ext)
		if _, err := os.Stat(iconPath); err == nil {
			return iconPath
		}
	}
	return ""
}

func iconThemeRoots() []string {
	baseDirs := []string{
		path.Join(os.Getenv("HOME"), ".local/share/icons"),
		path.Join(os.Getenv("HOME"), ".icons"),
		"/usr/local/share/icons",
		"/usr/share/icons",
		"/usr/share/pixmaps",
	}

	roots := make([]string, 0, len(baseDirs)*4)
	seen := map[string]struct{}{}
	for _, baseDir := range baseDirs {
		appendUniquePath(&roots, seen, baseDir)

		entries, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			appendUniquePath(&roots, seen, filepath.Join(baseDir, entry.Name()))
		}
	}
	return roots
}

func iconLookupSizeDirs(size int) []string {
	preferred := []string{
		intToString(size) + "x" + intToString(size),
		"64x64",
		"48x48",
		"32x32",
		"24x24",
		"22x22",
		"16x16",
		"128x128",
		"256x256",
	}

	result := make([]string, 0, len(preferred))
	seen := map[string]struct{}{}
	for _, dir := range preferred {
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}
		result = append(result, dir)
	}
	return result
}

func appendUniquePath(target *[]string, seen map[string]struct{}, value string) {
	cleaned := filepath.Clean(strings.TrimSpace(value))
	if cleaned == "" {
		return
	}
	if _, ok := seen[cleaned]; ok {
		return
	}
	seen[cleaned] = struct{}{}
	*target = append(*target, cleaned)
}

func dedupeIconNames(names []string) []string {
	result := make([]string, 0, len(names))
	seen := map[string]struct{}{}
	for _, name := range names {
		cleaned := strings.TrimSpace(name)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	return result
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
