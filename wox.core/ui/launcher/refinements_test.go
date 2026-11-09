package launcher

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"wox/ui/contract"
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

func TestRefinementDropdownHotkeysAndLocalSearch(t *testing.T) {
	refinements := []queryRefinement{
		{ID: "country", Type: "singleSelect", Hotkey: "ctrl+r", Options: []queryRefinementOption{
			{Value: "all", Title: "All"}, {Value: "FR", Title: "France", Keywords: []string{"法国"}},
			{Value: "JP", Title: "Japan", Keywords: []string{"日本"}},
		}},
		{ID: "genre", Type: "multiSelect", Hotkey: "ctrl+g", Options: []queryRefinementOption{{Value: "jazz"}, {Value: "rock"}}},
	}
	app := bindTestApp(&App{querySession: queryController{query: newInputQuery("radio jazz")}})
	app.applyRefinementsLocked(refinements)
	if !app.toggleRefinementBar() {
		t.Fatal("could not show refinements")
	}
	countryKey := woxui.KeyEvent{Key: "r", Modifiers: woxui.KeyModifierControl, Down: true}
	if !app.onRefinementHotkey(countryKey) || app.resultList.refinementPicker.id != "country" {
		t.Fatal("visible country hotkey did not open its dropdown")
	}
	app.resultList.refinementPicker.search = "日本"
	app.filterRefinementPicker(app.refinementByID("country"), "")
	picker := app.resultList.refinementPicker
	if !reflect.DeepEqual(picker.options, []int{2}) || picker.value != "JP" {
		t.Fatalf("search by keyword = %#v, want JP", picker)
	}
	if app.querySession.query.QueryText != "radio jazz" || app.querySession.query.QueryRefinements["country"] != "all" {
		t.Fatal("opening or searching modified the query")
	}
	genreKey := woxui.KeyEvent{Key: "g", Modifiers: woxui.KeyModifierControl, Down: true}
	if !app.onRefinementPickerKey(genreKey) || app.resultList.refinementPicker.id != "genre" {
		t.Fatal("multi-select hotkey did not switch the open dropdown")
	}
	app.applyRefinementsLocked(refinements[:1])
	if app.resultList.refinementPicker.id != "" {
		t.Fatal("picker remained open after its group disappeared")
	}
}

func TestRefinementMultiSelectCanRemainEmptyWithDefaults(t *testing.T) {
	refinement := queryRefinement{ID: "type", Type: "multiSelect", DefaultValue: []string{"text"},
		Options: []queryRefinementOption{{Value: "text"}}}
	app := bindTestApp(&App{querySession: queryController{query: newInputQuery("test ")}})
	app.applyRefinementsLocked([]queryRefinement{refinement})
	if got := app.querySession.query.QueryRefinements["type"]; got != "text" {
		t.Fatalf("initial refinement = %q, want default text", got)
	}
	app.querySession.query.QueryRefinements["type"] = ""
	app.applyRefinementsLocked([]queryRefinement{refinement})
	if got, exists := app.querySession.query.QueryRefinements["type"]; !exists || got != "" {
		t.Fatalf("explicitly cleared refinement = %q (exists: %v)", got, exists)
	}
	if got := nextRefinementHotkeyValues(refinement, nil); !reflect.DeepEqual(got, []string{"text"}) {
		t.Fatalf("multi-select hotkey from empty = %v, want text", got)
	}
}

func TestClearRefinementSelectionsRequeriesWithPluginDefaults(t *testing.T) {
	services := &sendQueryRecorderServices{}
	query := newInputQuery("radio jazz")
	query.QueryRefinements = map[string]string{"genre": "jazz", "language": "Chinese", "favorites": "true"}
	app := newSendQueryTestApp(services, query, showAppParams{})
	defer app.querySession.resetQueryLoadingLocked()
	refinements := []queryRefinement{
		{ID: "genre", Type: "singleSelect", Options: []queryRefinementOption{{Value: "all"}, {Value: "jazz"}}},
		{ID: "language", Type: "multiSelect", Options: []queryRefinementOption{{Value: "Chinese"}}},
		{ID: "favorites", Type: "toggle", Title: "Favorites"},
	}
	app.applyRefinementsLocked(refinements)
	app.resultList.refinementDefault.savedValues = query.QueryRefinements
	props := app.refinementViewProps(app.snapshot(), 0, 0, 1)
	if len(props.SummaryLabels) != 3 || props.OnClear == nil {
		t.Fatalf("want a clear action for all three active filters, got %v", props.SummaryLabels)
	}
	props.OnClear()
	started := services.startedQuery
	if started.QueryText != query.QueryText || started.QueryId == query.QueryID || !reflect.DeepEqual(started.QueryRefinements, map[string]string{"genre": "all"}) {
		t.Fatalf("query after clearing filters = %#v", started)
	}
	app.applyRefinementsLocked(refinements)
	props = app.refinementViewProps(app.snapshot(), 0, 0, 1)
	if len(props.SummaryLabels) != 0 || props.OnClear != nil || !reflect.DeepEqual(app.resultList.refinementDefault.savedValues, query.QueryRefinements) {
		t.Fatal("cleared filters returned or the saved default was removed")
	}
}

