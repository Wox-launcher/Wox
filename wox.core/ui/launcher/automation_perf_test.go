//go:build wox_automation

package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestAutomationQueryStateRejectsStaleFrames protects latency measurements from
// retained rows and completion belonging to the previous query generation.
func TestAutomationQueryStateRejectsStaleFrames(t *testing.T) {
	for _, test := range []struct {
		name          string
		resultsQuery  string
		input         string
		row           string
		frameComplete bool
		complete      bool
		empty         bool
		wantVisible   bool
		wantComplete  bool
	}{
		{name: "current complete", resultsQuery: "new", input: "terminal", row: "new-row", frameComplete: true, complete: true, wantVisible: true, wantComplete: true},
		{name: "current partial", resultsQuery: "new", input: "terminal", row: "new-row", wantVisible: true},
		{name: "retained query", resultsQuery: "old", input: "terminal", row: "new-row", frameComplete: true, complete: true},
		{name: "retained row", resultsQuery: "new", input: "terminal", row: "old-row", frameComplete: true, complete: true},
		{name: "retained input", resultsQuery: "new", input: "chrome", row: "new-row", frameComplete: true, complete: true},
		{name: "completion awaiting frame", resultsQuery: "new", input: "terminal", row: "new-row", complete: true, wantVisible: true},
		{name: "empty complete", resultsQuery: "new", input: "terminal", empty: true, frameComplete: true, complete: true, wantComplete: true},
		{name: "empty retained row", resultsQuery: "new", input: "terminal", empty: true, row: "old-row", frameComplete: true, complete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			app := &App{query: plainQuery{QueryID: "new", QueryText: "terminal"}, resultsQueryID: test.resultsQuery, queryComplete: test.complete}
			if !test.empty {
				app.results = []queryResult{{ID: "new-row", QueryID: "new"}}
			}
			app.host = woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
				status := "loading"
				if test.frameComplete {
					status = "complete"
				}
				children := []woxwidget.Widget{
					woxwidget.Semantics{AutomationID: "launcher.query.input", Role: woxui.AccessibilityRoleTextField, Label: "Query", Value: test.input, Child: woxwidget.Container{Width: 20, Height: 20}},
					woxwidget.Semantics{AutomationID: "launcher.results", Role: woxui.AccessibilityRoleList, Label: "Results", Value: status, Child: woxwidget.Container{Width: 20, Height: 20}},
				}
				if test.row != "" {
					children = append(children, woxwidget.Semantics{AutomationID: "launcher.result." + test.row, Role: woxui.AccessibilityRoleButton, Label: "Row", Child: woxwidget.Container{Width: 20, Height: 20}})
				}
				return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
			})
			app.host.AttachServices(formTableHostServices{})
			defer app.host.Dispose()
			app.host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 100}, PixelSize: woxui.PixelSize{Width: 100, Height: 100}, Scale: 1})
			state, err := app.AutomationQueryState()
			if err != nil {
				t.Fatal(err)
			}
			if state.Visible != test.wantVisible || state.Complete != test.wantComplete {
				t.Fatalf("readiness = %+v, want visible=%v complete=%v", state, test.wantVisible, test.wantComplete)
			}
		})
	}
}
