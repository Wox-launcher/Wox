//go:build wox_ui_smoke

package calculator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"wox/common"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	calculatorKeywordFieldID = "plugin-settings-field-0"
	smokeCalculatorKeyword   = "woxsmokecalc"
)

// openCalculatorSettings opens Calculator and waits until its keyword table is ready.
func openCalculatorSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide before calculator settings: %v", err)
	}
	smoke.OpenInstalledPluginSettings(t, ctx, client, common.CalculatorPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, calculatorKeywordFieldID+"-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for calculator settings: %v", err)
	}
}

// setCalculatorEnabled drives the header enable or disable action and waits until it finishes.
func setCalculatorEnabled(t *testing.T, ctx context.Context, client *automationdriver.Client, enabled bool) {
	t.Helper()
	openCalculatorSettings(t, ctx, client)
	if enabled {
		if !pluginButtonExists(t, ctx, client, "plugin-enable") {
			if pluginButtonExists(t, ctx, client, "plugin-disable") {
				return
			}
			t.Fatal("calculator settings has neither enable nor disable")
		}
		activatePluginButton(t, ctx, client, "plugin-enable", "plugin-disable")
		return
	}
	if !pluginButtonExists(t, ctx, client, "plugin-disable") {
		if pluginButtonExists(t, ctx, client, "plugin-enable") {
			return
		}
		t.Fatal("calculator settings has neither enable nor disable")
	}
	activatePluginButton(t, ctx, client, "plugin-disable", "plugin-enable")
}

func pluginButtonExists(t *testing.T, ctx context.Context, client *automationdriver.Client, buttonID string) bool {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read calculator plugin button %s: %v", buttonID, err)
	}
	button, found := automationdriver.Find(snapshot, buttonID)
	return found && button.Enabled
}

func activatePluginButton(t *testing.T, ctx context.Context, client *automationdriver.Client, buttonID, nextID string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		button, found := automationdriver.Find(snapshot, buttonID)
		return found && button.Enabled
	}); err != nil {
		t.Fatalf("wait for %s: %v", buttonID, err)
	}
	if err := client.Perform(ctx, buttonID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate %s: %v", buttonID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		next, found := automationdriver.Find(snapshot, nextID)
		_, previousFound := automationdriver.Find(snapshot, buttonID)
		return found && next.Enabled && !previousFound
	}); err != nil {
		t.Fatalf("wait for %s after %s: %v", nextID, buttonID, err)
	}
}

// assertCalculatorQuery reports whether the completed query exposes the calculator result 2.
func assertCalculatorQuery(t *testing.T, ctx context.Context, client *automationdriver.Client, query string, wantResult bool) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide before calculator query %q: %v", query, err)
	}
	smoke.ShowLauncher(t, ctx, client)
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, query)
	_, found := calculatorResult(snapshot)
	if found != wantResult {
		t.Fatalf("calculator query %q result=%t, want %t; results: %s", query, found, wantResult, formatLauncherResults(snapshot))
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func formatLauncherResults(snapshot woxwidget.AutomationSnapshot) string {
	parts := make([]string, 0)
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		parts = append(parts, node.Label)
		if len(parts) == 20 {
			break
		}
	}
	if len(parts) == 0 {
		return "(no results)"
	}
	return strings.Join(parts, ", ")
}

// setCalculatorTriggerKeywords replaces Calculator's keywords and reopens the table to confirm the save.
func setCalculatorTriggerKeywords(t *testing.T, ctx context.Context, client *automationdriver.Client, keywords []string) {
	t.Helper()
	if len(keywords) == 0 {
		t.Fatal("calculator trigger keywords must keep one row")
	}
	openCalculatorSettings(t, ctx, client)
	closeTableEditorIfOpen(t, ctx, client)
	collapseCalculatorKeywords(t, ctx, client)
	writeCalculatorKeyword(t, ctx, client, 0, keywords[0], false)
	for index := 1; index < len(keywords); index++ {
		rowIndex := smoke.ApplicationTableRowCount(t, ctx, client, calculatorKeywordFieldID)
		writeCalculatorKeyword(t, ctx, client, rowIndex, keywords[index], true)
	}
	waitForCalculatorKeywords(t, ctx, keywords)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after saving calculator keywords: %v", err)
	}
	openCalculatorSettings(t, ctx, client)
	closeTableEditorIfOpen(t, ctx, client)
	got := readCalculatorKeywords(t, ctx, client)
	if !sameKeywords(got, keywords) {
		t.Fatalf("calculator trigger keywords = %q, want %q", got, keywords)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after confirming calculator keywords: %v", err)
	}
}

