package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin"
)

func TestFlowPluginDirectoryAndRemoval(t *testing.T) {
	root := t.TempDir()
	if flowPluginDirectoryIsChild(root, root) {
		t.Fatal("collection root is not a plugin")
	}
	if flowPluginDirectoryIsChild(root, filepath.Join(root, "plugin", "nested")) {
		t.Fatal("grandchild should not be a flow plugin directory")
	}
	if flowPluginDirectoryIsChild(root, filepath.Join(root, "..", "outside")) {
		t.Fatal("path outside the collection should not match")
	}
	if !flowPluginDirectoryIsChild(root, filepath.Join(root, "plugin-id")) {
		t.Fatal("direct child should match")
	}

	keep := filepath.Join(root, "keep")
	drop := filepath.Join(root, "drop")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(drop, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keep, "plugin.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(drop, "plugin.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeFlowPluginDirectory(context.Background(), drop); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatalf("dropped directory still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(keep, "plugin.json")); err != nil {
		t.Fatal(err)
	}

	handled, err := Uninstall(context.Background(), &plugin.Instance{
		Metadata:        plugin.Metadata{Id: "x", Runtime: string(RuntimeJSONRPC)},
		PluginDirectory: keep,
	}, false, false, nil)
	if handled || err != nil {
		t.Fatalf("non-child uninstall handled=%v err=%v", handled, err)
	}
	if _, err := os.Stat(filepath.Join(keep, "plugin.json")); err != nil {
		t.Fatal(err)
	}
}
