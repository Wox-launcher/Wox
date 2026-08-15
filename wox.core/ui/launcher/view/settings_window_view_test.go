package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type settingsWindowHostServices struct{}

func (settingsWindowHostServices) MeasureText(string, woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{}, nil
}
func (settingsWindowHostServices) Invalidate() error               { return nil }
func (settingsWindowHostServices) InvalidateRect(woxui.Rect) error { return nil }
func (settingsWindowHostServices) SetTextInputState(woxui.TextInputState) error {
	return nil
}
func (settingsWindowHostServices) SetPointerCursor(woxui.PointerCursor) error { return nil }
func (settingsWindowHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func TestSettingsWindowMacKeepsTitleBarOutOfPageColumn(t *testing.T) {
	window := SettingsWindow(SettingsWindowProps{
		Width: 1200, Height: 800, PageID: "ui", Platform: "darwin", RailWidth: 240,
		TitleBar: woxwidget.Container{Width: 1200, Height: SettingsTitleBarHeight},
		Rail:     woxwidget.Container{Width: 240, Height: 760},
		Page:     woxwidget.Container{Width: 960, Height: 800},
	})

	root := window.(woxwidget.Semantics).Child.(woxwidget.Container).Child.(woxwidget.Stack)
	body := root.Children[0].Child.(woxwidget.Container)
	layout := body.Child.(woxwidget.Stack)
	if len(layout.Children) != 3 {
		t.Fatalf("macOS settings child count = %d, want page, rail, and title bar", len(layout.Children))
	}
	if page := layout.Children[0]; page.Left != 240 || page.Top != 0 {
		t.Fatalf("macOS page position = (%v, %v), want (240, 0)", page.Left, page.Top)
	}
	if rail := layout.Children[1]; rail.Left != 0 || rail.Top != SettingsTitleBarHeight {
		t.Fatalf("macOS rail position = (%v, %v), want (0, %v)", rail.Left, rail.Top, SettingsTitleBarHeight)
	}
}

func TestSettingsWindowWindowsRetainsFullWidthTitleBarRow(t *testing.T) {
	window := SettingsWindow(SettingsWindowProps{
		Width: 1200, Height: 800, PageID: "ui", Platform: "windows", RailWidth: 240,
		TitleBar: woxwidget.Container{Width: 1200, Height: SettingsTitleBarHeight},
		Rail:     woxwidget.Container{Width: 240, Height: 760},
		Page:     woxwidget.Container{Width: 960, Height: 760},
	})

	root := window.(woxwidget.Semantics).Child.(woxwidget.Container).Child.(woxwidget.Stack)
	body := root.Children[0].Child.(woxwidget.Container)
	layout, ok := body.Child.(woxwidget.Flex)
	if !ok {
		t.Fatalf("Windows settings layout type = %T, want woxwidget.Flex", body.Child)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("Windows settings row count = %d, want title bar and content", len(layout.Children))
	}
}

func TestSettingsTitleBarMacLimitsDragAreaToRail(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, RailWidth: 240, Platform: "darwin"}, "", "", nil, nil).(woxwidget.Stack)
	drag := titleBar.Children[1].Child.(woxwidget.Gesture)
	if width := drag.Child.(woxwidget.Container).Width; width != 240 {
		t.Fatalf("macOS title-bar drag width = %v, want rail width 240", width)
	}
}

func TestSettingsMacCloseUsesPixelPainter(t *testing.T) {
	closeControl := woxcomponent.MacTrafficLight("close", woxui.Color{}, "×", woxui.Color{}, true, false, true, woxcomponent.Theme{}, func() {}, nil, nil).(woxwidget.Gesture)
	symbol := closeControl.Child.(woxwidget.Align).Child.(woxwidget.Container).Child.(woxwidget.Align).Child
	painter, ok := symbol.(woxwidget.Painter)
	if !ok || painter.Width != 14 || painter.Height != 14 || painter.Paint == nil {
		t.Fatalf("macOS close symbol = %#v, want 14x14 pixel painter", symbol)
	}
}

func TestSettingsMacTrafficLightDarkensItsNativeColorWhilePressed(t *testing.T) {
	glyphColor := woxui.Color{R: 126, G: 100, B: 11, A: 255}
	control := woxcomponent.MacTrafficLight("minimize", woxui.Color{R: 250, G: 200, B: 0, A: 255}, "−", glyphColor, true, true, true, woxcomponent.Theme{}, func() {}, nil, nil).(woxwidget.Gesture)
	button := control.Child.(woxwidget.Align).Child.(woxwidget.Container)
	if button.Color != (woxui.Color{R: 215, G: 172, B: 0, A: 255}) {
		t.Fatalf("pressed macOS traffic light color = %#v, want darkened native yellow", button.Color)
	}
	symbol := button.Child.(woxwidget.Align).Child.(woxwidget.Container)
	if symbol.Color != glyphColor {
		t.Fatalf("pressed macOS traffic light glyph = %#v, want unchanged %#v", symbol.Color, glyphColor)
	}
}

func TestSettingsTitleBarLinuxCloseHoverUsesDangerHighlight(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Title: "Wox Settings", TitleWidth: 160, Platform: "linux"}, "close", "", nil, nil).(woxwidget.Stack)
	closeButton := titleBar.Children[2].Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	circle := closeButton.Child.(woxwidget.Align).Child.(woxwidget.Container)
	closeGlyph := circle.Child.(woxwidget.Align).Child.(woxwidget.Image)

	if circle.Width != 24 || circle.Height != 24 || circle.Radius != 12 {
		t.Fatalf("linux close hover geometry = %vx%v radius %v, want 24x24 circle", circle.Width, circle.Height, circle.Radius)
	}
	if closeButton.Color != (woxui.Color{}) {
		t.Fatalf("linux close button container color = %#v, want transparent", closeButton.Color)
	}

	if circle.Color != (woxui.Color{R: 232, G: 17, B: 35, A: 255}) {
		t.Fatalf("linux close button hover circle color = %#v, want danger red highlight", circle.Color)
	}
	if closeGlyph.Width != 16 || closeGlyph.Height != 16 {
		t.Fatalf("linux close glyph size = %vx%v, want 16x16", closeGlyph.Width, closeGlyph.Height)
	}
}

