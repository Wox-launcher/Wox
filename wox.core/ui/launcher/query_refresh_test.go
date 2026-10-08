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
	app.results = make([]queryResult, 8)
	app.resultsQueryID = app.query.QueryID
	app.selected = 1
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{SelectedResultId: "target"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		app.releaseRefreshResultHoldLocked()
		app.resetQueryLoadingLocked()
	})
	queryID := app.query.QueryID
	app.queryRefreshHoldTimer.Stop()
	app.queryRefreshHoldTimer = nil
	app.applyResults(queryID, []queryResult{{ID: "first-batch"}}, &queryLayout{}, nil, nil, 0, false)
	layout := queryLayout{Icon: woxImage{ImageType: "svg", ImageData: "latest-icon"}}
	refinements := []queryRefinement{{ID: "filter", Type: "select"}}
	queryContext := queryContext{PluginID: "settings"}
	app.applyResults(queryID, []queryResult{{ID: "latest"}, {ID: "target"}}, &layout, &refinements, &queryContext, 123, false)
	if len(app.results) != 8 || app.queryContextKnown || app.layout.Icon.ImageData != "" {
		t.Fatal("buffering applied part of the new snapshot")
	}
	app.expireRefreshResultHold(queryID)
	if len(app.results) != 2 || app.results[0].ID != "latest" || app.resultsQueryID != queryID || app.results[1].QueryID != queryID {
		t.Fatalf("expired refresh results = %#v, query %q", app.results, app.resultsQueryID)
	}
	if app.selected != 1 || app.queryComplete || !app.queryContextKnown || app.queryContext.PluginID != "settings" || app.layout.Icon.ImageData != "latest-icon" || len(app.refinements) != 1 || app.refinements[0].ID != "filter" {
		t.Fatal("expiration did not apply the latest selection and metadata coherently")
	}
	if app.queryRefreshHoldQueryID != "" || app.queryRefreshSnapshot != nil || app.queryRefreshHoldTimer != nil {
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
			app.results = test.previous
			app.resultsQueryID = app.query.QueryID
			app.layout = test.layout
			if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				app.releaseRefreshResultHoldLocked()
				app.resetQueryLoadingLocked()
			})
			if min(len(test.previous), 8) != min(len(test.next), 8) {
				t.Fatal("fixture must have equal capped entry counts")
			}
			app.applyResults(app.query.QueryID, test.next, &test.layout, nil, nil, 0, false)
			if len(app.results) != len(test.previous) || app.queryRefreshSnapshot == nil {
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
			app.results = make([]queryResult, 8)
			app.resultsQueryID = app.query.QueryID
			if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				app.releaseRefreshResultHoldLocked()
				app.resetQueryLoadingLocked()
			})
			queryID := app.query.QueryID
			app.applyResults(queryID, []queryResult{{ID: "buffered"}}, &queryLayout{}, nil, nil, 0, false)
			app.applyResults(queryID, make([]queryResult, test.count), &queryLayout{}, nil, nil, 0, test.complete)
			if len(app.results) != test.count || app.resultsQueryID != queryID || app.queryComplete != test.complete || app.queryRefreshSnapshot != nil || app.queryRefreshHoldTimer != nil {
				t.Fatal("ready snapshot did not commit and release the buffer")
			}
			app.expireRefreshResultHold(queryID)
			if len(app.results) != test.count || app.resultsQueryID != queryID {
				t.Fatal("expired callback changed the committed snapshot")
			}
		})
	}
}

// TestRefreshResultHoldDoesNotDelayTypedQuery verifies that typing cancels the buffer and paints the first batch.
func TestRefreshResultHoldDoesNotDelayTypedQuery(t *testing.T) {
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
	app.visible = true
	app.results = make([]queryResult, 8)
	app.resultsQueryID = app.query.QueryID
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		app.resetQueryTransitionLocked()
		app.releaseRefreshResultHoldLocked()
		app.resetQueryLoadingLocked()
	})
	refreshID := app.query.QueryID
	app.applyResults(refreshID, []queryResult{{ID: "buffered"}}, &queryLayout{}, nil, nil, 0, false)
	app.applyQueryTextChangeLocked("new query")
	if app.queryRefreshHoldQueryID != "" || app.queryRefreshSnapshot != nil || app.queryRefreshHoldTimer != nil {
		t.Fatal("typing retained the refresh buffer")
	}
	app.applyResults(app.query.QueryID, []queryResult{{ID: "typed"}}, &queryLayout{}, nil, nil, 0, false)
	app.expireRefreshResultHold(refreshID)
	if len(app.results) != 1 || app.results[0].ID != "typed" || app.resultsQueryID != app.query.QueryID {
		t.Fatal("typed query did not display its first snapshot")
	}
}

// TestRefreshResultHoldRejectsPreviousDeadline covers overlapping refresh generations.
func TestRefreshResultHoldRejectsPreviousDeadline(t *testing.T) {
	app := newSendQueryTestApp(&sendQueryRecorderServices{}, newInputQuery("setting"), showAppParams{MaxResultCount: 8})
	app.visible = true
	app.results = make([]queryResult, 8)
	app.resultsQueryID = app.query.QueryID
	t.Cleanup(func() {
		app.releaseRefreshResultHoldLocked()
		app.resetQueryLoadingLocked()
	})
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	previousID := app.query.QueryID
	app.applyResults(previousID, []queryResult{{ID: "previous"}}, &queryLayout{}, nil, nil, 0, false)
	if err := app.RefreshQuery(context.Background(), common.RefreshQueryOptions{}); err != nil {
		t.Fatal(err)
	}
	currentID := app.query.QueryID
	if app.queryRefreshSnapshot != nil {
		t.Fatal("new refresh retained the previous buffer")
	}
	app.applyResults(currentID, []queryResult{{ID: "current"}}, &queryLayout{}, nil, nil, 0, false)
	app.expireRefreshResultHold(previousID)
	if app.queryRefreshHoldQueryID != currentID || len(app.results) != 8 || app.queryRefreshSnapshot == nil {
		t.Fatal("old deadline changed the current refresh")
	}
	app.queryRefreshHoldTimer.Stop()
	app.queryRefreshHoldTimer = nil
	app.expireRefreshResultHold(currentID)
	if len(app.results) != 1 || app.results[0].ID != "current" {
		t.Fatal("current deadline did not publish its own snapshot")
	}
}
