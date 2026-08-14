package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxCheckboxMatchesFlutterGeometryAndSemantics(t *testing.T) {
	active := woxui.Color{R: 20, G: 40, B: 60, A: 255}
	checkbox := WoxCheckbox(CheckboxProps{ID: "filter", Label: "Filter", Value: true, OnChange: func(bool) {}, Theme: Theme{ActionSelected: active}}).(woxwidget.Semantics)
	stateful := checkbox.Child.(woxwidget.Focusable).Child
	visual := buildHoverable(stateful, false).(woxwidget.Gesture).Child.(woxwidget.Container)

	if checkbox.Role != woxui.AccessibilityRoleCheckBox || !checkbox.Checked {
		t.Fatalf("checkbox semantics = role %q checked %v", checkbox.Role, checkbox.Checked)
	}
	if visual.Width != 18 || visual.Height != 18 || visual.Radius != 4 || visual.Color != active {
		t.Fatalf("checkbox geometry = %vx%v radius %v color %#v", visual.Width, visual.Height, visual.Radius, visual.Color)
	}
	hovered := buildHoverable(stateful, true).(woxwidget.Gesture).Child.(woxwidget.Container)
	if hovered.Color == visual.Color {
		t.Fatal("checked checkbox hover background did not change")
	}
}
