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

func TestWoxButtonSupportsIntrinsicIconOnlyContent(t *testing.T) {
	icon := &woxui.Image{}
	button := WoxButton(ButtonProps{ID: "icon", Label: "Delete", Icon: icon, IconOnly: true, IntrinsicWidth: true, Height: 24, Theme: Theme{}})
	container := button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if container.Padding != (woxwidget.Insets{}) {
		t.Fatalf("icon-only padding = %+v, want zero", container.Padding)
	}
	if _, ok := container.Child.(woxwidget.Image); !ok {
		t.Fatalf("intrinsic icon-only child = %T, want woxwidget.Image", container.Child)
	}
}
