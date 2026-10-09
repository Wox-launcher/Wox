package manifest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/plugin"
)

func TestFlowStoreManifestsFromJSON(t *testing.T) {
	raw := []byte("\uFEFF" + `[
		{"ID":"py","Name":"Py","Description":"hello","Author":"ada","Version":"1.2.3","Language":"PYTHON","UrlDownload":"https://example.com/py.zip","IcoPath":"https://example.com/py.png","Website":"https://example.com/py","MinimumAppVersion":"9.9.9"},
		{"ID":"js","Name":"Js","Version":"1.0.0","Language":"JavaScript","UrlDownload":"https://example.com/js.zip"},
		{"ID":"ts","Name":"Ts","Version":"1.5.0","Language":"TypeScript","UrlDownload":"https://example.com/ts.zip","UrlSourceCode":"https://example.com/ts"},
		{"ID":"num","Name":"Num","Version":2,"Language":"python","UrlDownload":"https://example.com/num.zip"},
		{"ID":"exe","Name":"Exe","Version":"2.0.0","Language":"executable","UrlDownload":"https://example.com/exe.zip"},
		{"ID":"cs","Name":"Cs","Version":"3.0.0","Language":"csharp","UrlDownload":"https://example.com/cs.zip"},
		{"ID":"fs","Name":"Fs","Version":"3.0.0","Language":"F#","UrlDownload":"https://example.com/fs.zip"},
		{"ID":"a/b","Name":"Slash","Version":"1.0.0","Language":"python","UrlDownload":"https://example.com/slash.zip"},
		{"ID":"..","Name":"Dot","Version":"1.0.0","Language":"python","UrlDownload":"https://example.com/dot.zip"},
		{"ID":"dup","Name":"Old","Version":"1.0.0","Language":"python","UrlDownload":"https://example.com/old.zip"},
		{"ID":"DUP","Name":"New","Version":"1.1.0","Language":"python","UrlDownload":"https://example.com/new.zip","UrlSourceCode":"https://example.com/new"},
		{"ID":"noname","Version":"1.0.0","Language":"python","UrlDownload":"https://example.com/noname.zip"},
		{"ID":"nourl","Name":"NoUrl","Version":"1.0.0","Language":"python"}
	]`)

	manifests, err := flowStoreManifestsFromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]plugin.StorePluginManifest{}
	for _, manifest := range manifests {
		byID[strings.ToLower(manifest.Id)] = manifest
	}
	for _, hidden := range []string{"a/b", "..", "nourl"} {
		if _, found := byID[hidden]; found {
			t.Fatalf("catalog kept %s", hidden)
		}
	}
	for _, visible := range []string{"py", "js", "ts", "num", "exe", "cs", "fs", "dup", "noname"} {
		if _, found := byID[visible]; !found {
			t.Fatalf("catalog dropped %s", visible)
		}
	}
	if got := byID["py"]; got.MinWoxVersion != minWoxVersion || got.MinWoxVersion == "9.9.9" || got.IconUrl != "https://example.com/py.png" || got.Website != "https://example.com/py" {
		t.Fatalf("python manifest %#v", got)
	}
	if byID["py"].Runtime != RuntimeJSONRPC || byID["py"].Store != "flow" {
		t.Fatalf("runtime %s", byID["py"].Runtime)
	}
	if byID["cs"].Runtime != RuntimeDotNet || len(byID["cs"].SupportedOS) != 1 || byID["cs"].SupportedOS[0] != "Windows" {
		t.Fatalf("csharp %#v", byID["cs"])
	}
	if byID["fs"].Runtime != RuntimeDotNet || len(byID["fs"].SupportedOS) != 1 || byID["fs"].SupportedOS[0] != "Windows" {
		t.Fatalf("fsharp %#v", byID["fs"])
	}
	if len(byID["py"].SupportedOS) != 1 || byID["py"].SupportedOS[0] != "Windows" {
		t.Fatalf("os %#v", byID["py"].SupportedOS)
	}
	if byID["ts"].Website != "https://example.com/ts" {
		t.Fatalf("website %s", byID["ts"].Website)
	}
	if byID["num"].Version != "2" {
		t.Fatalf("numeric version %s", byID["num"].Version)
	}
	if byID["dup"].Version != "1.1.0" || byID["dup"].Name != "New" || byID["dup"].DownloadUrl != "https://example.com/new.zip" {
		t.Fatalf("duplicate %#v", byID["dup"])
	}
	if byID["noname"].Name != "noname" {
		t.Fatalf("name fallback %s", byID["noname"].Name)
	}
	if _, err := flowStoreManifestsFromJSON([]byte(`{"not":"an array"}`)); err == nil {
		t.Fatal("expected invalid catalog json to fail")
	}
	if manifests, err := flowStoreManifestsFromJSON([]byte(`[]`)); err != nil || len(manifests) != 0 {
		t.Fatalf("empty catalog manifests=%d err=%v", len(manifests), err)
	}
}

