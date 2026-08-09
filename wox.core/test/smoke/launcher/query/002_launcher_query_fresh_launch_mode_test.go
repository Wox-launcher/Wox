//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test002LauncherQueryFreshLaunchMode verifies that fresh launch mode discards the previous launcher query.
// Flow: select Start Fresh in General settings -> run a calculator query -> hide and show the launcher.
// Evidence: the reopened query input is empty and the previous calculator result is absent from the real UI.
func Test002LauncherQueryFreshLaunchMode(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previousMode := openGeneralSettingsAndReadChoice(t, ctx, client, "LaunchMode")
		freshMode := selectGeneralSettingChoice(t, ctx, client, "LaunchMode", 0)
		t.Cleanup(func() {
			if previousMode != freshMode {
				restoreGeneralSettingChoice(t, client, "LaunchMode", previousMode)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close General settings after selecting fresh launch mode: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		if !hasLauncherResultLabel(snapshot, "2") {
			t.Fatal("calculator result was not visible before hiding the launcher")
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher with calculator query: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == "" && !hasLauncherResultLabel(snapshot, "2")
		})
		if err != nil {
			t.Fatalf("wait for fresh launcher state after re-show: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
