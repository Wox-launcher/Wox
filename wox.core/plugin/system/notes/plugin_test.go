package notes

import (
	"context"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/fuzzymatch"
)

func TestPluginQueryRoutesListNewSearchAndDeleted(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	empty, err := repository.Create()
	if err != nil {
		t.Fatalf("create empty note: %v", err)
	}
	if err := repository.write(common.NoteRecord{ID: empty.ID, Document: EmptyDocument(), Revision: empty.Revision}); err != nil {
		t.Fatalf("seed empty note: %v", err)
	}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "中文 Roadmap"}}}); err != nil {
		t.Fatalf("save searchable note: %v", err)
	}
	list := p.Query(context.Background(), plugin.Query{})
	if len(list.Results) != 1 || list.Results[0].Id != created.ID {
		t.Fatalf("root query listed empty drafts: %#v", list.Results)
	}
	search := p.Query(context.Background(), plugin.Query{Command: "search"})
	if len(search.Results) != 1 || search.Results[0].Id != created.ID {
		t.Fatalf("search did not return note: %#v", search.Results)
	}
	if _, err := repository.Delete(created.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	deleted := p.Query(context.Background(), plugin.Query{Command: "deleted"})
	if len(deleted.Results) != 1 || deleted.Results[0].Id != created.ID || deleted.Results[0].Actions[0].Id != "restore" {
		t.Fatalf("deleted query did not expose restore: %#v", deleted.Results)
	}
	newResult := p.Query(context.Background(), plugin.Query{Command: "new"})
	if len(newResult.Results) != 1 || newResult.Results[0].Id != "notes:new" {
		t.Fatalf("new query mismatch: %#v", newResult.Results)
	}
	exactNewResult := p.Query(context.Background(), plugin.Query{Search: "new"})
	if len(exactNewResult.Results) != 1 || exactNewResult.Results[0].Id == "notes:new" {
		t.Fatalf("new without command delimiter must remain a note search: %#v", exactNewResult.Results)
	}
}

func TestNoteResultsShowTitleAndUpdatedAtTail(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	saved, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Roadmap\nmore body"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	list := p.Query(context.Background(), plugin.Query{})
	if len(list.Results) != 1 {
		t.Fatalf("expected one note result, got %#v", list.Results)
	}
	result := list.Results[0]
	if result.Title != "Roadmap" {
		t.Fatalf("note result title = %q", result.Title)
	}
	if result.SubTitle != "" {
		t.Fatalf("note result should omit subtitle, got %q", result.SubTitle)
	}
	wantTail := util.FormatTimestamp(saved.UpdatedAt)
	if len(result.Tails) != 1 || result.Tails[0].Text != wantTail {
		t.Fatalf("note result tail = %#v, want %q", result.Tails, wantTail)
	}
}

func TestNoteResultsPinActionAndPinnedGroup(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	saved, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Pinned note"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}
	unpinned := p.Query(context.Background(), plugin.Query{})
	if len(unpinned.Results) != 1 {
		t.Fatalf("expected one note result, got %#v", unpinned.Results)
	}
	if unpinned.Results[0].Group != "i18n:plugin_notes_group_today" || unpinned.Results[0].GroupScore != 90 {
		t.Fatalf("unpinned note group = %#v", unpinned.Results[0])
	}
	assertNoteActionIcons(t, unpinned.Results[0].Actions)
	if unpinned.Results[0].Actions[1].Name != "i18n:plugin_notes_action_pin" || unpinned.Results[0].Actions[1].Icon.String() != common.PinIcon.String() {
		t.Fatalf("pin action = %#v", unpinned.Results[0].Actions[1])
	}

	if _, err := repository.SetPinned(saved.ID, true); err != nil {
		t.Fatalf("pin note: %v", err)
	}
	pinned := p.Query(context.Background(), plugin.Query{})
	if len(pinned.Results) != 1 {
		t.Fatalf("expected pinned note result, got %#v", pinned.Results)
	}
	if pinned.Results[0].Group != "i18n:plugin_notes_group_pinned" || pinned.Results[0].GroupScore != 100 {
		t.Fatalf("pinned note group = %#v", pinned.Results[0])
	}
	if pinned.Results[0].Actions[1].Name != "i18n:plugin_notes_action_unpin" || pinned.Results[0].Actions[1].Icon.String() != common.UnpinIcon.String() {
		t.Fatalf("unpin action = %#v", pinned.Results[0].Actions[1])
	}
}

func assertNoteActionIcons(t *testing.T, actions []plugin.QueryResultAction) {
	t.Helper()
	want := map[string]string{
		"open":            common.OpenIcon.String(),
		"pin":             common.PinIcon.String(),
		"copy-link":       common.CopyIcon.String(),
		"export-markdown": common.InstallIcon.String(),
		"export-text":     common.TextIcon.String(),
		"export-html":     common.InstallIcon.String(),
		"delete":          common.TrashIcon.String(),
	}
	if len(actions) != len(want) {
		t.Fatalf("actions = %#v", actions)
	}
	for _, action := range actions {
		icon, ok := want[action.Id]
		if !ok || action.Icon.String() != icon || action.Icon.String() == "" {
			t.Fatalf("action %s icon = %q, want %q", action.Id, action.Icon.String(), icon)
		}
	}
}

