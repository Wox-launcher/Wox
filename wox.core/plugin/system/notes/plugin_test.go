package notes

import (
	"context"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/util/fuzzymatch"
)

func TestPluginQueryRoutesListNewSearchAndDeleted(t *testing.T) {
	repository, _ := newRepositoryForTest(t)
	p := &Plugin{repository: repository}
	created, err := repository.Create()
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if _, _, err := repository.Save(created.ID, created.Revision, common.NoteDocument{Blocks: []common.NoteBlock{{Text: "中文 Roadmap"}}}); err != nil {
		t.Fatalf("save searchable note: %v", err)
	}
	list := p.Query(context.Background(), plugin.Query{})
	if len(list.Results) != 1 || list.Results[0].Id != created.ID {
		t.Fatalf("root query did not list notes: %#v", list.Results)
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
