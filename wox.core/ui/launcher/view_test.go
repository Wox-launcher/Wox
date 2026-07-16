package launcher

import (
	"fmt"
	"testing"

	woxwidget "wox/ui/widget"
)

func TestBuildResultsOnlyBuildsViewportRows(t *testing.T) {
	results := make([]queryResult, 241)
	for index := range results {
		results[index] = queryResult{ID: fmt.Sprintf("result-%d", index), Title: fmt.Sprintf("Result %d", index), IsGroup: true}
	}
	app := &App{selected: -1}
	built := app.buildResults(viewSnapshot{results: results, selected: -1}, 760, 500)
	semantics := built.(woxwidget.Semantics)
	surface := semantics.Child.(woxwidget.Gesture)
	stack := surface.Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)
	container := scroll.Child.(woxwidget.Container)
	rows := container.Child.(woxwidget.Flex)

	if len(rows.Children) != 12 {
		t.Fatalf("built rows = %d, want 12 viewport rows including overscan", len(rows.Children))
	}
	if container.Height != 241*resultRowBaseHeight {
		t.Fatalf("virtual content height = %.0f, want %d", container.Height, 241*resultRowBaseHeight)
	}
}

func TestVisibleResultRangeAtTop(t *testing.T) {
	start, end := visibleResultRange(241, 0, 500, 0, 50, 0)
	if start != 0 || end != 12 {
		t.Fatalf("visible range = %d:%d, want 0:12", start, end)
	}
}

func TestVisibleResultRangeInMiddle(t *testing.T) {
	start, end := visibleResultRange(241, 500, 500, 0, 50, 0)
	if start != 8 || end != 22 {
		t.Fatalf("visible range = %d:%d, want 8:22", start, end)
	}
}

func TestVisibleResultRangeClampsAtEnd(t *testing.T) {
	start, end := visibleResultRange(12, 400, 200, 0, 50, 0)
	if start != 6 || end != 12 {
		t.Fatalf("visible range = %d:%d, want 6:12", start, end)
	}
}

func TestVisibleResultRangeHandlesEmptyResults(t *testing.T) {
	start, end := visibleResultRange(0, 0, 500, 0, 50, 0)
	if start != 0 || end != 0 {
		t.Fatalf("visible range = %d:%d, want 0:0", start, end)
	}
}
