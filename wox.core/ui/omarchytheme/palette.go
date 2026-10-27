// Package omarchytheme builds the Omarchy Auto system theme from Omarchy's current palette.
package omarchytheme

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"wox/common"
)

const (
	// ThemeID is the stable id stored in Wox settings. It is not a user theme file.
	ThemeID = "7c1e9a42-8b54-4d0f-a6e3-5f2b8c0d1e47"
	// ThemeName is the catalog name on every locale. "Auto" means it follows Omarchy.
	ThemeName = "Omarchy Auto"

	themeMinWoxVersion = "2.4.5"

	// Omarchy's default window is a square 2px ring, with compositor blur off.
	appBorderWidth = 2
)

// Palette is the shared color contract in an Omarchy colors.toml.
type Palette struct {
	Mode              string
	Accent            string
	Selection         string
	Muted             string
	Background        string
	DarkBackground    string
	DarkerBackground  string
	LighterBackground string
	Foreground        string
	DarkForeground    string
	LightForeground   string
	BrightForeground  string
	// Border is the opaque window ring. It follows the Hyprland active border,
	// then active_border_color, then accent.
	Border string
}

// fallbackPalette is Catppuccin, used when Omarchy is installed but the
// current colors.toml cannot be read yet.
var fallbackPalette = Palette{
	Mode:              "dark",
	Accent:            "#89B4FA",
	Selection:         "#45475A",
	Muted:             "#585B70",
	Background:        "#1E1E2E",
	DarkBackground:    "#161622",
	DarkerBackground:  "#101019",
	LighterBackground: "#313244",
	Foreground:        "#CDD6F4",
	DarkForeground:    "#6C7086",
	LightForeground:   "#BAC2DE",
	BrightForeground:  "#CDD6F4",
}

// ParsePalette reads the flat colors.toml Omarchy writes for every theme.
// The window ring reads hyprland_active_border, then active_border_color.
// Other unknown keys are ignored.
func ParsePalette(data []byte) (Palette, error) {
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		values[key] = parseTOMLValue(raw)
	}

	background, err := requiredColor(values, "background")
	if err != nil {
		return Palette{}, err
	}
	foreground, err := requiredColor(values, "foreground")
	if err != nil {
		return Palette{}, err
	}
	accent, err := requiredColor(values, "accent")
	if err != nil {
		return Palette{}, err
	}
	mode := "dark"
	if values["mode"] == "light" {
		mode = "light"
	}
	return Palette{
		Mode:              mode,
		Accent:            accent,
		Selection:         optionalColor(values, "selection"),
		Muted:             optionalColor(values, "muted"),
		Background:        background,
		DarkBackground:    optionalColor(values, "dark_background"),
		DarkerBackground:  optionalColor(values, "darker_background"),
		LighterBackground: optionalColor(values, "lighter_background"),
		Foreground:        foreground,
		DarkForeground:    optionalColor(values, "dark_foreground"),
		LightForeground:   optionalColor(values, "light_foreground"),
		BrightForeground:  optionalColor(values, "bright_foreground"),
		Border:            borderColor(values, accent),
	}, nil
}

