package launcher

import (
	"context"
	"testing"
	"wox/plugin"
	"wox/util"
)

func TestRemoveResultSelection(t *testing.T) {
	cases := []struct {
		name      string
		rows      []queryResult
		selected  int
		option    plugin.RemoveResultOption
		wantID    string
		wantCount int
	}{
		{"next", []queryResult{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 1, plugin.RemoveResultOption{ResultId: "b"}, "c", 2},
		{"previous at end", []queryResult{{ID: "a"}, {ID: "b"}}, 1, plugin.RemoveResultOption{ResultId: "b"}, "a", 1},
		{"preserve other", []queryResult{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 2, plugin.RemoveResultOption{ResultId: "a"}, "c", 2},
		{"explicit", []queryResult{{ID: "a"}, {ID: "b"}, {ID: "c"}}, 1, plugin.RemoveResultOption{ResultId: "b", SelectedResultId: "a"}, "a", 2},
		{"missing target", []queryResult{{ID: "a"}, {ID: "b"}}, 0, plugin.RemoveResultOption{ResultId: "a", SelectedResultId: "missing"}, "b", 1},
		{"empty group", []queryResult{{IsGroup: true}, {ID: "a"}, {IsGroup: true}, {ID: "b"}}, 1, plugin.RemoveResultOption{ResultId: "a"}, "b", 2},
		{"last", []queryResult{{IsGroup: true}, {ID: "a"}}, 1, plugin.RemoveResultOption{ResultId: "a"}, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := resultController{results: tc.rows, selected: tc.selected}
			if !state.removeResult(tc.option) || len(state.results) != tc.wantCount {
				t.Fatalf("rows: %+v", state.results)
			}
			selected := ""
			if state.selected >= 0 {
				selected = state.results[state.selected].ID
			}
			if selected != tc.wantID {
				t.Fatalf("selected %q, want %q", selected, tc.wantID)
			}
		})
	}
}

func TestRemoveResultRejectsStaleScope(t *testing.T) {
	for _, scope := range [][2]string{{"other", "query"}, {"session", "old"}} {
		app := bindTestApp(&App{sessionID: "session", querySession: queryController{query: plainQuery{QueryID: "query"}}, resultList: resultController{resultsQueryID: "query", results: []queryResult{{ID: "row"}}}})
		ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), scope[0]), scope[1])
		removed, err := app.RemoveResult(ctx, plugin.RemoveResultOption{ResultId: "row"})
		if err != nil || removed || len(app.resultList.results) != 1 {
			t.Fatalf("removed=%v err=%v", removed, err)
		}
	}
}
