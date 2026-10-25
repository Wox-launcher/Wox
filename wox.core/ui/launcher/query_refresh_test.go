package launcher

import (
	"context"
	"testing"

	"wox/common"
)

// TestRefreshResultHoldPublishesLatestSnapshot covers a slow plugin leaving usable partial results behind.
func TestRefreshResultHoldPublishesLatestSnapshot(t *testing.T) {
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
	app.visible = true
	app.resultList.results = make([]queryResult, 8)
	app.resultList.resultsQueryID = app.querySession.query.QueryID
	app.resultList.selected = 1
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{SelectedResultId: "target"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		app.querySession.releaseRefreshResultHoldLocked()
		app.querySession.resetQueryLoadingLocked()
	})
	queryID := app.querySession.query.QueryID
	app.querySession.queryRefreshHoldTimer.Stop()
	app.querySession.queryRefreshHoldTimer = nil
	app.applyResults(queryID, []queryResult{{ID: "first-batch"}}, &queryLayout{}, nil, nil, 0, false)
	layout := queryLayout{Icon: woxImage{ImageType: "svg", ImageData: "latest-icon"}}
	refinements := []queryRefinement{{ID: "filter", Type: "select"}}
	queryContext := queryContext{PluginID: "settings"}
	app.applyResults(queryID, []queryResult{{ID: "latest"}, {ID: "target"}}, &layout, &refinements, &queryContext, 123, false)
	if len(app.resultList.results) != 8 || app.querySession.queryContextKnown || app.resultList.layout.Icon.ImageData != "" {
		t.Fatal("buffering applied part of the new snapshot")
	}
	app.querySession.expireRefreshResultHold(queryID)
	if len(app.resultList.results) != 2 || app.resultList.results[0].ID != "latest" || app.resultList.resultsQueryID != queryID || app.resultList.results[1].QueryID != queryID {
		t.Fatalf("expired refresh results = %#v, query %q", app.resultList.results, app.resultList.resultsQueryID)
	}
	if app.resultList.selected != 1 || app.querySession.queryComplete || !app.querySession.queryContextKnown || app.querySession.queryContext.PluginID != "settings" || app.resultList.layout.Icon.ImageData != "latest-icon" || len(app.resultList.refinements) != 1 || app.resultList.refinements[0].ID != "filter" {
		t.Fatal("expiration did not apply the latest selection and metadata coherently")
	}
	if app.querySession.queryRefreshHoldQueryID != "" || app.querySession.queryRefreshSnapshot != nil || app.querySession.queryRefreshHoldTimer != nil {
		t.Fatal("expiration retained refresh state")
	}
}

// TestRefreshResultHoldUsesLayoutHeight covers snapshots whose capped entry counts hide a smaller viewport.
func TestRefreshResultHoldUsesLayoutHeight(t *testing.T) {
	for _, test := range []struct {
		name     string
		previous []queryResult
		next     []queryResult
		layout   queryLayout
	}{
		{
			name: "grouped list",
			previous: []queryResult{
				{IsGroup: true}, {}, {}, {}, {}, {IsGroup: true}, {}, {}, {}, {},
			},
			next: []queryResult{{IsGroup: true}, {}, {}, {}, {}, {}, {}, {}},
		},
		{
			name:     "grid",
			previous: make([]queryResult, 12),
			next:     make([]queryResult, 8),
			layout:   queryLayout{GridLayout: &gridLayout{Columns: 4}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
			app.visible = true
			app.resultList.results = test.previous
			app.resultList.resultsQueryID = app.querySession.query.QueryID
			app.resultList.layout = test.layout
			if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				app.querySession.releaseRefreshResultHoldLocked()
				app.querySession.resetQueryLoadingLocked()
			})
			if min(len(test.previous), 8) != min(len(test.next), 8) {
				t.Fatal("fixture must have equal capped entry counts")
			}
			app.applyResults(app.querySession.query.QueryID, test.next, &test.layout, nil, nil, 0, false)
			if len(app.resultList.results) != len(test.previous) || app.querySession.queryRefreshSnapshot == nil {
				t.Fatal("smaller result area replaced the retained snapshot")
			}
		})
	}
}

