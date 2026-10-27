package omarchytheme

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const hackermanColors = `
# Omarchy palette
mode = "dark"
accent = "#82FB9C"
selection = "#1f253a"
muted = "#2d3450"
background = "#0B0C16"
dark_background = "#080910"
darker_background = "#06060c"
lighter_background = "#151828"
foreground = "#ddf7ff"
dark_foreground = "#6a6e95"
light_foreground = "#b5c5db"
bright_foreground = "#ddf7ff"
hyprland_active_border = "rgba(26a269ee) rgba(2ec27eee) 45deg"
`

const flexokiLightColors = `
mode = "light"
accent = "#205EA6"
selection = "#CECDC3"
muted = "#B7B5AC"
background = "#FFFCF0"
dark_background = "#f2efe4"
darker_background = "#e5e2d8"
lighter_background = "#E6E4D9"
foreground = "#100F0F"
dark_foreground = "#878580"
light_foreground = "#403E3C"
bright_foreground = "#100F0F"
`

func TestParsePaletteReadsOmarchyRoles(t *testing.T) {
	palette, err := ParsePalette([]byte(hackermanColors))
	if err != nil {
		t.Fatal(err)
	}
	if palette.Mode != "dark" || palette.Accent != "#82FB9CFF" || palette.Selection != "#1F253AFF" || palette.Background != "#0B0C16FF" {
		t.Fatalf("palette = %#v", palette)
	}
	// Hackerman's window ring is the green gradient, not the brighter accent.
	if palette.Border != "#2AB274FF" {
		t.Fatalf("border = %s", palette.Border)
	}
	light, err := ParsePalette([]byte(flexokiLightColors))
	if err != nil {
		t.Fatal(err)
	}
	if light.Mode != "light" || light.Foreground != "#100F0FFF" || light.Border != "#205EA6FF" {
		t.Fatalf("light palette = %#v", light)
	}
}

