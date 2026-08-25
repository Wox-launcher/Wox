package launcher

import (
	"reflect"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

func TestRefinementToggleTooltipUsesPlatformHotkey(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_query_refinement_filters_tooltip": "过滤搜索结果({hotkey})",
	}}
	want := "过滤搜索结果(" + strings.Join(formatHotkeyLabels(primaryHotkey("f")), "+") + ")"
	if got := app.refinementToggleTooltip(); got != want {
		t.Fatalf("refinement tooltip = %q, want %q", got, want)
	}
}

func TestNextRefinementHotkeyValues(t *testing.T) {
	options := []queryRefinementOption{{Value: "all"}, {Value: "text"}, {Value: "file"}}
	tests := []struct {
		name       string
		refinement queryRefinement
		selected   []string
		want       []string
	}{
		{name: "single select advances", refinement: queryRefinement{Type: "singleSelect", Options: options}, selected: []string{"all"}, want: []string{"text"}},
		{name: "single select wraps", refinement: queryRefinement{Type: "singleSelect", Options: options}, selected: []string{"file"}, want: []string{"all"}},
		{name: "multi select selects all", refinement: queryRefinement{Type: "multiSelect", Options: options}, selected: []string{"text"}, want: []string{"all", "text", "file"}},
		{name: "multi select clears all", refinement: queryRefinement{Type: "multiSelect", Options: options}, selected: []string{"all", "text", "file"}, want: nil},
		{name: "toggle turns on", refinement: queryRefinement{Type: "toggle", Options: []queryRefinementOption{{Value: "enabled"}}}, want: []string{"enabled"}},
		{name: "toggle turns off", refinement: queryRefinement{Type: "toggle", Options: []queryRefinementOption{{Value: "enabled"}}}, selected: []string{"enabled"}, want: nil},
		{name: "toggle keeps explicit true default", refinement: queryRefinement{Type: "toggle", DefaultValue: []string{"true"}, Options: []queryRefinementOption{{Value: "false"}, {Value: "true"}}}, selected: []string{"false"}, want: []string{"true"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nextRefinementHotkeyValues(test.refinement, test.selected); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("next values = %v, want %v", got, test.want)
			}
		})
	}
}

func TestRefinementHotkeyTargetRequiresKeyDown(t *testing.T) {
	app := &App{refinements: []queryRefinement{{ID: "type", Type: "singleSelect", Hotkey: "cmd+t", Options: []queryRefinementOption{{Value: "all"}, {Value: "text"}}}}}
	event := woxui.KeyEvent{Key: "t", Modifiers: woxui.KeyModifierMeta}
	if app.onRefinementHotkey(event) {
		t.Fatal("key-up unexpectedly handled refinement hotkey")
	}
}

func TestBeginQueryTransitionKeepsVisibleResultsDuringGracePeriod(t *testing.T) {
	app := &App{
		query:          plainQuery{QueryID: "new-query", QueryText: "cb "},
		visible:        true,
		results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
		resultsQueryID: "old-query",
		selected:       0,
	}

	app.beginQueryTransitionLocked(false)
	timer := app.queryTransitionTimer
	if timer == nil {
		t.Fatal("query transition timer is nil, want stale-result grace period")
	}
	timer.Stop()
	app.queryTransitionTimer = nil

	if len(app.results) != 1 || app.results[0].ID != "old-result" {
		t.Fatalf("visible results = %#v, want previous snapshot during grace period", app.results)
	}
	if app.resultsQueryID != "old-query" || app.selected != 0 {
		t.Fatalf("visible result state = query %q selected %d, want old-query and 0", app.resultsQueryID, app.selected)
	}
}