// TestRefreshResultHoldCommitsReadySnapshots verifies both early height recovery and a smaller final response.
func TestRefreshResultHoldCommitsReadySnapshots(t *testing.T) {
	for _, test := range []struct {
		name     string
		count    int
		complete bool
	}{
		{name: "height recovered", count: 8},
		{name: "smaller final response", count: 2, complete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
			app.visible = true
			app.resultList.results = make([]queryResult, 8)
			app.resultList.resultsQueryID = app.querySession.query.QueryID
			if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				app.querySession.releaseRefreshResultHoldLocked()
				app.querySession.resetQueryLoadingLocked()
			})
			queryID := app.querySession.query.QueryID
			app.applyResults(queryID, []queryResult{{ID: "buffered"}}, &queryLayout{}, nil, nil, 0, false)
			app.applyResults(queryID, make([]queryResult, test.count), &queryLayout{}, nil, nil, 0, test.complete)
			if len(app.resultList.results) != test.count || app.resultList.resultsQueryID != queryID || app.querySession.queryComplete != test.complete || app.querySession.queryRefreshSnapshot != nil || app.querySession.queryRefreshHoldTimer != nil {
				t.Fatal("ready snapshot did not commit and release the buffer")
			}
			app.querySession.expireRefreshResultHold(queryID)
			if len(app.resultList.results) != test.count || app.resultList.resultsQueryID != queryID {
				t.Fatal("expired callback changed the committed snapshot")
			}
		})
	}
}

// TestRefreshResultHoldDoesNotDelayTypedQuery verifies that typing cancels the buffer and paints the first batch.
func TestRefreshResultHoldDoesNotDelayTypedQuery(t *testing.T) {
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
	app.visible = true
	app.resultList.results = make([]queryResult, 8)
	app.resultList.resultsQueryID = app.querySession.query.QueryID
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		app.querySession.resetQueryTransitionLocked()
		app.querySession.releaseRefreshResultHoldLocked()
		app.querySession.resetQueryLoadingLocked()
	})
	refreshID := app.querySession.query.QueryID
	app.applyResults(refreshID, []queryResult{{ID: "buffered"}}, &queryLayout{}, nil, nil, 0, false)
	app.applyQueryTextChangeLocked("new query")
	if app.querySession.queryRefreshHoldQueryID != "" || app.querySession.queryRefreshSnapshot != nil || app.querySession.queryRefreshHoldTimer != nil {
		t.Fatal("typing retained the refresh buffer")
	}
	app.applyResults(app.querySession.query.QueryID, []queryResult{{ID: "typed"}}, &queryLayout{}, nil, nil, 0, false)
	app.querySession.expireRefreshResultHold(refreshID)
	if len(app.resultList.results) != 1 || app.resultList.results[0].ID != "typed" || app.resultList.resultsQueryID != app.querySession.query.QueryID {
		t.Fatal("typed query did not display its first snapshot")
	}
}

// TestRefreshResultHoldRejectsPreviousDeadline covers overlapping refresh generations.
func TestRefreshResultHoldRejectsPreviousDeadline(t *testing.T) {
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
	app.visible = true
	app.resultList.results = make([]queryResult, 8)
	app.resultList.resultsQueryID = app.querySession.query.QueryID
	t.Cleanup(func() {
		app.querySession.releaseRefreshResultHoldLocked()
		app.querySession.resetQueryLoadingLocked()
	})
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	previousID := app.querySession.query.QueryID
	app.applyResults(previousID, []queryResult{{ID: "previous"}}, &queryLayout{}, nil, nil, 0, false)
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	currentID := app.querySession.query.QueryID
	if app.querySession.queryRefreshSnapshot != nil {
		t.Fatal("new refresh retained the previous buffer")
	}
	app.applyResults(currentID, []queryResult{{ID: "current"}}, &queryLayout{}, nil, nil, 0, false)
	app.querySession.expireRefreshResultHold(previousID)
	if app.querySession.queryRefreshHoldQueryID != currentID || len(app.resultList.results) != 8 || app.querySession.queryRefreshSnapshot == nil {
		t.Fatal("old deadline changed the current refresh")
	}
	app.querySession.queryRefreshHoldTimer.Stop()
	app.querySession.queryRefreshHoldTimer = nil
	app.querySession.expireRefreshResultHold(currentID)
	if len(app.resultList.results) != 1 || app.resultList.results[0].ID != "current" {
		t.Fatal("current deadline did not publish its own snapshot")
	}
}
