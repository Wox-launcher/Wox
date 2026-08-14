package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestSettingRowAlignsFirstLabelWithInlineTableTitle(t *testing.T) {
	row := SettingRow(SettingRowProps{Title: "Enable proxy", Width: 800, Kind: "bool"}).(woxwidget.Container)
	label := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)

	if top := row.Padding.Top + label.Padding.Top; top != 6 {
		t.Fatalf("setting label top offset = %v, want 6", top)
	}
}

func TestSettingRowDropdownUsesThemeTextColor(t *testing.T) {
	want := woxui.Color{R: 12, G: 34, B: 56, A: 255}
	row := SettingRow(SettingRowProps{ID: "LaunchMode", Title: "Launch mode", Value: "Continue", Width: 800, Theme: woxcomponent.Theme{ResultTitle: want}}).(woxwidget.Container)
	field := focusedControlGesture(row.Child.(woxwidget.Flex).Children[1].(woxwidget.Keyed).Child).Child.(woxwidget.Container)
	value := field.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.TextBlock)

	if value.Color != want {
		t.Fatalf("dropdown value color = %#v, want theme result title %#v", value.Color, want)
	}
	if value.Height != 18 || value.LineHeight != 18 {
		t.Fatalf("dropdown value slot = height %v line height %v, want 18/18", value.Height, value.LineHeight)
	}
}
