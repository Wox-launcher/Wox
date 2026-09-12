package view

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestGlanceThemeColors covers normal/hover rendering, transparency and legacy alpha.
func TestGlanceThemeColors(t *testing.T) {
	for _, custom := range []bool{false, true} {
		for _, hovered := range []bool{false, true} {
			text, normal, hover := woxui.Color{R: 30, A: 40}, woxui.Color{B: 40, A: 50}, woxui.Color{}
			theme := woxcomponent.Theme{QueryText: woxui.Color{R: 100, A: 200}}
			if custom {
				theme.GlanceFontColor, theme.GlanceIconColor, theme.GlanceBackgroundColor, theme.GlanceHoverBackgroundColor = &text, &hover, &normal, &hover
			}
			state := &glanceViewState{hovered: hovered}
			built := state.Build(woxwidget.StateContext{}, GlanceProps{Text: "12:00", Width: 100, Theme: theme, DensityScale: 1}).(woxwidget.Gesture).Child.(woxwidget.Container)
			label := built.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
			wantText, wantBackground := woxui.Color{R: 100, A: 160}, woxui.Color{}
			if hovered {
				wantBackground = woxui.Color{R: 100, A: 20}
			}
			if custom {
				wantText, wantBackground = text, normal
				if hovered {
					wantBackground = hover
				}
				if theme.GlanceIconTint() != hover {
					t.Fatal("transparent icon tint lost")
				}
			}
			if label.Color != wantText || built.Color != wantBackground {
				t.Fatalf("custom=%v hover=%v: unexpected colors", custom, hovered)
			}
		}
	}
}