// ThemeFromPalette maps one Omarchy palette onto a Wox theme.
// Surfaces stay opaque and square, matching Omarchy windows. The outline is
// the 2px window-border color. The selected label uses accent, with no side stripe.
func ThemeFromPalette(palette Palette, woxVersion string) (common.Theme, error) {
	background, err := normalizeHex(palette.Background)
	if err != nil {
		return common.Theme{}, fmt.Errorf("background: %w", err)
	}
	foreground, err := normalizeHex(palette.Foreground)
	if err != nil {
		return common.Theme{}, fmt.Errorf("foreground: %w", err)
	}
	accent, err := normalizeHex(palette.Accent)
	if err != nil {
		return common.Theme{}, fmt.Errorf("accent: %w", err)
	}
	secondary, err := normalizeHex(palette.secondary())
	if err != nil {
		return common.Theme{}, fmt.Errorf("secondary text: %w", err)
	}
	lighter, err := normalizeHex(palette.orBackground(palette.LighterBackground))
	if err != nil {
		return common.Theme{}, err
	}
	darker, err := normalizeHex(palette.orBackground(palette.DarkerBackground))
	if err != nil {
		return common.Theme{}, err
	}
	preview, err := normalizeHex(palette.orBackground(palette.DarkBackground))
	if err != nil {
		return common.Theme{}, err
	}

	appBackground := withAlpha(background, 1)
	text := withAlpha(foreground, 1)
	accentColor := withAlpha(accent, 1)
	secondaryColor := withAlpha(secondary, 1)
	queryBackground := withAlpha(lighter, 1)
	actionBackground := withAlpha(darker, 1)
	previewBackground := withAlpha(preview, 1)
	border := palette.windowBorder()
	// Hover is an opaque mix. The schema default is a translucent accent wash.
	activeBackground := appBackground
	hoverOverlay := text
	hoverWeight := 0.08
	if selection, selectionErr := normalizeHex(palette.Selection); selectionErr == nil {
		activeBackground = withAlpha(selection, 1)
		hoverOverlay = activeBackground
		hoverWeight = 0.5
	} else if mixed, mixErr := mixOpaque(appBackground, text, 0.18); mixErr == nil {
		activeBackground = mixed
	}
	hover, err := mixOpaque(appBackground, hoverOverlay, hoverWeight)
	if err != nil {
		return common.Theme{}, err
	}
	none := 0
	borderWidth := appBorderWidth
	resultPadding := 8

	definition := common.ThemeSchemaV2{
		SchemaVersion:                        2,
		MinWoxVersion:                        themeMinWoxVersion,
		ThemeId:                              ThemeID,
		ThemeName:                            ThemeName,
		ThemeAuthor:                          "Wox launcher",
		ThemeUrl:                             "https://www.github.com/wox-launcher/wox",
		Version:                              "1.0.0",
		Description:                          "i18n:theme_description",
		I18n:                                 themeDescriptions,
		IsSystem:                             true,
		IsInstalled:                          true,
		BaseBackgroundColor:                  appBackground,
		BaseTextColor:                        text,
		BaseAccentColor:                      accentColor,
		AppBorderColor:                       &border,
		AppBorderWidth:                       &borderWidth,
		AppBorderRadius:                      &none,
		AppBackgroundColor:                   &appBackground,
		OverlayBackgroundColor:               &appBackground,
		OverlayFontColor:                     &text,
		ResultContainerPaddingTop:            &resultPadding,
		ResultContainerPaddingBottom:         &resultPadding,
		ResultItemBorderRadius:               &none,
		ResultItemSubTitleColor:              &secondaryColor,
		ResultItemTailTextColor:              &secondaryColor,
		ResultItemHoverBackgroundColor:       &hover,
		ResultItemActiveBackgroundColor:      &activeBackground,
		ResultItemActiveTitleColor:           &accentColor,
		ResultItemActiveIndicatorWidth:       &none,
		QueryBoxBackgroundColor:              &queryBackground,
		QueryBoxBorderRadius:                 &none,
		QueryBoxCursorColor:                  &accentColor,
		QueryBoxTextSelectionBackgroundColor: &activeBackground,
		ActionContainerBackgroundColor:       &actionBackground,
		ActionContainerBorderWidth:           &none,
		ActionContainerHeaderFontColor:       &secondaryColor,
		ActionContainerBorderRadius:          &none,
		ActionItemBorderRadius:               &none,
		ActionItemActiveBackgroundColor:      &activeBackground,
		ActionItemActiveFontColor:            &accentColor,
		ActionQueryBoxBorderRadius:           &none,
		PreviewBackgroundColor:               &previewBackground,
		PreviewBorderRadius:                  &none,
		PreviewPropertyTitleColor:            &secondaryColor,
		ToolbarBackgroundColor:               &appBackground,
		ToolbarBorderWidth:                   &none,
		ToolbarFontColor:                     &secondaryColor,
	}

	encoded, err := json.Marshal(definition)
	if err != nil {
		return common.Theme{}, err
	}
	theme, err := common.ParseThemeDocument(encoded, woxVersion)
	if err != nil {
		return common.Theme{}, err
	}
	theme.IsSystem = true
	theme.IsInstalled = true
	return theme, nil
}

