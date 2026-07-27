package view

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestSettingsWindowMacKeepsTitleBarOutOfPageColumn(t *testing.T) {
	window := SettingsWindow(SettingsWindowProps{
		Width: 1200, Height: 800, PageID: "ui", Platform: "darwin", RailWidth: 240,
		TitleBar: woxwidget.Container{Width: 1200, Height: SettingsTitleBarHeight},
		Rail:     woxwidget.Container{Width: 240, Height: 760},
		Page:     woxwidget.Container{Width: 960, Height: 800},
	})

	body := window.(woxwidget.Semantics).Child.(woxwidget.Container)
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

	body := window.(woxwidget.Semantics).Child.(woxwidget.Container)
	layout, ok := body.Child.(woxwidget.Flex)
	if !ok {
		t.Fatalf("Windows settings layout type = %T, want woxwidget.Flex", body.Child)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("Windows settings row count = %d, want title bar and content", len(layout.Children))
	}
}

func TestSettingsTitleBarMacLimitsDragAreaToRail(t *testing.T) {
	titleBar := buildSettingsTitleBar(SettingsTitleBarProps{Width: 1200, RailWidth: 240, Platform: "darwin"}, "", nil).(woxwidget.Stack)
	drag := titleBar.Children[1].Child.(woxwidget.Gesture)
	if width := drag.Child.(woxwidget.Container).Width; width != 240 {
		t.Fatalf("macOS title-bar drag width = %v, want rail width 240", width)
	}
}
