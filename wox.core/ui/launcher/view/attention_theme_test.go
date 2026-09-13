package view

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestAttentionThemeColors covers normal/hover rendering and explicit v2 overrides.
func TestAttentionThemeColors(t *testing.T) {
	icon, label, background, border := woxui.Color{R: 10, A: 255}, woxui.Color{G: 20, A: 255}, woxui.Color{B: 30, A: 40}, woxui.Color{R: 40, A: 50}
	hoverBackground, hoverBorder := woxui.Color{B: 80, A: 90}, woxui.Color{R: 90, A: 100}
	for _, custom := range []bool{false, true} {
		for _, hovered := range []bool{false, true} {
			theme := woxcomponent.Theme{QueryText: woxui.Color{R: 200, A: 255}, Cursor: woxui.Color{G: 160, B: 255, A: 255}}
			if custom {
				theme.AttentionFontColor, theme.AttentionIconColor = &label, &icon
				theme.AttentionBackgroundColor, theme.AttentionBorderColor = &background, &border
				theme.AttentionHoverBackgroundColor, theme.AttentionHoverBorderColor = &hoverBackground, &hoverBorder
			}
			wantIcon, wantLabel, wantBackground, wantBorder := theme.AttentionBadgeColors(hovered)
			if custom {
				if wantLabel != label || wantIcon != icon {
					t.Fatalf("custom=%v hover=%v: text/icon overrides lost", custom, hovered)
				}
				if hovered && (wantBackground != hoverBackground || wantBorder != hoverBorder) {
					t.Fatalf("custom hover colors = %#v/%#v", wantBackground, wantBorder)
				}
				if !hovered && (wantBackground != background || wantBorder != border) {
					t.Fatalf("custom idle colors = %#v/%#v", wantBackground, wantBorder)
				}
			}
			built := (&attentionUnreadViewState{hovered: hovered}).Build(woxwidget.StateContext{}, AttentionUnreadProps{
				Width: 50, Theme: theme, DensityScale: 1, UnreadCount: 1, CountText: "1",
			}).(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
			wantBorderWidth := float32(0)
			if wantBorder.A > 0 {
				wantBorderWidth = 1
			}
			if built.Color != wantBackground || built.BorderColor != wantBorder || built.BorderWidth != wantBorderWidth {
				t.Fatalf("custom=%v hover=%v: badge colors = %#v/%#v width %.0f", custom, hovered, built.Color, built.BorderColor, built.BorderWidth)
			}
			if !custom && !hovered && (wantBackground.A != 0 || wantBorder.A != 0) {
				t.Fatalf("default idle Attention chrome = %#v/%#v, want transparent like Glance", wantBackground, wantBorder)
			}
		}
	}
}
