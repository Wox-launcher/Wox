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

func TestSettingsWindowWindowsSharesPageGutterWithTitleBar(t *testing.T) {
	window := SettingsWindow(SettingsWindowProps{
		Width: 1200, Height: 800, PageID: "ui", Platform: "windows", RailWidth: 240,
		TitleBar: woxwidget.Container{Width: 1200, Height: SettingsTitleBarHeight},
		Rail:     woxwidget.Container{Width: 240, Height: 760},
		Page:     woxwidget.Container{Width: 960, Height: 780},
	})

	root := window.(woxwidget.Semantics).Child.(woxwidget.Container).Child.(woxwidget.Stack)
	layout := root.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Stack)
	page := layout.Children[0]
	if page.Left != 240 || page.Top != 20 || page.Top+page.Child.(woxwidget.Semantics).Child.(woxwidget.Container).Height != 800 {
		t.Fatalf("Windows page frame = %#v, want top 20 and bottom at window edge", page)
	}
	if layout.Children[1].Top != SettingsTitleBarHeight || layout.Children[2].Top != 0 {
		t.Fatal("Windows rail and caption controls must retain their vertical positions")
	}
	// Catalogs have the smallest top inset; their first control must clear the draggable caption row.
	catalog := PluginSettingsPage(PluginSettingsPageProps{Width: 960, Height: 780}).(woxwidget.Container)
	if page.Top+catalog.Padding.Top < SettingsTitleBarHeight {
		t.Fatal("Windows catalog controls overlap the draggable title bar")
	}
}

func TestSettingsWindowLinuxSharesPageGutterWithTitleBar(t *testing.T) {
	window := SettingsWindow(SettingsWindowProps{
		Width: 1200, Height: 800, PageID: "ui", Platform: "linux", RailWidth: 240,
		TitleBar: woxwidget.Container{Width: 1200, Height: SettingsTitleBarHeight},
		Rail:     woxwidget.Container{Width: 240, Height: 760},
		Page:     woxwidget.Container{Width: 960, Height: 780},
	})

	root := window.(woxwidget.Semantics).Child.(woxwidget.Container).Child.(woxwidget.Stack)
	layout := root.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Stack)
	page := layout.Children[0]
	if page.Left != 240 || page.Top != 20 || page.Top+page.Child.(woxwidget.Semantics).Child.(woxwidget.Container).Height != 800 {
		t.Fatalf("Linux page frame = %#v, want top 20 and bottom at window edge", page)
	}
	if layout.Children[1].Top != SettingsTitleBarHeight || layout.Children[2].Top != 0 {
		t.Fatal("Linux rail stays below the caption while the page gutter shares that row")
	}
}

func TestSettingsTitleBarMacLimitsDragAreaToRail(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, RailWidth: 240, Platform: "darwin"}, "", nil).(woxwidget.Stack)
	drag := titleBar.Children[1].Child.(woxwidget.Gesture)
	if width := drag.Child.(woxwidget.Container).Width; width != 240 {
		t.Fatalf("macOS title-bar drag width = %v, want rail width 240", width)
	}
}

func TestSettingsTitleBarLinuxContinuesRailAndPlacesTitleBesideIcon(t *testing.T) {
	theme := woxcomponent.ControlTheme{TextSecondary: woxui.Color{R: 168, G: 168, B: 179, A: 255}}
	icon := &woxui.Image{Width: 32, Height: 32}
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{
		Width: 1200, RailWidth: 240, Title: "Wox Settings", TitleWidth: 160, Platform: "linux", AppIcon: icon, Theme: theme,
	}, "", nil).(woxwidget.Stack)

	divider := titleBar.Children[1]
	line := divider.Child.(woxwidget.Container)
	if divider.Left != 239 || line.Width != 1 || line.Height != SettingsTitleBarHeight || line.Color != settingsColorAlpha(theme.TextSecondary, 26) {
		t.Fatalf("Linux title-bar rail divider = %#v, want continuation at the rail edge", divider)
	}
	iconSlot := titleBar.Children[2]
	iconAlign := iconSlot.Child.(woxwidget.Align)
	iconImage := iconAlign.Child.(woxwidget.Image)
	if iconSlot.Left != 12 || iconAlign.Width != 20 || iconAlign.Height != SettingsTitleBarHeight || iconAlign.Vertical != 0.5 || iconImage.Source != icon || iconImage.Width != 20 || iconImage.Height != 20 {
		t.Fatalf("Linux title-bar icon = %#v, want a 20px mark at the left of the rail column", iconSlot)
	}
	title := titleBar.Children[3]
	titleAlignment, ok := title.Child.(woxwidget.Align)
	if !ok || title.Left != 40 || title.Right != 46 || !title.StretchWidth || titleAlignment.Height != SettingsTitleBarHeight || titleAlignment.Vertical != 0.5 {
		t.Fatalf("Linux title = %#v, want the caption beside the icon and clear of the close control", title)
	}
	closeButton := titleBar.Children[4]
	if !closeButton.AnchorRight {
		t.Fatal("Linux close control must stay anchored to the right")
	}
	for _, child := range titleBar.Children {
		if fill, ok := child.Child.(woxwidget.Container); ok && fill.Width == 240 && fill.Height == SettingsTitleBarHeight && fill.Color.A != 0 {
			t.Fatal("Linux title bar must not tint the rail column")
		}
	}
}

