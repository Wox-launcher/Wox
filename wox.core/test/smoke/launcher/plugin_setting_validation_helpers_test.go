//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	pluginSettingValidationPluginID      = "com.wox.smoke.pluginsetting.validation"
	pluginSettingValidationOtherPluginID = "com.wox.smoke.pluginsetting.validation.other"
	pluginSettingValidationTrigger       = "psetval"
	pluginSettingValidationOtherTrigger  = "psetother"
	pluginSettingValidationPluginFile    = "Wox.Plugin.SmokePluginSettingValidation.py"
	pluginSettingValidationOtherFile     = "Wox.Plugin.SmokePluginSettingValidationOther.py"
	pluginSettingAccessKeyFieldID        = "plugin-settings-field-1"
	pluginSettingAccessKeyErrorID        = "plugin-settings-field-1-error"
	pluginSettingValidationSavedKey      = "smoke-plugin-setting-key"
	pluginSettingValidationOtherTitle    = "plugin setting other ready"
)

// pluginSettingValidationOtherPluginSource returns an installed plugin with no required settings.
func pluginSettingValidationOtherPluginSource(id, name, trigger string) string {
	return `# {
#   "Id": "` + id + `",
#   "Name": "` + name + `",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["` + trigger + `"],
#   "SupportedOS": ["Windows", "Linux", "Macos"]
# }

from wox_plugin import PluginInitParams, Query, QueryResponse, Result, WoxImage

class SmokePlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        return QueryResponse(results=[
            Result(
                title="plugin setting other ready",
                icon=WoxImage.new_emoji("🧪"),
            )
        ])

plugin = SmokePlugin()
`
}

func pluginListAutomationID(pluginID string) string {
	return "plugin-list-" + pluginID
}

func describePluginSettingSnapshot(snapshot woxwidget.AutomationSnapshot) string {
	return automationdriver.DescribeNodes(snapshot, "plugin-search", pluginSettingAccessKeyFieldID, pluginSettingAccessKeyErrorID, pluginListAutomationID(pluginSettingValidationPluginID), pluginListAutomationID(pluginSettingValidationOtherPluginID))
}

func waitForPluginSettingValidationPlugin(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	waitForQueryRequirementForm(t, ctx, client, pluginSettingValidationTrigger+" ")
}

func waitForPluginSettingValidationOtherPlugin(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	waitForQueryRequirementResult(t, ctx, client, pluginSettingValidationOtherTrigger+" ", pluginSettingValidationOtherTitle)
}

func openPluginSettingAccessKey(t *testing.T, ctx context.Context, client *automationdriver.Client, pluginID string) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, pluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
		return found
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for plugin setting accessKey field: %v", err)
		}
		t.Fatalf("wait for plugin setting accessKey field: %s: %v", describePluginSettingSnapshot(current), err)
	}
}

func selectInstalledPlugin(t *testing.T, ctx context.Context, client *automationdriver.Client, pluginID string) {
	t.Helper()
	listID := pluginListAutomationID(pluginID)
	if err := client.Perform(ctx, "plugin-search", woxui.AccessibilityActionSetValue, pluginID); err != nil {
		t.Fatalf("filter installed plugin %q: %v", pluginID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, listID)
		return found
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for installed plugin %q: %v", pluginID, err)
		}
		t.Fatalf("wait for installed plugin %q: %s: %v", pluginID, describePluginSettingSnapshot(current), err)
	}
	if err := client.Perform(ctx, listID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select installed plugin %q: %v", pluginID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		item, found := automationdriver.Find(snapshot, listID)
		return found && item.Selected
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for installed plugin %q to stay selected: %v", pluginID, err)
		}
		t.Fatalf("wait for installed plugin %q to stay selected: %s: %v", pluginID, describePluginSettingSnapshot(current), err)
	}
}

func focusAutomationNode(t *testing.T, ctx context.Context, client *automationdriver.Client, automationID string) {
	t.Helper()
	if err := client.Perform(ctx, automationID, woxui.AccessibilityActionFocus, ""); err != nil {
		t.Fatalf("focus %s: %v", automationID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, automationID)
		return found && node.Focused
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for %s to take focus: %v", automationID, err)
		}
		t.Fatalf("wait for %s to take focus: %s: %v", automationID, describePluginSettingSnapshot(current), err)
	}
}

func setPluginSettingText(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID, value string) {
	t.Helper()
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set plugin setting %s: %v", fieldID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, fieldID)
		return found && field.Value == value
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for plugin setting %s to become %q: %v", fieldID, value, err)
		}
		t.Fatalf("wait for plugin setting %s to become %q: %s: %v", fieldID, value, describePluginSettingSnapshot(current), err)
	}
}

func waitForPluginSettingAccessKey(t *testing.T, ctx context.Context, client *automationdriver.Client, value string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
		return found && field.Value == value
	}); err != nil {
		current, snapErr := client.Snapshot(ctx)
		if snapErr != nil {
			t.Fatalf("wait for plugin setting accessKey %q: %v", value, err)
		}
		t.Fatalf("wait for plugin setting accessKey %q: %s: %v", value, describePluginSettingSnapshot(current), err)
	}
}

func pluginSettingFieldErrorVisible(snapshot woxwidget.AutomationSnapshot) bool {
	errorNode, found := automationdriver.Find(snapshot, pluginSettingAccessKeyErrorID)
	return found && strings.TrimSpace(errorNode.Value) != ""
}
