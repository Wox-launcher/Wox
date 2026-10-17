package system

import (
	"context"
	"os"
	"testing"
	"time"

	"wox/plugin"
	clipboardutil "wox/util/clipboard"
)

type clipboardImportAPI struct {
	plugin.API
	settings map[string]string
}

func (a *clipboardImportAPI) GetSetting(_ context.Context, key string) string {
	if a.settings == nil {
		return ""
	}
	return a.settings[key]
}

func (a *clipboardImportAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	if a.settings == nil {
		a.settings = map[string]string{}
	}
	a.settings[option.Key] = option.Value
	return plugin.SetSettingResult{Success: true}
}

func (a *clipboardImportAPI) Log(context.Context, plugin.LogLevel, string) {}

type clipboardImportDB struct {
	ClipboardDBInterface
	records []ClipboardRecord
	fail    bool
}

func (d *clipboardImportDB) GetRecent(context.Context, int, int) ([]ClipboardRecord, error) {
	return append([]ClipboardRecord(nil), d.records...), nil
}

func (d *clipboardImportDB) Insert(_ context.Context, record ClipboardRecord) error {
	if d.fail {
		return os.ErrClosed
	}
	d.records = append(d.records, record)
	return nil
}

func (d *clipboardImportDB) Delete(_ context.Context, id string) error {
	kept := d.records[:0]
	for _, record := range d.records {
		if record.ID != id {
			kept = append(kept, record)
		}
	}
	d.records = kept
	return nil
}

func TestImportClipboardDedupesPromotesAndKeepsCapacity(t *testing.T) {
	db := &clipboardImportDB{records: []ClipboardRecord{{
		ID: "old", Type: string(clipboardutil.ClipboardTypeText), Content: "keep", Timestamp: 10,
	}}}
	plugin := &ClipboardPlugin{api: &clipboardImportAPI{}, db: db, maxHistoryCount: 1}
	now := time.Now()
	results := plugin.ImportClipboard(context.Background(), []ImportedClipboard{
		{Text: "keep", Pinned: true, CreatedAt: now},
		{Text: "fresh", CreatedAt: now},
		{Text: "extra", CreatedAt: now},
		{Text: "old", CreatedAt: now.Add(-400 * 24 * time.Hour)},
	})
	if results[0].Status != clipboardImportImported || results[1].Status != clipboardImportImported || results[2].Status != clipboardImportCapacity || results[3].Status != clipboardImportExpired {
		t.Fatalf("results = %+v", results)
	}
	if len(db.records) != 1 || db.records[0].Content != "fresh" {
		t.Fatalf("history = %+v", db.records)
	}
}

func TestClipboardImageCommitRemovesTheNewFile(t *testing.T) {
	dest := t.TempDir() + "/clipboard_new.png"
	if err := os.WriteFile(dest, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin := &ClipboardPlugin{db: &clipboardImportDB{fail: true}}
	err := plugin.commitClipboardRecord(context.Background(), ClipboardRecord{ID: "1", Type: string(clipboardutil.ClipboardTypeImage)}, dest)
	if err == nil {
		t.Fatal("insert succeeded")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("new image remained: %v", statErr)
	}
}
