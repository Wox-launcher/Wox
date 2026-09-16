package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

func TestWebSearchDisabledMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	original := `[{"Keyword":"g","Enabled":true,"Extra":{"id":9007199254740993}},{"Keyword":"off","Enabled":false},{"Keyword":"legacy"},{"Keyword":"already","Disabled":false,"Enabled":true},null]`
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
	migration := &webSearchDisabledMigration{}
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
		assertWebSearchDisabled(t, entries[0], false)
		if string(entries[0]["Keyword"]) != `"g"` || !strings.Contains(string(entries[0]["Extra"]), "9007199254740993") {
			t.Fatal("unrelated JSON fields were lost")
		}
		if _, exists := entries[0]["Enabled"]; exists {
			t.Fatal("Enabled was kept on a migrated row")
		}
		assertWebSearchDisabled(t, entries[1], true)
		assertWebSearchDisabled(t, entries[2], true)
		assertWebSearchDisabled(t, entries[3], false)
		if string(entries[3]["Enabled"]) != "true" {
			t.Fatal("already-migrated Enabled leftover should stay untouched")
		}
		if entries[4] != nil {
			t.Fatal("null entry was dropped")
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
	for _, value := range []string{"", "{broken", "null", "[]", `[{"Enabled":false,"Disabled":true}]`} {
		if got, changed := migrateWebSearchDisabledJSON(value); changed || got != value {
			t.Fatalf("modified unsupported input: %q", value)
		}
	}
}

func assertWebSearchDisabled(t *testing.T, entry map[string]json.RawMessage, want bool) {
	t.Helper()
	var disabled bool
	if err := json.Unmarshal(entry["Disabled"], &disabled); err != nil || disabled != want {
		t.Fatalf("Disabled = %s, want %v", entry["Disabled"], want)
	}
}