func TestSettingsTitleBarCloseOnlyOmitsWindowsMinimize(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "windows", CloseOnly: true}, "", "", nil, nil).(woxwidget.Stack)
	if len(titleBar.Children) != 4 {
		t.Fatalf("close-only Windows title-bar child count = %d, want drag, title, border, and close", len(titleBar.Children))
	}
	closeButton, ok := titleBar.Children[3].Child.(woxwidget.Gesture)
	if !ok || closeButton.ID != "settings-window-close" {
		t.Fatalf("close-only Windows last control = %#v, want close button", titleBar.Children[3].Child)
	}
}

func TestSettingsTitleBarWindowsUsesInsetStretchAndRightAnchors(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "windows"}, "", "", nil, nil).(woxwidget.Stack)
	title := titleBar.Children[1]
	border := titleBar.Children[2]
	minimize := titleBar.Children[3]
	closeButton := titleBar.Children[4]

	if title.Left != 40 || title.Right != 92 || !title.StretchWidth {
		t.Fatalf("Windows title slot = left %.0f right %.0f stretch %v, want 40/92/true", title.Left, title.Right, title.StretchWidth)
	}
	if !border.StretchWidth || !border.AnchorBottom || !minimize.AnchorRight || minimize.Right != 46 || !closeButton.AnchorRight {
		t.Fatalf("Windows chrome anchors = border %v/%v minimize %v/%.0f close %v, want true/true true/46 true", border.StretchWidth, border.AnchorBottom, minimize.AnchorRight, minimize.Right, closeButton.AnchorRight)
	}
}

func TestSettingsTitleBarMacTrafficLightsGrayOutWhileUnfocused(t *testing.T) {
	theme := woxcomponent.Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}}
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "darwin", Theme: theme, Active: false}, "", "", nil, nil).(woxwidget.Stack)
	if len(titleBar.Children) != 5 {
		t.Fatalf("macOS title-bar child count = %d, want drag, divider, and three traffic lights", len(titleBar.Children))
	}
	inactive := woxcomponent.MacTrafficLightInactiveColor(theme)
	for _, child := range titleBar.Children[2:] {
		fill := macTrafficLightFill(t, child.Child)
		if fill != inactive {
			t.Fatalf("unfocused macOS traffic light = %#v, want inactive gray %#v", fill, inactive)
		}
	}
}

func TestSettingsTitleBarMacTrafficLightsKeepNativeColorsWhileFocused(t *testing.T) {
	theme := woxcomponent.Theme{Background: woxui.Color{R: 24, G: 24, B: 26, A: 255}}
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "darwin", Theme: theme, Active: true}, "", "", nil, nil).(woxwidget.Stack)
	closeFill := macTrafficLightFill(t, titleBar.Children[2].Child)
	minimizeFill := macTrafficLightFill(t, titleBar.Children[3].Child)
	if closeFill != (woxui.Color{R: 255, G: 92, B: 95, A: 255}) {
		t.Fatalf("focused close traffic light = %#v, want native red", closeFill)
	}
	if minimizeFill != (woxui.Color{R: 250, G: 200, B: 0, A: 255}) {
		t.Fatalf("focused minimize traffic light = %#v, want native yellow", minimizeFill)
	}
}

func macTrafficLightFill(t *testing.T, control woxwidget.Widget) woxui.Color {
	t.Helper()
	if gesture, ok := control.(woxwidget.Gesture); ok {
		control = gesture.Child
	}
	align, ok := control.(woxwidget.Align)
	if !ok {
		t.Fatalf("traffic light widget = %T, want Align or Gesture", control)
	}
	return align.Child.(woxwidget.Container).Color
}

func TestSettingsWindowOverlayPreservesHoveredIdentity(t *testing.T) {
	var overlayVisible bool
	var hoverStates []bool
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		var overlay woxwidget.Widget
		if overlayVisible {
			overlay = woxwidget.Container{Width: 80, Height: 40}
		}
		return SettingsWindow(SettingsWindowProps{
			Width: 240, Height: 120, PageID: "hover", Platform: "darwin",
			TitleBar: woxwidget.Painter{Width: 240, Height: SettingsTitleBarHeight},
			Rail:     woxwidget.Painter{},
			Page: woxwidget.Gesture{ID: "hover-anchor", OnHover: func(inside bool) {
				hoverStates = append(hoverStates, inside)
				overlayVisible = inside
			}, Child: woxwidget.Container{Width: 20, Height: 20}},
			Overlay: overlay, OverlayLeft: 100, OverlayTop: 60,
		})
	})
	host.AttachServices(settingsWindowHostServices{})
	defer host.Dispose()

	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 120}}
	var displayList woxui.DisplayList
	host.Frame(&displayList, frame)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 10, Y: 10}})
	host.Frame(&displayList, frame)

	if len(hoverStates) != 1 || !hoverStates[0] {
		t.Fatalf("hover states after showing overlay = %v, want [true]", hoverStates)
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave})
	host.Frame(&displayList, frame)
	if len(hoverStates) != 2 || hoverStates[1] {
		t.Fatalf("hover states after leaving window = %v, want [true false]", hoverStates)
	}
}
