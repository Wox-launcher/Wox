package ui

import (
	"strconv"
	"strings"

	"wox/common"
)

// ParseHexColor parses a #RRGGBB or #RRGGBBAA hex color string.
func ParseHexColor(s string) Color {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return Color{0, 0, 0, 1}
	}
	s = s[1:]
	parseHex := func(h string) float32 {
		v, _ := strconv.ParseInt(h, 16, 32)
		return float32(v) / 255.0
	}
	switch len(s) {
	case 6:
		return Color{parseHex(s[0:2]), parseHex(s[2:4]), parseHex(s[4:6]), 1.0}
	case 8:
		return Color{parseHex(s[0:2]), parseHex(s[2:4]), parseHex(s[4:6]), parseHex(s[6:8])}
	}
	return Color{0, 0, 0, 1}
}

// ParseRGBA parses a "rgba(r, g, b, a)" or "rgb(r, g, b)" string.
func ParseRGBA(s string) Color {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "rgba(")
	s = strings.TrimPrefix(s, "rgb(")
	s = strings.TrimSuffix(s, ")")
	parts := strings.Split(s, ",")
	if len(parts) < 3 {
		return Color{0, 0, 0, 1}
	}
	parseInt := func(p string) float32 {
		v, _ := strconv.ParseFloat(strings.TrimSpace(p), 32)
		return float32(v) / 255.0
	}
	parseAlpha := func(p string) float32 {
		v, _ := strconv.ParseFloat(strings.TrimSpace(p), 32)
		return float32(v)
	}
	r := parseInt(parts[0])
	g := parseInt(parts[1])
	b := parseInt(parts[2])
	a := float32(1.0)
	if len(parts) >= 4 {
		a = parseAlpha(parts[3])
	}
	return Color{r, g, b, a}
}

// ParseColor auto-detects hex or rgba format.
func ParseColor(s string) Color {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "#") {
		return ParseHexColor(s)
	}
	if strings.HasPrefix(s, "rgb") {
		return ParseRGBA(s)
	}
	return Color{0, 0, 0, 1}
}

// ThemeFromWoxTheme converts a Wox common.Theme (loaded from JSON) to ui.Theme
// for the native renderer.
func ThemeFromWoxTheme(woxTheme common.Theme) Theme {
	t := DefaultTheme()

	if woxTheme.AppBackgroundColor != "" {
		t.WindowBg = ParseColor(woxTheme.AppBackgroundColor)
	}
	if woxTheme.QueryBoxBackgroundColor != "" {
		t.QueryBoxBg = ParseColor(woxTheme.QueryBoxBackgroundColor)
	}
	if woxTheme.QueryBoxBorderRadius > 0 {
		t.QueryBoxRadius = float32(woxTheme.QueryBoxBorderRadius)
	}
	if woxTheme.QueryBoxFontColor != "" {
		t.QueryBoxFontColor = ParseColor(woxTheme.QueryBoxFontColor)
		t.TextPrimary = t.QueryBoxFontColor
	}
	if woxTheme.QueryBoxCursorColor != "" {
		t.QueryBoxCursorColor = ParseColor(woxTheme.QueryBoxCursorColor)
		t.CursorColor = t.QueryBoxCursorColor
	}
	if woxTheme.ResultItemTitleColor != "" {
		t.TextPrimary = ParseColor(woxTheme.ResultItemTitleColor)
	}
	if woxTheme.ResultItemSubTitleColor != "" {
		t.TextSecondary = ParseColor(woxTheme.ResultItemSubTitleColor)
	}
	if woxTheme.ResultItemActiveBackgroundColor != "" {
		t.SelectedBg = ParseColor(woxTheme.ResultItemActiveBackgroundColor)
	}
	if woxTheme.ResultItemActiveTitleColor != "" {
		t.SelectedTitleColor = ParseColor(woxTheme.ResultItemActiveTitleColor)
	}
	if woxTheme.ResultItemActiveSubTitleColor != "" {
		t.SelectedSubColor = ParseColor(woxTheme.ResultItemActiveSubTitleColor)
	}
	if woxTheme.ResultItemBorderRadius > 0 {
		t.ListItemRadius = float32(woxTheme.ResultItemBorderRadius)
	}
	if woxTheme.AppPaddingLeft > 0 {
		t.WindowPadding = float32(woxTheme.AppPaddingLeft)
	}

	// Preview panel colors. Fall back to sensible defaults when the theme omits
	// them so older themes still render a readable preview surface.
	if woxTheme.PreviewFontColor != "" {
		t.PreviewFontColor = ParseColor(woxTheme.PreviewFontColor)
	}
	if woxTheme.PreviewSplitLineColor != "" {
		t.PreviewSplitLineColor = ParseColor(woxTheme.PreviewSplitLineColor)
	}
	if woxTheme.PreviewPropertyTitleColor != "" {
		t.PreviewPropertyTitle = ParseColor(woxTheme.PreviewPropertyTitleColor)
	}
	if woxTheme.PreviewPropertyContentColor != "" {
		t.PreviewPropertyContent = ParseColor(woxTheme.PreviewPropertyContentColor)
	}

	// Toolbar colors.
	if woxTheme.ToolbarFontColor != "" {
		t.ToolbarFontColor = ParseColor(woxTheme.ToolbarFontColor)
	}
	if woxTheme.ToolbarBackgroundColor != "" {
		t.ToolbarBg = ParseColor(woxTheme.ToolbarBackgroundColor)
	}
	if woxTheme.ToolbarPaddingLeft > 0 {
		t.ToolbarPaddingLeft = float32(woxTheme.ToolbarPaddingLeft)
	}
	if woxTheme.ToolbarPaddingRight > 0 {
		t.ToolbarPaddingRight = float32(woxTheme.ToolbarPaddingRight)
	}

	return t
}
