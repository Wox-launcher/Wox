package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

func TestWebViewKeepInBackgroundMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	original := `[{"Keyword":"x","CacheDisabled":false,"Extra":{"id":9007199254740993}},{"Keyword":"fresh","CacheDisabled":true},{"Keyword":"legacy"},{"Keyword":"already","KeepInBackground":false,"CacheDisabled":true},null]`
	for _, row := range []database.PluginSetting{
		{PluginID: webViewPluginID, Key: "sites", Value: original},
		{PluginID: webViewPluginID, Key: "sites@windows", Value: original, IsLocal: true},
		{PluginID: webViewPluginID, Key: "unrelated", Value: original},
		{PluginID: "other", Key: "sites", Value: original},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	migration := &webViewKeepInBackgroundMigration{}
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
		if row.PluginID != webViewPluginID || row.Key == "unrelated" {
			if row.Value != original {
				t.Fatal("unrelated setting was changed")
			}
			continue
		}
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row.Value), &entries); err != nil {
			t.Fatal(err)
		}
		assertWebViewKeepInBackground(t, entries[0], true)
		if string(entries[0]["Keyword"]) != `"x"` || !strings.Contains(string(entries[0]["Extra"]), "9007199254740993") {
			t.Fatal("unrelated JSON fields were lost")
		}
		if _, exists := entries[0]["CacheDisabled"]; exists {
			t.Fatal("CacheDisabled was kept on a migrated row")
		}
		assertWebViewKeepInBackground(t, entries[1], false)
		assertWebViewKeepInBackground(t, entries[2], true)
		assertWebViewKeepInBackground(t, entries[3], false)
		if string(entries[3]["CacheDisabled"]) != "true" {
			t.Fatal("already-migrated CacheDisabled leftover should stay untouched")
		}
		if entries[4] != nil {
			t.Fatal("null entry was dropped")
		}
		if row.Key == "sites@windows" && !row.IsLocal {
			t.Fatal("local setting became syncable")
		}
	}
	var ops []database.Oplog
	if err := db.Find(&ops).Error; err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Key != "sites" {
		t.Fatalf("unexpected sync operations: %+v", ops)
	}
	for _, value := range []string{"", "{broken", "null", "[]", `[{"CacheDisabled":true,"KeepInBackground":false}]`} {
		if got, changed := migrateWebViewKeepInBackgroundJSON(value); changed || got != value {
			t.Fatalf("modified unsupported input: %q", value)
		}
	}
}

func assertWebViewKeepInBackground(t *testing.T, entry map[string]json.RawMessage, want bool) {
	t.Helper()
	var keepInBackground bool
	if err := json.Unmarshal(entry["KeepInBackground"], &keepInBackground); err != nil || keepInBackground != want {
		t.Fatalf("KeepInBackground = %s, want %v", entry["KeepInBackground"], want)
	}
}
