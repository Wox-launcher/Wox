package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxDropdownAddsHoverSurface(t *testing.T) {
	foreground := woxui.Color{R: 120, G: 140, B: 160, A: 255}
	dropdown := WoxDropdown(DropdownProps{ID: "mode", Label: "Mode", Value: "Auto", Width: 160, Height: 38, Foreground: foreground, OnTap: func() {}}).(woxwidget.Semantics)
	stateful := dropdown.Child.(woxwidget.Focusable).Child
	normal := buildHoverable(stateful, false).(woxwidget.Gesture)
	hovered := buildHoverable(stateful, true).(woxwidget.Gesture)

	if normal.OnHoverAt == nil {
		t.Fatal("dropdown does not retain hover input")
	}
	if normal.Child.(woxwidget.Container).Color.A != 0 {
		t.Fatal("dropdown normal background should remain transparent")
	}
	if got := hovered.Child.(woxwidget.Container).Color; got != controlHoverColor(woxui.Color{}, foreground) {
		t.Fatalf("dropdown hover background = %#v", got)
	}
}

func TestWoxDropdownOutlineFollowsValueText(t *testing.T) {
	foreground := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	dropdown := WoxDropdown(DropdownProps{
		ID: "mode", Label: "Mode", Value: "Auto", Width: 160, Height: 38, Foreground: foreground,
		Theme: ControlTheme{TextSecondary: woxui.Color{R: 255, A: 255}}, OnTap: func() {},
	}).(woxwidget.Semantics)
	trigger := buildHoverable(dropdown.Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)
	want := foreground
	want.A = 80
	if trigger.BorderColor != want {
		t.Fatalf("dropdown outline = %#v, want value text %#v", trigger.BorderColor, want)
	}
}

func TestWoxDropdownDefaultsToStandardControlHeight(t *testing.T) {
	dropdown := WoxDropdown(DropdownProps{ID: "mode", Label: "Mode", Value: "Auto", Width: 160, OnTap: func() {}}).(woxwidget.Semantics)
	stateful := dropdown.Child.(woxwidget.Focusable).Child
	trigger := buildHoverable(stateful, false).(woxwidget.Gesture).Child.(woxwidget.Container)
	if trigger.Height != SettingsControlHeight {
		t.Fatalf("dropdown default height = %.0f, want %.0f", trigger.Height, SettingsControlHeight)
	}
}

func TestWoxDropdownGivesLabelRemainingSpaceAfterTrailing(t *testing.T) {
	dropdown := WoxDropdown(DropdownProps{
		ID: "channel", Label: "Update channel", Value: "Stable channel", Trailing: "v2.4.4",
		Width: SettingsChoiceControlWidth, OnTap: func() {},
	}).(woxwidget.Semantics)
	trigger := buildHoverable(dropdown.Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)
	flex := trigger.Child.(woxwidget.Flex)
	if _, ok := flex.Children[0].(woxwidget.Expanded); !ok {
		t.Fatalf("label child = %T, want Expanded so a short version trailer cannot reserve a fixed column", flex.Children[0])
	}
	trailing, ok := flex.Children[2].(woxwidget.Text)
	if !ok || trailing.Value != "v2.4.4" {
		t.Fatalf("trailing child = %#v, want intrinsic version text", flex.Children[2])
	}
}
