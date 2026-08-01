package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxSearchFieldUsesHostFocusRing(t *testing.T) {
	cursor := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	subtitle := woxui.Color{R: 40, G: 50, B: 60, A: 255}
	icon := &woxui.Image{}
	controller := woxwidget.NewTextEditingController("existing")
	controller.SetText(controller.Text(), true)
	field := WoxSearchField(SearchFieldProps{
		ID: "search", Label: "Search", Width: 200, Focused: true, Controller: controller, SearchIcon: icon,
		Actions: []SearchFieldAction{{ID: "action", Width: 30}}, Theme: Theme{Cursor: cursor, ResultSubtitle: subtitle},
	}).(woxwidget.Container)

	if field.BorderColor != withAlpha(subtitle, 170) || field.BorderWidth != 1 {
		t.Fatalf("focused search border = %#v at %v, want neutral 1px border", field.BorderColor, field.BorderWidth)
	}
	input := field.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(TextFieldProps)
	if input.Controller != controller || input.Controller.State().Selection != (woxui.TextSelection{Anchor: 0, Focus: 8}) {
		t.Fatalf("search controller selection = %+v, want existing text selected", input.Controller.State().Selection)
	}
	if input.FocusRingColor != cursor || input.FocusRingOutsets.Left != 36 || input.FocusRingOutsets.Right != 34 {
		t.Fatalf("search focus ring = %#v with outsets %+v, want cursor with 36px left and 34px right", input.FocusRingColor, input.FocusRingOutsets)
	}
	action := field.Child.(woxwidget.Flex).Children[2].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(IconButtonProps)
	if action.Width != 30 || action.Height != 30 || action.Radius != 15 {
		t.Fatalf("search action geometry = %vx%v radius %v, want circular 30px button", action.Width, action.Height, action.Radius)
	}
	if inset := field.Child.(woxwidget.Flex).Children[3].(woxwidget.Container).Width; inset != 4 {
		t.Fatalf("search trailing inset = %v, want 4", inset)
	}
}
