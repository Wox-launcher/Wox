//go:build wox_ui_smoke

package general

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test001SettingGeneralLaunchModeFresh verifies that fresh launch mode discards the previous launcher query.
// Flow: select Start Fresh in General settings -> run a calculator query -> hide and show the launcher.
// Evidence: the reopened query input is empty and the previous calculator result is absent from the real UI.
func Test001SettingGeneralLaunchModeFresh(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previousMode := smoke.OpenGeneralSettingsAndReadChoice(t, ctx, client, "LaunchMode")
		freshMode := smoke.SelectSettingChoiceByIndex(t, ctx, client, "LaunchMode", 0)
		t.Cleanup(func() {
			if previousMode != freshMode {
				smoke.RestoreGeneralSettingChoice(t, client, "LaunchMode", previousMode)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close General settings after selecting fresh launch mode: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		if !smoke.HasLauncherResultLabel(snapshot, "2") {
			t.Fatal("calculator result was not visible before hiding the launcher")
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher with calculator query: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == "" && !smoke.HasLauncherResultLabel(snapshot, "2")
		})
		if err != nil {
			t.Fatalf("wait for fresh launcher state after re-show: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// Test001SettingGeneralLaunchModeContinue verifies that continue mode restores the previous launcher query and results.
// Flow: select Continue Last Query in General settings -> run a calculator query -> hide and show the launcher.
// Evidence: the reopened input still contains the query and its completed calculator result remains visible.
func Test001SettingGeneralLaunchModeContinue(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		previousMode := smoke.OpenGeneralSettingsAndReadChoice(t, ctx, client, "LaunchMode")
		continueMode := smoke.SelectSettingChoiceByIndex(t, ctx, client, "LaunchMode", 1)
		t.Cleanup(func() {
			if previousMode != continueMode {
				smoke.RestoreGeneralSettingChoice(t, client, "LaunchMode", previousMode)
			}
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close General settings after selecting continue launch mode: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
		if !smoke.HasLauncherResultLabel(snapshot, "2") {
			t.Fatal("calculator result was not visible before hiding the launcher")
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher with calculator query: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "1+1" &&
				resultsFound && results.Value == "complete" &&
				smoke.HasLauncherResultLabel(snapshot, "2")
		})
		if err != nil {
			t.Fatalf("wait for continued launcher state after re-show: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
