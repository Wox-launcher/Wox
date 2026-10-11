//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	queryHotkeyTableID  = "hotkey-settings-field-5"
	queryHotkeyChord    = "ctrl+alt+f12"
	queryHotkeyQuery    = "wox-smoke result-binding "
	queryHotkeyResult   = "Result binding fixture"
	queryHotkeyRunLog   = "result binding fixture executed"
	queryHotkeyRegister = "register normal hotkey: ctrl+alt+f12"
	queryHotkeyNormal   = "query-hotkey-preset-normal"
	queryHotkeySilent   = "query-hotkey-preset-silent"
)

// Test002SettingHotkeyQuery verifies a query hotkey opens its saved query and can run that query silently.
// Flow: save a query hotkey in Hotkey settings -> press it while the launcher is hidden -> switch the same hotkey to silent execution -> press it again.
// Evidence: the first press shows the fixture query and result; the second press keeps the launcher hidden and writes the fixture execution log.
func Test002SettingHotkeyQuery(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native query hotkey recording and activation are covered on Windows")
	}
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() { removeQueryHotkey(t, client) })
		removeQueryHotkey(t, client)

		saveQueryHotkey(t, ctx, client, false)
		triggerVisibleQueryHotkey(t, ctx, client)
		saveQueryHotkey(t, ctx, client, true)
		triggerSilentQueryHotkey(t, ctx, client)
	})
}

// saveQueryHotkey records the fixture chord, or switches that saved row to silent execution.
func saveQueryHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client, silent bool) {
	t.Helper()
	openQueryHotkeySettings(t, ctx, client)
	closeQueryHotkeyEditor(t, ctx, client)
	if silent {
		editQueryHotkey(t, ctx, client)
		if err := client.Perform(ctx, queryHotkeySilent, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("select silent query hotkey: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			preset, found := automationdriver.Find(snapshot, queryHotkeySilent)
			return found && preset.Selected
		}); err != nil {
			t.Fatalf("wait for silent query hotkey preset: %v", err)
		}
	} else {
		addQueryHotkey(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			preset, found := automationdriver.Find(snapshot, queryHotkeyNormal)
			return found && preset.Selected
		}); err != nil {
			t.Fatalf("wait for normal query hotkey preset: %v", err)
		}
	}
	logPath := queryHotkeyLogPath(t)
	logOffset := queryHotkeyLogSize(t, logPath)
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save query hotkey silent=%t: %v", silent, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, editing := automationdriver.Find(snapshot, "form-table-row-save")
		_, saved := queryHotkeyRowIndex(snapshot, queryHotkeyChord)
		return !editing && saved
	}); err != nil {
		t.Fatalf("wait for saved query hotkey silent=%t: %v", silent, err)
	}
	smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
		return int64(len(data)) >= logOffset && strings.Contains(string(data[logOffset:]), queryHotkeyRegister)
	})
}

// addQueryHotkey opens a new row and records the fixture chord and query.
func addQueryHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Perform(ctx, queryHotkeyTableID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add query hotkey: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, hotkeyFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		_, queryFound := automationdriver.Find(snapshot, "form-table-row-field-2")
		return hotkeyFound && queryFound
	}); err != nil {
		t.Fatalf("wait for query hotkey editor: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-1", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("start query hotkey recording: %v", err)
	}
	if err := smoke.SendNativeKeyChord("ctrl", "alt", "f12"); err != nil {
		t.Fatalf("record query hotkey: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, "form-table-row-field-1")
		return found && field.Value == queryHotkeyChord
	}); err != nil {
		t.Fatalf("wait for recorded query hotkey: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-2", woxui.AccessibilityActionSetValue, queryHotkeyQuery); err != nil {
		t.Fatalf("set query hotkey query: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, "form-table-row-field-2")
		return found && field.Value == queryHotkeyQuery
	}); err != nil {
		t.Fatalf("wait for query hotkey query: %v", err)
	}
}

// editQueryHotkey reopens the fixture row without changing its chord or query.
func editQueryHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	rowIndex := waitForQueryHotkeyRow(t, ctx, client)
	editID := fmt.Sprintf("%s-row-%d-edit", queryHotkeyTableID, rowIndex)
	if err := client.Perform(ctx, editID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("edit query hotkey: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		hotkey, hotkeyFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		query, queryFound := automationdriver.Find(snapshot, "form-table-row-field-2")
		_, silentFound := automationdriver.Find(snapshot, queryHotkeySilent)
		return hotkeyFound && hotkey.Value == queryHotkeyChord && queryFound && query.Value == queryHotkeyQuery && silentFound
	}); err != nil {
		t.Fatalf("wait for saved query hotkey editor: %v", err)
	}
}

func triggerVisibleQueryHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	hideLauncher(t, ctx, client)
	if err := smoke.SendNativeKeyChord("ctrl", "alt", "f12"); err != nil {
		t.Fatalf("press query hotkey: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Visible
	}); err != nil {
		t.Fatalf("wait for query hotkey to show launcher: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return inputFound && input.Value == queryHotkeyQuery && resultsFound && results.Value == "complete" && smoke.HasLauncherResultLabel(snapshot, queryHotkeyResult)
	}); err != nil {
		t.Fatalf("wait for query hotkey result %q: %v", queryHotkeyResult, err)
	}
}

func triggerSilentQueryHotkey(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	hideLauncher(t, ctx, client)
	logPath := queryHotkeyLogPath(t)
	logOffset := queryHotkeyLogSize(t, logPath)
	// A second edge covers the case where re-registration happened while the
	// previous chord was still down and the first press only completed the release.
	for range 2 {
		if err := smoke.SendNativeKeyChord("ctrl", "alt", "f12"); err != nil {
			t.Fatalf("press silent query hotkey: %v", err)
		}
	}
	smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
		return int64(len(data)) >= logOffset && strings.Contains(string(data[logOffset:]), queryHotkeyRunLog)
	})
	state, err := client.WindowState(ctx, "primary")
	if err != nil {
		t.Fatalf("read launcher state after silent query hotkey: %v", err)
	}
	if state.Visible {
		t.Fatal("silent query hotkey showed the launcher")
	}
}

func openQueryHotkeySettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide before query hotkey settings: %v", err)
	}
	if err := client.OpenSettings(ctx, "/hotkeys"); err != nil {
		t.Fatalf("open hotkey settings: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, queryHotkeyTableID+"-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for query hotkey table: %v", err)
	}
}

func hideLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return !state.Visible
	}); err != nil {
		t.Fatalf("wait for hidden launcher: %v", err)
	}
}

// removeQueryHotkey deletes the fixture row so the shared process releases the chord.
func removeQueryHotkey(t *testing.T, client *automationdriver.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
	defer cancel()
	openQueryHotkeySettings(t, ctx, client)
	closeQueryHotkeyEditor(t, ctx, client)
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read query hotkey rows: %v", err)
		}
		rowIndex, found := queryHotkeyRowIndex(snapshot, queryHotkeyChord)
		if !found {
			break
		}
		smoke.RemoveApplicationTableRow(t, ctx, client, queryHotkeyTableID, rowIndex)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after removing query hotkey: %v", err)
	}
}

func waitForQueryHotkeyRow(t *testing.T, ctx context.Context, client *automationdriver.Client) int {
	t.Helper()
	var rowIndex int
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		index, found := queryHotkeyRowIndex(snapshot, queryHotkeyChord)
		if found {
			rowIndex = index
		}
		return found
	}); err != nil {
		t.Fatalf("wait for query hotkey row: %v", err)
	}
	return rowIndex
}

func queryHotkeyRowIndex(snapshot woxwidget.AutomationSnapshot, hotkey string) (int, bool) {
	prefix := queryHotkeyTableID + "-row-"
	const suffix = "-cell-1"
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, prefix) || !strings.HasSuffix(node.AutomationID, suffix) {
			continue
		}
		if !strings.EqualFold(node.Value, hotkey) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(node.AutomationID, prefix), suffix)
		index, err := strconv.Atoi(middle)
		if err != nil {
			continue
		}
		return index, true
	}
	return 0, false
}

func closeQueryHotkeyEditor(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read query hotkey editor: %v", err)
	}
	if _, found := automationdriver.Find(snapshot, "form-table-row-cancel"); !found {
		return
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close query hotkey editor: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-row-save")
		return !found
	}); err != nil {
		t.Fatalf("wait for query hotkey editor to close: %v", err)
	}
}

func queryHotkeyLogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
}

func queryHotkeyLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}
