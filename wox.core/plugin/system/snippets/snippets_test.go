package snippets

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wox/common/icons"
	"wox/database"
	"wox/plugin"
	"wox/plugin/plugintest"
	"wox/plugin/system/migrateapply"
	"wox/setting"
)

func testRepository(t *testing.T) (*Repository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	return NewRepository(setting.NewPluginSettingStore(db, pluginID)), db
}

type snippetSettingsAPI struct {
	plugintest.StubAPI
	last plugin.SetSettingOption
}

func (a *snippetSettingsAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	a.last = option
	return plugin.SetSettingResult{Success: true}
}

func TestPluginSettingsUseLocalSwitchAndDedicatedIcon(t *testing.T) {
	api := &snippetSettingsAPI{}
	p := &Plugin{api: api}
	metadata := p.GetMetadata()
	if metadata.Icon != icons.Get(icons.PluginSnippets).String() || icons.Get(icons.PluginSnippets).IsEmpty() {
		t.Fatal("snippet plugin icon is missing")
	}
	if handled, err := p.HandleSettingUpdate(context.Background(), autoExpandSetting, "false"); !handled || err != nil || !api.last.IsLocal {
		t.Fatalf("local switch: handled=%v option=%+v error=%v", handled, api.last, err)
	}
}

func TestAutomaticExpansionSwitchRemainsLocal(t *testing.T) {
	r, db := testRepository(t)
	p := &Plugin{repository: r}
	if handled, err := p.HandleSettingUpdate(context.Background(), autoExpandSetting, "true"); !handled || err != nil {
		t.Fatalf("local switch: handled=%v error=%v", handled, err)
	}
	var row database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", pluginID, autoExpandSetting).First(&row).Error; err != nil || !row.IsLocal {
		t.Fatalf("local switch row: %+v %v", row, err)
	}
	var count int64
	if err := db.Model(&database.Oplog{}).Where("entity_id = ? AND key = ?", pluginID, autoExpandSetting).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("local switch sync entries: %d %v", count, err)
	}
}

func TestSettingsTableUsesOneSyncedValue(t *testing.T) {
	r, db := testRepository(t)
	p := &Plugin{repository: r, api: plugintest.StubAPI{}}
	ctx := context.Background()
	raw := `[{"name":"Address","keyword":";a ","text":"first"},{"name":"Phone","keyword":";p","text":"second"}]`
	if handled, err := p.HandleSettingUpdate(ctx, snippetsSetting, raw); !handled || err != nil {
		t.Fatalf("save: %t %v", handled, err)
	}
	records, err := r.List()
	if err != nil || len(records) != 2 || records[0].Keyword != ";a " || records[0].ID == "" || records[1].ID == "" {
		t.Fatalf("records: %#v %v", records, err)
	}
	var rows []database.PluginSetting
	if err := db.Where("plugin_id = ?", pluginID).Find(&rows).Error; err != nil || len(rows) != 1 || rows[0].Key != snippetsSetting || rows[0].IsLocal {
		t.Fatalf("settings: %#v %v", rows, err)
	}
	var oplog database.Oplog
	if err := db.Where("entity_id = ? AND key = ? AND operation = ?", pluginID, snippetsSetting, "upsert").First(&oplog).Error; err != nil {
		t.Fatalf("table sync: %v", err)
	}
	shown, ok := p.GetSettingOverride(ctx, snippetsSetting)
	if !ok || !strings.Contains(shown, `"keyword":";a "`) {
		t.Fatalf("keyword display: %q", shown)
	}
	if results := p.Query(ctx, plugin.Query{}).Results; len(results) != 2 || results[0].SubTitle != ";a " {
		t.Fatalf("query: %#v", results)
	}
	if handled, err := p.HandleSettingUpdate(ctx, snippetsSetting, `[]`); !handled || err != nil {
		t.Fatalf("clear: %t %v", handled, err)
	}
	if records, err := r.List(); err != nil || len(records) != 0 {
		t.Fatalf("cleared: %#v %v", records, err)
	}
}

func TestTableSyncRefreshesExpansion(t *testing.T) {
	r, _ := testRepository(t)
	p := &Plugin{repository: r, api: &snippetSettingsAPI{}, expansion: &expansion{}}
	ctx := context.Background()
	raw := `[{"id":"remote","name":"Remote","keyword":";one","text":"text"}]`
	if err := r.store.Set(snippetsSetting, raw); err != nil {
		t.Fatal(err)
	}
	p.onSettingChanged(ctx, snippetsSetting, raw)
	if _, ok := p.expansion.engine.matcher.input(";one"); !ok {
		t.Fatal("synced keyword not indexed")
	}
	if err := r.store.Set(snippetsSetting, `[]`); err != nil {
		t.Fatal(err)
	}
	p.onSettingChanged(ctx, snippetsSetting, `[]`)
	if _, ok := p.expansion.engine.matcher.input(";one"); ok {
		t.Fatal("deleted keyword still indexed")
	}
}

