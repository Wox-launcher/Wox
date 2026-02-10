package font

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var windowsRegFontLineRegex = regexp.MustCompile(`^\s*(.+?)\s+REG_\w+\s+.+$`)
var windowsRegFontSuffixRegex = regexp.MustCompile(`\s*\(.*\)\s*$`)

func parseFcListOutput(output string) []string {
	var fontFamilies []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 2 {
			continue
		}

		for _, family := range strings.Split(parts[1], ",") {
			family = strings.TrimSpace(family)
			if family != "" {
				fontFamilies = append(fontFamilies, family)
			}
		}
	}

	return fontFamilies
}

func parseWindowsRegFontsOutput(output string) []string {
	var fontFamilies []string
	for _, line := range strings.Split(output, "\n") {
		match := windowsRegFontLineRegex.FindStringSubmatch(line)
		if len(match) < 2 {
			continue
		}

		name := strings.TrimSpace(match[1])
		name = strings.TrimPrefix(name, "@")
		name = windowsRegFontSuffixRegex.ReplaceAllString(name, "")
		name = strings.TrimSpace(name)
		if name != "" {
			fontFamilies = append(fontFamilies, name)
		}
	}

	return fontFamilies
}

func parseSystemProfilerFontsOutput(output []byte) []string {
	result := gjson.GetBytes(output, "SPFontsDataType")
	if !result.Exists() || !result.IsArray() {
		return nil
	}

	var fontFamilies []string
	result.ForEach(func(_, value gjson.Result) bool {
		hasTypefaceFamily := false
		hasTypefaceItem := false
		typefaces := value.Get("typefaces")
		if typefaces.Exists() && typefaces.IsArray() {
			typefaces.ForEach(func(_, typeface gjson.Result) bool {
				hasTypefaceItem = true
				family := sanitizeSystemProfilerFontName(typeface.Get("family").String())
				if family == "" {
					family = sanitizeSystemProfilerFontName(typeface.Get("fullname").String())
				}
				if family != "" {
					fontFamilies = append(fontFamilies, family)
					hasTypefaceFamily = true
				}
				return true
			})
		}

		if !hasTypefaceFamily && !hasTypefaceItem {
			name := sanitizeSystemProfilerFontName(value.Get("_name").String())
			if name != "" {
				fontFamilies = append(fontFamilies, name)
			}
		}

		return true
	})

	return fontFamilies
}

func sanitizeSystemProfilerFontName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	if strings.HasPrefix(name, ".") {
		return ""
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == ".ttf" || ext == ".otf" || ext == ".ttc" || ext == ".dfont" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}

	return strings.TrimSpace(name)
}
