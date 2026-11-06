package pluginwindow

import (
	"testing"

	"wox/common"
	ui "wox/ui/runtime"
	"wox/util/overlay"
)

// TestControlThemeFollowsOverlayPalette guards all new roles at the feature boundary.
func TestControlThemeFollowsOverlayPalette(t *testing.T) {
	theme := common.Theme{AppBackgroundColor: "#111111", QueryBoxCursorColor: "#123456", ResultItemActiveTitleColor: "#345678", SuccessColor: "#456789", WarningColor: "#56789a", WarningTextColor: "#6789ab", ErrorColor: "#789abc"}
	overlay.SetThemeProvider(func() common.Theme { return theme })
	t.Cleanup(func() { overlay.SetThemeProvider(nil) })
	for _, light := range []bool{false, true} {
		if light {
			theme.AppBackgroundColor = "#eeeeee"
		}
		chrome := overlay.CurrentThemeChrome()
		got := controlTheme(chrome)
		if got.Dark == light || got.Overlay != chrome.Background {
			t.Fatal("theme uses the wrong surface appearance")
		}
		if got.Success != (ui.Color{R: 0x45, G: 0x67, B: 0x89, A: 255}) || got.Warning != (ui.Color{R: 0x56, G: 0x78, B: 0x9a, A: 255}) || got.WarningText != (ui.Color{R: 0x67, G: 0x89, B: 0xab, A: 255}) || got.Error != (ui.Color{R: 0x78, G: 0x9a, B: 0xbc, A: 255}) {
			t.Fatalf("hardcoded status colors: %+v", got)
		}
		if got.Focus != (ui.Color{R: 0x12, G: 0x34, B: 0x56, A: 255}) || got.SelectionText != (ui.Color{R: 0x34, G: 0x56, B: 0x78, A: 255}) {
			t.Fatal("lost focus/selection roles")
		}
		if got.Hover.A == 0 || got.Disabled.A == 0 || got.Link != got.Accent {
			t.Fatal("missing derived semantic role")
		}
	}
}
