package raycast

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin/thirdparty/migrate"
)

// writeLocalExtension creates a minimal built extension independently of the import implementation.
func writeLocalExtension(t *testing.T, root, id, name string) ExportExtension {
	t.Helper()
	directory := filepath.Join(root, id)
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := `{"name":"` + name + `","title":"Local Example","author":"author","commands":[{"name":"list","title":"List","mode":"view"}]}`
	for file, content := range map[string]string{"package.json": manifest, "list.js": `const R=require('react');const {List}=require('@raycast/api');exports.default=()=>R.createElement(List,null,R.createElement(List.Item,{title:'Local result'}));`} {
		if err := os.WriteFile(filepath.Join(directory, file), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return ExportExtension{UUID: id, Name: name, Author: "author"}
}

func TestExtensionCatalogUsesUnlockedBackupAndLocalIdentity(t *testing.T) {
	snapshotForTest = &woxSnapshot{}
	t.Cleanup(func() { snapshotForTest = nil })
	root := t.TempDir()
	valid := writeLocalExtension(t, root, "11111111-1111-4111-8111-111111111111", "example")
	mismatch := writeLocalExtension(t, root, "22222222-2222-4222-8222-222222222222", "wrong-local")
	mismatch.Name = "from-backup"
	missing := ExportExtension{UUID: "33333333-3333-4333-8333-333333333333", Name: "missing", Author: "author"}
	var payload Payload
	payload.NodeExtensions.Extensions = []ExportExtension{valid, mismatch, missing, valid, {UUID: "../outside", Name: "unsafe"}}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := Seal(raw, "test-password")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "backup.rayconfig")
	if err := os.WriteFile(file, blob, 0600); err != nil {
		t.Fatal(err)
	}
	i := &installation{extensionRoot: root}
	if _, err := i.Plugins(context.Background()); ExportCode(err) != migrate.CodeExportRequired {
		t.Fatalf("locked plugins: %v", err)
	}
	if err := i.Unlock(context.Background(), file, "test-password"); err != nil {
		t.Fatal(err)
	}
	categories, err := i.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var items []migrate.Item
	for _, category := range categories {
		if category.ID == migrate.CategoryPlugins {
			items = category.Items
		}
	}
	if len(items) != 3 {
		t.Fatalf("expected deduplicated plugin rows, got %+v", items)
	}
	if !items[0].Selectable || items[0].Kind != migrate.ItemKindPlugin || items[0].ID != "plugin:"+valid.UUID {
		t.Fatalf("valid plugin: %+v", items[0])
	}
	if items[1].Selectable || items[1].DetailKey != "raycast_extension_mismatch" {
		t.Fatalf("mismatch: %+v", items[1])
	}
	if items[2].Selectable || items[2].DetailKey != "raycast_extension_missing" {
		t.Fatalf("missing: %+v", items[2])
	}
	result, err := i.Import(context.Background(), []string{items[1].ID, items[2].ID, "plugin:unknown"})
	if err != nil || len(result.Plugins) != 3 {
		t.Fatalf("invalid import results: %+v %v", result, err)
	}
	for _, outcome := range result.Plugins {
		if outcome.Error == "" {
			t.Fatalf("unsupported import succeeded: %+v", outcome)
		}
	}
	if err := os.Remove(filepath.Join(root, valid.UUID, "list.js")); err != nil {
		t.Fatal(err)
	}
	plugins, err := i.Plugins(context.Background())
	if err != nil || plugins[0].Selectable {
		t.Fatal("catalog reused stale local code state")
	}
	i.Reset()
	if _, err := i.Plugins(context.Background()); ExportCode(err) != migrate.CodeExportRequired {
		t.Fatalf("reset plugins: %v", err)
	}
}

func TestExtensionReferencesCountTowardExportLimit(t *testing.T) {
	_, err := ParsePayload([]byte(`{"nodeExtensions":{"extensions":[{},{}]}}`), Limits{MaxEntries: 1})
	if ExportCode(err) != codeLimit {
		t.Fatalf("unbounded extension records: %v", err)
	}
}
