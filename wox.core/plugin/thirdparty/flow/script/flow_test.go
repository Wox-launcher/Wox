package script

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/plugin/thirdparty/flow/manifest"
)

func TestDetectFlowDialect(t *testing.T) {
	cases := []struct {
		name     string
		files    map[string]string
		language string
		entry    string
		want     string
	}{
		{name: "executable", language: "executable", entry: "app.exe", want: flowDialectV1},
		{name: "plain python", files: map[string]string{"main.py": "print('ok')\n"}, language: "python", entry: "main.py", want: flowDialectV2},
		{name: "argv python", files: map[string]string{"main.py": "import sys\nprint(sys.argv)\n"}, language: "python", entry: "main.py", want: flowDialectV1},
		{name: "vendored lib client", files: map[string]string{
			"main.py":                    "print('ok')\n",
			"lib/flowlauncher/client.py": "import sys\nprint(sys.argv)\n",
		}, language: "python", entry: "main.py", want: flowDialectV2},
		{name: "root client", files: map[string]string{
			"main.py":                "print('ok')\n",
			"flowlauncher/client.py": "import sys\nprint(sys.argv)\n",
		}, language: "python", entry: "main.py", want: flowDialectV1},
		{name: "argv library under lib", files: map[string]string{
			"main.py":       "print('ok')\n",
			"lib/client.py": "import sys\nprint(sys.argv)\n",
		}, language: "python", entry: "main.py", want: flowDialectV1},
		{name: "node argv", files: map[string]string{"main.js": "console.log(process.argv)\n"}, language: "javascript", entry: "main.js", want: flowDialectV1},
		{name: "node stdin", files: map[string]string{"main.js": "process.stdin.on('data', () => {})\nconsole.log(process.argv)\n"}, language: "javascript", entry: "main.js", want: flowDialectV2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			for name, content := range tc.files {
				writeScriptFile(t, directory, name, content)
			}
			if got := detectFlowDialect(directory, tc.language, tc.entry); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestParseFlowReply(t *testing.T) {
	reply, err := flowReplyFromValue(map[string]any{
		"jsonrpc": "2.0",
		"id":      "wox-1",
		"result":  nil,
	})
	if err != nil || len(reply.Results) != 0 {
		t.Fatalf("null result: %+v %v", reply, err)
	}

	output := strings.Join([]string{
		`{"method":"Flow.Launcher.ChangeQuery","parameters":["next"]}`,
		`{"jsonrpc":"2.0","id":"wox-2","result":{"result":[{"Title":"Hi","SubTitle":"There","Score":7,"CopyText":"copied","JsonRPCAction":{"method":"open_item","parameters":["x"],"dontHideAfterAction":true}}]},"settings":{"mode":"fast"}}`,
	}, "\n")
	bridge := &recordBridge{}
	reply, err = parseFlowOutput(context.Background(), bridge, output)
	if err != nil {
		t.Fatal(err)
	}
	if len(bridge.queries) != 1 || bridge.queries[0] != "next" {
		t.Fatalf("queries %#v", bridge.queries)
	}
	if len(reply.Results) != 1 || reply.Results[0].Title != "Hi" || reply.Results[0].Score != 7 {
		t.Fatalf("results %+v", reply.Results)
	}
	if !reply.HasSettings || reply.Settings["mode"] != "fast" {
		t.Fatalf("settings %+v", reply.Settings)
	}
	rows := flowQueryResults(t.TempDir(), replyIcon(), reply.Results, func(context.Context, string, []any) {})
	if len(rows) != 1 || len(rows[0].Actions) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	if !rows[0].Actions[0].IsDefault || !rows[0].Actions[0].PreventHideAfterAction || rows[0].Actions[0].Name != "Open item" {
		t.Fatalf("default action %+v", rows[0].Actions[0])
	}
	if rows[0].Actions[1].Name != "Copy" {
		t.Fatalf("copy action %+v", rows[0].Actions[1])
	}
}

func TestFlowScriptQueryRoundTrip(t *testing.T) {
	python := findPython(t)
	directory := t.TempDir()
	writeScriptFile(t, directory, "icon.png", "")
	writeScriptFile(t, directory, "main.py", `
from flowlauncher import FlowLauncher

class Hello(FlowLauncher):
    def query(self, query):
        self.change_query("next " + query)
        return [{
            "Title": "Hello " + query,
            "SubTitle": str(self.settings.get("mode", "")),
            "IcoPath": "icon.png",
            "Score": 7,
            "CopyText": "copied-text",
            "JsonRPCAction": {"method": "open_item", "parameters": [query], "dontHideAfterAction": True},
        }]

    def open_item(self, value):
        self.show_msg("opened", value)

if __name__ == "__main__":
    Hello()
`)
	clientDir := t.TempDir()
	if err := materializeFlowClient(clientDir); err != nil {
		t.Fatal(err)
	}
	bridge := &recordBridge{}
	session := newFlowSession("hello", directory, "main.py", "python", flowDialectV2, python, "", clientDir, bridge)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	defer session.Close()
	if err := session.Start(ctx); err != nil {
		t.Fatal(err)
	}
	reply, err := session.Invoke(ctx, "query", []any{map[string]any{
		"Search":        "world",
		"RawQuery":      "hi world",
		"ActionKeyword": "hi",
	}}, map[string]any{"mode": "fast"})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Results) != 1 || reply.Results[0].Title != "Hello world" || reply.Results[0].SubTitle != "fast" {
		t.Fatalf("reply %+v", reply.Results)
	}
	if len(bridge.queries) != 1 || bridge.queries[0] != "next world" {
		t.Fatalf("change query %#v", bridge.queries)
	}
	rows := flowQueryResults(directory, replyIcon(), reply.Results, nil)
	if !strings.HasSuffix(strings.ReplaceAll(rows[0].Icon.ImageData, "\\", "/"), "/icon.png") {
		t.Fatalf("icon %s", rows[0].Icon.ImageData)
	}
	if _, err := session.Invoke(ctx, "open_item", []any{"world"}, nil); err != nil {
		t.Fatal(err)
	}
	if len(bridge.notes) != 1 || bridge.notes[0][0] != "opened" || bridge.notes[0][1] != "world" {
		t.Fatalf("show msg %#v", bridge.notes)
	}
}

func TestFlowV1QueryParameterIsSearchText(t *testing.T) {
	got := flowV1CallParameters("query", []any{map[string]any{
		"Search":        "17min",
		"RawQuery":      "timer1 17min",
		"ActionKeyword": "timer1",
	}})
	if len(got) != 1 || got[0] != "17min" {
		t.Fatalf("query params %#v", got)
	}
	empty := flowV1CallParameters("query", []any{map[string]any{"Search": ""}})
	if len(empty) != 1 || empty[0] != "" {
		t.Fatalf("empty query params %#v", empty)
	}
	plain := flowV1CallParameters("query", []any{"17min"})
	if len(plain) != 1 || plain[0] != "17min" {
		t.Fatalf("string query params %#v", plain)
	}
	action := []any{"--always-on-top", "on", "17min"}
	rewritten := flowV1CallParameters("startTimer", action)
	if len(rewritten) != 3 || rewritten[0] != "--always-on-top" || action[0] != "--always-on-top" {
		t.Fatalf("action params %#v original %#v", rewritten, action)
	}
}

func TestFlowLauncherWorkDir(t *testing.T) {
	pluginDir := t.TempDir()
	root := t.TempDir()
	link, err := flowLauncherWorkDir(root, pluginDir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filepath.Clean(link), filepath.Clean(filepath.Join("FlowLauncher", "UserData", "Plugins"))) {
		t.Fatalf("link %s", link)
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDirectoryPath(target, pluginDir) && !sameDirectoryPath(filepath.Join(filepath.Dir(link), target), pluginDir) {
		t.Fatalf("link target %s", target)
	}
	settings, err := os.ReadFile(filepath.Join(root, "FlowLauncher", "UserData", "Settings", "Settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(settings), `"PluginSettings"`) {
		t.Fatalf("settings %s", settings)
	}
	again, err := flowLauncherWorkDir(root, pluginDir)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDirectoryPath(again, link) {
		t.Fatalf("recreated link %s", again)
	}
}

func TestFlowScriptLauncherWorkDir(t *testing.T) {
	python := findPython(t)
	directory := t.TempDir()
	writeScriptFile(t, directory, "main.py", `
import json, sys
from pathlib import Path
path = Path.cwd()
if "FlowLauncher" not in path.parts:
    raise SystemExit("missing FlowLauncher")
found = None
while len(path.parts) > 1:
    if (path / "Settings").is_dir():
        found = path
        break
    path = path.parent
if found is None or found.name != "UserData":
    raise SystemExit("missing UserData settings")
json.loads((found / "Settings" / "Settings.json").read_text(encoding="utf-8"))
request = json.loads(sys.argv[1])
print(json.dumps({"result": [{"Title": "emoji " + request["parameters"][0]}]}))
`)
	session := newFlowSession("emoji", directory, "main.py", "python", flowDialectV1, python, "", "", &recordBridge{})
	session.launcherRoot = t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reply, err := session.Invoke(ctx, "query", []any{map[string]any{"Search": "smile"}}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Results) != 1 || reply.Results[0].Title != "emoji smile" {
		t.Fatalf("reply %+v", reply.Results)
	}
}

func TestFlowScriptOneShotQuery(t *testing.T) {
	python := findPython(t)
	directory := t.TempDir()
	writeScriptFile(t, directory, "main.py", `
import json, sys
request = json.loads(sys.argv[1])
query = request["parameters"][0]
if not isinstance(query, str):
    raise SystemExit("query parameter is not a string")
print(json.dumps({"result": [{"Title": "v1 " + query}]}))
`)
	if got := detectFlowDialect(directory, "python", "main.py"); got != flowDialectV1 {
		t.Fatalf("dialect %s", got)
	}
	session := newFlowSession("oneshot", directory, "main.py", "python", flowDialectV1, python, "", "", &recordBridge{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	reply, err := session.Invoke(ctx, "query", []any{map[string]any{"Search": "world"}}, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.Results) != 1 || reply.Results[0].Title != "v1 world" {
		t.Fatalf("reply %+v", reply.Results)
	}
}

func TestDiscoverFlowScriptMetadata(t *testing.T) {
	root := t.TempDir()
	pythonDir := filepath.Join(root, "py")
	dotnetDir := filepath.Join(root, "cs")
	for _, directory := range []string{pythonDir, dotnetDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeScriptFile(t, pythonDir, "main.py", "")
	writeScriptFile(t, pythonDir, "plugin.json", `{"ID":"py","Name":"Py","Language":"python","ExecuteFileName":"main.py"}`)
	writeScriptFile(t, dotnetDir, "Plugin.dll", "")
	writeScriptFile(t, dotnetDir, "plugin.json", `{"ID":"cs","Name":"CS","Language":"csharp","ExecuteFileName":"Plugin.dll"}`)

	metadata, err := discoverMetadata(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata) != 1 || metadata[0].Id != "py" || metadata[0].Runtime != string(manifest.RuntimeJSONRPC) {
		t.Fatalf("metadata %+v", metadata)
	}
	host := &Host{}
	if _, err := host.LoadPlugin(context.Background(), metadata[0], pythonDir); err != nil {
		t.Fatal(err)
	}
	if _, err := host.LoadPlugin(context.Background(), plugin.Metadata{Id: "cs", Directory: dotnetDir}, dotnetDir); err == nil {
		t.Fatal("dotnet plugin was loaded by the script host")
	}
}

type recordBridge struct {
	mu      sync.Mutex
	queries []string
	notes   [][2]string
}

func (b *recordBridge) ChangeQuery(ctx context.Context, query string) {
	b.mu.Lock()
	b.queries = append(b.queries, query)
	b.mu.Unlock()
}

func (b *recordBridge) HideApp(ctx context.Context) {}

func (b *recordBridge) ShowApp(ctx context.Context) {}

func (b *recordBridge) Notify(ctx context.Context, title string, subtitle string) {
	b.mu.Lock()
	b.notes = append(b.notes, [2]string{title, subtitle})
	b.mu.Unlock()
}

func (b *recordBridge) CopyText(ctx context.Context, text string) {}

func (b *recordBridge) OpenPath(ctx context.Context, target string) error { return nil }

func (b *recordBridge) OpenDirectory(ctx context.Context, directory string, fileName string) error {
	return nil
}

func (b *recordBridge) ShellRun(ctx context.Context, command string) error { return nil }

func (b *recordBridge) Log(ctx context.Context, message string) {}

func findPython(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python", "python3"} {
		path, err := exec.LookPath(name)
		if err == nil {
			return path
		}
	}
	t.Skip("python is not installed")
	return ""
}

func replyIcon() common.WoxImage {
	return common.WoxImage{}
}

func writeScriptFile(t *testing.T, directory, name, content string) {
	t.Helper()
	path := filepath.Join(directory, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
