//go:build wox_ui_smoke

package query

import (
	"context"
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

const (
	resultBindingQuery       = "wox-smoke result-binding "
	resultBindingTitle       = "Result binding fixture"
	resultBindingAlias       = "smokebind"
	resultBindingHotkey      = "ctrl+alt+f12"
	resultBindingLogMarker   = "result binding fixture executed"
	setResultHotkeyAction    = "action-result-__system_set_result_hotkey__-"
	setResultAliasAction     = "action-result-__system_set_result_alias__-"
	resultBindingHotkeyField = "action-form-field-0"
	resultBindingAliasField  = "action-form-field-0"
	resultBindingFormSave    = "form-save"
)

// Test029LauncherResultBinding verifies a restorable result can keep both an alias and a global hotkey.
// Flow: query the fixture -> save an alias through its action form -> record a hotkey through its action form -> query the alias -> press the hotkey while the launcher is hidden.
// Evidence: title tags appear on the original and alias-restored rows, and the hidden hotkey writes the fixture execution log.
func Test029LauncherResultBinding(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		cleanupResultBindings(t, client)
		t.Cleanup(func() { cleanupResultBindings(t, client) })

		smoke.ShowLauncher(t, ctx, client)
		queryResultBindingFixture(t, ctx, client)
		saveResultBindingAlias(t, ctx, client)
		snapshot := waitForResultBindingTags(t, ctx, client, resultBindingQuery, resultBindingAlias)

		if resultBindingHotkeySupported() {
			recordResultBindingHotkey(t, ctx, client)
			snapshot = waitForResultBindingTags(t, ctx, client, resultBindingQuery, resultBindingAlias, resultBindingHotkey)
		}

		snapshot = waitForResultBindingTags(t, ctx, client, resultBindingAlias, resultBindingAlias)
		if resultBindingHotkeySupported() {
			assertResultBindingTags(t, snapshot, resultBindingAlias, resultBindingHotkey)
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		if !resultBindingHotkeySupported() {
			return
		}

		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher before result hotkey: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return !state.Visible
		}); err != nil {
			t.Fatalf("wait for hidden launcher before result hotkey: %v", err)
		}
		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		offset := resultBindingLogSize(t, logPath)
		if err := smoke.SendNativeKeyChord(strings.Split(resultBindingHotkey, "+")...); err != nil {
			t.Fatalf("send result binding hotkey %q: %v", resultBindingHotkey, err)
		}
		smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
			return int64(len(data)) >= offset && strings.Contains(string(data[offset:]), resultBindingLogMarker)
		})
		state, err := client.WindowState(ctx, "primary")
		if err != nil {
			t.Fatalf("read launcher state after result hotkey: %v", err)
		}
		if state.Visible {
			t.Fatal("result hotkey showed the launcher instead of executing silently")
		}
	})
}

func resultBindingHotkeySupported() bool {
	return runtime.GOOS == "windows"
}

func queryResultBindingFixture(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, resultBindingQuery)
	resultID, found := smoke.FindLauncherResult(snapshot, resultBindingTitle)
	if !found {
		t.Fatalf("result binding fixture %q was not found", resultBindingTitle)
	}
	result, _ := automationdriver.Find(snapshot, resultID)
	if !result.Selected {
		smoke.SelectLauncherResult(t, ctx, client, resultID)
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read result binding fixture after selecting it: %v", err)
		}
		return snapshot
	}
	return snapshot
}

func saveResultBindingAlias(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ActivateSelectedResultAction(t, ctx, client, setResultAliasAction)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, resultBindingAliasField)
		_, saveFound := automationdriver.Find(snapshot, resultBindingFormSave)
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for set-result-alias form: %v", err)
	}
	if err := client.Perform(ctx, resultBindingAliasField, woxui.AccessibilityActionSetValue, resultBindingAlias); err != nil {
		t.Fatalf("set result alias: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, resultBindingAliasField)
		return found && field.Value == resultBindingAlias
	}); err != nil {
		t.Fatalf("wait for result alias field %q: %v", resultBindingAlias, err)
	}
	if err := client.Perform(ctx, resultBindingFormSave, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save result alias: %v", err)
	}
	waitForResultBindingFormClosed(t, ctx, client)
}

func recordResultBindingHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ActivateSelectedResultAction(t, ctx, client, setResultHotkeyAction)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, fieldFound := automationdriver.Find(snapshot, resultBindingHotkeyField)
		_, saveFound := automationdriver.Find(snapshot, resultBindingFormSave)
		return fieldFound && saveFound && field.Description != ""
	}); err != nil {
		t.Fatalf("wait for set-result-hotkey form: %v", err)
	}
	if err := smoke.SendNativeKeyChord(strings.Split(resultBindingHotkey, "+")...); err != nil {
		t.Fatalf("record result binding hotkey %q: %v", resultBindingHotkey, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, resultBindingHotkeyField)
		return found && field.Value == resultBindingHotkey
	}); err != nil {
		t.Fatalf("wait for recorded result hotkey %q: %v", resultBindingHotkey, err)
	}
	if err := client.Perform(ctx, resultBindingFormSave, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save result hotkey: %v", err)
	}
	waitForResultBindingFormClosed(t, ctx, client)
}

// waitForResultBindingTags queries and waits until the fixture row exposes the saved alias or hotkey chips.
func waitForResultBindingTags(t *testing.T, ctx context.Context, client *automationdriver.Client, query string, tags ...string) woxwidget.AutomationSnapshot {
	t.Helper()
	smoke.ReplaceLauncherQuery(t, ctx, client, query)
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		result, found := resultBindingResult(snapshot)
		return found && resultBindingHasTags(snapshot, result, tags...)
	})
	if err != nil {
		result, found := resultBindingResult(snapshot)
		t.Fatalf("wait for result binding tags %v on %q: found=%v result=%+v: %v", tags, query, found, result, err)
	}
	return snapshot
}

func resultBindingResult(snapshot woxwidget.AutomationSnapshot) (woxui.AccessibilityNode, bool) {
	id, found := smoke.FindLauncherResult(snapshot, resultBindingTitle)
	if !found {
		return woxui.AccessibilityNode{}, false
	}
	return automationdriver.Find(snapshot, id)
}

func resultBindingHasTags(snapshot woxwidget.AutomationSnapshot, result woxui.AccessibilityNode, tags ...string) bool {
	haystack := result.Description + " " + result.Label
	for _, tag := range tags {
		if strings.Contains(haystack, tag) || strings.Contains(strings.ToUpper(haystack), strings.ToUpper(tag)) {
			continue
		}
		if _, found := automationdriver.Find(snapshot, "result-title-tag-result-binding-fixture-"+tag); found {
			continue
		}
		return false
	}
	return len(tags) > 0
}

func assertResultBindingTags(t *testing.T, snapshot woxwidget.AutomationSnapshot, tags ...string) {
	t.Helper()
	result, found := resultBindingResult(snapshot)
	if !found || !resultBindingHasTags(snapshot, result, tags...) {
		t.Fatalf("result binding tags %v missing: found=%v result=%+v", tags, found, result)
	}
}

func waitForResultBindingFormClosed(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, formOpen := automationdriver.Find(snapshot, resultBindingFormSave)
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return !formOpen && resultsFound && results.Value == "complete"
	}); err != nil {
		t.Fatalf("wait for result binding form to close: %v", err)
	}
}

// cleanupResultBindings deletes the fixture row from Settings so the shared process does not keep the binding.
func cleanupResultBindings(t *testing.T, client *automationdriver.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before clearing result bindings: %v", err)
	}
	if err := client.OpenSettings(ctx, "/hotkeys"); err != nil {
		t.Errorf("open Hotkey settings to clear result bindings: %v", err)
		return
	}
	if err := client.Perform(ctx, "settings-search-field", woxui.AccessibilityActionSetValue, "ResultBindings"); err != nil {
		t.Errorf("search ResultBindings: %v", err)
		return
	}
	if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
		t.Errorf("open ResultBindings from search: %v", err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.hotkey")
		return pageFound
	}); err != nil {
		t.Errorf("wait for Hotkey settings: %v", err)
		return
	}

	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Errorf("read ResultBindings table: %v", err)
			break
		}
		deleteID, found := resultBindingRowDeleteID(snapshot)
		if !found {
			break
		}
		if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Errorf("delete result binding %q: %v", deleteID, err)
			break
		}
		if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Errorf("confirm result binding deletion %q: %v", deleteID, err)
			break
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, stillPresent := automationdriver.Find(snapshot, deleteID)
			return !stillPresent
		}); err != nil {
			t.Errorf("wait for result binding row removal: %v", err)
			break
		}
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after clearing result bindings: %v", err)
	}
}

// resultBindingRowDeleteID maps the fixture title cell to its inline-table delete control.
func resultBindingRowDeleteID(snapshot woxwidget.AutomationSnapshot) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if !strings.Contains(node.AutomationID, "-row-") || !strings.HasSuffix(node.AutomationID, "-cell-0") {
			continue
		}
		if node.Label != resultBindingTitle && node.Value != resultBindingTitle {
			continue
		}
		return strings.TrimSuffix(node.AutomationID, "-cell-0") + "-delete", true
	}
	return "", false
}

func resultBindingLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}
