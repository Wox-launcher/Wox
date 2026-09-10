package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestScrollablePreviewTextUsesCompactHorizontalPadding(t *testing.T) {
	view := ScrollablePreviewText(ScrollablePreviewTextProps{
		ID: "test", Width: 320, Height: 200, FontSize: 15, LineHeight: 23,
	}).(woxwidget.Container)

	if view.Padding != (woxwidget.Insets{Left: 14, Top: 24, Right: 14, Bottom: 24}) {
		t.Fatalf("scrollable preview padding = %#v, want compact horizontal padding", view.Padding)
	}
	child := resolvedScrollViewProps(view.Child, woxui.Size{Width: 292, Height: 152})
	if child.Width != 292 || child.Height != 152 {
		t.Fatalf("scrollable preview viewport = %.0fx%.0f, want 292x152", child.Width, child.Height)
	}
}

func TestScrollablePreviewTextUsesReadOnlyTextField(t *testing.T) {
	const body = "Copy this streamed answer."
	view := ScrollablePreviewText(ScrollablePreviewTextProps{
		ID: "ai", Value: body, Width: 320, Height: 200, FontSize: 15, LineHeight: 23, Window: &woxui.Window{},
	}).(woxwidget.Container)
	field := scrollablePreviewTextField(t, view)
	if field.ID != "preview-text-ai" || field.Value != body || !field.ReadOnly || !field.Transparent {
		t.Fatalf("scrollable preview field = %#v, want a transparent read-only text field", field)
	}
}

func TestTextPreviewUsesReadOnlyTextField(t *testing.T) {
	const body = "Short quote"
	view := TextPreview(TextPreviewProps{
		ID: "quote", Value: body, Width: 400, Height: 300, FontSize: 17, LineHeight: 25,
		Theme: woxcomponent.Theme{PreviewText: woxui.Color{A: 255}}, Window: &woxui.Window{},
	}).(woxwidget.Stack)
	if len(view.Children) != 2 {
		t.Fatalf("quote preview children = %d, want decorative marks plus selectable text", len(view.Children))
	}
	if _, ok := view.Children[0].Child.(woxwidget.Painter); !ok {
		t.Fatalf("quote marks = %T, want a painter for the decorative quotes", view.Children[0].Child)
	}
	align, ok := view.Children[1].Child.(woxwidget.Align)
	if !ok || align.Horizontal != 0.5 || align.Vertical != 0.5 {
		t.Fatalf("quote text alignment = %#v, want a centered align", view.Children[1].Child)
	}
	field, ok := align.Child.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("quote text = %T, want a read-only text field", align.Child)
	}
	props := field.Widget.(woxcomponent.TextFieldProps)
	if props.ID != "preview-quote-quote" || props.Value != body || !props.ReadOnly || !props.Transparent {
		t.Fatalf("quote preview field = %#v, want a transparent read-only text field", props)
	}
}

func TestTextPreviewFitsRejectsOverflowingText(t *testing.T) {
	style := woxui.TextStyle{Size: 17}
	if !TextPreviewFits("Short", &woxui.Window{}, style, 400, 300, 25) {
		t.Fatal("a short quote should use the centered quote treatment")
	}
	long := "this overflow line is intentionally long enough to wrap and exceed the quote viewport height"
	if TextPreviewFits(long+"\n"+long+"\n"+long+"\n"+long+"\n"+long+"\n"+long+"\n"+long, &woxui.Window{}, style, 200, 120, 25) {
		t.Fatal("a tall quote should fall back to the scrollable text preview")
	}
}

func scrollablePreviewTextField(t *testing.T, view woxwidget.Container) woxcomponent.TextFieldProps {
	t.Helper()
	child := resolvedScrollViewProps(view.Child, woxui.Size{Width: 292, Height: 152})
	field, ok := child.Content.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("scrollable preview content = %T, want a read-only text field", child.Content)
	}
	return field.Widget.(woxcomponent.TextFieldProps)
}

func resolvedScrollViewProps(widget woxwidget.Widget, size woxui.Size) woxcomponent.ScrollViewProps {
	builder := widget.(woxwidget.LayoutBuilder)
	return builder.Build(size).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
}
