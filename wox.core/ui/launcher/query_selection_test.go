package launcher

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

// newQuerySelectionTestApp supplies the launcher controllers without a native UI event loop.
func newQuerySelectionTestApp(t *testing.T) *App {
	t.Helper()
	a := New(false, nil)
	a.uiCall = nil
	t.Cleanup(func() {
		a.resetQueryTransitionLocked()
		a.resetQueryLoadingLocked()
		a.cancel()
	})
	return a
}

// TestQueryUndoSelectionMatching covers text identity, duplicate rows, and positional fallback.
func TestQueryUndoSelectionMatching(t *testing.T) {
	target := queryResultSelection{title: "target", subTitle: "detail", index: 2}
	match := queryResult{ID: "new-id", Title: "target", SubTitle: "detail"}
	other := queryResult{Title: "other"}
	group := queryResult{Title: "target", SubTitle: "detail", IsGroup: true}
	for _, test := range []struct {
		name    string
		results []queryResult
		want    int
	}{
		{"reordered new ID", []queryResult{match, other, other}, 0},
		{"duplicate at original position", []queryResult{match, other, match}, 2},
		{"duplicate away from original position", []queryResult{match, match, other}, 0},
		{"subtitle must match", []queryResult{{Title: "target", SubTitle: "different"}, match, other}, 1},
		{"case must match", []queryResult{{Title: "Target", SubTitle: "detail"}, other, other}, 2},
		{"missing preserves position", []queryResult{other, other, other}, 2},
		{"overflow selects first", []queryResult{group, other}, 1},
		{"group is not a match", []queryResult{group, match, other}, 1},
		{"fallback skips group", []queryResult{other, other, group, other}, 3},
		{"final group falls back to first", []queryResult{other, other, group}, 0},
		{"only groups", []queryResult{group, group, group}, -1},
		{"empty", nil, -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			pending := &pendingResultSelection{queryID: "current", querySnapshot: &target}
			for _, complete := range []bool{false, true} {
				selected, preserved, keep := restoreRefreshSelection(test.results, pending, "current", complete)
				if selected != test.want || !preserved || keep == complete {
					t.Fatalf("selection = %d preserved=%v keep=%v complete=%v, want %d", selected, preserved, keep, complete, test.want)
				}
			}
		})
	}
}

// TestQuerySelectionSnapshotRejectsStaleResults prevents transition frames from polluting undo history.
func TestQuerySelectionSnapshotRejectsStaleResults(t *testing.T) {
	a := newQuerySelectionTestApp(t)
	a.query = newInputQuery("current")
	a.results = []queryResult{{Title: "live", SubTitle: "detail"}, {IsGroup: true}}
	a.resultsQueryID = a.query.QueryID
	a.selected = 0
	snapshot := a.captureQuerySnapshot()
	if snapshot.resultSelection == nil || *snapshot.resultSelection != (queryResultSelection{title: "live", subTitle: "detail", index: 0}) {
		t.Fatalf("live snapshot = %+v", snapshot.resultSelection)
	}
	a.appendQueryUndo(snapshot)
	a.selected = 1
	a.appendQueryUndo(a.captureQuerySnapshot())
	if len(a.queryHintEditorState.undo) != 1 || a.queryHintEditorState.undo[0].resultSelection != nil {
		t.Fatal("selection changes must update duplicate snapshots without adding undo steps")
	}
	for _, selected := range []int{-1, 1, 2} {
		a.selected = selected
		if a.captureQuerySnapshot().resultSelection != nil {
			t.Fatalf("invalid selected index %d was captured", selected)
		}
	}
	a.selected = 0
	a.resultsQueryID = "previous"
	if a.captureQuerySnapshot().resultSelection != nil {
		t.Fatal("stale results were captured for the new query")
	}
	target := queryResultSelection{title: "pending", index: 4}
	a.pendingSelection = &pendingResultSelection{queryID: a.query.QueryID, querySnapshot: &target}
	snapshot = a.captureQuerySnapshot()
	target.index = 9
	if snapshot.resultSelection == nil || snapshot.resultSelection.index != 4 {
		t.Fatal("pending undo target must be captured without aliasing mutable state")
	}
	a.pendingSelection.queryID = "previous"
	if a.captureQuerySnapshot().resultSelection != nil {
		t.Fatal("stale pending selection was captured")
	}
}

