package notes

import (
	"testing"
	"wox/common"
	"wox/database"
	"wox/setting"
)

// createTestGroup returns the stable ID allocated by the repository.
func createTestGroup(t *testing.T, r *Repository, name, parent string) string {
	t.Helper()
	if err := r.Organize(common.NotesTreeChange{Action: "create", Name: name, ParentID: parent}); err != nil {
		t.Fatal(err)
	}
	tree, err := readNotesTree(r.store)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range tree.Groups {
		if g.Name == name {
			return g.ID
		}
	}
	t.Fatal("group missing")
	return ""
}

func TestNotesTreeMoveDeletePreservesDocument(t *testing.T) {
	r, _ := newRepositoryForTest(t)
	a := createTestGroup(t, r, "A", "")
	b := createTestGroup(t, r, "B", a)
	draft, _ := r.Create()
	saved, _, err := r.Save(draft.ID, draft.Revision, ParseMarkdown("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Organize(common.NotesTreeChange{Action: "move", ID: saved.ID, ParentID: b}); err != nil {
		t.Fatal(err)
	}
	if err = r.Organize(common.NotesTreeChange{Action: "move", ID: a, Group: true, ParentID: b}); err == nil {
		t.Fatal("accepted descendant cycle")
	}
	if err = r.Organize(common.NotesTreeChange{Action: "delete", ID: a, Group: true}); err != nil {
		t.Fatal(err)
	}
	tree, _ := readNotesTree(r.store)
	groups := map[string]common.NoteGroup{}
	for _, g := range tree.Groups {
		groups[g.ID] = g
	}
	if !groups[a].Deleted || groups[b].ParentID != "" {
		t.Fatalf("delete lost hierarchy: %+v", tree)
	}
	if len(tree.Placements) != 1 || tree.Placements[0].ParentID != b {
		t.Fatal("descendant note moved out of its subgroup")
	}
	after, _ := r.Get(saved.ID)
	if after.Revision != saved.Revision || ToMarkdown(after.Document) != ToMarkdown(saved.Document) {
		t.Fatal("organization modified document")
	}
	if err = r.Organize(common.NotesTreeChange{Action: "delete", ID: b, Group: true}); err != nil {
		t.Fatal(err)
	}
	tree, _ = readNotesTree(r.store)
	if tree.Placements[0].ParentID != "" {
		t.Fatal("direct note not promoted")
	}
}

func TestNotesTreeOrderAndRollback(t *testing.T) {
	r, db := newRepositoryForTest(t)
	a := createTestGroup(t, r, "A", "")
	b := createTestGroup(t, r, "B", "")
	if err := r.Organize(common.NotesTreeChange{Action: "move", ID: a, Group: true, BeforeID: b}); err != nil {
		t.Fatal(err)
	}
	before, _ := readNotesTree(r.store)
	var count int64
	db.Model(&database.Oplog{}).Count(&count)
	if err := r.Organize(common.NotesTreeChange{Action: "move", ID: b, Group: true, ParentID: a, BeforeID: "missing"}); err == nil {
		t.Fatal("accepted missing sibling")
	}
	after, _ := readNotesTree(r.store)
	var afterCount int64
	db.Model(&database.Oplog{}).Count(&afterCount)
	if count != afterCount {
		t.Fatal("failed move wrote oplogs")
	}
	for _, g := range after.Groups {
		for _, old := range before.Groups {
			if old.ID == g.ID && old != g {
				t.Fatal("failed move modified group")
			}
		}
	}
}

func TestNotesTreeLateChildrenAndRemoteCycles(t *testing.T) {
	r, _ := newRepositoryForTest(t)
	for _, g := range []common.NoteGroup{{ID: "a", Deleted: true, ParentID: "b"}, {ID: "b", Name: "B"}, {ID: "c", ParentID: "d"}, {ID: "d", ParentID: "c"}} {
		if err := r.store.ApplyWithSync([]setting.PluginSettingChange{{Key: noteGroupPrefix + g.ID, Value: g, Syncable: true}}); err != nil {
			t.Fatal(err)
		}
	}
	tree, _ := readNotesTree(r.store)
	groups := map[string]common.NoteGroup{}
	for _, g := range tree.Groups {
		groups[g.ID] = g
	}
	if common.ResolveNoteParent("a", groups) != "b" {
		t.Fatal("late child not recovered through deleted ancestor")
	}
	for _, id := range []string{"missing", "c", "d"} {
		if common.ResolveNoteParent(id, groups) != "" {
			t.Fatal("malformed remote tree not exposed at root")
		}
	}
}

// TestNotesTreeWriteFailureRollsBack exercises the database batch, not just command validation.
func TestNotesTreeWriteFailureRollsBack(t *testing.T) {
	r, db := newRepositoryForTest(t)
	a := createTestGroup(t, r, "A", "")
	b := createTestGroup(t, r, "B", "")
	before, _ := readNotesTree(r.store)
	var count int64
	db.Model(&database.Oplog{}).Count(&count)
	// SQLite does not allow bound variables in trigger definitions; the generated UUID is trusted test data.
	if err := db.Exec("CREATE TRIGGER fail_tree_update BEFORE UPDATE ON plugin_settings WHEN NEW.key = '" + noteGroupPrefix + b + "' BEGIN SELECT RAISE(ABORT, 'injected failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	if err := r.Organize(common.NotesTreeChange{Action: "move", ID: a, Group: true, BeforeID: b}); err == nil {
		t.Fatal("injected failure was ignored")
	}
	after, _ := readNotesTree(r.store)
	var afterCount int64
	db.Model(&database.Oplog{}).Count(&afterCount)
	if count != afterCount {
		t.Fatal("partial oplog batch committed")
	}
	for _, g := range after.Groups {
		for _, old := range before.Groups {
			if old.ID == g.ID && old != g {
				t.Fatal("partial sibling reorder committed")
			}
		}
	}
}

func TestNotesTreeNoteOrder(t *testing.T) {
	r, _ := newRepositoryForTest(t)
	group := createTestGroup(t, r, "Group", "")
	var notes []common.NoteRecord
	for _, text := range []string{"first", "second", "third"} {
		draft, err := r.Create()
		if err != nil {
			t.Fatal(err)
		}
		note, _, err := r.Save(draft.ID, draft.Revision, ParseMarkdown(text))
		if err != nil {
			t.Fatal(err)
		}
		notes = append(notes, note)
	}
	for _, note := range notes {
		if err := r.Organize(common.NotesTreeChange{Action: "move", ID: note.ID, ParentID: group, First: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Organize(common.NotesTreeChange{Action: "move", ID: notes[0].ID, ParentID: group, BeforeID: notes[1].ID}); err != nil {
		t.Fatal(err)
	}
	positions := map[string]common.NotePlacement{}
	tree, err := readNotesTree(r.store)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range tree.Placements {
		positions[p.NoteID] = p
	}
	if positions[notes[2].ID].Order != 0 || positions[notes[0].ID].Order != 1 || positions[notes[1].ID].Order != 2 {
		t.Fatal("manual note order not persisted")
	}
	for _, note := range notes {
		after, err := r.Get(note.ID)
		if err != nil || after.Revision != note.Revision {
			t.Fatal("organization changed document revision")
		}
	}

}
