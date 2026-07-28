package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxButtonCentersContentInsideSymmetricPadding(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "test-button", Label: "Disable", Width: 96, Theme: Theme{}})
	semantics := button.(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	container := gesture.Child.(woxwidget.Container)
	if container.Height != 38 || container.Radius != 4 {
		t.Fatalf("button geometry = height %v radius %v, want height 38 radius 4", container.Height, container.Radius)
	}
	if container.Padding.Left != container.Padding.Right {
		t.Fatalf("button horizontal padding = %+v, want symmetric padding", container.Padding)
	}
	content := container.Child.(woxwidget.Align)
	if content.Horizontal != 0.5 || content.Vertical != 0.5 {
		t.Fatalf("button content alignment = (%v, %v), want (0.5, 0.5)", content.Horizontal, content.Vertical)
	}
	label := content.Child.(woxwidget.Text)
	if label.Style.Size != 13 || label.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("button label style = %+v, want regular 13px", label.Style)
	}
}