func TestBeginQueryTransitionKeepsVisibleResultsForMRU(t *testing.T) {
	app := &App{
		query:          plainQuery{QueryID: "mru-query"},
		show:           showAppParams{StartPage: "mru"},
		visible:        true,
		results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
		resultsQueryID: "old-query",
		selected:       0,
	}

	app.beginQueryTransitionLocked(false)
	timer := app.queryTransitionTimer
	if timer == nil {
		t.Fatal("MRU transition timer is nil, want stale-result grace period")
	}
	timer.Stop()
	app.queryTransitionTimer = nil

	if len(app.results) != 1 || app.results[0].ID != "old-result" {
		t.Fatalf("visible MRU transition results = %#v, want previous snapshot", app.results)
	}
	if app.resultsQueryID != "old-query" || app.selected != 0 {
		t.Fatalf("visible MRU transition state = query %q selected %d, want old-query and 0", app.resultsQueryID, app.selected)
	}
}

func TestBeginQueryTransitionClearsResultsWithoutVisibleGracePeriod(t *testing.T) {
	app := &App{
		query:          plainQuery{QueryID: "new-query", QueryText: "cb "},
		results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
		resultsQueryID: "old-query",
		selected:       0,
	}

	app.beginQueryTransitionLocked(false)

	if app.queryTransitionTimer != nil {
		t.Fatal("hidden query unexpectedly scheduled a stale-result grace period")
	}
	if len(app.results) != 0 || app.resultsQueryID != "" || app.selected != -1 {
		t.Fatalf("hidden result state = results %#v query %q selected %d, want cleared", app.results, app.resultsQueryID, app.selected)
	}
}

func TestQueryTextChangeKeepsConfirmedPluginLayoutWithinTriggerScope(t *testing.T) {
	icon := woxImage{ImageType: "svg", ImageData: "everything-icon"}
	app := &App{
		query:             plainQuery{QueryID: "old-query", QueryType: "input", QueryText: "e main.go"},
		queryContext:      queryContext{PluginID: "everything"},
		queryContextKnown: true,
		layout:            queryLayout{Icon: icon, GridLayout: &gridLayout{Columns: 4}},
	}

	app.applyQueryTextChangeLocked("e main.go;d")

	if app.layout.Icon != icon || app.layout.GridLayout == nil || app.layout.GridLayout.Columns != 4 {
		t.Fatalf("plugin layout = %#v, want previous confirmed layout", app.layout)
	}
}

func TestQueryTextChangeClearsPluginLayoutWhenTriggerScopeChanges(t *testing.T) {
	app := &App{
		query:             plainQuery{QueryID: "old-query", QueryType: "input", QueryText: "e main.go"},
		queryContext:      queryContext{PluginID: "everything"},
		queryContextKnown: true,
		layout:            queryLayout{Icon: woxImage{ImageType: "svg", ImageData: "everything-icon"}},
	}

	app.applyQueryTextChangeLocked("main.go")

	if app.layout.Icon.ImageData != "" {
		t.Fatalf("layout icon = %q, want cleared after leaving plugin trigger scope", app.layout.Icon.ImageData)
	}
}

func TestQueryLoadingOnlyAppearsForUnansweredCurrentQuery(t *testing.T) {
	app := &App{query: plainQuery{QueryID: "current", QueryText: "slow"}}
	app.startQueryLoadingLocked()
	if app.queryLoadingTimer == nil {
		t.Fatal("query loading timer is nil")
	}
	app.queryLoadingTimer.Stop()
	app.queryLoadingTimer = nil

	app.showQueryLoading("stale")
	if app.queryLoading {
		t.Fatal("stale query showed loading")
	}
	app.showQueryLoading("current")
	if !app.queryLoading {
		t.Fatal("unanswered current query did not show loading")
	}
	app.resetQueryLoadingLocked()
	app.resultsQueryID = "current"
	app.results = []queryResult{{ID: "result", QueryID: "current"}}
	app.showQueryLoading("current")
	if app.queryLoading {
		t.Fatal("answered current query showed loading")
	}
	app.results = nil
	app.queryComplete = false
	app.showQueryLoading("current")
	if !app.queryLoading {
		t.Fatal("non-final empty response stopped loading")
	}
}