func TestNoteResultGroupUsesClipboardStyleBuckets(t *testing.T) {
	now := util.GetSystemTimestamp()
	cases := []struct {
		record    common.NoteRecord
		wantGroup string
		wantScore int64
	}{
		{common.NoteRecord{PinnedAt: now, UpdatedAt: now}, "i18n:plugin_notes_group_pinned", 100},
		{common.NoteRecord{UpdatedAt: now}, "i18n:plugin_notes_group_today", 90},
		{common.NoteRecord{UpdatedAt: now - 1000*60*60*25}, "i18n:plugin_notes_group_yesterday", 80},
		{common.NoteRecord{UpdatedAt: now - 1000*60*60*48}, "i18n:plugin_notes_group_history", 10},
		{common.NoteRecord{PinnedAt: now, DeletedAt: now, UpdatedAt: now}, "i18n:plugin_notes_group_today", 90},
	}
	for _, testCase := range cases {
		group, score := noteResultGroup(testCase.record)
		if group != testCase.wantGroup || score != testCase.wantScore {
			t.Fatalf("group(%#v) = %q/%d, want %q/%d", testCase.record, group, score, testCase.wantGroup, testCase.wantScore)
		}
	}
}

func TestNotesSearchMatcherSupportsPinyin(t *testing.T) {
	if result := fuzzymatch.FuzzyMatch("中文 Roadmap", "zhongwen", true); !result.IsMatch {
		t.Fatal("Notes pinyin search matcher did not match Chinese title")
	}
}

func TestNoteDeepLinkUsesStablePluginID(t *testing.T) {
	if got := noteDeepLink("note-id"); got != "wox://plugin/"+common.NotesPluginID+"?action=open&id=note-id" {
		t.Fatalf("unexpected deep link: %s", got)
	}
}

func TestNotesMetadataEnablesMRU(t *testing.T) {
	metadata := (&Plugin{}).GetMetadata()
	if !metadata.IsSupportFeature(plugin.MetadataFeatureMRU) {
		t.Fatal("notes plugin must declare the MRU feature")
	}
	params, err := metadata.GetFeatureParamsForMRU()
	if err != nil || params.HashBy != "scorekey" {
		t.Fatalf("MRU hash params = %#v, err=%v", params, err)
	}
}

func TestNotesQueryActionsCarryMRUContext(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Roadmap"}}}); err != nil {
		t.Fatalf("save note: %v", err)
	}
	list := p.Query(context.Background(), plugin.Query{})
	if len(list.Results) != 1 {
		t.Fatalf("expected one note result, got %#v", list.Results)
	}
	for _, action := range list.Results[0].Actions {
		if action.ContextData[notesMRUContextKey] != created.ID {
			t.Fatalf("action %s context = %#v", action.Id, action.ContextData)
		}
	}
	newResult := p.Query(context.Background(), plugin.Query{Command: "new"})
	if len(newResult.Results) != 1 || newResult.Results[0].ScoreKey != "note:new" {
		t.Fatalf("new result = %#v", newResult.Results)
	}
	if newResult.Results[0].Actions[0].ContextData[notesMRUContextKey] != notesMRUNewID {
		t.Fatalf("new action context = %#v", newResult.Results[0].Actions[0].ContextData)
	}
}

func TestNotesMRURestoreRebuildsCurrentNote(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	saved, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "Updated title"}}})
	if err != nil {
		t.Fatalf("save note: %v", err)
	}

	restored, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{notesMRUContextKey: saved.ID},
	})
	if err != nil {
		t.Fatalf("restore note: %v", err)
	}
	if restored.Id != saved.ID || restored.Title != "Updated title" || restored.Group != "" {
		t.Fatalf("restored note = %#v", restored)
	}
	if restored.ScoreKey != "note:"+saved.ID || restored.Actions[0].ContextData[notesMRUContextKey] != saved.ID {
		t.Fatalf("restored note identity = %#v", restored)
	}

	newResult, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{notesMRUContextKey: notesMRUNewID},
	})
	if err != nil || newResult.Id != "notes:new" {
		t.Fatalf("restore new note = %#v, err=%v", newResult, err)
	}

	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{}); err == nil {
		t.Fatal("empty context should fail restore")
	}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{notesMRUContextKey: "missing"},
	}); err == nil {
		t.Fatal("missing note should fail restore")
	}
	if _, err := repository.Delete(saved.ID); err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if _, err := p.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{notesMRUContextKey: saved.ID},
	}); err == nil {
		t.Fatal("deleted note should fail restore")
	}
}
