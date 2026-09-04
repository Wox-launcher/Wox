package window

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// existingFilesystemDirectory returns path only when it names a real directory.
// Virtual shell folders, missing paths, and files are rejected.
func existingFilesystemDirectory(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "::") {
		return ""
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	return path
}

// filesystemPathFromShellLocationURL converts a Shell.Application LocationURL
// (file:///C:/Users/...) into a real directory path. Non-file and virtual URLs
// are rejected so Quick Switch never navigates to a shell namespace.
func filesystemPathFromShellLocationURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "file") {
		return ""
	}

	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	p, err = url.PathUnescape(p)
	if err != nil {
		return ""
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}

	// file:///C:/Users/... keeps a leading slash in url.Path.
	if len(p) >= 3 && p[0] == '/' && unicode.IsLetter(rune(p[1])) && p[2] == ':' {
		p = p[1:]
	}
	return existingFilesystemDirectory(filepath.Clean(filepath.FromSlash(p)))
}
