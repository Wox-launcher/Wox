package dotnet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin/thirdparty/flow/manifest"
)

func TestDotNetMetadataAndLoad(t *testing.T) {
	root := t.TempDir()
	dotnetDir := filepath.Join(root, "cs")
	pythonDir := filepath.Join(root, "py")
	for _, directory := range []string{dotnetDir, pythonDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeDotNetFile(t, dotnetDir, "Plugin.dll", "")
	writeDotNetFile(t, dotnetDir, "plugin.json", `{"ID":"cs","Name":"CS","Language":"CSharp","ExecuteFileName":"Plugin.dll"}`)
	writeDotNetFile(t, pythonDir, "main.py", "")
	writeDotNetFile(t, pythonDir, "plugin.json", `{"ID":"py","Name":"Py","Language":"python","ExecuteFileName":"main.py"}`)

	metadata, err := MetadataFrom(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 || metadata[0].Id != "cs" || metadata[0].Runtime != string(manifest.RuntimeDotNet) {
		t.Fatalf("metadata %+v", metadata)
	}
	host := &Host{}
	if discovered, err := host.DiscoverMetadata(context.Background()); err != nil || len(discovered) != 0 {
		t.Fatalf("discover %+v %v", discovered, err)
	}
	if _, err := host.LoadPlugin(context.Background(), metadata[0], dotnetDir); err == nil {
		t.Fatal("dotnet host started a plugin")
	}
}

func writeDotNetFile(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
