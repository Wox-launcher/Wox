package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsDemoOverlayClampsAndForwardsHover(t *testing.T) {
	var hoverStates []bool
	overlay, left, top := SettingsDemoOverlay(
		OnboardingProps{Labels: map[string]string{}, Theme: woxcomponent.Theme{}},
		OnboardingStep{ID: "queryHotkeys", Title: "Query Hotkeys"},
		woxui.Rect{X: 1080, Y: 700, Width: 18, Height: 18},
		1152,
		768,
		woxcomponent.Theme{},
		func(inside bool) { hoverStates = append(hoverStates, inside) },
	)
	semantics := overlay.(woxwidget.Semantics)
	if semantics.AutomationID != "settings-demo-overlay-queryHotkeys" {
		t.Fatalf("overlay automation ID = %q", semantics.AutomationID)
	}
	if left != 418 || top != 228 {
		t.Fatalf("overlay placement = (%v, %v), want (418, 228)", left, top)
	}
	gesture := semantics.Child.(woxwidget.Gesture)
	panel := gesture.Child.(woxwidget.Container)
	if panel.Width != 680 || panel.Height != 460 {
		t.Fatalf("overlay size = %vx%v, want 680x460", panel.Width, panel.Height)
	}
	gesture.OnHover(true)
	gesture.OnHover(false)
	if len(hoverStates) != 2 || !hoverStates[0] || hoverStates[1] {
		t.Fatalf("overlay hover states = %v, want [true false]", hoverStates)
	}
}