// TestChangeQueryUndoRedoSelection exercises repeated navigation before the restored queries respond.
func TestChangeQueryUndoRedoSelection(t *testing.T) {
	a := newQuerySelectionTestApp(t)
	a.setQuery(newInputQuery("first"))
	a.applyResults(a.query.QueryID, []queryResult{{Title: "other"}, {Title: "first target", SubTitle: "detail"}}, nil, nil, nil, 0, true)
	a.selectResult(1)
	a.setQuery(newInputQuery("second"))
	a.applyResults(a.query.QueryID, []queryResult{{Title: "second target"}, {Title: "other"}}, nil, nil, nil, 0, true)
	a.selectResult(0)
	a.setQuery(newInputQuery("third"))
	a.applyResults(a.query.QueryID, []queryResult{{Title: "other"}, {Title: "other"}, {Title: "third target"}}, nil, nil, nil, 0, true)
	a.selectResult(2)
	for _, step := range []struct{ key, query, target string }{
		{"z", "second", "second target"},
		{"z", "first", "first target"},
		{"y", "second", "second target"},
		{"y", "third", "third target"},
		{"z", "second", "second target"},
		{"y", "third", "third target"},
	} {
		previousID := a.query.QueryID
		if !a.onQueryHintKey(woxui.KeyEvent{Key: woxui.Key(step.key), Modifiers: queryPrimaryModifier(), Down: true}) {
			t.Fatal("undo/redo key was not handled")
		}
		if a.query.QueryText != step.query || a.query.QueryID == previousID || a.pendingSelection == nil || a.pendingSelection.queryID != a.query.QueryID || a.pendingSelection.querySnapshot.title != step.target {
			t.Fatalf("%s query=%+v pending=%+v, want query=%s target=%s", step.key, a.query, a.pendingSelection, step.query, step.target)
		}
	}
	a.applyResults(a.query.QueryID, []queryResult{{Title: "third target"}, {Title: "other"}}, nil, nil, nil, 0, true)
	if a.selected != 0 || a.pendingSelection != nil {
		t.Fatal("completed redo must select the reordered target and clear pending state")
	}
}

type immediateQuerySelectionServices struct {
	contract.Services
	results []plugin.QueryResultUI
}

// StartQuery returns inline to ensure the undo target is installed before query dispatch.
func (s immediateQuerySelectionServices) StartQuery(ctx context.Context, request contract.QueryRequest, view contract.QueryView) error {
	view.ApplyQueryResponse(ctx, contract.QueryResponse{
		QueryID: request.Query.QueryId, IsFinal: true,
		Response: plugin.QueryResponseUI{Results: s.results},
	})
	return nil
}

// TestQueryUndoSelectionSynchronousResponse catches binding the target after services have already responded.
func TestQueryUndoSelectionSynchronousResponse(t *testing.T) {
	for _, test := range []struct {
		name   string
		target *queryResultSelection
		want   int
	}{
		{"restore target", &queryResultSelection{title: "target", subTitle: "detail", index: 0}, 1},
		{"no saved selection", nil, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := newQuerySelectionTestApp(t)
			a.services = immediateQuerySelectionServices{results: []plugin.QueryResultUI{{Title: "other"}, {Title: "target", SubTitle: "detail"}}}
			a.applyQuerySnapshot(queryHintSnapshot{text: "restored", resultSelection: test.target})
			if a.selected != test.want || a.pendingSelection != nil || !a.queryComplete {
				t.Fatalf("inline response selected=%d pending=%+v complete=%v", a.selected, a.pendingSelection, a.queryComplete)
			}
		})
	}
}

