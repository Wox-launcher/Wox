//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test031LauncherPluginSettingRequiredBlur verifies clearing a required plugin setting and leaving the field shows the not-empty error under that input.
// Flow: install a plugin with a required accessKey -> open its Settings -> enter a value -> blur -> clear the field -> blur again.
// Evidence: the accessKey field stays empty on the same plugin and shows a validator error directly under the input.
func Test031LauncherPluginSettingRequiredBlur(t *testing.T) {
	writeQueryRequirementPlugin(t, pluginSettingValidationPluginFile, queryRequirementAnyQueryPluginSource(pluginSettingValidationPluginID, "Plugin Setting Validation Smoke", pluginSettingValidationTrigger))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		waitForPluginSettingValidationPlugin(t, ctx, client)
		openPluginSettingAccessKey(t, ctx, client, pluginSettingValidationPluginID)
		focusAutomationNode(t, ctx, client, pluginSettingAccessKeyFieldID)
		setPluginSettingText(t, ctx, client, pluginSettingAccessKeyFieldID, "temporary-key")
		focusAutomationNode(t, ctx, client, "plugin-search")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
			return found && field.Value == "temporary-key" && !pluginSettingFieldErrorVisible(snapshot)
		}); err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait for filled accessKey without a field error: %v", err)
			}
			t.Fatalf("wait for filled accessKey without a field error: %s: %v", describePluginSettingSnapshot(current), err)
		}

		focusAutomationNode(t, ctx, client, pluginSettingAccessKeyFieldID)
		setPluginSettingText(t, ctx, client, pluginSettingAccessKeyFieldID, "")
		focusAutomationNode(t, ctx, client, "plugin-search")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
			item, selected := automationdriver.Find(snapshot, pluginListAutomationID(pluginSettingValidationPluginID))
			return found && field.Value == "" && pluginSettingFieldErrorVisible(snapshot) && selected && item.Selected
		})
		if err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait for accessKey not_empty error after blur: %v", err)
			}
			t.Fatalf("wait for accessKey not_empty error after blur: %s: %v", describePluginSettingSnapshot(current), err)
		}
		if !pluginSettingFieldErrorVisible(snapshot) {
			t.Fatal("cleared required accessKey did not show a field error after blur")
		}
	})
}
