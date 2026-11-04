//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test004LauncherSinglefileNodeToolbar verifies a Node plugin can display and clear a toolbar message through its host.
// Flow: load the plugin -> query its trigger -> open its toolbar actions -> clear the message.
// Evidence: the status and action appear in the launcher, and the host callback removes the status.
func Test004LauncherSinglefileNodeToolbar(t *testing.T) {
	const source = `// {"Id":"com.wox.smoke.toolbar.node","Name":"Node Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"NODEJS","TriggerKeywords":["sfsmtoolbarjs"],"SupportedOS":["Windows","Linux","Macos"]}
let api
class SmokePlugin {
  async init(ctx, params) { api = params.API }
  async query(ctx, query) {
    await api.ShowToolbarMsg(ctx, { Id: "host-toolbar", Title: "Node host toolbar ready", Actions: [{ Id: "clear-host", Name: "Clear host status", PreventHideAfterAction: true, Action: async (ctx) => { await api.ClearToolbarMsg(ctx, "host-toolbar") } }] })
    return { Results: [{ Title: "Node toolbar fixture", Actions: [] }] }
  }
}
module.exports.plugin = new SmokePlugin()
`
	writeSingleFilePlugin(t, "Wox.Plugin.ToolbarNodeSmoke.js", source)
	verifySinglefileToolbar(t, "sfsmtoolbarjs ", "Node toolbar fixture", "Node host toolbar ready")
}

// Test004LauncherSinglefilePythonToolbar verifies a Python plugin can display and clear a toolbar message through its host.
// Flow: load the plugin -> query its trigger -> open its toolbar actions -> clear the message.
// Evidence: the status and action appear in the launcher, and the host callback removes the status.
func Test004LauncherSinglefilePythonToolbar(t *testing.T) {
	const source = `# {"Id":"com.wox.smoke.toolbar.python","Name":"Python Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"PYTHON","TriggerKeywords":["sfsmtoolbarpy"],"SupportedOS":["Windows","Linux","Macos"]}
from wox_plugin import QueryResponse, Result, ToolbarMsg, ToolbarMsgAction, WoxImage
class SmokePlugin:
    async def init(self, ctx, params):
        self.api = params.api
    async def query(self, ctx, query):
        async def clear(ctx, action_ctx):
            await self.api.clear_toolbar_msg(ctx, "host-toolbar")
        await self.api.show_toolbar_msg(ctx, ToolbarMsg(id="host-toolbar", title="Python host toolbar ready", actions=[ToolbarMsgAction(id="clear-host", name="Clear host status", prevent_hide_after_action=True, action=clear)]))
        return QueryResponse(results=[Result(title="Python toolbar fixture", icon=WoxImage.new_emoji("🧪"))])
plugin = SmokePlugin()
`
	writeSingleFilePlugin(t, "Wox.Plugin.ToolbarPythonSmoke.py", source)
	verifySinglefileToolbar(t, "sfsmtoolbarpy ", "Python toolbar fixture", "Python host toolbar ready")
}

// verifySinglefileToolbar exercises the SDK host callback through the visible toolbar panel.
func verifySinglefileToolbar(t *testing.T, query, result, title string) {
	t.Helper()
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		waitForPluginResult(t, ctx, client, query, result)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == title
		})
		if err != nil {
			t.Fatalf("wait for plugin toolbar %q: %v", title, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-clear-host")
			return found
		})
		if err != nil {
			t.Fatalf("wait for host toolbar action: %v", err)
		}
		action, _ := automationdriver.FindByAutomationIDPrefix(snapshot, "action-toolbar-clear-host")
		if err := client.Perform(ctx, action.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || status.Value != title
		}); err != nil {
			t.Fatalf("host action did not clear toolbar: %v", err)
		}
	})
}
