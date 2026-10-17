package raycast

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/plugin/thirdparty/migrate"
	"wox/util"
)

func TestDetectUsesTheTestMarker(t *testing.T) {
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
	t.Setenv(util.TestRaycastInstallEnv, root)
	if got, _ := detectInstall(); got != "" {
		t.Fatalf("missing marker returned %s", got)
	}
	if err := os.WriteFile(filepath.Join(root, installMarker), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, _ := detectInstall(); got != root {
		t.Fatalf("marker = %s", got)
	}
	source := migrationSource{}
	installation, err := source.Detect(context.Background())
	if err != nil || installation == nil || installation.Name() != "Raycast" {
		t.Fatalf("detect = %#v %v", installation, err)
	}
}

func TestUnlockCatalogAndReset(t *testing.T) {
	snapshotForTest = &woxSnapshot{}
	t.Cleanup(func() { snapshotForTest = nil })
	app := filepath.Join(t.TempDir(), "Notes.exe")
	if err := os.WriteFile(app, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := []byte(`{"settings":{"commands":[{"extensionId":"e:r:applications","id":"open::=::` + jsonEscape(app) + `","name":"Notes","alias":"nt"}]}}`)
	blob, err := Seal(payload, "secret")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "settings.rayconfig")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	item := &installation{}
	if _, err := item.Catalog(context.Background()); ExportCode(err) != "export_required" {
		t.Fatalf("locked catalog = %v", err)
	}
	if _, err := item.Import(context.Background(), []string{"setting:query_alias:0"}); ExportCode(err) != "export_required" {
		t.Fatalf("locked import = %v", err)
	}
	if err := item.Unlock(context.Background(), path, "nope"); ExportCode(err) != codeAuth {
		t.Fatalf("auth = %v", err)
	}
	if err := item.Unlock(context.Background(), path, "secret"); err != nil {
		t.Fatal(err)
	}
	catalog, err := item.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var aliasID string
	for _, category := range catalog {
		if category.ID != migrate.CategoryQueries {
			continue
		}
		if len(category.Items) != 1 || category.Items[0].ID != "setting:query_alias:0" || !category.Items[0].Selectable {
			t.Fatalf("queries = %+v", category.Items)
		}
		aliasID = category.Items[0].ID
	}
	if aliasID == "" {
		t.Fatalf("catalog = %+v", catalog)
	}
	result, err := item.Import(context.Background(), []string{aliasID, "not-selected"})
	if err != nil || len(result.Settings) != 1 || result.Settings[0].Key != "QueryAliases" {
		t.Fatalf("import = %+v %v", result, err)
	}
	if !jsonContains(result.Settings[0].Value, `"Shortcut":"nt"`) {
		t.Fatalf("alias json = %s", result.Settings[0].Value)
	}
	generation := item.Generation()
	item.Reset()
	if item.Generation() == generation {
		t.Fatal("reset did not bump the session")
	}
	item.Reset()
	if _, err := item.Catalog(context.Background()); ExportCode(err) != "export_required" {
		t.Fatalf("after reset = %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("export file changed")
	}
}

func TestStaleUnlockDoesNotRestoreData(t *testing.T) {
	snapshotForTest = &woxSnapshot{}
	t.Cleanup(func() { snapshotForTest = nil })
	blob, err := Seal([]byte(`{"settings":{"general":{"openAtLogin":true}}}`), "secret")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "settings.rayconfig")
	if err := os.WriteFile(path, blob, 0o644); err != nil {
		t.Fatal(err)
	}
	item := &installation{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := item.Unlock(ctx, path, "secret"); err == nil {
		t.Fatal("canceled unlock succeeded")
	}
	if _, err := item.Catalog(context.Background()); ExportCode(err) != "export_required" {
		t.Fatalf("catalog after cancel = %v", err)
	}
}

func jsonEscape(value string) string {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) < 2 {
		return value
	}
	return string(raw[1 : len(raw)-1])
}

func jsonContains(raw, part string) bool {
	return strings.Contains(raw, part)
}
