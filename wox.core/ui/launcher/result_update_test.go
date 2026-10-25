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
			app := bindTestApp(&App{
				sessionID:    "session",
				querySession: queryController{query: plainQuery{QueryID: "current"}},
				resultList: resultController{
					resultsQueryID: "current",
					results:        []queryResult{{ID: "row", Title: "before"}},
					selected:       -1,
				},
			})
			if tc.retained {
				app.resultList.resultsQueryID = "old"
			}
			if tc.transition {
				app.uiCall = func(fn func()) error {
					app.querySession.query.QueryID = "next"
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
			if tc.want && app.resultList.results[0].Title != title {
				t.Fatal("current query's update was not applied")
			}
			if !tc.want && app.resultList.results[0].Title != "before" {
				t.Fatal("obsolete update overwrote the visible row")
			}
		})
	}
}

func TestScriptUpdateRejectsMissingCurrentDocument(t *testing.T) {
	for _, selected := range []int{-1, 0, 1} {
		app := bindTestApp(&App{
			sessionID:    "session",
			querySession: queryController{query: plainQuery{QueryID: "query"}},
			resultList: resultController{
				resultsQueryID: "query",
				results:        []queryResult{{ID: "row", Title: "before"}, {ID: "other"}},
				selected:       selected,
			},
		})
		ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "session"), "query")
		command := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeWebView, PreviewData: `{"updateMode":"script","script":"tick()"}`}
		title := "after"
		accepted, err := app.UpdateResult(ctx, plugin.UpdatableResult{Id: "row", Title: &title, Preview: &command})
		if err != nil || accepted || app.resultList.results[0].Title != "before" {
			t.Fatal("script without current document was applied")
		}
	}
}
func TestQueryPreviewRejectsScriptInstructions(t *testing.T) {
	for _, data := range []string{`{"updateMode":"script","script":"tick()"}`, `{"html":"<p>hi</p>","updateMode":"script","script":"tick()"}`, `{"html":"<p>hi</p>","script":""}`} {
		if _, err := decodeWebViewPreview(data); err == nil {
			t.Fatal("query accepted script instruction")
		}
	}
}
