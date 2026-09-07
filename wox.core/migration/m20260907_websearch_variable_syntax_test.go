package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/database"

	"gorm.io/gorm"
)

func TestWebSearchVariableSyntaxMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	original := `[{"Title":"Find {query} {wox:parameter:query 123}","Urls":["https://example.com/?q={wox:parameter:query}&lower={lower_query}","https://example.org/{upper_query}/{query}"],"Keyword":"{query}","Extra":{"id":9007199254740993}},null]`
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
	migration := &webSearchVariableSyntaxMigration{}
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
		if string(entries[0]["Title"]) != `"Find {wox:parameter?name=query} {wox:parameter?name=query 123}"` {
			t.Fatal(row.Value)
		}
		var urls []string
		if err := json.Unmarshal(entries[0]["Urls"], &urls); err != nil || len(urls) != 2 {
			t.Fatal(row.Value)
		}
		if urls[0] != "https://example.com/?q={wox:parameter?name=query}&lower={wox:parameter?name=query&case=lower}" || urls[1] != "https://example.org/{wox:parameter?name=query&case=upper}/{wox:parameter?name=query}" {
			t.Fatal(row.Value)
		}
		if string(entries[0]["Keyword"]) != `"{query}"` || !strings.Contains(string(entries[0]["Extra"]), "9007199254740993") || entries[1] != nil || strings.Contains(row.Value, "{lower_query}") || strings.Contains(row.Value, "{upper_query}") {
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
	for _, value := range []string{"", "{broken", "null", "[]", `[{"Title":false,"Urls":42}]`} {
		if got, changed := migrateWebSearchVariableSyntaxJSON(value); changed || got != value {
			t.Fatalf("modified unsupported input: %q", value)
		}
	}
}