func TestMergeFlowStoreManifestsSkipsCollisions(t *testing.T) {
	root := t.TempDir()
	existing := []plugin.StorePluginManifest{{Id: "wox-plugin", Name: "Wox"}}
	fetched := []plugin.StorePluginManifest{
		{Id: "wox-plugin", Name: "flow copy"},
		{Id: "Wox-Plugin", Name: "case copy"},
		{Id: "flow-ok", Name: "ok"},
		{Id: "installed-wox", Name: "clash"},
		{Id: "installed-flow", Name: "update"},
	}
	installed := []*plugin.Instance{
		{Metadata: plugin.Metadata{Id: "installed-wox"}, PluginDirectory: filepath.Join(t.TempDir(), "installed-wox@1.0.0")},
		{Metadata: plugin.Metadata{Id: "installed-flow"}, PluginDirectory: filepath.Join(root, "installed-flow")},
	}

	merged := mergeFlowStoreManifests(existing, fetched, installed, root)
	var ids []string
	for _, manifest := range merged {
		ids = append(ids, manifest.Id)
	}
	if strings.Join(ids, ",") != "wox-plugin,flow-ok,installed-flow" {
		t.Fatalf("merged %#v", ids)
	}
}

func TestFlowStoreInstallRejection(t *testing.T) {
	root := t.TempDir()
	manifest := plugin.StorePluginManifest{
		Id:          "same",
		Name:        "Same",
		Version:     "1.0.0",
		Runtime:     RuntimeJSONRPC,
		DownloadUrl: "https://example.com/same.zip",
	}
	outside := &plugin.Instance{
		Metadata:        plugin.Metadata{Id: "same", Version: "0.1.0"},
		PluginDirectory: filepath.Join(t.TempDir(), "same@0.1.0"),
	}
	if err := flowStoreInstallRejection(manifest, outside, root); err == nil || !strings.Contains(err.Error(), "not a flow plugin") {
		t.Fatalf("collision err %v", err)
	}

	newer := &plugin.Instance{
		Metadata:        plugin.Metadata{Id: "same", Version: "2.0.0"},
		PluginDirectory: filepath.Join(root, "same"),
	}
	if err := flowStoreInstallRejection(manifest, newer, root); err == nil || !strings.Contains(err.Error(), "already installed") {
		t.Fatalf("version err %v", err)
	}

	sameVersion := &plugin.Instance{
		Metadata:        plugin.Metadata{Id: "same", Version: "1.0.0"},
		PluginDirectory: filepath.Join(root, "same"),
	}
	if err := flowStoreInstallRejection(manifest, sameVersion, root); err != nil {
		t.Fatal(err)
	}
	manifest.Version = "1.2.0"
	if err := flowStoreInstallRejection(manifest, sameVersion, root); err != nil {
		t.Fatal(err)
	}
	manifest.Version = "not-semver"
	newer.Metadata.Version = "also-not"
	if err := flowStoreInstallRejection(manifest, newer, root); err != nil {
		t.Fatal(err)
	}
}

