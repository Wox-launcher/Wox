package launcher

import (
	"reflect"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
)

func TestRefinementToggleTooltipUsesPlatformHotkey(t *testing.T) {
	app := bindTestApp(&App{
		language: languageCatalog{
			translations: map[string]string{
				"ui_query_refinement_filters_tooltip": "过滤搜索结果({hotkey})",
			},
		},
	})
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
	app := bindTestApp(&App{resultList: resultController{refinements: []queryRefinement{{ID: "type", Type: "singleSelect", Hotkey: "cmd+t", Options: []queryRefinementOption{{Value: "all"}, {Value: "text"}}}}}})
	event := woxui.KeyEvent{Key: "t", Modifiers: woxui.KeyModifierMeta}
	if app.onRefinementHotkey(event) {
		t.Fatal("key-up unexpectedly handled refinement hotkey")
	}
}

func TestBeginQueryTransitionKeepsVisibleResultsDuringGracePeriod(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "new-query", QueryText: "cb "}},
		visible:      true,
		resultList: resultController{
			results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
			resultsQueryID: "old-query",
			selected:       0,
		},
	})

	app.querySession.beginQueryTransitionLocked(false)
	timer := app.querySession.queryTransitionTimer
	if timer == nil {
		t.Fatal("query transition timer is nil, want stale-result grace period")
	}
	timer.Stop()
	app.querySession.queryTransitionTimer = nil

	if len(app.resultList.results) != 1 || app.resultList.results[0].ID != "old-result" {
		t.Fatalf("visible results = %#v, want previous snapshot during grace period", app.resultList.results)
	}
	if app.resultList.resultsQueryID != "old-query" || app.resultList.selected != 0 {
		t.Fatalf("visible result state = query %q selected %d, want old-query and 0", app.resultList.resultsQueryID, app.resultList.selected)
	}
}

func TestBeginQueryTransitionKeepsVisibleResultsForMRU(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "mru-query"}},
		show:         showAppParams{StartPage: "mru"},
		visible:      true,
		resultList: resultController{
			results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
			resultsQueryID: "old-query",
			selected:       0,
		},
	})

	app.querySession.beginQueryTransitionLocked(false)
	timer := app.querySession.queryTransitionTimer
	if timer == nil {
		t.Fatal("MRU transition timer is nil, want stale-result grace period")
	}
	timer.Stop()
	app.querySession.queryTransitionTimer = nil

	if len(app.resultList.results) != 1 || app.resultList.results[0].ID != "old-result" {
		t.Fatalf("visible MRU transition results = %#v, want previous snapshot", app.resultList.results)
	}
	if app.resultList.resultsQueryID != "old-query" || app.resultList.selected != 0 {
		t.Fatalf("visible MRU transition state = query %q selected %d, want old-query and 0", app.resultList.resultsQueryID, app.resultList.selected)
	}
}

func TestBeginQueryTransitionClearsResultsWithoutVisibleGracePeriod(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{query: plainQuery{QueryID: "new-query", QueryText: "cb "}},
		resultList: resultController{
			results:        []queryResult{{ID: "old-result", QueryID: "old-query"}},
			resultsQueryID: "old-query",
			selected:       0,
		},
	})

	app.querySession.beginQueryTransitionLocked(false)

	if app.querySession.queryTransitionTimer != nil {
		t.Fatal("hidden query unexpectedly scheduled a stale-result grace period")
	}
	if len(app.resultList.results) != 0 || app.resultList.resultsQueryID != "" || app.resultList.selected != -1 {
		t.Fatalf("hidden result state = results %#v query %q selected %d, want cleared", app.resultList.results, app.resultList.resultsQueryID, app.resultList.selected)
	}
}

func TestQueryTextChangeKeepsConfirmedPluginLayoutWithinTriggerScope(t *testing.T) {
	icon := woxImage{ImageType: "svg", ImageData: "everything-icon"}
	app := bindTestApp(&App{
		querySession: queryController{
			query:             plainQuery{QueryID: "old-query", QueryType: "input", QueryText: "e main.go"},
			queryContext:      queryContext{PluginID: "everything"},
			queryContextKnown: true,
		},
		resultList: resultController{layout: queryLayout{Icon: icon, GridLayout: &gridLayout{Columns: 4}}},
	})

	app.applyQueryTextChangeLocked("e main.go;d")

	if app.resultList.layout.Icon != icon || app.resultList.layout.GridLayout == nil || app.resultList.layout.GridLayout.Columns != 4 {
		t.Fatalf("plugin layout = %#v, want previous confirmed layout", app.resultList.layout)
	}
}

func TestQueryTextChangeClearsPluginLayoutWhenTriggerScopeChanges(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{
			query:             plainQuery{QueryID: "old-query", QueryType: "input", QueryText: "e main.go"},
			queryContext:      queryContext{PluginID: "everything"},
			queryContextKnown: true,
		},
		resultList: resultController{layout: queryLayout{Icon: woxImage{ImageType: "svg", ImageData: "everything-icon"}}},
	})

	app.applyQueryTextChangeLocked("main.go")

	if app.resultList.layout.Icon.ImageData != "" {
		t.Fatalf("layout icon = %q, want cleared after leaving plugin trigger scope", app.resultList.layout.Icon.ImageData)
	}
}

func TestQueryLoadingOnlyAppearsForUnansweredCurrentQuery(t *testing.T) {
	app := bindTestApp(&App{querySession: queryController{query: plainQuery{QueryID: "current", QueryText: "slow"}}})
	app.querySession.startQueryLoadingLocked()
	if app.querySession.queryLoadingTimer == nil {
		t.Fatal("query loading timer is nil")
	}
	app.querySession.queryLoadingTimer.Stop()
	app.querySession.queryLoadingTimer = nil

	app.querySession.showQueryLoading("stale")
	if app.querySession.queryLoading {
		t.Fatal("stale query showed loading")
	}
	app.querySession.showQueryLoading("current")
	if !app.querySession.queryLoading {
		t.Fatal("unanswered current query did not show loading")
	}
	app.querySession.resetQueryLoadingLocked()
	app.resultList.resultsQueryID = "current"
	app.resultList.results = []queryResult{{ID: "result", QueryID: "current"}}
	app.querySession.showQueryLoading("current")
	if app.querySession.queryLoading {
		t.Fatal("answered current query showed loading")
	}
	app.resultList.results = nil
	app.querySession.queryComplete = false
	app.querySession.showQueryLoading("current")
	if !app.querySession.queryLoading {
		t.Fatal("non-final empty response stopped loading")
	}
}
