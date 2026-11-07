package test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
	"wox/plugin/system/snippets"
	"wox/setting"
)

// TestDisabledSnippetsImport checks import and settings access across a disabled plugin lifecycle.
func TestDisabledSnippetsImport(t *testing.T) {
	NewTestSuite(t)
	ctx := context.Background()
	manager := plugin.GetPluginManager()
	if err := manager.DisablePlugin(ctx, common.SnippetsPluginID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := manager.EnablePlugin(ctx, common.SnippetsPluginID); err != nil {
			t.Errorf("restore snippets plugin: %v", err)
		}
	})
	item := migrateapply.Snippet{Source: "raycast", SourceID: uuid.NewString(), Title: "disabled import test", Keyword: ";disabled-test ", Text: "imported content"}
	if migrateapply.ImportSnippets == nil {
		t.Fatal("snippet import hook is unavailable")
	}
	results := migrateapply.ImportSnippets(ctx, []migrateapply.Snippet{item})
	if len(results) != 1 || results[0].Status != "imported" {
		t.Fatalf("import while disabled: %+v", results)
	}
	// A fresh instance models settings access before the disabled plugin has initialized.
	value, ok := (&snippets.Plugin{}).GetSettingOverride(ctx, "snippetsTable")
	var records []snippets.Record
	if !ok || json.Unmarshal([]byte(value), &records) != nil {
		t.Fatalf("read settings before initialization: %q", value)
	}
	var imported snippets.Record
	for _, record := range records {
		if record.ImportID == item.SourceID {
			imported = record
			break
		}
	}
	if imported.ID == "" || imported.Keyword != item.Keyword || imported.Text != item.Text {
		t.Fatalf("imported record not available in settings: %+v", imported)
	}
	t.Cleanup(func() {
		store := setting.NewPluginSettingStore(database.GetDB(), common.SnippetsPluginID)
		var rows []snippets.Record
		if err := store.Get("snippetsTable", &rows); err != nil {
			t.Errorf("load cleanup table: %v", err)
			return
		}
		kept := make([]snippets.Record, 0, len(rows))
		for _, row := range rows {
			if row.ID != imported.ID {
				kept = append(kept, row)
			}
		}
		if err := store.SetWithSync("snippetsTable", kept, true); err != nil {
			t.Errorf("remove imported snippet: %v", err)
		}
	})
	if err := manager.EnablePlugin(ctx, common.SnippetsPluginID); err != nil {
		t.Fatal(err)
	}
	p := manager.GetSystemPlugin(common.SnippetsPluginID)
	response := p.Query(ctx, plugin.Query{Search: item.Title})
	if len(response.Results) != 1 || response.Results[0].Title != item.Title {
		t.Fatalf("query after enabling: %+v", response.Results)
	}
}
