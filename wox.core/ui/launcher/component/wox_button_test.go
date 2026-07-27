package component

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestWoxButtonCentersContentInsideSymmetricPadding(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "test-button", Label: "Disable", Width: 96, Theme: Theme{}})
	semantics := button.(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	container := gesture.Child.(woxwidget.Container)
	if container.Padding.Left != container.Padding.Right {
		t.Fatalf("button horizontal padding = %+v, want symmetric padding", container.Padding)
	}
	content := container.Child.(woxwidget.Align)
	if content.Horizontal != 0.5 || content.Vertical != 0.5 {
		t.Fatalf("button content alignment = (%v, %v), want (0.5, 0.5)", content.Horizontal, content.Vertical)
	}
}
