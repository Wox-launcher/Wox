package view

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestResultHoverOverridePreservesSelection verifies list and grid rendering agree on state precedence.
func TestResultHoverOverridePreservesSelection(t *testing.T) {
	active := woxui.Color{R: 10, A: 255}
	for _, hover := range []woxui.Color{{}, {G: 100, A: 90}} {
		for _, selected := range []bool{false, true} {
			theme := woxcomponent.Theme{SelectedBackground: active, ResultItemHoverBackgroundColor: &hover}
			want := hover
			if selected {
				want = active
			}
			row := launcherResultRow(launcherResultRowProps{Item: LauncherResultItem{ID: "hover", Title: "Result", Hovered: true, Selected: selected}, RowWidth: 300, RowHeight: 50, InnerRowWidth: 300, BaseHeight: 50, Theme: theme}).(woxwidget.Semantics)
			background := row.Child.(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Boundary[launcherResultBackgroundProps])
			if background.Build(background.Props).(woxwidget.Container).Color != want {
				t.Fatal("list hover replaced selection or lost transparency")
			}
			cell := launcherGridResultView(LauncherGridResult{ID: "hover", Hovered: true, Selected: selected}, LauncherGridProps{CellWidth: 120, CellHeight: 110, VisualWidth: 100, VisualHeight: 70, Theme: theme}).(woxwidget.Semantics).Child.(woxwidget.Gesture)
			frame := cell.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack).Children[0].Child.(woxwidget.Boundary[launcherGridFrameProps])
			if frame.Build(frame.Props).(woxwidget.Container).BorderColor != want {
				t.Fatal("grid hover replaced selection or lost transparency")
			}
		}
	}
}
