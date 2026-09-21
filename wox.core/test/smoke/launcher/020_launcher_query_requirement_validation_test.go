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
	queryRequirementValidationTrigger    = "qreqval"
	queryRequirementValidationPluginID   = "com.wox.smoke.queryrequirement.validation"
	queryRequirementValidationReadyTitle = "query requirement ready:"
	queryRequirementValidationPluginFile = "Wox.Plugin.SmokeQueryRequirementValidation.py"
)

// Test020LauncherQueryRequirementValidation verifies saving the setup form with an empty required field stays on the form and shows the validator error.
// Flow: install a plugin that requires accessKey -> query its trigger -> save without entering a value.
// Evidence: the form remains visible, the plugin query result does not appear, and the empty field shows its validator error.
func Test020LauncherQueryRequirementValidation(t *testing.T) {
	writeQueryRequirementPlugin(t, queryRequirementValidationPluginFile, queryRequirementAnyQueryPluginSource(queryRequirementValidationPluginID, "Query Requirement Validation Smoke", queryRequirementValidationTrigger))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		blocked := waitForQueryRequirementForm(t, ctx, client, queryRequirementValidationTrigger+" ")
		smoke.AssertNoDiagnostics(t, blocked)

		if err := client.Perform(ctx, queryRequirementSaveID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save empty query requirement settings: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			errorNode, found := automationdriver.Find(snapshot, queryRequirementErrorID)
			return found && strings.TrimSpace(errorNode.Value) != "" && queryRequirementFormVisible(snapshot)
		})
		if err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait for query requirement validation error: %v", err)
			}
			t.Fatalf("wait for query requirement validation error: %s: %v", describeQueryRequirementSnapshot(current), err)
		}
		if smoke.HasLauncherResultLabel(snapshot, queryRequirementValidationReadyTitle) {
			t.Fatalf("empty save exposed the plugin result %q", queryRequirementValidationReadyTitle)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
