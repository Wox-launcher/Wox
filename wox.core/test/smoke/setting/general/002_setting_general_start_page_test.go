//go:build wox_ui_smoke

package general

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test002SettingGeneralStartPageBlank verifies that Blank Page suppresses eligible recent items for an empty query.
// Flow: create a durable MRU item -> select fresh launch and Blank Page -> show the launcher with an empty query.
// Evidence: the real launcher input is empty and exposes no result rows despite the persisted MRU seed.
func Test002SettingGeneralStartPageBlank(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureStartPage(t, ctx, client, 0)
		seedConverterMRU(t, ctx, client)

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == "" && !hasLauncherResults(snapshot)
		})
		if err != nil {
			t.Fatalf("wait for blank Start Page: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// Test002SettingGeneralStartPageMRUAfterQueryHotkey verifies clearing a shortcut query restores recent items.
// Flow: seed an MRU item -> save a cb shortcut in Settings -> trigger it while hidden -> clear its query.
// Evidence: the shortcut opens cb results, then the empty completed query displays the seeded MRU item.
func Test002SettingGeneralStartPageMRUAfterQueryHotkey(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native query hotkey recording and activation are covered on Windows")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureStartPage(t, ctx, client, 1)
		mruLabel := seedConverterMRU(t, ctx, client)
		const tableID = "hotkey-settings-field-5"
		if err := client.OpenSettings(ctx, "/hotkeys"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			add, found := automationdriver.Find(snapshot, tableID+"-add")
			return found && add.Enabled
		}); err != nil {
			t.Fatal(err)
		}
		rowIndex := smoke.ApplicationTableRowCount(t, ctx, client, tableID)
		if err := client.Perform(ctx, tableID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "form-table-row-field-2")
			return found
		}); err != nil {
			t.Fatal(err)
		}
		if err := client.Perform(ctx, "form-table-row-field-1", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if err := smoke.SendNativeKeyChord("ctrl", "alt", "f12"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, "form-table-row-field-1")
			return found && field.Value == "ctrl+alt+f12"
		}); err != nil {
			t.Fatalf("wait for query hotkey recording: %v", err)
		}
		if err := client.Perform(ctx, "form-table-row-field-2", woxui.AccessibilityActionSetValue, "cb "); err != nil {
			t.Fatal(err)
		}
		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		logInfo, err := os.Stat(logPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
			defer cancel()
			if err := client.OpenSettings(cleanupCtx, "/hotkeys"); err != nil {
				t.Error(err)
				return
			}
			if _, err := client.WaitFor(cleanupCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, found := automationdriver.Find(snapshot, fmt.Sprintf("%s-row-%d-delete", tableID, rowIndex))
				return found
			}); err != nil {
				t.Error(err)
				return
			}
			smoke.RemoveApplicationTableRow(t, cleanupCtx, client, tableID, rowIndex)
		})
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, editing := automationdriver.Find(snapshot, "form-table-row-save")
			_, saved := automationdriver.Find(snapshot, fmt.Sprintf("%s-row-%d-edit", tableID, rowIndex))
			return !editing && saved
		}); err != nil {
			t.Fatalf("wait for saved query hotkey: %v", err)
		}
		// The row appears before asynchronous OS registration finishes.
		smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
			return int64(len(data)) >= logInfo.Size() && strings.Contains(string(data[logInfo.Size():]), "register normal hotkey: ctrl+alt+f12")
		})
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		if err := smoke.SendNativeKeyChord("ctrl", "alt", "f12"); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Visible
		}); err != nil {
			t.Fatalf("wait for query hotkey to show launcher: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "cb " && resultsFound && results.Value == "complete"
		}); err != nil {
			t.Fatalf("wait for clipboard shortcut query: %v", err)
		}
		smoke.ReplaceLauncherQuery(t, ctx, client, "")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "" && resultsFound && results.Value == "complete" && smoke.HasLauncherResultLabel(snapshot, mruLabel)
		})
		if err != nil {
			t.Fatalf("wait for MRU after clearing query hotkey: %v; snapshot: %+v", err, snapshot)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// Test002SettingGeneralStartPageMRU verifies that Most Recently Used restores eligible items for an empty query.
// Flow: create a durable MRU item -> select fresh launch and Most Recently Used -> show the launcher with an empty query.
// Evidence: the real launcher keeps an empty input while exposing the restored Converter result in a completed generation.
func Test002SettingGeneralStartPageMRU(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		configureStartPage(t, ctx, client, 1)
		mruLabel := seedConverterMRU(t, ctx, client)

		smoke.ShowLauncher(t, ctx, client)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == "" && resultsFound && results.Value == "complete" && smoke.HasLauncherResultLabel(snapshot, mruLabel)
		})
		if err != nil {
			t.Fatalf("wait for MRU Start Page result %q: %v", mruLabel, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
