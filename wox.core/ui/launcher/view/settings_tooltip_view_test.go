package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsInlineTooltipOverlayAnchorsNearField(t *testing.T) {
	overlay, left, top := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 900, Height: 640, Anchor: woxui.Rect{X: 520, Y: 180, Width: 14, Height: 14}, Message: "Tooltip content", Side: "left", Theme: woxcomponent.Theme{},
	})
	if overlay == nil {
		t.Fatal("expected tooltip overlay")
	}
	container, ok := overlay.(woxwidget.Container)
	if !ok {
		t.Fatalf("overlay type = %T, want woxwidget.Container", overlay)
	}
	if container.Width <= 0 || container.Height <= 0 {
		t.Fatalf("tooltip size = %.0fx%.0f, want positive bounds", container.Width, container.Height)
	}
	if left >= 520 {
		t.Fatalf("tooltip left = %.0f, want left of anchor", left)
	}
	if top < settingsInlineTooltipMargin || top+container.Height > 640-settingsInlineTooltipMargin {
		t.Fatalf("tooltip top = %.0f height = %.0f, want clamped inside window", top, container.Height)
	}
}

func TestSettingsInlineTooltipOverlayFlipsInsideWindow(t *testing.T) {
	overlay, left, _ := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 420, Height: 300, Anchor: woxui.Rect{X: 6, Y: 140, Width: 12, Height: 12}, Message: "Tooltip content", Side: "left", Theme: woxcomponent.Theme{},
	})
	if overlay == nil {
		t.Fatal("expected tooltip overlay")
	}
	if left <= 20 {
		t.Fatalf("tooltip left = %.0f, want fallback to right side when left side overflows", left)
	}
}

func TestSettingsInlineTooltipOverlayReturnsNilForEmptyMessage(t *testing.T) {
	overlay, _, _ := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{Width: 600, Height: 400, Message: "   "})
	if overlay != nil {
		t.Fatalf("overlay = %#v, want nil for empty message", overlay)
	}
}
