//go:build wox_ui_smoke

package ui

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	queryCompletionPrefix = "theme r"
	queryCompletionText   = "theme restore "
	queryCompletionSuffix = "estore "
)

// Test004SettingUIQueryCompletionHint verifies that query completion hints appear and accept only while enabled.
// Flow: enable hints -> enter a uniquely completable Theme command and press Tab -> disable hints and enter the same prefix.
// Evidence: the live suffix appears then Tab commits the complete command, while the disabled query exposes no completion suffix.
func Test004SettingUIQueryCompletionHint(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousValue := smoke.OpenSettingsAndReadSwitch(t, ctx, client, "/appearance", "EnableQueryCompletionHint")
		t.Cleanup(func() {
			if !previousValue {
				smoke.RestoreSettingSwitch(t, client, "/appearance", "EnableQueryCompletionHint", previousValue)
			}
		})
		smoke.SetSettingSwitch(t, ctx, client, "EnableQueryCompletionHint", true)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close UI settings after enabling query completion hints: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		waitForQueryCompletion(t, ctx, client, queryCompletionPrefix, queryCompletionSuffix)
		if err := client.PressKey(ctx, woxui.KeyTab, 0); err != nil {
			t.Fatalf("accept query completion hint with Tab: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			_, hintFound := automationdriver.Find(snapshot, "launcher.query.completion")
			return inputFound && input.Value == queryCompletionText && !hintFound
		}); err != nil {
			t.Fatalf("wait for accepted query completion: %v", err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher after accepting query completion: %v", err)
		}

		smoke.OpenSettingsAndReadSwitch(t, ctx, client, "/appearance", "EnableQueryCompletionHint")
		smoke.SetSettingSwitch(t, ctx, client, "EnableQueryCompletionHint", false)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close UI settings after disabling query completion hints: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		smoke.ReplaceLauncherQuery(t, ctx, client, queryCompletionPrefix)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			_, hintFound := automationdriver.Find(snapshot, "launcher.query.completion")
			return inputFound && input.Value == queryCompletionPrefix && resultsFound && results.Value == "complete" && !hintFound
		})
		if err != nil {
			t.Fatalf("wait for disabled query completion hint to remain absent: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// waitForQueryCompletion enters one prefix and waits for its live inline completion suffix.
func waitForQueryCompletion(t *testing.T, ctx context.Context, client *automationdriver.Client, prefix, suffix string) {
	t.Helper()
	smoke.ReplaceLauncherQuery(t, ctx, client, prefix)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		hint, hintFound := automationdriver.Find(snapshot, "launcher.query.completion")
		return inputFound && input.Value == prefix && hintFound && hint.Value == suffix
	}); err != nil {
		t.Fatalf("wait for query completion suffix %q: %v", suffix, err)
	}
}
