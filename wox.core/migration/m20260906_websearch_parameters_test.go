package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

// TestWebSearchParametersMigration covers field preservation, local settings, sync and repeat-safe execution.
func TestWebSearchParametersMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	original := `[{"Title":"Find {query}","Urls":["https://example.com/?q={query}&lower={lower_query}","https://example.org/{upper_query}/{query}"],"Browser":"system","Keyword":"{query}","Extra":{"id":9007199254740993}},null]`
	for _, row := range []database.PluginSetting{
		{PluginID: webSearchPluginID, Key: "webSearches", Value: original},
		{PluginID: webSearchPluginID, Key: "webSearches@windows", Value: original, IsLocal: true},
		{PluginID: webSearchPluginID, Key: "unrelated", Value: original},
		{PluginID: "other", Key: "webSearches", Value: original},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	migration := &webSearchParametersMigration{}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error { return migration.Up(context.Background(), tx) }); err != nil {
			t.Fatal(err)
		}
	}
	var rows []database.PluginSetting
	if err := db.Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.PluginID != webSearchPluginID || row.Key == "unrelated" {
			if row.Value != original {
				t.Fatal("unrelated setting was changed")
			}
			continue
		}
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.Value), &entries); err != nil {
			t.Fatal(err)
		}
		if string(entries[0]["Title"]) != `"Find {wox:parameter?name=query}"` || string(entries[0]["Browser"]) != `"default"` {
			t.Fatal(row.Value)
		}
		var urls []string
		if err := json.Unmarshal(entries[0]["Urls"], &urls); err != nil || len(urls) != 2 {
			t.Fatal(row.Value)
		}
		if urls[0] != "https://example.com/?q={wox:parameter?name=query}&lower={wox:parameter?name=query&case=lower}" || urls[1] != "https://example.org/{wox:parameter?name=query&case=upper}/{wox:parameter?name=query}" {
			t.Fatal(row.Value)
		}
		if string(entries[0]["Keyword"]) != `"{query}"` || !strings.Contains(string(entries[0]["Extra"]), "9007199254740993") || entries[1] != nil {
			t.Fatal("unrelated JSON fields were lost")
		}
		if row.Key == "webSearches@windows" && !row.IsLocal {
			t.Fatal("local setting became syncable")
		}
	}
	var ops []database.Oplog
	if err := db.Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Key != "webSearches" {
		t.Fatalf("unexpected sync operations: %+v", ops)
	}
	for _, value := range []string{"", "{broken", "null", "[]", `[{"Title":false,"Urls":42,"Browser":42}]`} {
		if got, changed := migrateWebSearchParametersJSON(value); changed || got != value {
			t.Fatalf("modified unsupported input: %q", value)
		}
	}
}

// TestWebSearchMigrationRollback ensures a failed sync write cannot leave partially migrated settings.
func TestWebSearchMigrationRollback(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	original := `[{"Title":"{query}","Urls":["https://example.com/{query}"],"Browser":"default"}]`
	if err := db.Create(&database.PluginSetting{PluginID: webSearchPluginID, Key: "webSearches", Value: original}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TRIGGER reject_websearch_oplog BEFORE INSERT ON oplogs BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error; err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(func(tx *gorm.DB) error { return (&webSearchParametersMigration{}).Up(context.Background(), tx) })
	if err == nil {
		t.Fatal("expected sync write failure")
	}
	var row database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", webSearchPluginID, "webSearches").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Value != original {
		t.Fatal("failed migration changed stored data")
	}
}
