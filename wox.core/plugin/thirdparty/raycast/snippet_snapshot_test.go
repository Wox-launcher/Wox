package raycast

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"wox/common"
	"wox/database"
	"wox/setting"
)

// TestSnippetSnapshotUsesSyncedTable checks repeat-import decisions against stored table rows.
func TestSnippetSnapshotUsesSyncedTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}); err != nil {
		t.Fatal(err)
	}
	store := setting.NewPluginSettingStore(db, common.SnippetsPluginID)
	raw := `[{"id":"one","name":"Address","keyword":";addr ","text":"hello","importSource":"raycast","importId":"source"}]`
	if err := store.Set("snippetsTable", raw); err != nil {
		t.Fatal(err)
	}
	var snapshot woxSnapshot
	readSnippetsSnapshotFromStore(&snapshot, store)
	payload := Payload{}
	payload.Snippets.Snippets = []Snippet{{ID: "source", Title: "Address", Keyword: ";addr ", Text: "hello"}}
	rows := mapSnippetRows(payload, snapshot)
	if len(rows) != 1 || rows[0].selectable || rows[0].status != statusExists {
		t.Fatalf("repeat import: %+v", rows)
	}
	payload.Snippets.Snippets[0].Text = "changed"
	rows = mapSnippetRows(payload, snapshot)
	if rows[0].selectable || rows[0].status != statusConflict {
		t.Fatalf("changed source: %+v", rows)
	}
}
