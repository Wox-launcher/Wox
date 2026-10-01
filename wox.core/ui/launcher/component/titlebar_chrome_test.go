package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWindowCloseChromeUsesSharedWindowsGeometry(t *testing.T) {
	props := WindowCloseChromeProps{ID: "test-close", Width: 420, Platform: "windows", Theme: ControlTheme{ChromeText: woxui.Color{A: 255}}}
	chrome := (&windowCloseChromeState{}).Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	if chrome.Width != 420 || chrome.Height != TitleBarHeight || len(chrome.Children) != 1 || !chrome.Children[0].AnchorRight {
		t.Fatalf("Windows close chrome = %#v, want one right-aligned 420x40 control", chrome)
	}
	button := chrome.Children[0].Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if button.Width != TitleBarControlWidth || button.Height != TitleBarHeight {
		t.Fatalf("Windows close geometry = %vx%v, want 46x40", button.Width, button.Height)
	}
}

func TestWindowCloseChromeWindowsIncludesMinimizeAndMaximize(t *testing.T) {
	props := WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: 420, Platform: "windows", Theme: ControlTheme{ChromeText: woxui.Color{A: 255}},
		OnMinimize: func() {}, OnMaximize: func() {}, OnClose: func() {},
	}
	chrome := (&windowCloseChromeState{}).Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	if len(chrome.Children) != 3 {
		t.Fatalf("Windows caption child count = %d, want close, maximize, and minimize", len(chrome.Children))
	}
	ids := []string{
		chrome.Children[0].Child.(woxwidget.Gesture).ID,
		chrome.Children[1].Child.(woxwidget.Gesture).ID,
		chrome.Children[2].Child.(woxwidget.Gesture).ID,
	}
	if ids[0] != "notes.toolbar.close" || ids[1] != "notes.toolbar.maximize" || ids[2] != "notes.toolbar.minimize" {
		t.Fatalf("Windows caption ids = %#v, want close/maximize/minimize", ids)
	}
	if !chrome.Children[1].AnchorRight || chrome.Children[1].Right != TitleBarControlWidth || chrome.Children[2].Right != TitleBarControlWidth*2 {
		t.Fatalf("Windows caption anchors = %#v, want maximize at 46 and minimize at 92", chrome.Children)
	}
}

func TestWindowCloseChromeMacLeavesCaptionsToAppKit(t *testing.T) {
	props := WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: 420, Platform: "darwin",
		OnMinimize: func() {}, OnMaximize: func() {}, OnClose: func() {},
	}
	chrome := (&windowCloseChromeState{}).Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	if chrome.Width != 420 || chrome.Height != TitleBarHeight || len(chrome.Children) != 0 {
		t.Fatalf("macOS chrome = %#v, want an empty title-bar footprint for native captions", chrome)
	}
}

func TestWindowCloseChromeWindowsCentersCaptionIcons(t *testing.T) {
	props := WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: 420, Platform: "windows", Theme: ControlTheme{ChromeText: woxui.Color{A: 255}},
		OnMinimize: func() {}, OnMaximize: func() {}, OnClose: func() {},
	}
	chrome := (&windowCloseChromeState{}).Build(woxwidget.StateContext{}, props).(woxwidget.Stack)
	for _, child := range chrome.Children {
		align := windowsCaptionAlign(t, child.Child)
		if align.Horizontal != 0.5 || align.Vertical != 0.5 {
			t.Fatalf("Windows caption align = %#v, want centered in the 40-unit title bar", align)
		}
		icon, ok := align.Child.(woxwidget.Image)
		if !ok || icon.Width != TitleBarWindowsIconSize || icon.Height != TitleBarWindowsIconSize {
			t.Fatalf("Windows caption icon = %#v, want a 12-unit SVG instead of a font glyph", align.Child)
		}
	}
}

func TestWindowCloseChromeWindowsUsesRestoreGlyphWhenMaximized(t *testing.T) {
	theme := ControlTheme{ChromeText: woxui.Color{A: 255}}
	normal := windowsCaptionAlign(t, WindowsTitleBarButton("notes.toolbar.maximize", "maximize", false, theme, func() {}, nil)).Child.(woxwidget.Image)
	restored := windowsCaptionAlign(t, WindowsTitleBarButton("notes.toolbar.maximize", "restore", false, theme, func() {}, nil)).Child.(woxwidget.Image)
	if normal.Source == nil || restored.Source == nil || normal.Source.ID() == restored.Source.ID() {
		t.Fatal("maximized caption button must switch to the overlapping restore squares")
	}
	chrome := (&windowCloseChromeState{}).Build(woxwidget.StateContext{}, WindowCloseChromeProps{
		ID: "notes.toolbar.close", Width: 420, Platform: "windows", Theme: theme, Maximized: true,
		OnMinimize: func() {}, OnMaximize: func() {}, OnClose: func() {},
	}).(woxwidget.Stack)
	icon := windowsCaptionAlign(t, chrome.Children[1].Child).Child.(woxwidget.Image)
	if icon.Source.ID() != restored.Source.ID() {
		t.Fatal("maximized Windows chrome should use the restore glyph, not the single maximize square")
	}
}

func windowsCaptionAlign(t *testing.T, control woxwidget.Widget) woxwidget.Align {
	t.Helper()
	gesture, ok := control.(woxwidget.Gesture)
	if !ok {
		t.Fatalf("caption control = %#v, want gesture", control)
	}
	return gesture.Child.(woxwidget.Container).Child.(woxwidget.Align)
}

func TestTitleBarChromeWidthReservesTrailingCaptionButtons(t *testing.T) {
	if got := TitleBarChromeWidth("windows", true, true); got != TitleBarControlWidth*3 {
		t.Fatalf("Windows chrome width = %.0f, want 138", got)
	}
	if got := TitleBarChromeWidth("darwin", true, true); got != 0 {
		t.Fatalf("macOS chrome width = %.0f, want 0 because traffic lights sit on the left", got)
	}
}