func TestStageFlowPluginFlatAndNested(t *testing.T) {
	root := filepath.Join(t.TempDir(), "flow-jsonrpc")

	flat := t.TempDir()
	writeFlowPlugin(t, flat, "flat-id", "Flat", "print('flat')\n")
	flatMeta, err := stageFlowPlugin(flat, root, "flat-id")
	if err != nil {
		t.Fatal(err)
	}
	if flatMeta.Directory != filepath.Join(root, "flat-id") {
		t.Fatalf("directory %s", flatMeta.Directory)
	}
	assertFlowScript(t, flatMeta.Directory, "flat-id")

	nested := t.TempDir()
	pluginDir := filepath.Join(nested, "Nested")
	if err := os.MkdirAll(filepath.Join(pluginDir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nested, "__MACOSX"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(nested, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFlowFile(t, filepath.Join(nested, "__MACOSX"), "plugin.json", "not a plugin")
	writeFlowPlugin(t, pluginDir, "nested-id", "Nested", "print('nested')\n")
	nestedMeta, err := stageFlowPlugin(nested, root, "nested-id")
	if err != nil {
		t.Fatal(err)
	}
	assertFlowScript(t, nestedMeta.Directory, "nested-id")
	if _, err := os.Stat(filepath.Join(root, "flat-id", "main.py")); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(flatMeta.Directory, "old.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	replacement := t.TempDir()
	writeFlowPlugin(t, replacement, "flat-id", "Flat", "print('new')\n")
	if _, err := stageFlowPlugin(replacement, root, "flat-id"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "flat-id", "old.txt")); !os.IsNotExist(err) {
		t.Fatalf("old file remained: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "flat-id", "main.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "print('new')\n" {
		t.Fatalf("replaced body %s", body)
	}
}

func TestStageFlowPluginRejectsMismatch(t *testing.T) {
	root := filepath.Join(t.TempDir(), "flow-jsonrpc")
	flat := t.TempDir()
	writeFlowPlugin(t, flat, "flat-id", "Flat", "print('flat')\n")
	if _, err := stageFlowPlugin(flat, root, "other-id"); err == nil {
		t.Fatal("expected id mismatch")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("mismatch created %s: %v", root, err)
	}

	dotnet := t.TempDir()
	writeFlowFile(t, dotnet, "main.dll", "dll")
	writeFlowFile(t, dotnet, "plugin.json", `{
		"ID": "cs-id",
		"Name": "Cs",
		"Language": "csharp",
		"ExecuteFileName": "main.dll",
		"Version": "1.0.0"
	}`)
	dotnetMeta, err := stageFlowPlugin(dotnet, root, "cs-id")
	if err != nil {
		t.Fatal(err)
	}
	dotnetDescriptor, err := Parse(dotnetMeta.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if dotnetDescriptor.Kind != KindDotNet || dotnetDescriptor.Metadata.Runtime != string(RuntimeDotNet) {
		t.Fatalf("kind %s runtime %s", dotnetDescriptor.Kind, dotnetDescriptor.Metadata.Runtime)
	}
	if len(dotnetDescriptor.Metadata.SupportedOS) != 1 || dotnetDescriptor.Metadata.SupportedOS[0] != "Windows" {
		t.Fatalf("os %#v", dotnetDescriptor.Metadata.SupportedOS)
	}

	ambiguous := t.TempDir()
	for _, name := range []string{"one", "two"} {
		directory := filepath.Join(ambiguous, name)
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFlowPlugin(t, directory, name, name, "print('x')\n")
	}
	if _, err := locateFlowPluginDirectory(ambiguous); err == nil {
		t.Fatal("expected two plugin directories to be rejected")
	}
}

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
	writeFlowFile(t, keep, "plugin.json", "{}")
	writeFlowFile(t, drop, "plugin.json", "{}")
	if err := removeFlowPluginDirectory(context.Background(), drop); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatalf("dropped directory still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(keep, "plugin.json")); err != nil {
		t.Fatal(err)
	}

	handled, err := uninstallFlowStorePlugin(context.Background(), &plugin.Instance{
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

func writeFlowPlugin(t *testing.T, directory, id, name, source string) {
	t.Helper()
	writeFlowFile(t, directory, "main.py", source)
	writeFlowFile(t, directory, "plugin.json", `{
		"ID": "`+id+`",
		"Name": "`+name+`",
		"Language": "python",
		"ExecuteFileName": "main.py",
		"Version": "1.0.0"
	}`)
}

func assertFlowScript(t *testing.T, directory, id string) {
	t.Helper()
	descriptor, err := Parse(directory)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Kind != KindScript || descriptor.Metadata.Id != id || descriptor.Metadata.Runtime != string(RuntimeJSONRPC) {
		t.Fatalf("descriptor kind=%s id=%s runtime=%s", descriptor.Kind, descriptor.Metadata.Id, descriptor.Metadata.Runtime)
	}
}
