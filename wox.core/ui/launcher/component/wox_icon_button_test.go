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

func TestWoxIconButtonDisabledSkipsHoverAndTap(t *testing.T) {
	tapped := false
	props := IconButtonProps{
		ID: "locked", Label: "Edit", Icon: woxwidget.Painter{Width: 16, Height: 16},
		Width: 26, Height: 24, HoverBackground: woxui.Color{A: 40}, Disabled: true,
		OnTap: func() { tapped = true },
	}
	state := &iconButtonState{hovered: true}
	built := state.Build(woxwidget.StateContext{}, props).(woxwidget.Semantics)
	if !built.Disabled || len(built.Actions) != 0 {
		t.Fatal("disabled icon button must expose disabled semantics without activation")
	}
	gesture := built.Child.(woxwidget.Focusable).Child.(woxwidget.Gesture)
	if gesture.OnTap != nil {
		t.Fatal("disabled icon button must not keep a tap handler")
	}
	if background := gesture.Child.(woxwidget.Container).Color; background.A != 0 {
		t.Fatalf("disabled hover background = %#v, want no hover fill", background)
	}
	if tapped {
		t.Fatal("disabled icon button must not invoke OnTap")
	}
}

func TestWoxIconButtonSelectedKeepsActiveBackground(t *testing.T) {
	selectedColor := woxui.Color{R: 20, G: 30, B: 40, A: 40}
	props := IconButtonProps{ID: "underline", Label: "Underline", Icon: woxwidget.Painter{Width: 16, Height: 16}, Width: 28, Height: 28, Radius: 6, Selected: true, SelectedBackground: selectedColor}
	built := (&iconButtonState{}).Build(woxwidget.StateContext{}, props).(woxwidget.Semantics)
	if !built.Selected {
		t.Fatal("selected icon button must expose selected semantics")
	}
	background := built.Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Color
	if background != selectedColor {
		t.Fatalf("selected background = %#v, want %#v", background, selectedColor)
	}
}

func TestSharedIconGlyphsUseSVGImages(t *testing.T) {
	color := woxui.Color{R: 10, G: 20, B: 30, A: 255}
	glyphs := []woxwidget.Widget{
		CloseGlyph(16, color),
		SearchGlyph(18, color),
		PinGlyph(15, color),
		AddGlyph(18, color),
		DeleteGlyph(15, color),
		ChatBubbleGlyph(22, color),
		MenuGlyph(18, color),
		SidebarGlyph(18, color),
		ChevronGlyph(16, color, false),
		ChevronGlyph(16, color, true),
		CopyGlyph(14, color),
		EditGlyph(14, color),
		RefreshGlyph(14, color),
		DebugGlyph(16, color),
		ClockGlyph(16, color),
		FilterListGlyph(15, color),
		CheckGlyph(12, color),
		CheckCircleGlyph(14, color),
		ErrorGlyph(14, color),
		ToolGlyph(16, color),
		ArticleGlyph(16, color),
		TerminalGlyph(16, color),
		PlayArrowGlyph(14, color),
		HourglassGlyph(14, color),
		ModelTrainingGlyph(18, color),
		KeyboardArrowDownGlyph(14, color),
		KeyboardArrowRightGlyph(16, color),
		ExtensionGlyph(18, color),
		FormatGlyph("block", 16, color),
		FormatGlyph("bold", 16, color),
		FormatGlyph("link", 16, color),
		FormatGlyph("table-insert-row", 16, color),
		FormatGlyph("table-delete", 16, color),
		FormatGlyph("more", 16, color),
	}
	for index, glyph := range glyphs {
		image, ok := glyph.(woxwidget.Image)
		if !ok || image.Source == nil || image.Width <= 0 || image.Height <= 0 {
			t.Fatalf("glyph %d = %#v", index, glyph)
		}
	}
}