// TestQueryUndoSelectionStreaming covers both snapshot replacement and pushed batches, including stale responses.
func TestQueryUndoSelectionStreaming(t *testing.T) {
	for _, appendResults := range []bool{false, true} {
		name := "snapshots"
		if appendResults {
			name = "pushed batches"
		}
		t.Run(name, func(t *testing.T) {
			a := newQuerySelectionTestApp(t)
			a.applyQuerySnapshot(queryHintSnapshot{text: "restored", resultSelection: &queryResultSelection{title: "target", subTitle: "detail", index: 3}})
			queryID := a.query.QueryID
			a.applyResults(queryID, []queryResult{{Title: "other"}}, nil, nil, nil, 0, false)
			if a.selected != 0 || a.pendingSelection == nil {
				t.Fatal("partial overflow must select the first result and retain the target")
			}
			a.applyResults("old", nil, nil, nil, nil, 0, true)
			if appended, err := a.appendTypedResults("old", []queryResult{{Title: "target"}}); err != nil || appended || a.pendingSelection == nil {
				t.Fatal("stale results must not consume the target")
			}
			results := []queryResult{{Title: "target", SubTitle: "detail", Preview: queryPreview{PreviewType: "text", PreviewData: "target preview"}}}
			if appendResults {
				// The state update runs before resizing the uninitialized unit-test window.
				if _, err := a.appendTypedResults(queryID, results); err != nil && err.Error() != "window is not initialized" {
					t.Fatalf("append results: %v", err)
				}
			} else {
				a.applyResults(queryID, append([]queryResult{{Title: "other"}}, results...), nil, nil, nil, 0, false)
			}
			if a.selected != 1 || a.pendingSelection == nil || a.results[a.selected].Preview.PreviewData != "target preview" {
				t.Fatal("late results must restore the selected result and its preview")
			}
			a.applyResults(queryID, append([]queryResult(nil), a.results...), nil, nil, nil, 0, true)
			if a.selected != 1 || a.pendingSelection != nil {
				t.Fatal("final results must restore the target and clear pending state")
			}
		})
	}
}

// TestQueryUndoSelectionCancellation lets query transitions and explicit navigation supersede restoration.
func TestQueryUndoSelectionCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		act    func(*App)
		manual bool
	}{
		{"new query", func(a *App) { a.setQuery(newInputQuery("new")) }, false},
		{"reset", func(a *App) { a.resetQuery(newInputQuery("")) }, false},
		{"edit", func(a *App) { a.queryHintChanged() }, false},
		{"click selected row", func(a *App) { a.selectResult(0) }, true},
		{"arrow", func(a *App) { a.moveSelection(1) }, true},
		{"group jump", func(a *App) { a.moveSelectionByGroup(1) }, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := newQuerySelectionTestApp(t)
			a.applyQuerySnapshot(queryHintSnapshot{text: "restored", resultSelection: &queryResultSelection{title: "target", index: 4}})
			a.applyResults(a.query.QueryID, []queryResult{{ID: "other", Title: "other"}, {IsGroup: true}, {ID: "another", Title: "another"}}, nil, nil, nil, 0, false)
			test.act(a)
			if !test.manual && a.pendingSelection != nil || test.manual && (a.pendingSelection == nil || a.pendingSelection.querySnapshot != nil) {
				t.Fatal("explicit input retained the pending undo target")
			}
			if test.manual {
				selected := a.results[a.selected]
				a.applyResults(a.query.QueryID, []queryResult{{Title: "target"}, selected}, nil, nil, nil, 0, false)
				a.applyResults(a.query.QueryID, []queryResult{{Title: "target"}, selected}, nil, nil, nil, 0, true)
				if a.selected != 1 || a.pendingSelection != nil {
					t.Fatal("late matching results must not override explicit selection")
				}
			}
		})
	}
}
