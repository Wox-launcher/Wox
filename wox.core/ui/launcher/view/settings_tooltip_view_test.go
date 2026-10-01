package view

import (
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsInlineTooltipOverlayAnchorsNearField(t *testing.T) {
	overlay, left, top := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 900, Height: 640, Anchor: woxui.Rect{X: 520, Y: 180, Width: 14, Height: 14}, Message: "Tooltip content", Side: "left", Theme: woxcomponent.ControlTheme{},
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
		Width: 420, Height: 300, Anchor: woxui.Rect{X: 6, Y: 140, Width: 12, Height: 12}, Message: "Tooltip content", Side: "left", Theme: woxcomponent.ControlTheme{},
	})
	if overlay == nil {
		t.Fatal("expected tooltip overlay")
	}
	if left <= 20 {
		t.Fatalf("tooltip left = %.0f, want fallback to right side when left side overflows", left)
	}
}

func TestSettingsInlineTooltipOverlayAnchorsAboveField(t *testing.T) {
	overlay, left, top := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 900, Height: 640, Anchor: woxui.Rect{X: 520, Y: 180, Width: 14, Height: 14}, Message: "Tooltip content", Side: "top", Theme: woxcomponent.ControlTheme{},
	})
	if overlay == nil {
		t.Fatal("expected tooltip overlay")
	}
	container, ok := overlay.(woxwidget.Container)
	if !ok {
		t.Fatalf("overlay type = %T, want woxwidget.Container", overlay)
	}
	if top+container.Height > 180 {
		t.Fatalf("tooltip top = %.0f height = %.0f, want above the anchor", top, container.Height)
	}
	center := left + container.Width/2
	if center < 520 || center > 534 {
		t.Fatalf("tooltip center = %.0f, want horizontally centered on the 520-534 anchor", center)
	}
}

func TestSettingsInlineTooltipOverlayFlipsBelowWhenTopOverflows(t *testing.T) {
	overlay, _, top := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 900, Height: 640, Anchor: woxui.Rect{X: 520, Y: 4, Width: 14, Height: 14}, Message: "Tooltip content", Side: "top", Theme: woxcomponent.ControlTheme{},
	})
	if overlay == nil {
		t.Fatal("expected tooltip overlay")
	}
	if top < 18 {
		t.Fatalf("tooltip top = %.0f, want below the anchor when the top side overflows", top)
	}
}

func TestSettingsInlineTooltipWidthFollowsText(t *testing.T) {
	short := settingsInlineTooltipContainer(t, "Hi")
	medium := settingsInlineTooltipContainer(t, "用于显示或隐藏Wox的快捷键")
	long := settingsInlineTooltipContainer(t, strings.Repeat("设置说明", 80))
	if short.Width >= medium.Width {
		t.Fatalf("tooltip widths = short %.0f medium %.0f, want the longer label to be wider", short.Width, medium.Width)
	}
	if short.Width >= 120 {
		t.Fatalf("short tooltip width = %.0f, want it to hug the text", short.Width)
	}
	if long.Width > settingsInlineTooltipMaxWidth+0.5 {
		t.Fatalf("long tooltip width = %.0f, want the native cap %.0f", long.Width, settingsInlineTooltipMaxWidth)
	}
	if long.Height <= medium.Height {
		t.Fatalf("long tooltip height = %.0f, medium = %.0f, want the capped width to wrap", long.Height, medium.Height)
	}
}

func settingsInlineTooltipContainer(t *testing.T, message string) woxwidget.Container {
	t.Helper()
	overlay, _, _ := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{
		Width: 900, Height: 640, Anchor: woxui.Rect{X: 520, Y: 180, Width: 14, Height: 14}, Message: message, Side: "top", Theme: woxcomponent.ControlTheme{},
	})
	container, ok := overlay.(woxwidget.Container)
	if !ok {
		t.Fatalf("overlay type = %T, want woxwidget.Container", overlay)
	}
	return container
}

func TestSettingsInlineTooltipOverlayReturnsNilForEmptyMessage(t *testing.T) {
	overlay, _, _ := SettingsInlineTooltipOverlay(SettingsInlineTooltipProps{Width: 600, Height: 400, Message: "   "})
	if overlay != nil {
		t.Fatalf("overlay = %#v, want nil for empty message", overlay)
	}
}