type refinementDefaultTestServices struct {
	contract.Services
	writes chan map[string]string
}

func (s refinementDefaultTestServices) SetRefinementDefault(_ context.Context, _ contract.RefinementScope, values map[string]string) error {
	s.writes <- values
	return nil
}

func TestRefinementDefaultBookmarkMatchesAndReplacesCurrentCombination(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	query := newInputQuery("radio stations")
	query.QueryRefinements = map[string]string{"genre": "jazz", "language": "fr,en"}
	app := bindTestApp(&App{lifecycleCtx: ctx, querySession: queryController{query: query}, resultList: resultController{refinementOpen: true}})
	service := refinementDefaultTestServices{writes: make(chan map[string]string, 1)}
	app.services = service
	completed := make(chan func(), 1)
	app.uiCall = func(fn func()) error { completed <- fn; return nil }
	refinements := []queryRefinement{
		{ID: "genre", Type: "singleSelect", Options: []queryRefinementOption{{Value: "jazz"}, {Value: "rock"}}},
		{ID: "language", Type: "multiSelect"},
	}
	app.applyRefinementDefaults(query.QueryID, &queryContext{refinementDefaults: contract.RefinementDefaults{
		Scope: contract.RefinementScope{PluginID: "radio"}, SavedValues: map[string]string{"genre": "jazz", "language": "en,fr"},
	}}, refinements, true)
	app.applyRefinementsLocked(refinements)
	values := app.querySession.query.QueryRefinements
	bookmark := func(wantSaved bool) func() {
		t.Helper()
		button := app.refinementViewProps(app.snapshot(), 400, 44, 1).DefaultButton
		if button == nil || button.Saved != wantSaved || button.Busy {
			t.Fatalf("bookmark = %+v, want enabled with Saved=%t", button, wantSaved)
		}
		return button.OnToggle
	}
	finishWrite := func(want map[string]string) {
		t.Helper()
		select {
		case got := <-service.writes:
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("saved filters = %v, want %v", got, want)
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for bookmark write")
		}
		select {
		case fn := <-completed:
			fn()
		case <-ctx.Done():
			t.Fatal("timed out waiting for bookmark completion")
		}
	}

	bookmark(true)
	values["genre"] = "rock"
	bookmark(false)()
	values["genre"] = "jazz"
	finishWrite(map[string]string{"genre": "rock", "language": "fr,en"})
	bookmark(false)
	values["genre"] = "rock"
	bookmark(true)()
	finishWrite(nil)
	bookmark(false)
}

func TestValidateRestoredRefinementsUsesCurrentOptions(t *testing.T) {
	refinements := []queryRefinement{
		{ID: "genre", Type: "singleSelect", DefaultValue: []string{"all"}, Options: []queryRefinementOption{{Value: "jazz"}, {Value: "all"}}},
		{ID: "language", Type: "singleSelect", Options: []queryRefinementOption{{Value: "chinese"}}},
		{ID: "tags", Type: "multiSelect", DefaultValue: []string{"news"}},
	}
	saved := map[string]string{"genre": "obsolete", "language": "chinese", "tags": "", "removed": "value"}
	want := map[string]string{"genre": "all", "language": "chinese", "tags": ""}
	if got := validateRestoredRefinements(refinements, saved); !reflect.DeepEqual(got, want) {
		t.Fatalf("validated refinements = %v, want %v", got, want)
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

func TestRefinementHotkeyUsesConfiguredShortcut(t *testing.T) {
	app := bindTestApp(&App{language: languageCatalog{translations: map[string]string{
		"ui_query_refinement_filters_tooltip": "Filter ({hotkey})",
	}}})
	app.generalSettings.ApplyData(settingsData{RefinementHotkey: "alt+g"})
	if got := app.refinementHotkey(); got != "alt+g" {
		t.Fatalf("configured refinement shortcut = %q", got)
	}
	if got, want := app.refinementToggleTooltip(), "Filter ("+strings.Join(formatHotkeyLabels("alt+g"), "+")+")"; got != want {
		t.Fatalf("tooltip = %q, want %q", got, want)
	}
	if !hotkeyMatches(app.refinementHotkey(), woxui.KeyEvent{Key: "g", Modifiers: woxui.KeyModifierAlt, Down: true}) {
		t.Fatal("configured shortcut must match its key event")
	}
	if hotkeyMatches(app.refinementHotkey(), woxui.KeyEvent{Key: "f", Modifiers: woxui.KeyModifierControl, Down: true}) {
		t.Fatal("old shortcut must not remain active after customization")
	}
}
