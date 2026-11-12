//go:build wox_ui_smoke

package clipboard

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	woxclipboard "wox/util/clipboard"
)

// Test004LauncherPluginClipboardLocalSend follows the launcher handoff into a Node plugin.
func Test004LauncherPluginClipboardLocalSend(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		userDir := os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment)
		if userDir == "" {
			t.Fatal("missing smoke user directory")
		}
		pluginDir := filepath.Join(userDir, "plugins", "single-file")
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatal(err)
		}
		fixture := filepath.Join(pluginDir, "LocalSendSmoke.js")
		if err := os.WriteFile(fixture, []byte(localSendSmokeSource), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Remove(fixture) })
		for {
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "send-smoke ")
			if smoke.HasLauncherResultLabel(snapshot, "LocalSend smoke ready") {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("LocalSend fixture never became ready")
			case <-time.After(250 * time.Millisecond):
			}
		}
		imagePath, width, height, expectedColor := createClipboardSmokeImage(t)
		injectClipboardSmokeImage(t, ctx, imagePath, expectedColor, width, height)
		dimensions := clipboardImageDimensions(width, height)
		runAction := func(key string) {
			t.Helper()
			labels := make(map[string]bool)
			for _, code := range []i18n.LangCode{i18n.LangCodeEnUs, i18n.LangCodeZhCn} {
				translations, err := i18n.GetI18nManager().GetLangMap(ctx, code)
				if err != nil {
					t.Fatal(err)
				}
				labels[translations[key]] = true
			}
			panel := smoke.OpenResultActionPanel(t, ctx, client)
			for _, node := range panel.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "action-result-") && node.Role == woxui.AccessibilityRoleMenuItem && labels[node.Label] {
					if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
						t.Fatal(err)
					}
					return
				}
			}
			t.Fatalf("clipboard action %s was missing", key)
		}
		sendAndCheck := func(expected string) {
			t.Helper()
			runAction("plugin_clipboard_send_via_localsend")
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				input, found := automationdriver.Find(snapshot, "launcher.query.input")
				return found && input.Value == "send-smoke " && smoke.HasLauncherResultLabel(snapshot, expected)
			}); err != nil {
				t.Fatalf("wait for LocalSend handoff in the ordinary query: %v", err)
			}
		}

		expectedImage := fmt.Sprintf("Original image %dx%d", width, height)
		waitForClipboardImageResult(t, ctx, client, dimensions)
		smoke.SelectLauncherResultDescriptionContains(t, ctx, client, dimensions)
		sendAndCheck(expectedImage)
		waitForClipboardImageResult(t, ctx, client, dimensions)
		smoke.SelectLauncherResultDescriptionContains(t, ctx, client, dimensions)
		runAction("plugin_clipboard_mark_favorite")
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "cb fav ")
		smoke.SelectLauncherResultDescriptionContains(t, ctx, client, dimensions)
		sendAndCheck(expectedImage)

		const clipboardText = "LocalSend 剪贴板文字"
		if err := woxclipboard.WriteText(clipboardText); err != nil {
			t.Fatal(err)
		}
		for {
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "cb ")
			if smoke.HasLauncherResultLabel(snapshot, clipboardText) {
				break
			}
			select {
			case <-ctx.Done():
				t.Fatal("copied text did not appear in Clipboard")
			case <-time.After(250 * time.Millisecond):
			}
		}
		smoke.SelectLauncherResultLabelPrefix(t, ctx, client, clipboardText)
		sendAndCheck("Original text " + clipboardText)
	})
}

const localSendSmokeSource = `// {
//   "Id": "407fdbba-bdab-4bc1-aa09-b46e811155a6",
//   "Name": "LocalSend Smoke",
//   "Version": "0.1.2",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "TriggerKeywords": ["send-smoke"],
//   "SupportedOS": ["Windows", "Linux", "Macos"]
// }
const fs = require("node:fs")
module.exports.plugin = {
  async init() {},
  async query(ctx, query) {
    let title = "LocalSend smoke ready"
    const file = query.ContextData?.["localsend.filePath"]
    const text = query.ContextData?.["localsend.text"]
    if (text !== undefined) {
      title = "Original text " + text
    } else if (file) {
      const png = fs.readFileSync(file)
      title = "Original image " + png.readUInt32BE(16) + "x" + png.readUInt32BE(20)
    }
    return [{ Title: title, Icon: { ImageType: "emoji", ImageData: "🖼" } }]
  }
}
`
