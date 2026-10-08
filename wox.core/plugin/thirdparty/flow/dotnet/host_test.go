package dotnet

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/plugin"
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
	loaded, err := (&Host{}).LoadPlugin(context.Background(), metadata[0], dotnetDir)
	if err != nil {
		t.Fatal(err)
	}
	initer, ok := loaded.(plugin.FallibleInit)
	if !ok {
		t.Fatal("dotnet plugin does not report init errors")
	}
	if err := initer.InitWithError(context.Background(), plugin.InitParams{}); err == nil {
		t.Fatal("empty assembly started")
	}
}

func TestPrepareHostDirectoryUsesExtractedHost(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, hostDLLName), []byte("dll"), 0o644); err != nil {
		t.Fatal(err)
	}
	hostDirectoryOverride = dir
	t.Cleanup(func() { hostDirectoryOverride = "" })
	got, errText := prepareHostDirectory()
	if errText != "" || got != dir {
		t.Fatalf("directory %q error %q", got, errText)
	}

	hostDirectoryOverride = t.TempDir()
	if _, errText = prepareHostDirectory(); errText == "" {
		t.Fatal("missing loader was accepted")
	}
}

func TestFlowFieldRelativeParsesImage(t *testing.T) {
	directory := t.TempDir()
	icon := filepath.Join(directory, "icon.png")
	if got := flowFieldRelative(directory, common.NewWoxImageAbsolutePath(icon).String()); got != "icon.png" {
		t.Fatalf("relative %s", got)
	}
	if got := flowFieldRelative(directory, common.NewWoxImageUrl("https://example.com/a.png").String()); got != "" {
		t.Fatalf("url %s", got)
	}
	outside := filepath.Join(t.TempDir(), "other.png")
	if got := flowFieldRelative(directory, common.NewWoxImageAbsolutePath(outside).String()); got != outside {
		t.Fatalf("outside %s", got)
	}
}

func writeDotNetFile(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
