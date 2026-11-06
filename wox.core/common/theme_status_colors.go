package common

import "image/color"

// ThemeStatusColors is the semantic status palette for custom theme-aware surfaces.
// Warning is a badge/background fill; WarningText is its contrasting foreground.
type ThemeStatusColors struct {
	Success, Warning, WarningText, Error color.NRGBA
}

// StatusColors applies authored overrides with appearance-aware defaults. The
// caller supplies its surface appearance because overlays may differ from the
// launcher background; persisted theme data is never rewritten for defaults.
func (t Theme) StatusColors(dark bool) ThemeStatusColors {
	result := ThemeStatusColors{
		Success:     color.NRGBA{R: 74, G: 222, B: 128, A: 255},
		Warning:     color.NRGBA{R: 253, G: 230, B: 138, A: 255},
		WarningText: color.NRGBA{R: 102, G: 60, A: 255},
		Error:       color.NRGBA{R: 232, G: 95, B: 95, A: 255},
	}
	if !dark {
		result.Success = color.NRGBA{R: 21, G: 128, B: 61, A: 255}
		result.Error = color.NRGBA{R: 185, G: 28, B: 28, A: 255}
	}
	for _, override := range []struct {
		value  string
		target *color.NRGBA
	}{
		{t.SuccessColor, &result.Success}, {t.WarningColor, &result.Warning},
		{t.WarningTextColor, &result.WarningText}, {t.ErrorColor, &result.Error},
	} {
		if parsed, ok := ParseThemeColor(override.value); ok {
			*override.target = parsed
		}
	}
	return result
}