func TestImportIsIdempotentAndPreservesEditedSnippet(t *testing.T) {
	r, _ := testRepository(t)
	item := migrateapply.Snippet{Source: "raycast", SourceID: "source-1", Fingerprint: "original", Title: "Address", Keyword: ";a ", Text: "hello"}
	if got := r.importOne(item); got.Status != "imported" {
		t.Fatalf("first: %+v", got)
	}
	if got := r.importOne(item); got.Status != "exists" {
		t.Fatalf("repeat: %+v", got)
	}
	records, err := r.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("records: %#v %v", records, err)
	}
	records[0].Text = "edited"
	raw, _ := json.Marshal(records)
	if err := r.saveTable(string(raw)); err != nil {
		t.Fatal(err)
	}
	if got := r.importOne(item); got.Status != "conflict" {
		t.Fatalf("edited import: %+v", got)
	}
	records, err = r.List()
	if err != nil || len(records) != 1 || records[0].Text != "edited" {
		t.Fatalf("overwritten: %#v %v", records, err)
	}
}

func TestTemplateVariablesAndClipboardFailure(t *testing.T) {
	now := time.Date(2026, 11, 7, 14, 3, 5, 0, time.UTC)
	calls := 0
	got, err := expandTemplate(`{wox:date}|{wox:date?format=yyyy/MM/dd HH:mm:ss}|{wox:clipboard_text}|{wox:clipboard_text}|{unknown}|{wox:uuid}`, now, func() (string, error) { calls++; return "内容", nil })
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(got, "|")
	if len(parts) != 6 || parts[0] != "2026-11-07" || parts[1] != "2026/11/07 14:03:05" || parts[2] != "内容" || parts[3] != "内容" || parts[4] != "{unknown}" || calls != 1 {
		t.Fatalf("got %q, calls=%d", got, calls)
	}
	if _, err := uuid.Parse(parts[5]); err != nil {
		t.Fatal(err)
	}
	literal, err := expandTemplate(`{wox:date?format=yyyy/2024/MM/dd}`, now, nil)
	if err != nil || literal != "2026/2024/11/07" {
		t.Fatalf("date format changed literal digits: %q, %v", literal, err)
	}
	unsupported := `{wox:date?format=MMM d, yyyy}`
	if got, err := expandTemplate(unsupported, now, nil); err != nil || got != unsupported {
		t.Fatalf("unsupported date format changed: %q, %v", got, err)
	}
	if _, err := expandTemplate("{wox:clipboard_text}", now, func() (string, error) { return "", errors.New("unavailable") }); err == nil {
		t.Fatal("expected clipboard error")
	}
}

func TestTemplatePreservesUnsupportedNestedPlaceholders(t *testing.T) {
	input := `before {unsupported {wox:clipboard_text}} {wox:date?format="{wox:uuid}"} {wox:date} {unfinished {wox:uuid}`
	read := false
	got, err := expandTemplate(input, time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), func() (string, error) {
		read = true
		return "private", nil
	})
	if err != nil || read || got != `before {unsupported {wox:clipboard_text}} {wox:date?format="{wox:uuid}"} 2026-11-07 {unfinished {wox:uuid}` {
		t.Fatalf("nested placeholders changed: got=%q read=%v err=%v", got, read, err)
	}
}

func TestMatcherLiteralLongestAndSyncedConflicts(t *testing.T) {
	var m matcher
	m.replace([]Record{{ID: "short", Keyword: "x "}, {ID: "long", Keyword: ";x "}, {ID: "c1", Keyword: ";conflict"}, {ID: "c2", Keyword: ";conflict"}})
	for _, c := range ";x" {
		if _, ok := m.input(string(c)); ok {
			t.Fatal("matched before space")
		}
	}
	if got, ok := m.input(" "); !ok || got.ID != "long" {
		t.Fatalf("match %#v %t", got, ok)
	}
	for _, c := range ";conflict" {
		if _, ok := m.input(string(c)); ok {
			t.Fatal("conflicting keyword must be disabled")
		}
	}
	m.reset()
	m.input(";x")
	m.backspace()
	if _, ok := m.input(" "); ok {
		t.Fatal("backspace must remove buffered input")
	}
	m.input(";x")
	m.input("界")
	if _, ok := m.input(" "); ok {
		t.Fatal("non-ASCII input must invalidate context")
	}
}

