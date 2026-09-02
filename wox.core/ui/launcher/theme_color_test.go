package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestThemeColorHSVRoundTripAndCSSEncoding(t *testing.T) {
	parsed, ok := decodeThemeColor("rgba(22, 22, 26, 0.52)")
	if !ok || parsed.A != 132 {
		t.Fatalf("parsed Flutter CSS alpha = %#v, %t, want alpha 132", parsed, ok)
	}
	original := woxui.Color{R: 22, G: 22, B: 26, A: 132}
	roundTrip := themeColorFromHSV(themeColorToHSV(original))
	if roundTrip != original {
		t.Fatalf("HSV round trip = %#v, want %#v", roundTrip, original)
	}
	if encoded := encodeThemeColor(roundTrip); encoded != "#16161A84" {
		t.Fatalf("encoded color = %q, want #16161A84", encoded)
	}
}

func TestThemeEditorExposesActiveResultTail(t *testing.T) {
	for _, token := range themeEditorTokens() {
		if token.key == "ResultItemActiveTailTextColor" {
			return
		}
	}
	t.Fatal("theme editor is missing ResultItemActiveTailTextColor")
}

func TestPaletteMapsResultTailIndependentlyFromSubtitle(t *testing.T) {
	theme := paletteForTheme(themeData{
		ResultItemSubTitleColor:       "#0A141E",
		ResultItemTailTextColor:       "#C85028",
		ResultItemActiveTailTextColor: "#28B45A",
	}).componentTheme()
	if theme.ResultTail != (woxui.Color{R: 0xC8, G: 0x50, B: 0x28, A: 255}) {
		t.Fatalf("result tail = %#v, want the dedicated tail token", theme.ResultTail)
	}
	if theme.SelectedTail != (woxui.Color{R: 0x28, G: 0xB4, B: 0x5A, A: 255}) {
		t.Fatalf("selected tail = %#v, want the dedicated active tail token", theme.SelectedTail)
	}
	if theme.ResultSubtitle == theme.ResultTail {
		t.Fatalf("result tail reused subtitle color %#v", theme.ResultSubtitle)
	}
}

func TestSettingsThemeEditorAppliesAndCancelsLiveColor(t *testing.T) {
	raw := map[string]any{"ThemeName": "Test"}
	for _, token := range themeEditorTokens() {
		raw[token.key] = "#000000"
	}
	state := newThemeEditorState("settings-theme|test", raw)
	state.dialogMode = "token"
	state.dialogToken = "AppBackgroundColor"
	state.dialogOriginal = "#000000"
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(state)
	app := &App{themeSettings: controller}

	app.updateThemeEditorDialogColor(func(color themeColorHSV) themeColorHSV {
		color.hue = 0
		color.saturation = 1
		color.value = 1
		return color
	})
	if app.palette.background != (woxui.Color{R: 255, A: 255}) {
		t.Fatalf("live background = %#v, want red", app.palette.background)
	}

	app.cancelThemeEditorDialog()
	if app.palette.background != (woxui.Color{A: 255}) {
		t.Fatalf("cancelled background = %#v, want black", app.palette.background)
	}
}
