//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test032LauncherPluginSettingRequiredSwitch verifies leaving a plugin with an invalid required setting does not persist the empty value.
// Flow: save accessKey -> confirm it after switching away and back -> clear it -> select another plugin -> return.
// Evidence: the original plugin can be left while the field is empty, and returning restores the last saved accessKey without a field error.
func Test032LauncherPluginSettingRequiredSwitch(t *testing.T) {
	writeQueryRequirementPlugin(t, pluginSettingValidationPluginFile, queryRequirementAnyQueryPluginSource(pluginSettingValidationPluginID, "Plugin Setting Validation Smoke", pluginSettingValidationTrigger))
	writeQueryRequirementPlugin(t, pluginSettingValidationOtherFile, pluginSettingValidationOtherPluginSource(pluginSettingValidationOtherPluginID, "Plugin Setting Validation Other", pluginSettingValidationOtherTrigger))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		waitForPluginSettingValidationPlugin(t, ctx, client)
		waitForPluginSettingValidationOtherPlugin(t, ctx, client)
		openPluginSettingAccessKey(t, ctx, client, pluginSettingValidationPluginID)
		focusAutomationNode(t, ctx, client, pluginSettingAccessKeyFieldID)
		setPluginSettingText(t, ctx, client, pluginSettingAccessKeyFieldID, pluginSettingValidationSavedKey)
		selectInstalledPlugin(t, ctx, client, pluginSettingValidationOtherPluginID)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, fieldFound := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
			return !fieldFound
		}); err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait to leave the required-setting plugin: %v", err)
			}
			t.Fatalf("wait to leave the required-setting plugin: %s: %v", describePluginSettingSnapshot(current), err)
		}

		selectInstalledPlugin(t, ctx, client, pluginSettingValidationPluginID)
		waitForPluginSettingAccessKey(t, ctx, client, pluginSettingValidationSavedKey)

		focusAutomationNode(t, ctx, client, pluginSettingAccessKeyFieldID)
		setPluginSettingText(t, ctx, client, pluginSettingAccessKeyFieldID, "")
		selectInstalledPlugin(t, ctx, client, pluginSettingValidationOtherPluginID)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, fieldFound := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
			item, selected := automationdriver.Find(snapshot, pluginListAutomationID(pluginSettingValidationOtherPluginID))
			return !fieldFound && selected && item.Selected
		}); err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait to leave the plugin after clearing accessKey: %v", err)
			}
			t.Fatalf("wait to leave the plugin after clearing accessKey: %s: %v", describePluginSettingSnapshot(current), err)
		}

		selectInstalledPlugin(t, ctx, client, pluginSettingValidationPluginID)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, pluginSettingAccessKeyFieldID)
			return found && field.Value == pluginSettingValidationSavedKey && !pluginSettingFieldErrorVisible(snapshot)
		}); err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait for restored accessKey after invalid switch: %v", err)
			}
			t.Fatalf("wait for restored accessKey after invalid switch: %s: %v", describePluginSettingSnapshot(current), err)
		}
	})
}
