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
	focusCalls := 0
	focusChanges := 0
	controller := woxwidget.NewTextEditingController("existing")
	controller.SetText(controller.Text(), true)
	field := WoxSearchField(SearchFieldProps{
		ID: "search", Label: "Search", Width: 200, Focused: true, Controller: controller, SearchIcon: icon,
		Actions: []SearchFieldAction{{ID: "action", Width: 30}}, Theme: Theme{Cursor: cursor, ResultSubtitle: subtitle}, OnFocus: func() { focusCalls++ },
		OnFocusChange: func(focused bool) {
			if focused {
				focusChanges++
			}
		},
	}).(woxwidget.Container)

	if field.Height != SettingsSearchHeight || field.BorderColor != withAlpha(subtitle, 170) || field.BorderWidth != 1 {
		t.Fatalf("focused search border = %#v at %v, want neutral 1px border", field.BorderColor, field.BorderWidth)
	}
	stack := field.Child.(woxwidget.Stack)
	input := stack.Children[0].Child.(woxwidget.Stateful).Widget.(TextFieldProps)
	if input.Controller != controller || input.Controller.State().Selection != (woxui.TextSelection{Anchor: 0, Focus: 8}) {
		t.Fatalf("search controller selection = %+v, want existing text selected", input.Controller.State().Selection)
	}
	if input.Width != 200 || input.Height != SettingsSearchHeight || input.Padding.Left != 36+2 || input.Padding.Right != 6+34 || input.Padding.Top != 10 || input.Padding.Bottom != 10 {
		t.Fatalf("search input geometry = %.0f with padding %+v, want full-width input with leading/trailing insets", input.Width, input.Padding)
	}
	if input.FocusRingColor != cursor || input.FocusRingOutsets != (woxwidget.Insets{}) {
		t.Fatalf("search focus ring = %#v with outsets %+v, want cursor on the full-width field", input.FocusRingColor, input.FocusRingOutsets)
	}
	input.OnFocusChange(true)
	if focusCalls != 1 || focusChanges != 1 {
		t.Fatalf("search focus callbacks = %d/%d, want both callbacks after focusing the full-width field", focusCalls, focusChanges)
	}
	overlay := stack.Children[1].Child.(woxwidget.Flex)
	if _, ok := overlay.Children[0].(woxwidget.Align); !ok {
		t.Fatalf("search leading icon overlay = %T, want non-interactive overlay", overlay.Children[0])
	}
	action := overlay.Children[2].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(IconButtonProps)
	if action.Width != 30 || action.Height != 30 || action.Radius != 15 {
		t.Fatalf("search action geometry = %vx%v radius %v, want circular 30px button", action.Width, action.Height, action.Radius)
	}
	if inset := overlay.Children[3].(woxwidget.Container).Width; inset != 4 {
		t.Fatalf("search trailing inset = %v, want 4", inset)
	}
}

func TestWoxSearchFieldHoverSurfaceIncludesLeadingIcon(t *testing.T) {
	field := WoxSearchField(SearchFieldProps{
		ID: "search", Label: "Search", Width: 200, SearchIcon: &woxui.Image{}, Theme: Theme{},
	}).(woxwidget.Container)
	stack := field.Child.(woxwidget.Stack)
	input := stack.Children[0].Child.(woxwidget.Stateful).Widget.(TextFieldProps)

	if input.Width != field.Width || input.Padding.Left != 38 {
		t.Fatalf("search hover surface geometry = width %.0f, left padding %.0f; want full field width and 36px icon inset", input.Width, input.Padding.Left)
	}
	if _, ok := stack.Children[1].Child.(woxwidget.Flex).Children[0].(woxwidget.Gesture); ok {
		t.Fatal("leading icon overlay should let the full-width text field receive hover events")
	}
}