func collapseCalculatorKeywords(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	for {
		count := smoke.ApplicationTableRowCount(t, ctx, client, calculatorKeywordFieldID)
		if count <= 1 {
			if count == 0 {
				t.Fatal("calculator trigger keyword table is empty")
			}
			return
		}
		smoke.RemoveApplicationTableRow(t, ctx, client, calculatorKeywordFieldID, count-1)
	}
}

func writeCalculatorKeyword(t *testing.T, ctx context.Context, client *automationdriver.Client, rowIndex int, keyword string, create bool) {
	t.Helper()
	controlID := fmt.Sprintf("%s-row-%d-edit", calculatorKeywordFieldID, rowIndex)
	if create {
		controlID = calculatorKeywordFieldID + "-add"
	}
	if err := client.Perform(ctx, controlID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open calculator keyword row %d: %v", rowIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for calculator keyword editor: %v", err)
	}
	setFormTableText(t, ctx, client, 0, keyword)
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save calculator keyword %q: %v", keyword, err)
	}
	waitForSavedCalculatorRow(t, ctx, client, rowIndex)
}

func waitForSavedCalculatorRow(t *testing.T, ctx context.Context, client *automationdriver.Client, rowIndex int) {
	t.Helper()
	editID := fmt.Sprintf("%s-row-%d-edit", calculatorKeywordFieldID, rowIndex)
	deleteID := fmt.Sprintf("%s-row-%d-delete", calculatorKeywordFieldID, rowIndex)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		edit, editFound := automationdriver.Find(snapshot, editID)
		remove, deleteFound := automationdriver.Find(snapshot, deleteID)
		add, addFound := automationdriver.Find(snapshot, calculatorKeywordFieldID+"-add")
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return editFound && edit.Enabled && deleteFound && remove.Enabled && addFound && add.Enabled && !editorFound
	}); err != nil {
		t.Fatalf("wait for calculator keyword row %d: %v", rowIndex, err)
	}
}

func readCalculatorKeywords(t *testing.T, ctx context.Context, client *automationdriver.Client) []string {
	t.Helper()
	count := smoke.ApplicationTableRowCount(t, ctx, client, calculatorKeywordFieldID)
	keywords := make([]string, 0, count)
	for index := 0; index < count; index++ {
		editID := fmt.Sprintf("%s-row-%d-edit", calculatorKeywordFieldID, index)
		if err := client.Perform(ctx, editID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("inspect calculator keyword row %d: %v", index, err)
		}
		var value string
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, "form-table-row-field-0")
			if !found {
				return false
			}
			value = field.Value
			return true
		}); err != nil {
			t.Fatalf("read calculator keyword row %d: %v", index, err)
		}
		keywords = append(keywords, value)
		if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("close calculator keyword row %d: %v", index, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "form-table-row-save")
			return !found
		}); err != nil {
			t.Fatalf("wait for calculator keyword inspection to close: %v", err)
		}
	}
	return keywords
}

func closeTableEditorIfOpen(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read calculator table editor: %v", err)
	}
	if _, found := automationdriver.Find(snapshot, "form-table-row-cancel"); !found {
		return
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close calculator table editor: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-row-save")
		return !found
	}); err != nil {
		t.Fatalf("wait for calculator table editor to close: %v", err)
	}
}

func setFormTableText(t *testing.T, ctx context.Context, client *automationdriver.Client, index int, value string) {
	t.Helper()
	fieldID := "form-table-row-field-" + strconv.Itoa(index)
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set calculator table field %d: %v", index, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, fieldID)
		return found && field.Value == value
	}); err != nil {
		t.Fatalf("confirm calculator table field %d: %v", index, err)
	}
}

func sameKeywords(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, keyword := range left {
		counts[keyword]++
	}
	for _, keyword := range right {
		counts[keyword]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

// waitForCalculatorKeywords polls the saved trigger-keyword list until it matches keywords.
func waitForCalculatorKeywords(t *testing.T, ctx context.Context, keywords []string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	var lastErr error
	for {
		value, err := readPluginSetting(t, common.CalculatorPluginID, "TriggerKeywords")
		if err == nil && storedKeywordsMatch(value, keywords) {
			return
		}
		if err == sql.ErrNoRows {
			value = ""
			err = nil
		}
		last, lastErr = value, err
		select {
		case <-ctx.Done():
			t.Fatalf("wait for calculator keywords %q: last=%q err=%v: %v", keywords, last, lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}

func storedKeywordsMatch(value string, keywords []string) bool {
	var stored []string
	if err := json.Unmarshal([]byte(value), &stored); err != nil {
		return false
	}
	return sameKeywords(stored, keywords)
}

func readPluginSetting(t *testing.T, pluginID, key string) (string, error) {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	database, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return "", err
	}
	defer database.Close()
	var value string
	err = database.QueryRow("SELECT value FROM plugin_settings WHERE plugin_id = ? AND key = ?", pluginID, key).Scan(&value)
	return value, err
}
