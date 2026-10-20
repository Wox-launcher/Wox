package raycast

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"wox/database"
	"wox/plugin"
	"wox/plugin/thirdparty/migrate"
	"wox/plugin/thirdparty/raycast/extension"
	"wox/util"
)

// TestExtensionWizardImport verifies the same Unlock/Catalog/Import contract used by onboarding.
// Optional local verification inputs remain in environment variables, never committed fixtures or logs.
func TestExtensionWizardImport(t *testing.T) {
	const childData = "WOX_RAYCAST_WIZARD_TEST_DATA"
	if os.Getenv(childData) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestExtensionWizardImport$", "-test.v", "-test.timeout=90s")
		cmd.Env = append(os.Environ(), childData+"="+t.TempDir())
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated migration: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required")
	}
	bundle, err := os.ReadFile("../../../resource/hosts/raycast-host.js")
	if err != nil {
		t.Skip("build Raycast runtime first")
	}
	data := os.Getenv(childData)
	t.Setenv(util.TestWoxDataDirEnv, data)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(data, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(util.NewTraceContext(), 60*time.Second)
	defer cancel()
	if err := database.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if db, err := database.GetDB().DB(); err == nil {
			_ = db.Close()
		}
	})
	if err := os.WriteFile(filepath.Join(util.GetLocation().GetHostDirectory(), "raycast-host.js"), bundle, 0600); err != nil {
		t.Fatal(err)
	}
	plugin.ResolveNodePath = func(context.Context) (string, error) { return node, nil }
	snapshotForTest = &woxSnapshot{}
	backup, password, root := os.Getenv("WOX_RAYCAST_VERIFY_BACKUP"), os.Getenv("WOX_RAYCAST_VERIFY_PASSWORD"), os.Getenv("WOX_RAYCAST_VERIFY_EXTENSIONS")
	if backup == "" {
		root = t.TempDir()
		ref := writeLocalExtension(t, root, "11111111-1111-4111-8111-111111111111", "example")
		var payload Payload
		payload.NodeExtensions.Extensions = []ExportExtension{ref}
		raw, _ := json.Marshal(payload)
		password = "fixture-password"
		blob, err := Seal(raw, password)
		if err != nil {
			t.Fatal(err)
		}
		backup = filepath.Join(t.TempDir(), "fixture.rayconfig")
		if err := os.WriteFile(backup, blob, 0600); err != nil {
			t.Fatal(err)
		}
	}
	i := &installation{extensionRoot: root}
	if err := i.Unlock(ctx, backup, password); err != nil {
		t.Fatal(err)
	}
	defer i.Reset()
	catalog, err := i.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var selected []string
	for _, category := range catalog {
		if category.ID == migrate.CategoryPlugins {
			for _, item := range category.Items {
				t.Logf("catalog %s status=%s selectable=%t reason=%s", item.Title, item.Status, item.Selectable, item.DetailKey)
				if item.Selectable {
					selected = append(selected, item.ID)
				}
			}
		}
	}
	if len(selected) == 0 {
		t.Fatal("no importable local plugins")
	}
	result, err := i.Import(ctx, selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Plugins) != len(selected) {
		t.Fatalf("missing outcomes: %+v", result.Plugins)
	}
	for _, outcome := range result.Plugins {
		if outcome.Error != "" {
			t.Fatalf("import %s: %s", outcome.ID, outcome.Error)
		}
	}
	manager := plugin.GetPluginManager()
	instances := manager.GetPluginInstances()
	t.Cleanup(func() {
		for _, instance := range manager.GetPluginInstances() {
			manager.UnloadPlugin(context.Background(), instance)
		}
	})
	for _, instance := range instances {
		if instance.Metadata.Runtime != string(extension.Runtime) {
			continue
		}
		if err := instance.WaitInit(ctx); err != nil {
			t.Fatal(err)
		}
		keywords := instance.GetTriggerKeywords()
		if len(keywords) != 1 || keywords[0] == "*" || keywords[0] == "" {
			t.Fatalf("imported %s keywords = %v, want one explicit keyword", instance.Metadata.Name, keywords)
		}
		rows := instance.Plugin.Query(ctx, plugin.Query{TriggerKeyword: keywords[0], Search: "", SessionId: "wizard-test"})
		if len(rows.Results) == 0 {
			t.Fatalf("no commands for %s", instance.Metadata.Name)
		}
		t.Logf("installed %s commands=%d", instance.Metadata.Name, len(rows.Results))
	}
	plugins, err := i.Plugins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	imported := 0
	for _, p := range plugins {
		if p.Status == migrate.PluginImported {
			imported++
		}
	}
	if imported != len(selected) {
		t.Fatalf("imported status: got %d want %d", imported, len(selected))
	}
	again, err := i.Import(ctx, selected)
	if err != nil || len(again.Plugins) != len(selected) {
		t.Fatal("retry is not idempotent")
	}
	for _, outcome := range again.Plugins {
		if outcome.Error != "" {
			t.Fatalf("retry %s: %s", outcome.ID, outcome.Error)
		}
	}
	// Run a deterministic fixture view through the installed runtime during ordinary CI.
	if os.Getenv("WOX_RAYCAST_VERIFY_BACKUP") == "" {
		instance := instances[0]
		response := instance.Plugin.Query(ctx, plugin.Query{Id: "query", SessionId: "wizard-test", Command: "list"})
		if len(response.Results) != 1 || response.Results[0].Title != "Local result" {
			t.Fatalf("runtime: %+v", response)
		}
	}
	t.Logf("verified %d local imports; original plugin directories and backup were read-only", len(selected))
}