func TestEngineWaitsForReleaseAndInvalidatesQueuedWork(t *testing.T) {
	var e engine
	e.reload([]Record{{ID: "a", Keyword: ";a ", Text: "value"}})
	for i, c := range ";a " {
		event := inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 42, text: string(c)}
		if work := e.onInput(event); work != nil {
			t.Fatal("work started before key release")
		}
		if i != 2 {
			e.onInput(inputEvent{key: event.key, target: 42})
		}
	}
	work := e.onInput(inputEvent{key: 3, target: 42})
	if work == nil || work.deletes != 3 || !e.valid(*work) {
		t.Fatalf("work = %#v", work)
	}
	e.onInput(inputEvent{kind: inputText, down: true, key: 4, target: 42, text: "x"})
	if e.valid(*work) {
		t.Fatal("subsequent typing must cancel queued work")
	}
	e.reload([]Record{{ID: "a", Keyword: ";a ", Text: "value"}})
	for i, c := range ";a " {
		e.onInput(inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 42, text: string(c)})
		e.onInput(inputEvent{key: uint32(i + 1), target: 42})
	}
	// A fresh complete match must be invalidated by a focus change before it can run.
	e.reload([]Record{{ID: "a", Keyword: "x", Text: "value"}})
	e.onInput(inputEvent{kind: inputText, down: true, key: 4, target: 42, text: "x"})
	work = e.onInput(inputEvent{key: 4, target: 42})
	e.onInput(inputEvent{kind: inputText, down: true, key: 5, target: 43, text: "z"})
	if work == nil || e.valid(*work) {
		t.Fatal("focus change must cancel queued work")
	}
}

func TestEngineUndoCountsGraphemesAndCancelsOnInput(t *testing.T) {
	var e engine
	e.reload([]Record{{ID: "a", Keyword: ";a", Text: "A😀"}})
	for i, c := range ";a" {
		e.onInput(inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 10, text: string(c)})
		work := e.onInput(inputEvent{key: uint32(i + 1), target: 10})
		if work != nil {
			e.completed(*work, "A😀", false)
		}
	}
	e.onInput(inputEvent{kind: inputBackspace, down: true, key: 9, target: 10})
	undo := e.onInput(inputEvent{key: 9, target: 10})
	if undo == nil || !undo.undo || undo.deletes != 1 || undo.record.Text != ";a" {
		t.Fatalf("undo = %#v", undo)
	}
	e.onInput(inputEvent{kind: inputInvalidate, down: true})
	if e.valid(*undo) {
		t.Fatal("pointer input without a focus target must cancel queued undo")
	}
}

func TestEngineDoesNotUndoAmbiguousGraphemes(t *testing.T) {
	for _, expanded := range []string{"A👩‍💻", "e\u0301", "🇺🇸"} {
		var e engine
		e.reload([]Record{{ID: "a", Keyword: ";a", Text: expanded}})
		for i, c := range ";a" {
			e.onInput(inputEvent{kind: inputText, down: true, key: uint32(i + 1), target: 10, text: string(c)})
			if work := e.onInput(inputEvent{key: uint32(i + 1), target: 10}); work != nil {
				e.completed(*work, expanded, false)
			}
		}
		e.onInput(inputEvent{kind: inputBackspace, down: true, key: 9, target: 10})
		if undo := e.onInput(inputEvent{key: 9, target: 10}); undo != nil {
			t.Fatalf("unsafe undo for %q: %#v", expanded, undo)
		}
	}
}

func TestEngineUndoCountsCodepointsForVerifiedControls(t *testing.T) {
	for _, tc := range []struct {
		expanded string
		deletes  int
	}{{"A👩‍💻", 3}, {"e\u0301", 1}, {"🇺🇸", 1}} {
		var e engine
		e.reload([]Record{{ID: "a", Keyword: "x", Text: tc.expanded}})
		e.onInput(inputEvent{kind: inputText, down: true, key: 1, target: 10, text: "x"})
		work := e.onInput(inputEvent{key: 1, target: 10})
		if work == nil {
			t.Fatal("missing expansion")
		}
		e.completed(*work, tc.expanded, true)
		e.onInput(inputEvent{kind: inputBackspace, down: true, key: 8, target: 10})
		undo := e.onInput(inputEvent{key: 8, target: 10})
		if undo == nil || undo.deletes != tc.deletes {
			t.Fatalf("undo for %q: %#v", tc.expanded, undo)
		}
	}
}

func TestEngineAllowsCRLFUndoOnlyForVerifiedControls(t *testing.T) {
	for _, allow := range []bool{false, true} {
		var e engine
		e.reload([]Record{{ID: "a", Keyword: "x", Text: "a\r\nb"}})
		e.onInput(inputEvent{kind: inputText, down: true, key: 1, target: 10, text: "x"})
		work := e.onInput(inputEvent{key: 1, target: 10})
		if work == nil {
			t.Fatal("missing expansion")
		}
		e.completed(*work, "a\r\nb", allow)
		e.onInput(inputEvent{kind: inputBackspace, down: true, key: 8, target: 10})
		undo := e.onInput(inputEvent{key: 8, target: 10})
		if !allow && undo != nil {
			t.Fatal("unverified CRLF undo must remain disabled")
		}
		if allow && (undo == nil || undo.deletes != 2 || undo.record.Text != "x") {
			t.Fatalf("verified CRLF undo: %#v", undo)
		}
	}
}
