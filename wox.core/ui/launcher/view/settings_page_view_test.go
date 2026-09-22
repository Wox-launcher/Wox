package view

import (
	"strings"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingsPageUsesSymmetricHorizontalInsets(t *testing.T) {
	page := SettingsPage(SettingsPageProps{Width: 1000, Height: 700}).(woxwidget.Container)
	if page.Padding.Left != 40 || page.Padding.Right != 8 || SettingsPageContentWidth(1000) != float32(920) {
		t.Fatalf("settings page geometry = padding %.0f/%.0f content %.0f, want 40/8/%.0f", page.Padding.Left, page.Padding.Right, SettingsPageContentWidth(1000), float32(920))
	}
	scroll := page.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if scroll.Width != float32(952) {
		t.Fatalf("settings page scroll width = %.0f, want the form column %.0f", scroll.Width, float32(920))
	}
}

func TestSettingsPageKeepsWideDashboardsOnTheFullInsetWidth(t *testing.T) {
	if SettingsPageWideContentWidth(1000) != 920 {
		t.Fatalf("wide settings content = %.0f, want 920", SettingsPageWideContentWidth(1000))
	}
	page := SettingsPage(SettingsPageProps{Width: 1000, Height: 700, Wide: true}).(woxwidget.Container)
	scroll := page.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if scroll.Width != 952 {
		t.Fatalf("wide settings scroll width = %.0f, want 952", scroll.Width)
	}
}

func TestSettingsPageFormColumnUsesAvailableWidthWhenNarrow(t *testing.T) {
	if got := SettingsPageContentWidth(600); got != 520 {
		t.Fatalf("narrow settings form width = %.0f, want 520", got)
	}
}

func TestSettingsPageDefaultWindowLeavesNoFormMargin(t *testing.T) {
	page := SettingsPage(SettingsPageProps{
		Width: woxcomponent.SettingsWindowWidth - woxcomponent.SettingsRailWidth(woxcomponent.SettingsWindowWidth), Height: 700,
	}).(woxwidget.Container)
	scroll := page.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if scroll.Width != woxcomponent.SettingsWindowWidth-woxcomponent.SettingsRailMaxWidth-woxcomponent.SettingsPageHorizontalInset-8 {
		t.Fatalf("default settings scroll = %.0f, want the form column to fill the page", scroll.Width)
	}
}

func TestSettingRowAlignsFirstLabelWithInlineTableTitle(t *testing.T) {
	row := SettingRow(SettingRowProps{Title: "Enable proxy", Width: 800, Kind: "bool"}).(woxwidget.Container)
	label := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)

	if top := row.Padding.Top + label.Padding.Top; top != 6 {
		t.Fatalf("setting label top offset = %v, want 6", top)
	}
}

func TestSettingRowSwitchUsesTheSameTrailingValueSlotAsDropdown(t *testing.T) {
	row := SettingRow(SettingRowProps{Title: "Enable proxy", Width: 800, Kind: "bool"}).(woxwidget.Container)
	value := row.Child.(woxwidget.Flex).Children[1].(woxwidget.Align)
	if value.Width != woxcomponent.SettingsChoiceControlWidth || value.Horizontal != 1 || value.Vertical != 0.5 {
		t.Fatalf("switch slot = width %.0f alignment %.1f/%.1f, want dropdown-aligned %.0f/1/0.5", value.Width, value.Horizontal, value.Vertical, woxcomponent.SettingsChoiceControlWidth)
	}
}

func TestSettingRowDropdownUsesThemeTextColor(t *testing.T) {
	want := woxui.Color{R: 12, G: 34, B: 56, A: 255}
	row := SettingRow(SettingRowProps{
		ID: "LaunchMode", Title: "Launch mode", Value: "Continue", Width: 800,
		Theme: woxcomponent.ControlTheme{Text: want, TextSecondary: woxui.Color{R: 255, A: 255}},
	}).(woxwidget.Container)
	field := focusedControlGesture(row.Child.(woxwidget.Flex).Children[1].(woxwidget.Keyed).Child).Child.(woxwidget.Container)
	value := field.Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.TextBlock)

	if value.Color != want {
		t.Fatalf("dropdown value color = %#v, want theme result title %#v", value.Color, want)
	}
	wantBorder := want
	wantBorder.A = 80
	if field.BorderColor != wantBorder {
		t.Fatalf("dropdown outline = %#v, want value text %#v", field.BorderColor, wantBorder)
	}
	if value.Height != 18 || value.LineHeight != 18 {
		t.Fatalf("dropdown value slot = height %v line height %v, want 18/18", value.Height, value.LineHeight)
	}
}

// TestSettingRowWrapsHelpWithoutOverlappingNextRow covers translated text at mixed display scales.
func TestSettingRowWrapsHelpWithoutOverlappingNextRow(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		for _, width := range []float32{520, 700} {
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
				return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
					woxwidget.Keyed{Key: "long", Child: SettingRow(SettingRowProps{ID: "long", Title: "Input method", Description: strings.Repeat("Long translated description ", 16), Width: width, Kind: "bool"})},
					woxwidget.Keyed{Key: "next", Child: SettingRow(SettingRowProps{ID: "next", Title: "Next", Width: width, Kind: "bool"})},
				}}
			})
			host.AttachServices(actionSearchHostServices{})
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 1000}, PixelSize: woxui.PixelSize{Width: int(width * scale), Height: int(1000 * scale)}, Scale: scale})
			first, ok := host.BoundsForKey("long")
			next, nextOK := host.BoundsForKey("next")
			if !ok || !nextOK || first.Height <= woxcomponent.SettingsRowHeight || next.Y < first.Y+first.Height {
				t.Fatalf("width %v scale %v: wrapped row %+v overlaps next row %+v or failed to grow", width, scale, first, next)
			}
			host.Dispose()
		}
	}
}

// TestSettingsScrollbarUsesRightGutter preserves content width across window sizes.
func TestSettingsScrollbarUsesRightGutter(t *testing.T) {
	for _, width := range []float32{600, 880, 1100} {
		page := SettingsPage(SettingsPageProps{Width: width, Height: 700}).(woxwidget.Container)
		scroll := page.Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		content := scroll.Content.(woxwidget.Container)
		if content.Width != width-80 || scroll.Width-content.Width != 32 || page.Padding.Left+scroll.Width != width-8 {
			t.Fatalf("width %v: content %v, scroll %v, padding %+v", width, content.Width, scroll.Width, page.Padding)
		}
	}
}