// Fallback is the theme shown on Omarchy before a palette file is readable.
func Fallback(woxVersion string) (common.Theme, error) {
	return ThemeFromPalette(fallbackPalette, woxVersion)
}

func (p Palette) secondary() string {
	if p.DarkForeground != "" {
		return p.DarkForeground
	}
	if p.Muted != "" {
		return p.Muted
	}
	return p.Foreground
}

func (p Palette) orBackground(color string) string {
	if color != "" {
		return color
	}
	return p.Background
}

// windowBorder is the opaque ring. An empty Border means the palette was not
// read from colors.toml, so the ring falls back to accent.
func (p Palette) windowBorder() string {
	if p.Border != "" {
		return p.Border
	}
	return withAlpha(p.Accent, 1)
}

// borderColor picks one solid color for Omarchy's window ring.
// A Hyprland gradient is averaged because Wox paints a single outline.
func borderColor(values map[string]string, accent string) string {
	if color := solidBorder(values["hyprland_active_border"]); color != "" {
		return color
	}
	if color := solidBorder(values["active_border_color"]); color != "" {
		return color
	}
	return withAlpha(accent, 1)
}

// solidBorder averages the colors in a Hyprland border string. An empty result
// means the string did not contain a color.
func solidBorder(value string) string {
	return averageOpaque(parseBorderStops(value))
}

// parseBorderStops reads Hyprland rgb/rgba hex colors and CSS hex colors.
// Hyprland's rgba() is RRGGBBAA, not CSS decimal channels. Alpha is discarded.
func parseBorderStops(value string) []string {
	lower := strings.ToLower(value)
	var stops []string
	for i := 0; i < len(lower); i++ {
		switch {
		case strings.HasPrefix(lower[i:], "rgba("), strings.HasPrefix(lower[i:], "rgb("):
			open := strings.IndexByte(lower[i:], '(')
			close := strings.IndexByte(lower[i:], ')')
			if open < 0 || close < 0 {
				return stops
			}
			if color, ok := hyprHexColor(lower[i+open+1 : i+close]); ok {
				stops = append(stops, color)
			}
			i += close
		case lower[i] == '#':
			end := i + 1
			for end < len(lower) && end-i <= 8 && isHexDigit(lower[end]) {
				end++
			}
			if color, err := normalizeHex(lower[i:end]); err == nil {
				stops = append(stops, withAlpha(color, 1))
			}
			if end > i {
				i = end - 1
			}
		}
	}
	return stops
}

// hyprHexColor accepts Hyprland's hex rgb and rgba forms and drops the alpha byte.
func hyprHexColor(token string) (string, bool) {
	token = strings.TrimSpace(token)
	if strings.Contains(token, ",") || (len(token) != 6 && len(token) != 8) {
		return "", false
	}
	if _, err := hex.DecodeString(token); err != nil {
		return "", false
	}
	return "#" + strings.ToUpper(token[:6]) + "FF", true
}

func isHexDigit(char byte) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')
}

