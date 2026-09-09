package font

import (
	"context"
	"fmt"

	"wox/util"

	"golang.org/x/sys/windows/registry"
)

var fallbackWindowsFontFamilies = []string{
	"Segoe UI",
	"Microsoft YaHei UI",
	"Arial",
}

const windowsFontsRegistryPath = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`

func getSystemFontFamilies(ctx context.Context) []string {
	// DirectWrite is the same collection CreateTextFormat uses, so it includes
	// per-user installs and returns typographic family names instead of registry face labels.
	families, err := enumerateDirectWriteFontFamilies()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to enumerate windows fonts via DirectWrite: %s", err.Error()))
		families = enumerateRegistryFontFamilies()
	}
	if len(families) == 0 {
		return fallbackWindowsFontFamilies
	}

	return append(families, fallbackWindowsFontFamilies...)
}

// enumerateRegistryFontFamilies reads HKLM and HKCU font values as a fallback when DirectWrite is unavailable.
func enumerateRegistryFontFamilies() []string {
	var fontFamilies []string
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		key, err := registry.OpenKey(root, windowsFontsRegistryPath, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, err := key.ReadValueNames(-1)
		key.Close()
		if err != nil {
			continue
		}
		for _, name := range names {
			if family := sanitizeWindowsRegistryFontName(name); family != "" {
				fontFamilies = append(fontFamilies, family)
			}
		}
	}
	return fontFamilies
}
