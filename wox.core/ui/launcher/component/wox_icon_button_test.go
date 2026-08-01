package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxIconButtonOwnsHoverAndTapGesture(t *testing.T) {
	hoverColor := woxui.Color{R: 20, G: 30, B: 40, A: 25}
	props := IconButtonProps{ID: "close", Label: "Close", Icon: woxwidget.Painter{Width: 16, Height: 16}, Width: 28, Height: 28, Radius: 14, HoverBackground: hoverColor, OnTap: func() {}}
	state := &iconButtonState{}
	normal := state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	if normal.OnTap == nil || normal.OnHoverAt == nil {
		t.Fatal("icon button hover and tap are not handled by the same gesture")
	}
	if background := normal.Child.(woxwidget.Container).Color; background.A != 0 {
		t.Fatalf("icon button default background = %#v, want transparent", background)
	}
	state.hovered = true
	hovered := state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if hovered.Color != hoverColor {
		t.Fatalf("icon button hover background = %#v, want %#v", hovered.Color, hoverColor)
	}
}

func TestSharedIconGlyphsUseFixedPainterBounds(t *testing.T) {
	color := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	glyphs := []woxwidget.Widget{
		ChevronGlyph(16, color, false),
		ChevronGlyph(16, color, true),
		CopyGlyph(14, color),
		EditGlyph(14, color),
		RefreshGlyph(14, color),
		DebugGlyph(16, color),
	}
	for index, glyph := range glyphs {
		painter, ok := glyph.(woxwidget.Painter)
		if !ok || painter.Width <= 0 || painter.Height <= 0 || painter.Paint == nil {
			t.Fatalf("glyph %d = %#v", index, glyph)
		}
	}
}
