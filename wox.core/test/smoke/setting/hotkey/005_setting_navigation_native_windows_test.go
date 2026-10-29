//go:build wox_ui_smoke && windows

package hotkey

import (
	"context"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test005SettingNavigationNativeWindows verifies Alt navigation and result numbering through Windows keyboard input.
// Flow: enable Emacs and navigate -> switch to Vim -> press Alt+J during the hold delay -> release and hold Alt again -> select the second fixture by number.
// Evidence: the selected result changes, the interrupted hold stays unnumbered, and the fresh hold numbers and activates the second result.
func Test005SettingNavigationNativeWindows(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previous := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/hotkey", navigationSetting)
		t.Cleanup(func() { smoke.RestoreSettingChoice(t, client, "/hotkey", navigationSetting, previous) })
		smoke.SelectSettingChoiceByIndex(t, ctx, client, navigationSetting, 1)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		t.Cleanup(func() { _ = smoke.SendNativeKeyEvent("alt", false) })
		const gridQuery = "wox-smoke grid-500 "
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, gridQuery)
		for _, step := range []struct{ key, selected string }{{"f", "launcher.result.perf-grid-0001"}, {"b", "launcher.result.perf-grid-0000"}} {
			sendNavigationNativeKey(t, "alt", true)
			sendNavigationNativeKey(t, step.key, true)
			sendNavigationNativeKey(t, step.key, false)
			sendNavigationNativeKey(t, "alt", false)
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				result, found := automationdriver.Find(snapshot, step.selected)
				input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
				return found && result.Selected && inputFound && input.Value == gridQuery
			}); err != nil {
				t.Fatalf("wait for native grid selection %s: %v", step.selected, err)
			}
		}
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, navigationQuery)
		selectFirstNavigationResult(t, ctx, client)
		for _, step := range []struct{ key, selected string }{{"n", secondResult}, {"p", firstResult}} {
			sendNavigationNativeKey(t, "alt", true)
			sendNavigationNativeKey(t, step.key, true)
			sendNavigationNativeKey(t, step.key, false)
			sendNavigationNativeKey(t, "alt", false)
			waitNavigationResult(t, ctx, client, step.selected)
		}
		smoke.OpenSettingsAndReadChoice(t, ctx, client, "/hotkey", navigationSetting)
		smoke.SelectSettingChoiceByIndex(t, ctx, client, navigationSetting, 2)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, navigationQuery)
		selectFirstNavigationResult(t, ctx, client)
		started := time.Now()
		sendNavigationNativeKey(t, "alt", true)
		sendNavigationNativeKey(t, "j", true)
		sendNavigationNativeKey(t, "j", false)
		if time.Since(started) >= 300*time.Millisecond {
			t.Fatal("native interruption arrived too late to test the pending hold")
		}
		waitNavigationResult(t, ctx, client, secondResult)
		assertNoQuickNumbers(t, ctx, client, 700*time.Millisecond)
		sendNavigationNativeKey(t, "alt", false)
		sendNavigationNativeKey(t, "alt", true)
		snapshot, err := client.WaitFor(ctx, navigationResultsNumbered)
		if err != nil {
			t.Fatalf("wait for native Alt hold numbers: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		number := resultNumber(snapshot, secondResult)
		sendNavigationNativeKey(t, number, true)
		sendNavigationNativeKey(t, number, false)
		sendNavigationNativeKey(t, "alt", false)
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && !state.Visible && state.Lifecycle == "hidden"
		}); err != nil {
			t.Fatalf("wait for native numbered action: %v", err)
		}
	})
}

func sendNavigationNativeKey(t *testing.T, key string, down bool) {
	t.Helper()
	if err := smoke.SendNativeKeyEvent(key, down); err != nil {
		t.Fatalf("send native key %s down=%t: %v", key, down, err)
	}
}
