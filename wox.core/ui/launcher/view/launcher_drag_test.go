package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLauncherResultRowWiresNativeDragStart(t *testing.T) {
	dragged := false
	row := launcherResultRow(launcherResultRowProps{
		Item:     LauncherResultItem{ID: "result", Title: "Report", OnDragStart: func() { dragged = true }},
		RowWidth: 320, RowHeight: 50, InnerRowWidth: 320, BaseHeight: 50, IconSize: 28, IconGap: 10,
		Theme: woxcomponent.Theme{}, TitleStyle: woxui.TextStyle{Size: 14}, SubtitleStyle: woxui.TextStyle{Size: 12},
	})
	semantics := row.(woxwidget.Semantics)
	gesture := semantics.Child.(woxwidget.Gesture)
	gesture.OnDragStart()
	if !dragged {
		t.Fatal("launcher result row did not forward drag start")
	}
}

func TestLauncherGridResultWiresNativeDragStart(t *testing.T) {
	dragged := false
	cell := launcherGridResultView(LauncherGridResult{ID: "result", Title: "Report", OnDragStart: func() { dragged = true }}, LauncherGridProps{
		Width: 320, Height: 100, CellWidth: 80, CellHeight: 80, VisualWidth: 60, VisualHeight: 60, Columns: 4,
		Theme: woxcomponent.Theme{}, DensityScale: 1,
	})
	gesture := cell.(woxwidget.Gesture)
	gesture.OnDragStart()
	if !dragged {
		t.Fatal("launcher grid cell did not forward drag start")
	}
}
