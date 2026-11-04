//go:build wox_ui_smoke

package update

import (
	"context"
	"strings"
	"testing"

	"wox/i18n"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// Test001LauncherPluginUpdateErrorToolbar verifies a failed update check remains actionable in the update query.
// Flow: check updates against a controlled invalid manifest -> open the message actions -> copy the error -> leave the query.
// Evidence: the full error reaches the toolbar and clipboard, then disappears with the plugin context.
func Test001LauncherPluginUpdateErrorToolbar(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "update ")
		labels := map[string]map[string]bool{"plugin_update_action_check": {}, "ui_toolbar_copy_error": {}}
		for _, lang := range i18n.GetSupportedLanguages() {
			texts, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			for key := range labels {
				labels[key][texts[key]] = true
			}
		}
		if err := client.Perform(ctx, "result-toolbar-more", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "action-result-") && labels["plugin_update_action_check"][node.Label] {
					return true
				}
			}
			return false
		})
		if err != nil {
			t.Fatalf("wait for update check action: %v", err)
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-result-") && labels["plugin_update_action_check"][node.Label] {
				if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		const errorText = "failed to unmarshal version manifest"
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && strings.Contains(status.Value, errorText)
		})
		if err != nil {
			status, _ := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for update error: status=%q: %v", status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Perform(ctx, "launcher.toolbar.status", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "action-toolbar-") && labels["ui_toolbar_copy_error"][node.Label] {
					return true
				}
			}
			return false
		})
		if err != nil {
			t.Fatalf("wait for copy update error action: %v", err)
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-toolbar-") && labels["ui_toolbar_copy_error"][node.Label] {
				if err := client.Perform(ctx, node.AutomationID, woxui.AccessibilityActionActivate, ""); err != nil {
					t.Fatal(err)
				}
				break
			}
		}
		if value, err := clipboard.ReadText(); err != nil || !strings.Contains(value, errorText) {
			t.Fatalf("copied update error = %q, err=%v", value, err)
		}
		smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !strings.Contains(status.Value, errorText)
		}); err != nil {
			t.Fatalf("update error survived leaving the query: %v", err)
		}
	})
}
