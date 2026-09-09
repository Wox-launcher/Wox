package util

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	// PluginPackageExtension is the packaged plugin archive suffix.
	PluginPackageExtension = ".wox"
	// PluginPackageMIMEType is the Linux/shared MIME type for .wox archives.
	PluginPackageMIMEType = "application/x-wox-plugin"
	pluginPackageURLMIME  = "x-scheme-handler/wox"
)

// IsPluginPackagePath reports whether path looks like a Wox plugin archive.
func IsPluginPackagePath(path string) bool {
	return strings.EqualFold(filepath.Ext(strings.TrimSpace(path)), PluginPackageExtension)
}

// PluginPackageInstallDeepLink builds the deeplink used to open the local installer.
func PluginPackageInstallDeepLink(filePath string) string {
	return "wox://install?path=" + url.QueryEscape(filePath)
}

// CollectStartupDeepLinks keeps protocol URLs and converts .wox file arguments
// into install deeplinks so a second process can forward them to the running instance.
func CollectStartupDeepLinks(args []string) []string {
	links := make([]string, 0, 1)
	seen := make(map[string]struct{}, len(args))
	add := func(link string) {
		if link == "" {
			return
		}
		if _, exists := seen[link]; exists {
			return
		}
		seen[link] = struct{}{}
		links = append(links, link)
	}

	for _, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(arg), "wox://") {
			add(arg)
			continue
		}
		if filePath, ok := PluginPackagePathFromArg(arg); ok {
			add(PluginPackageInstallDeepLink(filePath))
		}
	}
	return links
}

// PluginPackagePathFromArg accepts a filesystem path or file:// URL and returns
// a cleaned local .wox path when the argument is a plugin package.
func PluginPackagePathFromArg(arg string) (string, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", false
	}

	if filePath, ok := pathFromFileURL(arg); ok {
		if !IsPluginPackagePath(filePath) {
			return "", false
		}
		return filepath.Clean(filePath), true
	}
	if !IsPluginPackagePath(arg) {
		return "", false
	}

	abs, err := filepath.Abs(arg)
	if err != nil {
		return filepath.Clean(arg), true
	}
	return abs, true
}

// pathFromFileURL converts file:// URLs from desktop launches into local paths.
func pathFromFileURL(raw string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(raw), "file://") {
		return "", false
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "file" {
		return "", false
	}

	filePath := parsed.Path
	if parsed.Opaque != "" && filePath == "" {
		filePath = parsed.Opaque
	}
	if filePath == "" {
		return "", false
	}

	if runtime.GOOS == "windows" {
		if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
			filePath = `\\` + parsed.Host + filepath.FromSlash(filePath)
			return filePath, true
		}
		if strings.HasPrefix(filePath, "/") && len(filePath) >= 3 && filePath[2] == ':' {
			filePath = filePath[1:]
		}
		return filepath.FromSlash(filePath), true
	}

	return filePath, true
}
