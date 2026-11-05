//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"testing"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test004LauncherSinglefileNodeToolbar verifies Node toolbar updates use the newest host callback.
// Flow: load the plugin -> show a system notice -> publish two messages with the same action ID -> execute the restored action -> clear.
// Evidence: the notice covers both updates, then the newest callback publishes its completion status and clears.
func Test004LauncherSinglefileNodeToolbar(t *testing.T) {
	const source = `// {"Id":"com.wox.smoke.toolbar.node","Name":"Node Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"NODEJS","TriggerKeywords":["sfsmtoolbarjs"],"SupportedOS":["Windows","Linux","Macos"]}
let api
class SmokePlugin {
  async init(ctx, params) { api = params.API }
  async query(ctx, query) {
    await api.ShowToolbarMsg(ctx, { Id: "old", Title: "Node host old status", IsPersist: true, Actions: [{ Id: "run-host", Name: "Run old callback", PreventHideAfterAction: true, Action: async (ctx) => { await api.ShowToolbarMsg(ctx, { Id: "wrong", Title: "Node host stale callback executed" }) } }] })
    await api.ShowToolbarMsg(ctx, { Id: "host-toolbar", Title: "Node host toolbar ready", IsPersist: true, Actions: [{ Id: "run-host", Name: "Run latest callback", PreventHideAfterAction: true, Action: async (ctx) => { await api.ShowToolbarMsg(ctx, { Id: "done", Title: "Node host latest callback executed", IsPersist: true, Actions: [{ Id: "clear-host", Name: "Clear host status", PreventHideAfterAction: true, Action: async (ctx) => { await api.ClearToolbarMsg(ctx, "done") } }] }) } }] })
    return { Results: [{ Title: "Node toolbar fixture", Actions: [] }] }
  }
}
module.exports.plugin = new SmokePlugin()
`
	writeSingleFilePlugin(t, "Wox.Plugin.ToolbarNodeSmoke.js", source)
	verifySinglefileToolbar(t, "sfsmtoolbarjs ", "Node toolbar fixture", "Node host toolbar ready", "Node host latest callback executed")
}

// Test004LauncherSinglefilePythonToolbar verifies Python toolbar updates use the newest host callback.
// Flow: load the plugin -> show a system notice -> publish two messages with the same action ID -> execute the restored action -> clear.
// Evidence: the notice covers both updates, then the newest callback publishes its completion status and clears.
func Test004LauncherSinglefilePythonToolbar(t *testing.T) {
	const source = `# {"Id":"com.wox.smoke.toolbar.python","Name":"Python Toolbar Smoke","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"PYTHON","TriggerKeywords":["sfsmtoolbarpy"],"SupportedOS":["Windows","Linux","Macos"]}
from wox_plugin import QueryResponse, Result, ToolbarMsg, ToolbarMsgAction, WoxImage
class SmokePlugin:
    async def init(self, ctx, params):
        self.api = params.api
    async def query(self, ctx, query):
        async def old(ctx, action_ctx):
            await self.api.show_toolbar_msg(ctx, ToolbarMsg(id="wrong", title="Python host stale callback executed"))
        async def clear(ctx, action_ctx):
            await self.api.clear_toolbar_msg(ctx, "done")
        async def latest(ctx, action_ctx):
            await self.api.show_toolbar_msg(ctx, ToolbarMsg(id="done", title="Python host latest callback executed", is_persist=True, actions=[ToolbarMsgAction(id="clear-host", name="Clear host status", prevent_hide_after_action=True, action=clear)]))
        await self.api.show_toolbar_msg(ctx, ToolbarMsg(id="old", title="Python host old status", is_persist=True, actions=[ToolbarMsgAction(id="run-host", name="Run old callback", prevent_hide_after_action=True, action=old)]))
        await self.api.show_toolbar_msg(ctx, ToolbarMsg(id="host-toolbar", title="Python host toolbar ready", is_persist=True, actions=[ToolbarMsgAction(id="run-host", name="Run latest callback", prevent_hide_after_action=True, action=latest)]))
        return QueryResponse(results=[Result(title="Python toolbar fixture", icon=WoxImage.new_emoji("🧪"))])
plugin = SmokePlugin()
`
	writeSingleFilePlugin(t, "Wox.Plugin.ToolbarPythonSmoke.py", source)
	verifySinglefileToolbar(t, "sfsmtoolbarpy ", "Python toolbar fixture", "Python host toolbar ready", "Python host latest callback executed")
}

// verifySinglefileToolbar exercises the latest SDK host callback through the visible toolbar panel.
func verifySinglefileToolbar(t *testing.T, query, result, title, completed string) {
	t.Helper()
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		waitForPluginResult(t, ctx, client, query, result)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke startup-update ")
		arm, found := smoke.FindLauncherResult(snapshot, "Arm update confirmation fixture")
		if !found {
			t.Fatal("update confirmation fixture not found")
		}
		if err := client.Perform(ctx, arm, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return !state.Visible }); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		updateTitles := map[string]bool{}
		for _, lang := range i18n.GetSupportedLanguages() {
			texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			updateTitles[texts["ui_update_success"]] = true
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && updateTitles[status.Value]
		}); err != nil {
			t.Fatalf("wait for system notice: %v", err)
		}
		snapshot = waitForPluginResult(t, ctx, client, query, result)
		status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
		if !found || !updateTitles[status.Value] {
			t.Fatalf("system notice did not cover host updates: found=%t status=%q", found, status.Value)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == title
		})
		if err != nil {
			t.Fatalf("wait for plugin toolbar %q: %v", title, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		activateHostToolbarAction(t, ctx, client, "action-toolbar-run-host")
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && status.Value == completed
		})
		if err != nil {
			t.Fatalf("wait for latest host callback %q: %v", completed, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		activateHostToolbarAction(t, ctx, client, "action-toolbar-clear-host")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || status.Value != completed
		}); err != nil {
			t.Fatalf("host action did not clear toolbar: %v", err)
		}
	})
}

// activateHostToolbarAction opens the current message's own action panel.
func activateHostToolbarAction(t *testing.T, ctx context.Context, client *automationdriver.Client, prefix string) {
	t.Helper()
	if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatal(err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.FindByAutomationIDPrefix(snapshot, prefix)
		return found
	})
	if err != nil {
		t.Fatalf("wait for host toolbar action %q: %v", prefix, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	action, _ := automationdriver.FindByAutomationIDPrefix(snapshot, prefix)
	if err := client.Perform(ctx, action.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatal(err)
	}
}