// averageOpaque collapses gradient stops into one opaque color.
func averageOpaque(colors []string) string {
	var red, green, blue, count int
	for _, color := range colors {
		raw, err := hex.DecodeString(strings.TrimPrefix(color, "#"))
		if err != nil || len(raw) < 3 {
			continue
		}
		red += int(raw[0])
		green += int(raw[1])
		blue += int(raw[2])
		count++
	}
	if count == 0 {
		return ""
	}
	channel := func(sum int) int {
		return (sum + count/2) / count
	}
	return fmt.Sprintf("#%02X%02X%02XFF", channel(red), channel(green), channel(blue))
}

// mixOpaque blends overlay into base. Hover uses it so the row does not become translucent.
func mixOpaque(base, overlay string, overlayWeight float64) (string, error) {
	if overlayWeight < 0 {
		overlayWeight = 0
	}
	if overlayWeight > 1 {
		overlayWeight = 1
	}
	baseColor, err := normalizeHex(base)
	if err != nil {
		return "", err
	}
	overlayColor, err := normalizeHex(overlay)
	if err != nil {
		return "", err
	}
	baseRGB, err := hex.DecodeString(baseColor[1:7])
	if err != nil {
		return "", err
	}
	overlayRGB, err := hex.DecodeString(overlayColor[1:7])
	if err != nil {
		return "", err
	}
	channel := func(from, to byte) byte {
		return byte(math.Round(float64(from) + (float64(to)-float64(from))*overlayWeight))
	}
	return fmt.Sprintf("#%02X%02X%02XFF", channel(baseRGB[0], overlayRGB[0]), channel(baseRGB[1], overlayRGB[1]), channel(baseRGB[2], overlayRGB[2])), nil
}

func requiredColor(values map[string]string, key string) (string, error) {
	color, err := normalizeHex(values[key])
	if err != nil {
		return "", fmt.Errorf("omarchy palette %s: %w", key, err)
	}
	return color, nil
}

func optionalColor(values map[string]string, key string) string {
	color, err := normalizeHex(values[key])
	if err != nil {
		return ""
	}
	return color
}

func parseTOMLValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	quote := value[0]
	if quote == '"' || quote == '\'' {
		rest := value[1:]
		if end := strings.IndexByte(rest, quote); end >= 0 {
			return rest[:end]
		}
	}
	if end := strings.IndexAny(value, " \t#"); end >= 0 {
		return strings.TrimSpace(value[:end])
	}
	return value
}

// normalizeHex accepts #RGB, #RRGGBB, and #RRGGBBAA. The result is opaque #RRGGBBAA.
func normalizeHex(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "#") {
		return "", fmt.Errorf("invalid color %q", value)
	}
	raw := value[1:]
	if len(raw) == 3 {
		raw = string([]byte{raw[0], raw[0], raw[1], raw[1], raw[2], raw[2]})
	}
	if len(raw) != 6 && len(raw) != 8 {
		return "", fmt.Errorf("invalid color %q", value)
	}
	if _, err := hex.DecodeString(raw); err != nil {
		return "", fmt.Errorf("invalid color %q", value)
	}
	raw = strings.ToUpper(raw)
	if len(raw) == 6 {
		raw += "FF"
	}
	return "#" + raw, nil
}

// withAlpha replaces the alpha channel. Floor matches Wox theme color normalization.
func withAlpha(hexColor string, alpha float64) string {
	normalized, err := normalizeHex(hexColor)
	if err != nil {
		return hexColor
	}
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	return normalized[:7] + fmt.Sprintf("%02X", uint8(math.Floor(255*alpha)))
}

var themeDescriptions = map[string]map[string]string{
	"en_US": {"theme_description": "Follows the current Omarchy theme."},
	"zh_CN": {"theme_description": "跟随当前的 Omarchy 主题。"},
	"ru_RU": {"theme_description": "Следует текущей теме Omarchy."},
	"pt_BR": {"theme_description": "Acompanha o tema atual do Omarchy."},
	"ko_KR": {"theme_description": "현재 Omarchy 테마를 따릅니다."},
	"ja_JP": {"theme_description": "現在の Omarchy テーマに合わせます。"},
}
