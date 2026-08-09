package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestScrollablePreviewTextUsesCompactHorizontalPadding(t *testing.T) {
	view := ScrollablePreviewText(ScrollablePreviewTextProps{
		ID: "test", Width: 320, Height: 200, Layout: woxwidget.TextBlockLayout{Size: woxui.Size{Height: 300}},
	}).(woxwidget.Container)

	if view.Padding != (woxwidget.Insets{Left: 14, Top: 24, Right: 14, Bottom: 24}) {
		t.Fatalf("scrollable preview padding = %#v, want compact horizontal padding", view.Padding)
	}
	child := resolvedScrollViewProps(view.Child, woxui.Size{Width: 292, Height: 152})
	if child.Width != 292 || child.Height != 152 {
		t.Fatalf("scrollable preview viewport = %.0fx%.0f, want 292x152", child.Width, child.Height)
	}
}

func resolvedScrollViewProps(widget woxwidget.Widget, size woxui.Size) woxcomponent.ScrollViewProps {
	builder := widget.(woxwidget.LayoutBuilder)
	return builder.Build(size).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
}
