package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

func TestBorderDragMoveAreaProvidesFourEdgeDragGestures(t *testing.T) {
	dragged := 0
	area := BorderDragMoveArea(100, 80, 5, woxwidget.Container{}, func() { dragged++ }).(woxwidget.Stack)
	if len(area.Children) != 5 {
		t.Fatalf("border drag child count = %d, want content plus four edges", len(area.Children))
	}

	wantPositions := []struct {
		left float32
		top  float32
	}{
		{left: 0, top: 0},
		{left: 0, top: 75},
		{left: 0, top: 5},
		{left: 95, top: 5},
	}
	for index, want := range wantPositions {
		child := area.Children[index+1]
		if child.Left != want.left || child.Top != want.top {
			t.Fatalf("edge %d position = (%v, %v), want (%v, %v)", index, child.Left, child.Top, want.left, want.top)
		}
		gesture, ok := child.Child.(woxwidget.Gesture)
		if !ok || gesture.OnDragStart == nil {
			t.Fatalf("edge %d does not expose a drag gesture", index)
		}
		gesture.OnDragStart()
	}
	if dragged != 4 {
		t.Fatalf("drag callback count = %d, want four", dragged)
	}
}

func TestPreviewHoverCloseRevealsCloseButton(t *testing.T) {
	state := &previewHoverCloseState{}
	props := PreviewHoverCloseProps{Width: 500, Height: 300, Child: woxwidget.Container{}, Label: "Close", Theme: woxcomponent.Theme{}, OnClose: func() {}}

	hidden := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	if len(hidden.Children) != 1 {
		t.Fatalf("hidden child count = %d, want preview only", len(hidden.Children))
	}

	state.hovered = true
	shown := state.Build(woxwidget.StateContext{}, props).(woxwidget.Gesture).Child.(woxwidget.Stack)
	if len(shown.Children) != 2 || shown.Children[1].Left != 452 || shown.Children[1].Top != 20 {
		t.Fatalf("shown close placement = %#v", shown.Children)
	}
	button := shown.Children[1].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if button.OnHoverAt == nil || button.OnTap == nil || button.Width != 28 || button.Height != 28 {
		t.Fatalf("close icon button props = %+v, want hoverable 28x28 button", button)
	}
}
