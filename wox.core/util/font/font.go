package font

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	systemFontFamilies []string
	loadFontFamilies   sync.Once
)

func GetSystemFontFamilies(ctx context.Context) []string {
	loadFontFamilies.Do(func() {
		systemFontFamilies = normalizeFontFamilies(getSystemFontFamilies(ctx))
	})

	result := make([]string, len(systemFontFamilies))
	copy(result, systemFontFamilies)
	return result
}

func normalizeFontFamilies(fontFamilies []string) []string {
	dedup := map[string]string{}
	for _, fontFamily := range fontFamilies {
		fontFamily = strings.TrimSpace(fontFamily)
		if fontFamily == "" {
			continue
		}

		fontFamily = strings.Trim(fontFamily, "\"'")
		if fontFamily == "" {
			continue
		}

		lower := strings.ToLower(fontFamily)
		if _, exists := dedup[lower]; !exists {
			dedup[lower] = fontFamily
		}
	}

	result := make([]string, 0, len(dedup))
	for _, fontFamily := range dedup {
		result = append(result, fontFamily)
	}

	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i]) < strings.ToLower(result[j])
	})

	return result
}

func NormalizeConfiguredFontFamily(fontFamily string, availableFontFamilies []string) string {
	fontFamily = strings.TrimSpace(fontFamily)
	if fontFamily == "" {
		return ""
	}

	matchAvailable := func(target string) string {
		for _, available := range availableFontFamilies {
			if strings.EqualFold(target, available) {
				return available
			}
		}
		return ""
	}

	if matched := matchAvailable(fontFamily); matched != "" {
		return matched
	}

	ext := strings.ToLower(filepath.Ext(fontFamily))
	if ext == ".ttf" || ext == ".otf" || ext == ".ttc" || ext == ".dfont" {
		base := strings.TrimSuffix(fontFamily, filepath.Ext(fontFamily))
		base = strings.TrimSpace(base)

		if matched := matchAvailable(base); matched != "" {
			return matched
		}

		base = strings.ReplaceAll(base, "-", " ")
		base = strings.TrimSpace(base)
		if matched := matchAvailable(base); matched != "" {
			return matched
		}

		return ""
	}

	return fontFamily
}
