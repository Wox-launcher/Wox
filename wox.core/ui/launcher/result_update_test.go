package launcher

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/util"
)

func TestUpdateResultRejectsObsoleteQueryScope(t *testing.T) {
	cases := []struct {
		name       string
		sessionID  string
		queryID    string
		retained   bool
		transition bool
		want       bool
	}{
		{name: "current", sessionID: "session", queryID: "current", want: true},
		{name: "old query", sessionID: "session", queryID: "old"},
		{name: "other window", sessionID: "other", queryID: "current"},
		{name: "retained results", sessionID: "session", queryID: "current", retained: true},
		{name: "transition during dispatch", sessionID: "session", queryID: "current", transition: true},
		{name: "legacy unscoped", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := &App{
				sessionID: "session", query: plainQuery{QueryID: "current"}, resultsQueryID: "current",
				results: []queryResult{{ID: "row", Title: "before"}}, selected: -1,
			}
			if tc.retained {
				app.resultsQueryID = "old"
			}
			if tc.transition {
				app.uiCall = func(fn func()) error {
					app.query.QueryID = "next"
					fn()
					return nil
				}
			}
			ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), tc.sessionID), tc.queryID)
			title := "after"
			updated, err := app.UpdateResult(ctx, plugin.UpdatableResult{Id: "row", Title: &title})
			if err != nil || updated != tc.want {
				t.Fatalf("update = %v, err = %v, want %v", updated, err, tc.want)
			}
			if tc.want && app.results[0].Title != title {
				t.Fatal("current query's update was not applied")
			}
			if !tc.want && app.results[0].Title != "before" {
				t.Fatal("obsolete update overwrote the visible row")
			}
		})
	}
}
