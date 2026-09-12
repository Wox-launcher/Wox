package launcher

import (
	"encoding/json"
	"os"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
)

// TestThemeIndependentChrome covers fallback, explicit zero, and persisted custom settings.
func TestThemeIndependentChrome(t *testing.T) {
	for _, tc := range []struct {
		name, extra         string
		panel, row, divider float32
		marker, border      woxui.Color
	}{
		{"legacy", "", 8, 4, 1, woxui.Color{R: 0x12, G: 0x34, B: 0x56, A: 255}, woxui.Color{R: 0xAB, G: 0xCD, B: 0xEF, A: 26}},
		{"custom", `,"ActionContainerBorderRadius":12,"ActionItemBorderRadius":6,"ToolbarBorderWidth":2,"ToolbarBorderColor":"#10203080","ResultItemActiveBorderLeftColor":"#40506080"`, 12, 6, 2, woxui.Color{R: 0x40, G: 0x50, B: 0x60, A: 128}, woxui.Color{R: 0x10, G: 0x20, B: 0x30, A: 128}},
		{"disabled", `,"ActionContainerBorderRadius":0,"ActionItemBorderRadius":0,"ToolbarBorderWidth":0,"ToolbarBorderColor":"#00000000","ResultItemActiveBorderLeftColor":"#00000000"`, 0, 0, 0, woxui.Color{}, woxui.Color{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var core common.Theme
			if err := json.Unmarshal([]byte(`{"ActionQueryBoxBorderRadius":8,"ResultItemBorderRadius":4,"QueryBoxCursorColor":"#123456","ToolbarFontColor":"#ABCDEF"`+tc.extra+`}`), &core); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(core)
			if err != nil {
				t.Fatal(err)
			}
			var draft themeData
			if err := json.Unmarshal(encoded, &draft); err != nil {
				t.Fatal(err)
			}
			for _, data := range []themeData{fromCoreTheme(core), draft} {
				palette := paletteForTheme(data)
				theme := palette.componentTheme()
				if theme.ActionContainerRadius != tc.panel || theme.ActionItemRadius != tc.row || theme.ToolbarBorderWidth != tc.divider || theme.ToolbarBorder != tc.border || theme.SelectedBorderLeftColor != tc.marker {
					t.Fatalf("unexpected chrome: %#v", theme)
				}
				if palette.actionQueryRadius != 8 || palette.resultItemRadius != 4 {
					t.Fatal("independent chrome changed query or result radius")
				}
			}
		})
	}
}

// TestBuiltinThemeChromePreservesLegacyAppearance guards explicit built-in values against visual drift.
func TestBuiltinThemeChromePreservesLegacyAppearance(t *testing.T) {
	for _, name := range []string{"dark", "light", "glass"} {
		data, err := os.ReadFile("../../common/testdata/theme_v1/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var core common.Theme
		if err := json.Unmarshal(data, &core); err != nil {
			t.Fatal(err)
		}
		explicit := fromCoreTheme(core)
		legacy := explicit
		legacy.ActionContainerBorderRadius, legacy.ActionItemBorderRadius, legacy.ToolbarBorderWidth = nil, nil, nil
		legacy.ToolbarBorderColor, legacy.ResultItemActiveBorderLeftColor = "", ""
		if paletteForTheme(explicit) != paletteForTheme(legacy) {
			t.Fatalf("%s explicit chrome changed legacy appearance", name)
		}
	}
}
