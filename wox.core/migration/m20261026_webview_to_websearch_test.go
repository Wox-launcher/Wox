package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/cloudsync"
	"wox/database"

	"gorm.io/gorm"
)

// TestWebViewToWebSearchMigration covers collisions, local scopes, legacy flags and repeat execution.
func TestWebViewToWebSearchMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	existing := `[{"Keyword":"x","Title":"Existing search","Extra":123},{"Keyword":"x-web"}]`
	source := `[{"Keyword":"x","Url":"https://x.com","UserAgent":"mobile_safari","InjectCss":"body{}","KeepInBackground":true,"Icon":{"ImageType":"url","ImageData":"https://x.com/icon"},"Extra":9007199254740993},{"Keyword":"fresh","Url":"https://example.com","CacheDisabled":true,"Disabled":true}]`
	for _, row := range []database.PluginSetting{
		{PluginID: webSearchPluginID, Key: "webSearches", Value: existing},
		{PluginID: webViewPluginID, Key: "sites", Value: source},
		{PluginID: webViewPluginID, Key: "sites@windows", Value: `[{"Keyword":"local","Url":"https://local.test"}]`, IsLocal: true},
		{PluginID: webViewPluginID, Key: "unrelated", Value: "untouched"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return (&webViewToWebSearchMigration{}).Up(context.Background(), tx)
		}); err != nil {
			t.Fatal(err)
		}
	}
	var target database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", webSearchPluginID, "webSearches").First(&target).Error; err != nil {
		t.Fatal(err)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(target.Value), &entries); err != nil || len(entries) != 4 {
		t.Fatalf("migrated entries = %s, err=%v", target.Value, err)
	}
	if string(entries[0]["Title"]) != `"Existing search"` || string(entries[2]["Keyword"]) != `"x-web-2"` ||
		string(entries[2]["Urls"]) != `["https://x.com"]` || string(entries[2]["OpenInWebView"]) != "true" ||
		string(entries[2]["UserAgent"]) != `"mobile_safari"` || string(entries[2]["InjectCSS"]) != `"body{}"` ||
		string(entries[2]["Extra"]) != "9007199254740993" || !strings.Contains(string(entries[2]["Icon"]), "https://x.com/icon") {
		t.Fatal("migration lost settings or overwrote an existing search")
	}
	assertWebViewKeepInBackground(t, entries[2], true)
	assertWebViewKeepInBackground(t, entries[3], false)
	if string(entries[3]["Disabled"]) != "true" {
		t.Fatal("disabled site was enabled")
	}
	var local database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", webSearchPluginID, "webSearches@windows").First(&local).Error; err != nil || !local.IsLocal {
		t.Fatalf("local setting lost its scope: %+v %v", local, err)
	}
	if !strings.Contains(local.Value, `"Existing search"`) {
		t.Fatal("platform override lost inherited global searches")
	}
	var remaining int64
	db.Model(&database.PluginSetting{}).Where("plugin_id = ? AND (key = ? OR key LIKE ?)", webViewPluginID, "sites", "sites@%").Count(&remaining)
	if remaining != 0 {
		t.Fatal("source sites remain after migration")
	}
	var unrelated database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", webViewPluginID, "unrelated").First(&unrelated).Error; err != nil || unrelated.Value != "untouched" {
		t.Fatal("unrelated setting was changed")
	}
	var ops []database.Oplog
	db.Order("id").Find(&ops)
	if len(ops) != 2 || ops[0].EntityID != webSearchPluginID || ops[0].Key != "webSearches" || ops[1].EntityID != webViewPluginID || ops[1].Key != "sites" {
		t.Fatalf("unexpected sync operations: %+v", ops)
	}
}

// TestWebViewToWebSearchRollback leaves both settings intact when either parsing or sync publication fails.
func TestWebViewToWebSearchRollback(t *testing.T) {
	for _, failure := range []string{"source", "target", "sync"} {
		t.Run(failure, func(t *testing.T) {
			db := openQuickJumpMigrationDB(t)
			source := `[{"Keyword":"x","Url":"https://x.com"}]`
			target := `[{"Keyword":"g","Title":"Google"}]`
			if failure == "source" {
				source = "{broken"
			}
			if failure == "target" {
				target = "{broken"
			}
			for _, row := range []database.PluginSetting{
				{PluginID: webViewPluginID, Key: "sites", Value: source},
				{PluginID: webSearchPluginID, Key: "webSearches", Value: target},
			} {
				if err := db.Create(&row).Error; err != nil {
					t.Fatal(err)
				}
			}
			if failure == "sync" {
				if err := db.Exec("CREATE TRIGGER reject_merge_oplog BEFORE INSERT ON oplogs BEGIN SELECT RAISE(ABORT, 'test failure'); END").Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Transaction(func(tx *gorm.DB) error {
				return (&webViewToWebSearchMigration{}).Up(context.Background(), tx)
			}); err == nil {
				t.Fatal("expected migration failure")
			}
			var rows []database.PluginSetting
			db.Find(&rows)
			if len(rows) != 2 {
				t.Fatal("failed migration deleted settings")
			}
			for _, row := range rows {
				if row.PluginID == webViewPluginID && row.Value != source || row.PluginID == webSearchPluginID && row.Value != target {
					t.Fatal("failed migration changed settings")
				}
			}
		})
	}
}

// TestWebViewMigrationKeepsLocalAndDisabled avoids enabling a disabled plugin's sites or uploading local data.
func TestWebViewMigrationKeepsLocalAndDisabled(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	for _, row := range []database.PluginSetting{
		{PluginID: webViewPluginID, Key: "sites", Value: `[{"Keyword":"x","Url":"https://x.com"}]`, IsLocal: true},
		{PluginID: webViewPluginID, Key: "Disabled", Value: "true"},
		{PluginID: webSearchPluginID, Key: "webSearches", Value: `[{"Keyword":"g"}]`},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	pending := database.Oplog{EntityType: cloudsync.EntityPluginSetting, EntityID: webSearchPluginID,
		Key: "webSearches", Operation: cloudsync.OpUpsert, Value: `[{"Keyword":"g"}]`}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return (&webViewToWebSearchMigration{}).Up(context.Background(), tx)
	}); err != nil {
		t.Fatal(err)
	}
	var target database.PluginSetting
	db.Where("plugin_id = ? AND key = ?", webSearchPluginID, "webSearches").First(&target)
	if !target.IsLocal || !strings.Contains(target.Value, `"Disabled":true`) {
		t.Fatal("local or disabled policy was lost")
	}
	var count int64
	db.Model(&database.Oplog{}).Where("cloud_sync_discarded = ?", false).Count(&count)
	if count != 0 {
		t.Fatal("local data was published to sync")
	}
}