func TestParsePaletteBorderPrefersWindowGradient(t *testing.T) {
	gradient, err := ParsePalette([]byte("background = \"#101315\"\nforeground = \"#cacccc\"\naccent = \"#798186\"\nhyprland_active_border = \"rgba(798186ee) rgba(caccccee)\"\nactive_border_color = \"#a8adb0\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if gradient.Border != "#A2A7A9FF" {
		t.Fatalf("gradient border = %s", gradient.Border)
	}
	solid, err := ParsePalette([]byte("background = \"#16242d\"\nforeground = \"#d6e2ee\"\naccent = \"#8bc9eb\"\nactive_border_color = \"#f2fcff\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if solid.Border != "#F2FCFFFF" {
		t.Fatalf("solid border = %s", solid.Border)
	}
}

func TestParsePaletteRequiresBackground(t *testing.T) {
	if _, err := ParsePalette([]byte("foreground = \"#fff\"\naccent = \"#abc\"\n")); err == nil {
		t.Fatal("missing background should fail")
	}
}

func TestThemeFromPaletteFollowsOmarchyColors(t *testing.T) {
	palette, err := ParsePalette([]byte(hackermanColors))
	if err != nil {
		t.Fatal(err)
	}
	theme, err := ThemeFromPalette(palette, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	if theme.ThemeId != ThemeID || theme.ThemeName != ThemeName || !theme.IsSystem || !theme.UsesCustomWindowChrome() {
		t.Fatalf("identity = %+v chrome=%v", theme.ThemeName, theme.UsesCustomWindowChrome())
	}
	if theme.AppBorderWidth == nil || *theme.AppBorderWidth != 2 || theme.AppBorderRadius == nil || *theme.AppBorderRadius != 0 {
		t.Fatalf("border geometry width=%v radius=%v", theme.AppBorderWidth, theme.AppBorderRadius)
	}
	if theme.ResolvedColors()["AppBorderColor"] != "#2AB274FF" {
		t.Fatalf("border color = %s", theme.ResolvedColors()["AppBorderColor"])
	}
	if theme.AppBackgroundColor != "#0B0C16FF" || theme.ToolbarBackgroundColor != "#0B0C16FF" {
		t.Fatalf("app %s toolbar %s", theme.AppBackgroundColor, theme.ToolbarBackgroundColor)
	}
	if theme.OverlayBackgroundColor != "#0B0C16FF" || theme.OverlayFontColor != "#DDF7FFFF" {
		t.Fatalf("overlay = %s %s", theme.OverlayBackgroundColor, theme.OverlayFontColor)
	}
	if theme.ResultItemTitleColor != "#DDF7FFFF" || theme.ResultItemActiveTitleColor != "#82FB9CFF" || theme.ResultItemActiveIndicatorColor != "#82FB9CFF" {
		t.Fatalf("text %s active %s accent %s", theme.ResultItemTitleColor, theme.ResultItemActiveTitleColor, theme.ResultItemActiveIndicatorColor)
	}
	if theme.ResultItemActiveBackgroundColor != "#1F253AFF" {
		t.Fatalf("selection = %s", theme.ResultItemActiveBackgroundColor)
	}
	hover, err := mixOpaque("#0B0C16", "#1F253A", 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if theme.ResolvedColors()["ResultItemHoverBackgroundColor"] != hover || len(hover) != 9 || hover[7:] != "FF" {
		t.Fatalf("hover = %s", theme.ResolvedColors()["ResultItemHoverBackgroundColor"])
	}
	if theme.QueryBoxBackgroundColor != "#151828FF" || theme.QueryBoxBorderRadius != 0 || theme.ResultItemBorderRadius != 0 {
		t.Fatalf("query %s radius %d result radius %d", theme.QueryBoxBackgroundColor, theme.QueryBoxBorderRadius, theme.ResultItemBorderRadius)
	}
	if theme.ResultItemSubTitleColor != "#6A6E95FF" {
		t.Fatalf("subtitle %s", theme.ResultItemSubTitleColor)
	}
	if theme.ActionContainerBackgroundColor != "#06060CFF" || theme.ActionContainerBorderWidth == nil || *theme.ActionContainerBorderWidth != 0 {
		t.Fatalf("action = %s", theme.ActionContainerBackgroundColor)
	}
	if theme.ResolvedColors()["PreviewBackgroundColor"] != "#080910FF" {
		t.Fatalf("preview = %s", theme.ResolvedColors()["PreviewBackgroundColor"])
	}
	if theme.ResultContainerPaddingTop != 8 || theme.ResultContainerPaddingBottom != 8 {
		t.Fatalf("result padding = %d %d", theme.ResultContainerPaddingTop, theme.ResultContainerPaddingBottom)
	}
	if theme.ResultItemActiveIndicatorWidth == nil || *theme.ResultItemActiveIndicatorWidth != 0 {
		t.Fatal("side indicator should stay off")
	}
	if theme.Description != "i18n:theme_description" || theme.I18n["zh_CN"]["theme_description"] == "" || theme.I18n["en_US"]["theme_description"] == "" {
		t.Fatalf("description = %s", theme.Description)
	}

	lightPalette, err := ParsePalette([]byte(flexokiLightColors))
	if err != nil {
		t.Fatal(err)
	}
	light, err := ThemeFromPalette(lightPalette, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	if light.ResultItemTitleColor != "#100F0FFF" || light.ResultItemActiveTitleColor != "#205EA6FF" || light.ResultItemActiveBackgroundColor != "#CECDC3FF" {
		t.Fatalf("light theme text %s accent %s selection %s", light.ResultItemTitleColor, light.ResultItemActiveTitleColor, light.ResultItemActiveBackgroundColor)
	}
	if !light.UsesCustomWindowChrome() || light.ResolvedColors()["AppBorderColor"] != "#205EA6FF" || light.AppBackgroundColor != "#FFFCF0FF" {
		t.Fatalf("light chrome=%v border=%s background=%s", light.UsesCustomWindowChrome(), light.ResolvedColors()["AppBorderColor"], light.AppBackgroundColor)
	}
}

func TestAvailableIn(t *testing.T) {
	root := t.TempDir()
	missingColors := filepath.Join(root, "missing.toml")
	if availableIn(filepath.Join(root, "missing-share"), missingColors) {
		t.Fatal("missing install should be unavailable")
	}
	share := filepath.Join(root, "omarchy")
	if err := os.Mkdir(share, 0o755); err != nil {
		t.Fatal(err)
	}
	if !availableIn(share, missingColors) {
		t.Fatal("share directory should enable the theme")
	}
	colors := filepath.Join(root, "colors.toml")
	if err := os.WriteFile(colors, []byte("mode = \"dark\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !availableIn(filepath.Join(root, "absent"), colors) {
		t.Fatal("colors file should enable the theme")
	}
}

func TestLoadLivePaletteWhenOmarchyPresent(t *testing.T) {
	if !Available() {
		t.Skip("Omarchy is not installed")
	}
	theme, err := Load("2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	border := theme.ResolvedColors()["AppBorderColor"]
	if theme.ThemeId != ThemeID || theme.ThemeName != ThemeName || !theme.UsesCustomWindowChrome() || !strings.HasSuffix(theme.AppBackgroundColor, "FF") || !strings.HasSuffix(border, "FF") {
		t.Fatalf("live theme = %s %s chrome=%v background=%s border=%s", theme.ThemeId, theme.ThemeName, theme.UsesCustomWindowChrome(), theme.AppBackgroundColor, border)
	}
	if theme.AppBorderWidth == nil || *theme.AppBorderWidth != 2 || theme.AppBorderRadius == nil || *theme.AppBorderRadius != 0 {
		t.Fatal("live theme should use Omarchy's square 2px ring")
	}
}
