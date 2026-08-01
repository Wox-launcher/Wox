package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

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
