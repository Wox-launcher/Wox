package common

import (
	"fmt"
	"image/color"
	"math"
	"strings"
)

// ThemePluginColors is the small opaque palette plugins use for HTML previews
// and other custom surfaces. Values are #RRGGBB with no alpha.
type ThemePluginColors struct {
	Background    string
	Text          string
	SecondaryText string
	Border        string
	Accent        string
	AccentText    string
	Selection     string
	Dark          bool
}

var (
	themePluginDarkBackdrop  = color.NRGBA{R: 0x12, G: 0x12, B: 0x14, A: 0xFF}
	themePluginLightBackdrop = color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF}
	themePluginDarkDefaults  = ThemePluginColors{
		Background:    "#1E1E1E",
		Text:          "#E6E6E6",
		SecondaryText: "#A0A0A0",
		Border:        "#3A3A3A",
		Accent:        "#3B82F6",
		AccentText:    "#FFFFFF",
		Selection:     "#264F78",
		Dark:          true,
	}
	themePluginLightDefaults = ThemePluginColors{
		Background:    "#F5F5F5",
		Text:          "#1A1A1A",
		SecondaryText: "#5C5C5C",
		Border:        "#D0D0D0",
		Accent:        "#2563EB",
		AccentText:    "#FFFFFF",
		Selection:     "#CDE4FF",
		Dark:          false,
	}
)

// PluginColors maps the active Wox theme to opaque HTML-safe colors.
func (t Theme) PluginColors() ThemePluginColors {
	resolved := t.ResolvedColors()
	// Dark/light follows the window fill, not a low-alpha preview wash.
	// Wox Light paints PreviewBackgroundColor as #45454509 over a light app
	// surface; using that overlay's RGB would mis-classify the theme as dark.
	appRaw := firstThemeColorBy(usableFillColor, t.AppBackgroundColor, t.QueryBoxBackgroundColor)
	surface, dark := flattenBackground(appRaw)
	defaults := themePluginDarkDefaults
	if !dark {
		defaults = themePluginLightDefaults
	}
	backdrop, _ := ParseThemeColor(surface)
	// Content and preview fills are stacked surfaces, not fallback alternatives.
	background := flattenThemeColor(t.AppContentBackgroundColor, backdrop, surface)
	backdrop, _ = ParseThemeColor(background)
	background = flattenThemeColor(resolved["PreviewBackgroundColor"], backdrop, background)
	backdrop, _ = ParseThemeColor(background)
	text := flattenThemeColor(firstThemeColor(t.PreviewFontColor, t.ResultItemTitleColor, t.QueryBoxFontColor), backdrop, defaults.Text)
	secondary := flattenThemeColor(firstThemeColor(t.PreviewPropertyTitleColor, t.ResultItemSubTitleColor, resolved["PreviewTagFontColor"]), backdrop, defaults.SecondaryText)
	border := flattenThemeColor(firstThemeColor(resolved["PreviewBorderColor"], t.PreviewSplitLineColor, resolved["AppBorderColor"]), backdrop, defaults.Border)
	accent := flattenThemeColor(firstThemeColor(t.ResultItemActiveBackgroundColor, t.ResultItemActiveIndicatorColor, t.QueryBoxCursorColor), backdrop, defaults.Accent)
	// AccentText is displayed on Accent, so translucent text must use that fill.
	accentBackdrop, _ := ParseThemeColor(accent)
	accentText := flattenThemeColor(firstThemeColor(t.ResultItemActiveTitleColor, t.PreviewFontColor, t.ResultItemTitleColor), accentBackdrop, defaults.AccentText)
	selection := flattenThemeColor(firstThemeColor(t.PreviewTextSelectionColor, t.QueryBoxTextSelectionBackgroundColor, t.ResultItemActiveBackgroundColor), backdrop, defaults.Selection)

	return ThemePluginColors{
		Background:    background,
		Text:          text,
		SecondaryText: secondary,
		Border:        border,
		Accent:        accent,
		AccentText:    accentText,
		Selection:     selection,
		Dark:          dark,
	}
}

func firstThemeColor(values ...string) string {
	return firstThemeColorBy(usableThemeColor, values...)
}

// firstThemeColorBy selects a usable value in fallback order for either schema.
func firstThemeColorBy(usable func(string) bool, values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); usable(value) {
			return value
		}
	}
	return ""
}

func usableThemeColor(value string) bool {
	if value == "" {
		return false
	}
	parsed, ok := ParseThemeColor(value)
	return ok && parsed.A > 0
}

func usableFillColor(value string) bool {
	if value == "" {
		return false
	}
	parsed, ok := ParseThemeColor(value)
	return ok && parsed.A >= 48
}

// flattenBackground estimates an opaque window surface for HTML previews.
func flattenBackground(value string) (string, bool) {
	if !usableThemeColor(value) {
		return themePluginDarkDefaults.Background, true
	}
	parsed, _ := ParseThemeColor(value)
	guess := parsed
	guess.A = 255
	backdrop := themePluginDarkBackdrop
	if colorLuminance(guess) >= 140 {
		backdrop = themePluginLightBackdrop
	}
	hex := flattenOnto(parsed, backdrop)
	flat, _ := ParseThemeColor(hex)
	return hex, colorLuminance(flat) < 140
}

// flattenThemeColor composites a visible color or preserves the supplied fallback.
func flattenThemeColor(value string, backdrop color.NRGBA, fallback string) string {
	if !usableThemeColor(value) {
		return fallback
	}
	parsed, _ := ParseThemeColor(value)
	return flattenOnto(parsed, backdrop)
}

// flattenOnto composites a color over an opaque surface.
func flattenOnto(c, backdrop color.NRGBA) string {
	if c.A == 255 {
		return formatOpaqueHex(c)
	}
	alpha := float64(c.A) / 255
	mixed := color.NRGBA{
		R: uint8(math.Round(float64(c.R)*alpha + float64(backdrop.R)*(1-alpha))),
		G: uint8(math.Round(float64(c.G)*alpha + float64(backdrop.G)*(1-alpha))),
		B: uint8(math.Round(float64(c.B)*alpha + float64(backdrop.B)*(1-alpha))),
		A: 255,
	}
	return formatOpaqueHex(mixed)
}

func formatOpaqueHex(c color.NRGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func colorLuminance(c color.NRGBA) float64 {
	return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
}
