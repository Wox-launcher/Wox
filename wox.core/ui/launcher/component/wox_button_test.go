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
	gesture := buildHoverable(focusable.Child, false).(woxwidget.Gesture)
	container := gesture.Child.(woxwidget.Container)
	if container.Height != 32 || container.Radius != 4 {
		t.Fatalf("button geometry = height %v radius %v, want height 32 radius 4", container.Height, container.Radius)
	}
	if container.Padding.Left != container.Padding.Right {
		t.Fatalf("button horizontal padding = %+v, want symmetric padding", container.Padding)
	}
	content := container.Child.(woxwidget.Align)
	if content.Horizontal != 0.5 || content.Vertical != 0.5 {
		t.Fatalf("button content alignment = (%v, %v), want (0.5, 0.5)", content.Horizontal, content.Vertical)
	}
	label := content.Child.(woxwidget.Text)
	if label.Style.Size != CompactButtonFontSize || label.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("button label style = %+v, want semibold %.0fpx", label.Style, CompactButtonFontSize)
	}
}

func TestWoxButtonCentersIntrinsicContentVertically(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "apply", Label: "应用", FontSize: 13, IntrinsicWidth: true})
	container := buildHoverable(button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)

	wantPadding := (float32(32) - float32(13)*1.35) / 2
	if container.Width != 0 || container.Padding.Top != wantPadding || container.Padding.Bottom != wantPadding {
		t.Fatalf("intrinsic button = width %v padding %+v, want natural width and 13px-centered content", container.Width, container.Padding)
	}
}

func TestWoxButtonUsesSharedGeometry(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "add", Label: "Add"})
	container := buildHoverable(button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)

	if container.Height != 32 || container.Padding.Left != 12 || container.Padding.Right != 12 {
		t.Fatalf("button geometry = height %v padding %+v, want 32px height and symmetric 12px padding", container.Height, container.Padding)
	}
	if container.Padding.Top != container.Padding.Bottom {
		t.Fatalf("button vertical padding = %+v, want symmetric padding", container.Padding)
	}
}

func TestWoxButtonCentersIconAndLabel(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "add", Label: "添加", Icon: &woxui.Image{}, IconSize: 18})
	container := buildHoverable(button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)
	content, ok := container.Child.(woxwidget.Flex)
	if !ok || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("button icon and label layout = %#v, want horizontal flex with centered cross axis", container.Child)
	}
	if container.Padding.Top != container.Padding.Bottom {
		t.Fatalf("icon button vertical padding = %+v, want symmetric padding", container.Padding)
	}
}

func TestWoxButtonUsesContentWidthWhenWidthOmitted(t *testing.T) {
	button := WoxButton(ButtonProps{ID: "cancel", Label: "Cancel"})
	container := buildHoverable(button.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child, false).(woxwidget.Gesture).Child.(woxwidget.Container)
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

func TestWoxButtonUsesVariantAwareHoverOverlay(t *testing.T) {
	base := woxui.Color{R: 20, G: 40, B: 60, A: 255}
	foreground := woxui.Color{R: 220, G: 230, B: 240, A: 255}
	button := WoxButton(ButtonProps{ID: "save", Label: "Save", Variant: ButtonPrimary, OnTap: func() {}, Theme: Theme{ActionSelected: base, ActionSelectedText: foreground}}).(woxwidget.Semantics)
	stateful := button.Child.(woxwidget.Focusable).Child
	normal := buildHoverable(stateful, false).(woxwidget.Gesture)
	hovered := buildHoverable(stateful, true).(woxwidget.Gesture)

	if normal.OnHoverAt == nil {
		t.Fatal("button does not retain hover input")
	}
	if normal.Child.(woxwidget.Container).Color != base {
		t.Fatal("button normal background changed")
	}
	want := controlHoverColor(base, foreground)
	if got := hovered.Child.(woxwidget.Container).Color; got != want {
		t.Fatalf("button hover background = %#v, want %#v", got, want)
	}
}
