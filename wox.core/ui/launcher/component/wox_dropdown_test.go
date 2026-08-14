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
