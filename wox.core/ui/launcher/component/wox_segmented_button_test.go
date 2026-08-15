package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxSegmentedButtonAddsHoverSurface(t *testing.T) {
	foreground := woxui.Color{R: 220, G: 230, B: 240, A: 255}
	theme := Theme{ResultSubtitle: foreground}
	button := WoxSegmentedButton(SegmentedButtonProps{ID: "period-30d", Label: "最近 30 天", Width: 100, Theme: theme, OnTap: func() {}}).(woxwidget.Semantics)
	stateful := button.Child.(woxwidget.Focusable).Child
	normal := buildHoverable(stateful, false).(woxwidget.Gesture)
	hovered := buildHoverable(stateful, true).(woxwidget.Gesture)

	if normal.OnHoverAt == nil {
		t.Fatal("segmented button does not retain hover input")
	}
	if normal.Child.(woxwidget.Container).Color != (woxui.Color{}) {
		t.Fatalf("normal segmented button background = %#v, want transparent", normal.Child.(woxwidget.Container).Color)
	}
	if got, want := hovered.Child.(woxwidget.Container).Color, controlHoverColor(woxui.Color{}, foreground); got != want {
		t.Fatalf("hovered segmented button background = %#v, want %#v", got, want)
	}
}

func TestWoxSegmentedButtonPreservesSelectedStateOnHover(t *testing.T) {
	selected := woxui.Color{R: 70, G: 80, B: 90, A: 255}
	foreground := woxui.Color{R: 240, G: 242, B: 244, A: 255}
	theme := Theme{SelectedBackground: selected, SelectedTitle: foreground}
	button := WoxSegmentedButton(SegmentedButtonProps{ID: "period-30d", Label: "最近 30 天", Width: 100, Selected: true, Theme: theme}).(woxwidget.Semantics)
	if !button.Selected {
		t.Fatal("selected segmented button did not expose selected semantics")
	}
	hovered := buildHoverable(button.Child.(woxwidget.Focusable).Child, true).(woxwidget.Gesture)
	if got, want := hovered.Child.(woxwidget.Container).Color, controlHoverColor(selected, foreground); got != want {
		t.Fatalf("selected hover background = %#v, want %#v", got, want)
	}
}

func TestWoxSegmentedButtonDisablesHoverAndTapWhileLoading(t *testing.T) {
	button := WoxSegmentedButton(SegmentedButtonProps{ID: "period-loading", Label: "最近 7 天", Width: 100, Disabled: true, Theme: Theme{}, OnTap: func() {}}).(woxwidget.Semantics)
	if !button.Disabled || !button.Child.(woxwidget.Focusable).Disabled {
		t.Fatal("disabled segmented button did not propagate disabled semantics")
	}
	gesture := buildHoverable(button.Child.(woxwidget.Focusable).Child, true).(woxwidget.Gesture)
	if gesture.OnTap != nil {
		t.Fatal("disabled segmented button retained tap callback")
	}
	if gesture.Child.(woxwidget.Container).Color != (woxui.Color{}) {
		t.Fatalf("disabled segmented button background = %#v, want no hover surface", gesture.Child.(woxwidget.Container).Color)
	}
}
