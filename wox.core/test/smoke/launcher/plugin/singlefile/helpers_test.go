//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

const (
	pythonTrigger = "sfsmpy"
	nodeTrigger   = "sfsmjs"
	pythonID      = "com.wox.smoke.singlefile.python"
	nodeID        = "com.wox.smoke.singlefile.nodejs"
)

func singleFilePluginDirectory(t *testing.T) string {
	t.Helper()
	userDir := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDir == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	directory := filepath.Join(userDir, "plugins", "single-file")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatalf("create single-file plugin directory: %v", err)
	}
	return directory
}

func writeSingleFilePlugin(t *testing.T, fileName, content string) string {
	t.Helper()
	path := filepath.Join(singleFilePluginDirectory(t), fileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write single-file plugin %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path)
	})
	return path
}

func pythonPluginSource(title string) string {
	return `# {
#   "Id": "` + pythonID + `",
#   "Name": "Single-file Python Smoke",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["` + pythonTrigger + `"],
#   "SupportedOS": ["Windows", "Linux", "Macos"]
# }

from wox_plugin import CopyParams, CopyType, PluginInitParams, Query, QueryResponse, Result, ResultAction, WoxImage

class SmokePlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        async def copy_action(ctx, action_ctx):
            await self.api.copy(ctx, CopyParams(type=CopyType.TEXT, text="single-file-python-copied"))

        return QueryResponse(results=[
            Result(
                title="` + title + `",
                icon=WoxImage.new_emoji("🧪"),
                actions=[ResultAction(name="Copy", is_default=True, action=copy_action)],
            )
        ])

plugin = SmokePlugin()
`
}

func nodePluginSource(title string) string {
	return `// {
//   "Id": "` + nodeID + `",
//   "Name": "Single-file Node Smoke",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "TriggerKeywords": ["` + nodeTrigger + `"],
//   "SupportedOS": ["Windows", "Linux", "Macos"]
// }

class SmokePlugin {
  async init(ctx, params) {
    this.api = params.API
  }

  async query(ctx, query) {
    return {
      Results: [{
        Title: "` + title + `",
        Icon: { ImageType: "emoji", ImageData: "🧪" },
        Actions: []
      }]
    }
  }
}

module.exports.plugin = new SmokePlugin()
`
}

func launcherResults(snapshot woxwidget.AutomationSnapshot) []woxui.AccessibilityNode {
	results := make([]woxui.AccessibilityNode, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			results = append(results, node)
		}
	}
	return results
}

func resultByLabel(snapshot woxwidget.AutomationSnapshot, label string) (woxui.AccessibilityNode, bool) {
	for _, result := range launcherResults(snapshot) {
		if result.Label == label {
			return result, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

func waitForPluginResult(t *testing.T, ctx context.Context, client *automationdriver.Client, query, title string) woxwidget.AutomationSnapshot {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var last woxwidget.AutomationSnapshot
	for {
		last = smoke.ReplaceLauncherQuery(t, ctx, client, query)
		if _, found := resultByLabel(last, title); found {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for single-file result %q from %q: last results = %+v: %v", title, query, launcherResults(last), ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForClipboardText(t *testing.T, ctx context.Context, expected string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		text, err := clipboard.ReadText()
		if err == nil && text == expected {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for clipboard text %q: %v", expected, ctx.Err())
		case <-ticker.C:
		}
	}
}
