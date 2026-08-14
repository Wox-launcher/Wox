package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxListItemPreservesCustomRowStyle(t *testing.T) {
	radius := float32(4)
	background := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	border := woxui.Color{R: 40, G: 50, B: 60, A: 255}
	focus := woxui.Color{R: 70, G: 80, B: 90, A: 255}
	item := WoxListItem(ListItemProps{
		ID: "item", Label: "Item", Width: 120, Height: 40, Radius: &radius,
		Background: &background, BorderColor: border, BorderWidth: 1, Selected: true, Theme: Theme{Cursor: focus},
	})

	semantics := item.(woxwidget.Semantics)
	if !semantics.Selected {
		t.Fatal("list item semantics should preserve selected state")
	}
	focusable := semantics.Child.(woxwidget.Focusable)
	if focusable.FocusRingColor != focus || focusable.FocusRingRadius != radius {
		t.Fatalf("list item focus ring = %#v at %v, want %#v at %v", focusable.FocusRingColor, focusable.FocusRingRadius, focus, radius)
	}
	container := focusable.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.Color != background || container.BorderColor != border || container.BorderWidth != 1 || container.Radius != radius {
		t.Fatalf("list item style = %#v, want custom background, border, and radius", container)
	}
}

func TestWoxListItemCanSkipKeyboardFocus(t *testing.T) {
	item := WoxListItem(ListItemProps{ID: "group", Label: "Group", SkipFocus: true, Theme: Theme{}})
	semantics := item.(woxwidget.Semantics)
	if _, ok := semantics.Child.(woxwidget.Gesture); !ok {
		t.Fatalf("skip-focus child = %T, want pointer gesture without focusable wrapper", semantics.Child)
	}
}

func TestWoxListItemHoversOnlyWhenClickable(t *testing.T) {
	hover := woxui.Color{R: 30, G: 40, B: 50, A: 25}
	clickable := WoxListItem(ListItemProps{ID: "page", Label: "Page", HoverBackground: &hover, OnTap: func() {}, Theme: Theme{}}).(woxwidget.Semantics)
	stateful := clickable.Child.(woxwidget.Focusable).Child
	hovered := buildHoverable(stateful, true).(woxwidget.Gesture).Child.(woxwidget.Container)
	if hovered.Color != hover {
		t.Fatalf("clickable list item hover = %#v, want %#v", hovered.Color, hover)
	}

	group := WoxListItem(ListItemProps{ID: "group", Label: "Group", SkipFocus: true, HoverBackground: &hover, Theme: Theme{}}).(woxwidget.Semantics)
	gesture := group.Child.(woxwidget.Gesture)
	if gesture.OnHoverAt != nil || gesture.Child.(woxwidget.Container).Color == hover {
		t.Fatal("non-clickable list item should remain static")
	}
}
