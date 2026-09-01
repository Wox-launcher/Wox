package launcher

import (
	"fmt"
	"testing"

	woxwidget "wox/ui/widget"
)

func TestBuildResultsOnlyBuildsViewportRows(t *testing.T) {
	results := make([]queryResult, 241)
	for index := range results {
		results[index] = queryResult{ID: fmt.Sprintf("result-%d", index), Title: fmt.Sprintf("Result %d", index)}
	}
	app := &App{selected: -1}
	built := app.buildResults(viewSnapshot{results: results, selected: -1}, 760, 500, 1)
	semantics := built.(woxwidget.Semantics)
	retained := semantics.Child.(woxwidget.Stateful)
	state := retained.CreateState()
	state.InitState(woxwidget.StateContext{}, retained.Widget)
	defer state.Dispose()
	surface := state.Build(woxwidget.StateContext{}, retained.Widget).(woxwidget.Gesture)
	stack := surface.Child.(woxwidget.Stack)
	scroll := stack.Children[0].Child.(woxwidget.ScrollView)
	container := scroll.Child.(woxwidget.Container)
	rows := container.Child.(woxwidget.Flex)

	if len(rows.Children) != 12 {
		t.Fatalf("built rows = %d, want 12 viewport rows including overscan", len(rows.Children))
	}
	resultRowBaseHeight := launcherDensityMetricsFor("").resultRowBaseHeight
	if container.Height != 241*resultRowBaseHeight {
		t.Fatalf("virtual content height = %.0f, want %.0f", container.Height, 241*resultRowBaseHeight)
	}
}

// TestBuildContentReportsCompletionWithoutResults covers the automation contract that a
// finished query stays observable when it produced nothing. Without the status node a
// wait for completion cannot tell an empty result set from a query still in flight.
func TestBuildContentReportsCompletionWithoutResults(t *testing.T) {
	app := &App{selected: -1}
	for _, complete := range []bool{false, true} {
		built := app.buildContent(viewSnapshot{selected: -1, queryComplete: complete}, 760, 0, 1)
		semantics, ok := built.(woxwidget.Semantics)
		if !ok {
			t.Fatalf("empty result content = %T, want a semantics node carrying query completion", built)
		}
		if semantics.AutomationID != "launcher.results" {
			t.Fatalf("empty result automation ID = %q, want launcher.results", semantics.AutomationID)
		}
		want := "loading"
		if complete {
			want = "complete"
		}
		if semantics.Value != want {
			t.Fatalf("empty result status for complete=%v = %q, want %q", complete, semantics.Value, want)
		}
	}
}

func TestLauncherPreparedSectionEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, launcherPreparedSectionProps{})
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

func TestVisibleListResultRangeUsesShorterGroupHeaders(t *testing.T) {
	results := []queryResult{{Title: "App"}, {Title: "Files", IsGroup: true}, {Title: "readme.txt"}}
	if height := listResultsContentHeight(results, 0, 0, 56, 28, 0); height != 140 {
		t.Fatalf("mixed content height = %.0f, want 140", height)
	}
	start, end := visibleListResultRange(results, 0, 70, 0, 56, 28, 0)
	if start != 0 || end != 3 {
		t.Fatalf("mixed visible range = %d:%d, want 0:3 including overscan", start, end)
	}
}
