package component

import (
	"testing"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestHotkeyThemeColors checks actual glyph, fill and stroke, including a transparent border.
func TestHotkeyThemeColors(t *testing.T) {
	foreground := woxui.Color{R: 12, G: 34, B: 56, A: 100}
	background := woxui.Color{R: 56, G: 34, B: 12, A: 80}
	transparent := woxui.Color{}
	for _, custom := range []bool{false, true} {
		theme := Theme{}
		if custom {
			theme.ToolbarHotkeyFontColor, theme.ToolbarHotkeyBackgroundColor, theme.ToolbarHotkeyBorderColor = &foreground, &background, &transparent
		}
		props := HotkeyProps{Toolbar: true, Theme: &theme, Labels: []string{"Enter"}, Foreground: woxui.Color{A: 255}, Window: &woxui.Window{}}
		built, _ := WoxHotkey(props)
		key := built.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
		fill := key.Children[0].Child.(woxwidget.Container)
		label := key.Children[2].Child.(woxwidget.Text)
		wantText, wantFill, wantBorder := props.Foreground, props.Background, props.Foreground
		if custom {
			wantText, wantFill, wantBorder = foreground, background, transparent
		}
		if fill.Color != wantFill || label.Color != wantText {
			t.Fatal("keycap colors did not reach rendering")
		}
		actual, expected := &woxui.DisplayList{}, &woxui.DisplayList{}
		bounds := woxui.Rect{Width: key.Width, Height: key.Height}
		key.Children[1].Child.(woxwidget.Painter).Paint(actual, bounds)
		expected.StrokeRoundedRect(bounds, 4, 1, wantBorder)
		if err := actual.Compare(expected); err != nil {
			t.Fatal(err)
		}
	}
}

// TestThemeDetailFallbacks separates absence from transparent and preserves v1 behavior.
func TestThemeDetailFallbacks(t *testing.T) {
	theme := Theme{SelectedBackground: woxui.Color{R: 12, A: 200}, PreviewSplit: woxui.Color{G: 12, A: 100}}
	if theme.ResultHoverColor().A != 50 || theme.ActionDividerColor() != theme.PreviewSplit {
		t.Fatal("legacy fallback changed")
	}
	transparent := woxui.Color{}
	theme.ResultItemHoverBackgroundColor, theme.ActionContainerDividerColor = &transparent, &transparent
	if theme.ResultHoverColor() != transparent || theme.ActionDividerColor() != transparent {
		t.Fatal("transparent override treated as absent")
	}
}

// TestSelectedHotkeyKeepsSurfaceColors prevents dark light-theme keycaps on a blue selected row.
func TestSelectedHotkeyKeepsSurfaceColors(t *testing.T) {
	dark, transparent := woxui.Color{R: 69, G: 69, B: 69, A: 255}, woxui.Color{}
	white, blue := woxui.Color{R: 255, G: 255, B: 255, A: 255}, woxui.Color{R: 57, G: 105, B: 164, A: 255}
	theme := Theme{ToolbarHotkeyFontColor: &dark, ToolbarHotkeyBorderColor: &dark, ToolbarHotkeyBackgroundColor: &transparent}
	built, _ := WoxHotkey(HotkeyProps{Theme: &theme, Selected: true, Labels: []string{"Enter"}, Foreground: white, Background: blue, Window: &woxui.Window{}})
	key := built.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	if key.Children[0].Child.(woxwidget.Container).Color != blue || key.Children[2].Child.(woxwidget.Text).Color != white {
		t.Fatal("normal colors overrode selected keycap")
	}
	actual, expected := &woxui.DisplayList{}, &woxui.DisplayList{}
	bounds := woxui.Rect{Width: key.Width, Height: key.Height}
	key.Children[1].Child.(woxwidget.Painter).Paint(actual, bounds)
	expected.StrokeRoundedRect(bounds, 4, 1, white)
	if err := actual.Compare(expected); err != nil {
		t.Fatal(err)
	}
}

// TestHotkeySurfaceStates isolates all three palettes, including transparent selected borders.
func TestHotkeySurfaceStates(t *testing.T) {
	toolbar, normal, active := woxui.Color{R: 100, A: 255}, woxui.Color{G: 100, A: 255}, woxui.Color{B: 100, A: 255}
	transparent := woxui.Color{}
	theme := Theme{ToolbarHotkeyFontColor: &toolbar, ToolbarHotkeyBackgroundColor: &toolbar, ToolbarHotkeyBorderColor: &toolbar, ActionItemHotkeyFontColor: &normal, ActionItemHotkeyBackgroundColor: &normal, ActionItemHotkeyBorderColor: &normal, ActionItemActiveHotkeyFontColor: &active, ActionItemActiveHotkeyBackgroundColor: &active, ActionItemActiveHotkeyBorderColor: &transparent}
	for _, tc := range []struct {
		toolbar, selected bool
		color, border     woxui.Color
	}{{true, false, toolbar, toolbar}, {false, false, normal, normal}, {false, true, active, transparent}} {
		built, _ := WoxHotkey(HotkeyProps{Theme: &theme, Toolbar: tc.toolbar, Selected: tc.selected, Labels: []string{"Enter"}, Window: &woxui.Window{}})
		cap := built.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
		if cap.Children[0].Child.(woxwidget.Container).Color != tc.color || cap.Children[2].Child.(woxwidget.Text).Color != tc.color {
			t.Fatal("keycap state palette leaked")
		}
		actual, expected := &woxui.DisplayList{}, &woxui.DisplayList{}
		bounds := woxui.Rect{Width: cap.Width, Height: cap.Height}
		cap.Children[1].Child.(woxwidget.Painter).Paint(actual, bounds)
		expected.StrokeRoundedRect(bounds, 4, 1, tc.border)
		if err := actual.Compare(expected); err != nil {
			t.Fatal(err)
		}
	}
}
