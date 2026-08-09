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
		top           float32
		bottom        float32
		anchorRight   bool
		anchorBottom  bool
		stretchWidth  bool
		stretchHeight bool
	}{
		{stretchWidth: true},
		{anchorBottom: true, stretchWidth: true},
		{top: 5, bottom: 5, stretchHeight: true},
		{top: 5, bottom: 5, anchorRight: true, stretchHeight: true},
	}
	for index, want := range wantPositions {
		child := area.Children[index+1]
		if child.Top != want.top || child.Bottom != want.bottom || child.AnchorRight != want.anchorRight || child.AnchorBottom != want.anchorBottom || child.StretchWidth != want.stretchWidth || child.StretchHeight != want.stretchHeight {
			t.Fatalf("edge %d layout = %+v, want top/bottom %.0f/%.0f anchors %v/%v stretch %v/%v", index, child, want.top, want.bottom, want.anchorRight, want.anchorBottom, want.stretchWidth, want.stretchHeight)
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
	if len(shown.Children) != 2 || !shown.Children[1].AnchorRight || shown.Children[1].Right != 20 || shown.Children[1].Top != 20 {
		t.Fatalf("shown close placement = %#v", shown.Children)
	}
	button := shown.Children[1].Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if button.OnHoverAt == nil || button.OnTap == nil || button.Width != 28 || button.Height != 28 {
		t.Fatalf("close icon button props = %+v, want hoverable 28x28 button", button)
	}
}
