package launcher

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestBuildResultsOnlyBuildsViewportRows(t *testing.T) {
	results := make([]queryResult, 241)
	for index := range results {
		results[index] = queryResult{ID: fmt.Sprintf("result-%d", index), Title: fmt.Sprintf("Result %d", index)}
	}
	app := &App{selected: -1}
	built := app.buildResults(viewSnapshot{results: results, selected: -1}, 760, 500, 1, 0)
	semantics := built.(woxwidget.Semantics)
	retained := semantics.Child.(woxwidget.Container).Child.(woxwidget.Semantics).Child.(woxwidget.Stateful)
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
		built := app.buildContent(viewSnapshot{selected: -1, queryComplete: complete}, 760, 0, 1, 0)
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

// TestListViewportKeepsGuttersOutsideRows checks the eight-row budget, end scrolling,
// and compact footer gap across theme geometry and launcher density.
func TestListViewportKeepsGuttersOutsideRows(t *testing.T) {
	for _, bottom := range []float32{0, 8} {
		for _, densityName := range []string{"compact", "normal", "comfortable"} {
			for _, count := range []int{4, 8, 12} {
				for _, show := range []showAppParams{
					{}, {HideToolbar: true}, {QueryBoxAtBottom: true},
					{QueryBoxAtBottom: true, HideToolbar: true},
					{HideQueryBox: true}, {HideQueryBox: true, HideToolbar: true},
				} {
					palette := defaultPalette()
					palette.resultContainerPadding = woxwidget.Insets{Top: 8, Bottom: bottom}
					palette.appPadding = woxwidget.Insets{Top: 12, Bottom: 12}
					density := launcherDensityMetricsFor(densityName)
					results := make([]queryResult, count)
					for index := range results {
						results[index].ID = fmt.Sprint(index)
					}
					padding := launcherListPadding(palette, show)
					rowHeight := density.resultRowHeight(palette)
					height := float32(launcherResultAreaHeight(results, queryLayout{}, 760, 8, int(rowHeight), int(padding.Top), int(padding.Bottom), density.groupHeaderHeight()))
					for _, selected := range []int{0, count - 1} {
						app := &App{selected: selected}
						snapshot := viewSnapshot{results: results, selected: selected, palette: palette, densityMetrics: density, show: show}
						built := app.buildResults(snapshot, 760, height, 2, 40).(woxwidget.Semantics).Child.(woxwidget.Container)
						if padding.Bottom > 0 && !woxui.SupportsEdgeFade() && built.Height != height {
							t.Fatalf("padded result paint height=%v, want %v", built.Height, height)
						}
						scroll := app.resultScroll
						if scroll.viewport != float32(min(count, 8))*rowHeight || scroll.content != float32(count)*rowHeight {
							t.Fatalf("count=%d density=%s: scroll=%+v row=%v", count, densityName, scroll, rowHeight)
						}
						if selected == 0 && scroll.offset != 0 || selected == count-1 && scroll.offset+scroll.viewport != scroll.content {
							t.Fatalf("selected=%d scroll=%+v", selected, scroll)
						}
						if count > 8 {
							props := built.Child.(woxwidget.Semantics).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
							wantUnderlay := float32(40)
							if padding.Bottom > 0 {
								wantUnderlay = 0
								if woxui.SupportsEdgeFade() {
									wantUnderlay = 40 + padding.Bottom
								}
							}
							if props.UnderlayHeight != wantUnderlay {
								t.Fatalf("underlay=%v, want %v", props.UnderlayHeight, wantUnderlay)
							}
						}
						if !woxui.SupportsEdgeFade() && !show.HideToolbar && built.Padding.Bottom != bottom {
							t.Fatalf("toolbar gutter=%v, want %v", built.Padding.Bottom, bottom)
						}
					}
				}
			}
		}
	}
}

// TestListScrollKeepsContentCoordinates covers first-group recovery and detached pointer scrolling.
func TestListScrollKeepsContentCoordinates(t *testing.T) {
	results := make([]queryResult, 12)
	results[0].IsGroup = true
	palette := defaultPalette()
	density := launcherDensityMetricsFor("")
	content := listResultsContentHeight(results, 0, 0, density.resultRowHeight(palette), density.groupHeaderHeight(), resultRowGap)
	for _, detached := range []bool{false, true} {
		scroll := resolveResultScroll(results, nil, 1, 760, 200, content, scrollController{offset: 100}, detached, palette, density)
		want := float32(0)
		if detached {
			want = 100
		}
		if scroll.offset != want {
			t.Fatalf("detached=%v offset=%v, want %v", detached, scroll.offset, want)
		}
	}
}

// TestFooterSamplingPreservesPaddingHeight keeps layout space while painting through it.
func TestFooterSamplingPreservesPaddingHeight(t *testing.T) {
	if !woxui.SupportsEdgeFade() {
		t.Skip("native renderer has no edge mask")
	}
	for _, border := range []float32{0, 1} {
		palette := defaultPalette()
		palette.toolbarBorderWidth = border
		palette.resultContainerPadding = woxwidget.Insets{Top: 8, Bottom: 8}
		results := make([]queryResult, 20)
		for i := range results {
			results[i].ID = fmt.Sprint(i)
		}
		app := &App{selected: -1}
		built := app.buildResults(viewSnapshot{results: results, selected: -1, palette: palette}, 760, 416, 2, 40).(woxwidget.Semantics).Child.(woxwidget.Container)
		props := built.Child.(woxwidget.Semantics).Child.(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		wantGap, wantHeight := float32(8), float32(456)
		if props.UnderlayHeight != wantGap+40 || built.Height != wantHeight || app.resultScroll.viewport != 400 {
			t.Fatalf("border=%v extension=%v height=%v viewport=%v", border, props.UnderlayHeight, built.Height, app.resultScroll.viewport)
		}
	}
}