func TestSettingsTitleBarLinuxCloseHoverUsesDangerHighlight(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Title: "Wox Settings", TitleWidth: 160, Platform: "linux"}, "close", nil).(woxwidget.Stack)
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
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "windows", CloseOnly: true}, "", nil).(woxwidget.Stack)
	if len(titleBar.Children) != 3 {
		t.Fatalf("close-only Windows title-bar child count = %d, want drag, title, and close", len(titleBar.Children))
	}
	closeButton, ok := titleBar.Children[2].Child.(woxwidget.Gesture)
	if !ok || closeButton.ID != "settings-window-close" {
		t.Fatalf("close-only Windows last control = %#v, want close button", titleBar.Children[2].Child)
	}
}

func TestSettingsTitleBarCustomContentKeepsPlatformControlsClear(t *testing.T) {
	content := woxwidget.Container{Width: 454, Height: SettingsTitleBarHeight}
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{
		Width: 500, Platform: "windows", CloseOnly: true, Content: content,
	}, "", nil).(woxwidget.Stack)

	clip, ok := titleBar.Children[1].Child.(woxwidget.Clip)
	if !ok || titleBar.Children[1].Left != 0 || clip.Width != 454 || clip.Child != content {
		t.Fatalf("windows custom title content = %#v, want 454-wide content before close control", titleBar.Children[1])
	}

	left, width := TitleBarContentFrame("darwin", true, 500)
	if left != 44 || width != 456 {
		t.Fatalf("macOS custom title content frame = %.0f/%.0f, want 44/456", left, width)
	}
}

func TestSettingsTitleBarWindowsUsesInsetStretchAndRightAnchors(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, Platform: "windows"}, "", nil).(woxwidget.Stack)
	title := titleBar.Children[1]
	minimize := titleBar.Children[2]
	closeButton := titleBar.Children[3]

	if title.Left != 40 || title.Right != 92 || !title.StretchWidth {
		t.Fatalf("Windows title slot = left %.0f right %.0f stretch %v, want 40/92/true", title.Left, title.Right, title.StretchWidth)
	}
	titleAlignment, ok := title.Child.(woxwidget.Align)
	if !ok || title.Top != 0 || titleAlignment.Height != SettingsTitleBarHeight || titleAlignment.Vertical != 0.5 {
		t.Fatalf("Windows title alignment = top %.0f child %#v, want full-height vertical center", title.Top, title.Child)
	}
	if !minimize.AnchorRight || minimize.Right != 46 || !closeButton.AnchorRight {
		t.Fatalf("Windows chrome anchors = minimize %v/%.0f close %v, want true/46 true", minimize.AnchorRight, minimize.Right, closeButton.AnchorRight)
	}
}

func TestSettingsTitleBarWindowsContinuesRailWithoutHorizontalDivider(t *testing.T) {
	theme := woxcomponent.ControlTheme{TextSecondary: woxui.Color{R: 168, G: 168, B: 179, A: 255}}
	for _, closeOnly := range []bool{false, true} {
		titleBar := buildSettingsTitleBar(SettingsTitleBarProps{
			Width: 1200, RailWidth: 240, Platform: "windows", CloseOnly: closeOnly, Theme: theme,
		}, "", nil).(woxwidget.Stack)
		tint := titleBar.Children[0].Child.(woxwidget.Container)
		if tint.Width != 240 || tint.Height != SettingsTitleBarHeight || tint.Color != settingsRailBackground(theme, false) {
			t.Fatalf("Windows title-bar rail tint = %#v, want full-height matching rail surface", tint)
		}
		drag := titleBar.Children[1].Child.(woxwidget.Gesture)
		if drag.Child.(woxwidget.Container).Width != 1200 {
			t.Fatal("Windows title bar must retain full-width dragging")
		}
		divider := titleBar.Children[2]
		line := divider.Child.(woxwidget.Container)
		if divider.Left != 239 || line.Width != 1 || line.Height != SettingsTitleBarHeight || line.Color != settingsColorAlpha(theme.TextSecondary, 26) {
			t.Fatalf("Windows title-bar rail divider = %#v, want continuation at rail edge", divider)
		}
		for _, child := range titleBar.Children {
			if line, ok := child.Child.(woxwidget.Container); ok && line.Height == 1 && child.StretchWidth {
				t.Fatal("Windows title bar must blend into the content without a horizontal divider")
			}
		}
	}
}

func TestSettingsTitleBarMacLeavesCaptionsToAppKit(t *testing.T) {
	for _, active := range []bool{false, true} {
		titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, RailWidth: 240, Platform: "darwin", Active: active}, "", nil).(woxwidget.Stack)
		if len(titleBar.Children) != 3 {
			t.Fatalf("macOS title bar = %#v, want rail tint, drag area, and divider without simulated captions", titleBar)
		}
	}
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
