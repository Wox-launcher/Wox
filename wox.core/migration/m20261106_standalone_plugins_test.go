package migration

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"testing"
	"wox/database"
)

// TestStandalonePluginMigration preserves new settings and local-only data on repeat runs.
func TestStandalonePluginMigration(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	old := []database.PluginSetting{
		{PluginID: oldTimerPluginID, Key: "activeTimers", Value: `[{"id":"t","deadlineUnixMs":1000}]`, IsLocal: true},
		{PluginID: oldDictationPluginID, Key: "history", Value: `[{"id":"h","content":"hello","timestamp":1}]`},
		{PluginID: oldDictationPluginID, Key: "dictionary", Value: `[{"phrase":"Wox"}]`},
		{PluginID: oldDictationPluginID, Key: "model", Value: "old-model"},
		{PluginID: oldDictationPluginID, Key: "actions", Value: `[{"id":"default","type":"default","hotkey":"hold:right_shift","aiRefineEnabled":true,"model":"model-a"},{"id":"custom","type":"custom","hotkey":"ctrl+k","extra":"keep"}]`},
		{PluginID: newDictationPluginID, Key: "model", Value: "new-model"},
	}
	for _, row := range old {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	for range 2 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			return migrateStandalonePlugins(context.Background(), tx, filepath.Join(root, "source"), filepath.Join(root, "destination"))
		}); err != nil {
			t.Fatal(err)
		}
	}
	values := map[string]string{"history": old[1].Value, "dictionary": old[2].Value, "model": "new-model", "defaultHotkey": "hold:right_shift", "defaultAIRefineEnabled": "true", "defaultAIModel": "model-a"}
	for key, want := range values {
		var row database.PluginSetting
		if err := db.Where("plugin_id = ? AND key = ?", newDictationPluginID, key).First(&row).Error; err != nil || row.Value != want {
			t.Fatalf("%s: %+v %v", key, row, err)
		}
	}
	var actions database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", newDictationPluginID, "actions").First(&actions).Error; err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if json.Unmarshal([]byte(actions.Value), &rows) != nil || len(rows) != 1 || rows[0]["id"] != "custom" || rows[0]["extra"] != "keep" {
		t.Fatalf("actions: %s", actions.Value)
	}
	var timer database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", newTimerPluginID, "activeTimers").First(&timer).Error; err != nil || !timer.IsLocal || timer.Value != old[0].Value {
		t.Fatalf("timer: %+v %v", timer, err)
	}
	var count int64
	db.Model(&database.Oplog{}).Where("entity_id = ?", newTimerPluginID).Count(&count)
	if count != 0 {
		t.Fatal("local timer was uploaded")
	}
	db.Model(&database.Oplog{}).Where("entity_id = ?", newDictationPluginID).Count(&count)
	if count != 6 {
		t.Fatalf("oplog count = %d", count)
	}
}

// TestStandaloneMigrationRetriesAfterDatabaseRollback keeps copied files when the transaction fails.
func TestStandaloneMigrationRetriesAfterDatabaseRollback(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	if err := db.Create(&database.PluginSetting{PluginID: oldDictationPluginID, Key: "history", Value: "history"}).Error; err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	source, destination := filepath.Join(root, "source"), filepath.Join(root, "destination")
	model := "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17"
	if err := os.MkdirAll(filepath.Join(source, model), 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"model.int8.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(source, model, file), []byte(file), 0644); err != nil {
			t.Fatal(err)
		}
	}
	failure := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := migrateStandalonePlugins(context.Background(), tx, source, destination); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("expected rollback, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, model, "model.int8.onnx")); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return migrateStandalonePlugins(context.Background(), tx, source, destination)
	}); err != nil {
		t.Fatal(err)
	}
	var row database.PluginSetting
	if err := db.Where("plugin_id = ? AND key = ?", newDictationPluginID, "history").First(&row).Error; err != nil || row.Value != "history" {
		t.Fatalf("settings were not retried: %+v, %v", row, err)
	}
}

// TestStandaloneMigrationRollsBackWhenLegacyActionsAreInvalid keeps the old data intact.
func TestStandaloneMigrationRollsBackWhenLegacyActionsAreInvalid(t *testing.T) {
	db := openQuickJumpMigrationDB(t)
	for _, row := range []database.PluginSetting{
		{PluginID: oldTimerPluginID, Key: "activeTimers", Value: "[]", IsLocal: true},
		{PluginID: oldDictationPluginID, Key: "actions", Value: "not-json"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	err := db.Transaction(func(tx *gorm.DB) error {
		return migrateStandalonePlugins(context.Background(), tx, filepath.Join(root, "source"), filepath.Join(root, "destination"))
	})
	if err == nil {
		t.Fatal("invalid actions were accepted")
	}
	var count int64
	db.Model(&database.PluginSetting{}).Where("plugin_id = ? OR plugin_id = ?", newTimerPluginID, newDictationPluginID).Count(&count)
	if count != 0 {
		t.Fatalf("partial migration committed %d rows", count)
	}
}
