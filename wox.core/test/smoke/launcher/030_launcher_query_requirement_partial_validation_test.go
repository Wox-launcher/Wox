//go:build wox_ui_smoke

package query

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const (
	queryRequirementPartialTrigger    = "qreqpart"
	queryRequirementPartialPluginID   = "com.wox.smoke.queryrequirement.partial"
	queryRequirementPartialClientID   = "smoke-client-id"
	queryRequirementPartialPluginFile = "Wox.Plugin.SmokeQueryRequirementPartial.py"
	queryRequirementPartialReadyTitle = "query requirement ready:"
)

// Test030LauncherQueryRequirementPartialValidation verifies an empty required field keeps its own error when another required field is filled.
// Flow: install a plugin that requires accessKey and clientId -> query its trigger -> fill only clientId -> save.
// Evidence: the form stays open, only the empty accessKey field shows a validator error, and the plugin result does not appear.
func Test030LauncherQueryRequirementPartialValidation(t *testing.T) {
	writeQueryRequirementPlugin(t, queryRequirementPartialPluginFile, queryRequirementTwoFieldPluginSource(queryRequirementPartialPluginID, "Query Requirement Partial Smoke", queryRequirementPartialTrigger))
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		blocked := waitForQueryRequirementForm(t, ctx, client, queryRequirementPartialTrigger+" ")
		if _, found := automationdriver.Find(blocked, queryRequirementFieldAutomationID(1)); !found {
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, found := automationdriver.Find(snapshot, queryRequirementFieldAutomationID(1))
				return found && queryRequirementFormVisible(snapshot)
			}); err != nil {
				current, snapErr := client.Snapshot(ctx)
				if snapErr != nil {
					t.Fatalf("wait for second query requirement field: %v", err)
				}
				t.Fatalf("wait for second query requirement field: %s: %v", describeQueryRequirementSnapshot(current), err)
			}
		}
		smoke.AssertNoDiagnostics(t, blocked)

		fillQueryRequirementField(t, ctx, client, queryRequirementFieldAutomationID(1), queryRequirementPartialClientID)
		saveQueryRequirementForm(t, ctx, client)

		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			accessError, accessFound := automationdriver.Find(snapshot, queryRequirementErrorAutomationID(0))
			_, clientErrorFound := automationdriver.Find(snapshot, queryRequirementErrorAutomationID(1))
			return queryRequirementFormVisible(snapshot) && accessFound && strings.TrimSpace(accessError.Value) != "" && !clientErrorFound
		})
		if err != nil {
			current, snapErr := client.Snapshot(ctx)
			if snapErr != nil {
				t.Fatalf("wait for accessKey-only query requirement error: %v", err)
			}
			t.Fatalf("wait for accessKey-only query requirement error: %s: %v", describeQueryRequirementSnapshot(current), err)
		}
		if smoke.HasLauncherResultLabel(snapshot, queryRequirementPartialReadyTitle) {
			t.Fatalf("partial save exposed the plugin result %q", queryRequirementPartialReadyTitle)
		}
		clientField, clientFound := automationdriver.Find(snapshot, queryRequirementFieldAutomationID(1))
		if !clientFound || clientField.Value != queryRequirementPartialClientID {
			t.Fatalf("filled clientId = %q found=%t, want to keep %q", clientField.Value, clientFound, queryRequirementPartialClientID)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
