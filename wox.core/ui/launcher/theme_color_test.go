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

func TestLauncherResetPreservesSettingsThemeEditor(t *testing.T) {
	settingsState := newThemeEditorState("settings-theme|test", map[string]any{"ThemeName": "Test"})
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(settingsState)
	app := &App{themeSettings: controller}

	app.clearLauncherThemeEditorPreview()
	if controller.ThemeEditor() != settingsState {
		t.Fatal("launcher reset cleared the Settings theme editor")
	}

	controller.SetThemeEditor(newThemeEditorState("query|result|test", map[string]any{"ThemeName": "Test"}))
	app.clearLauncherThemeEditorPreview()
	if controller.ThemeEditor() != nil {
		t.Fatal("launcher reset retained the launcher theme editor")
	}
}
