package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxButtonCentersContentInsideSymmetricPadding(t *testing.T) {
	focusColor := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	button := WoxButton(ButtonProps{ID: "test-button", Label: "Disable", Width: 96, Theme: Theme{Cursor: focusColor}})
	semantics := button.(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	if focusable.FocusRingColor != focusColor || focusable.FocusRingRadius != 4 {
		t.Fatalf("button focus ring = %#v at %v, want %#v at 4", focusable.FocusRingColor, focusable.FocusRingRadius, focusColor)
	}
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

func TestWoxButtonCentersIntrinsicContentVertically(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "apply", Label: "应用", Height: 36, IntrinsicWidth: true})
	container := button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if container.Width != 0 || container.Padding.Top <= 0 {
		t.Fatalf("intrinsic button = width %v top padding %v, want natural width and centered content", container.Width, container.Padding.Top)
	}
}

func TestWoxButtonUsesSharedCompactGeometry(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "add", Label: "Add", Size: ButtonCompact})
	container := button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if container.Height != 30 || container.Padding.Left != 12 || container.Padding.Right != 12 {
		t.Fatalf("compact button geometry = height %v padding %+v, want 30px height and symmetric 12px padding", container.Height, container.Padding)
	}
}

func TestWoxButtonUsesContentWidthWhenWidthOmitted(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "cancel", Label: "Cancel", Height: 36})
	container := button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.Width != 0 {
		t.Fatalf("omitted width = %v, want content-sized button like Flutter WoxButton", container.Width)
	}
	if _, isAlign := container.Child.(woxwidget.Align); isAlign {
		t.Fatal("content-sized buttons must not wrap labels in an expanding Align")
	}
	if label, ok := container.Child.(woxwidget.Text); !ok || label.Value != "Cancel" {
		t.Fatalf("content-sized label = %#v", container.Child)
	}
}
