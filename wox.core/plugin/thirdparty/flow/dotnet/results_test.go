package dotnet

import (
	"testing"

	"wox/plugin"
)

func TestSplitResultUpdateKeepsPublishedRowsAndPushesOnlyNewOnes(t *testing.T) {
	shown := []shownDotNetRow{
		{ID: "local", Title: "192.168.0.1", SubTitle: "local"},
		{ID: "link", Title: "fe80::1", SubTitle: "link"},
	}
	rows := []plugin.QueryResult{
		{Title: "192.168.0.1", SubTitle: "local"},
		{Title: "203.0.113.5", SubTitle: "public"},
		{Title: "fe80::1", SubTitle: "link"},
	}

	updates, extra, next := splitResultUpdate(shown, rows)
	if len(updates) != 2 || len(extra) != 1 {
		t.Fatalf("updates %d extra %d", len(updates), len(extra))
	}
	if updates[0].Id != "local" || updates[1].Id != "link" || *updates[1].Title != "fe80::1" {
		t.Fatalf("updates = %#v", updates)
	}
	if extra[0].Title != "203.0.113.5" || extra[0].Id == "" || extra[0].Id == "local" || extra[0].Id == "link" {
		t.Fatalf("extra = %#v", extra[0])
	}
	if len(next) != 3 || next[0].ID != "local" || next[1].ID != extra[0].Id || next[2].ID != "link" {
		t.Fatalf("next = %#v", next)
	}
}

func TestSplitResultUpdateRenamesARowInPlace(t *testing.T) {
	shown := []shownDotNetRow{{ID: "row", Title: "placeholder"}}
	rows := []plugin.QueryResult{{Title: "updated"}}

	updates, extra, next := splitResultUpdate(shown, rows)
	if len(extra) != 0 || len(updates) != 1 || updates[0].Id != "row" || *updates[0].Title != "updated" {
		t.Fatalf("updates %#v extra %#v", updates, extra)
	}
	if len(next) != 1 || next[0].ID != "row" || next[0].Title != "updated" {
		t.Fatalf("next %#v", next)
	}
}

func TestSplitResultUpdateBeforePublishPushesEveryRow(t *testing.T) {
	rows := []plugin.QueryResult{{Title: "192.168.0.1", SubTitle: "local"}, {Title: "192.168.10.1", SubTitle: "local"}}
	updates, extra, next := splitResultUpdate(nil, rows)
	if len(updates) != 0 || len(extra) != 2 || len(next) != 2 {
		t.Fatalf("updates %d extra %d next %d", len(updates), len(extra), len(next))
	}
	if extra[0].Id == "" || extra[1].Id == "" || extra[0].Id == extra[1].Id {
		t.Fatalf("extra ids %#v %#v", extra[0].Id, extra[1].Id)
	}
}

func TestSplitResultUpdateKeepsARowThatDisappeared(t *testing.T) {
	shown := []shownDotNetRow{{ID: "public", Title: "203.0.113.5"}, {ID: "local", Title: "192.168.0.1"}}
	rows := []plugin.QueryResult{{Title: "192.168.0.1"}}

	updates, extra, next := splitResultUpdate(shown, rows)
	if len(updates) != 1 || updates[0].Id != "local" || len(extra) != 0 {
		t.Fatalf("updates %#v extra %#v", updates, extra)
	}
	if len(next) != 2 || next[0].ID != "local" || next[1].ID != "public" {
		t.Fatalf("next %#v", next)
	}
}
