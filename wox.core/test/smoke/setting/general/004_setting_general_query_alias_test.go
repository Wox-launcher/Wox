//go:build wox_ui_smoke

package general

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	queryAliasFieldID = "general-settings-field-0"
	smokeQueryAlias   = "woxsmokealias"
	smokeAliasQuery   = "1+1"
)

// Test004SettingGeneralQueryAlias verifies a query alias expands into the aliased query.
// Flow: add an alias in General settings -> reopen the row to confirm it was saved -> query the alias -> remove the alias.
// Evidence: the alias query shows calculator result 2, and the same query does not show it after the alias is removed.
func Test004SettingGeneralQueryAlias(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			removeQueryAlias(t, cleanupCtx, client, smokeQueryAlias)
		})

		removeQueryAlias(t, ctx, client, smokeQueryAlias)
		addQueryAlias(t, ctx, client, smokeQueryAlias, smokeAliasQuery)
		waitForQueryAlias(t, ctx, smokeQueryAlias, true)
		confirmQueryAlias(t, ctx, client, smokeQueryAlias, smokeAliasQuery)
		assertAliasQuery(t, ctx, client, true)

		removeQueryAlias(t, ctx, client, smokeQueryAlias)
		waitForQueryAlias(t, ctx, smokeQueryAlias, false)
		assertAliasQuery(t, ctx, client, false)
	})
}

func openQueryAliasSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide before query alias settings: %v", err)
	}
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open general settings: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, queryAliasFieldID+"-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for query alias table: %v", err)
	}
}

func addQueryAlias(t *testing.T, ctx context.Context, client *automationdriver.Client, shortcut, query string) {
	t.Helper()
	openQueryAliasSettings(t, ctx, client)
	closeAliasEditorIfOpen(t, ctx, client)
	if err := client.Perform(ctx, queryAliasFieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add query alias row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, shortcutFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, queryFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return shortcutFound && queryFound && saveFound
	}); err != nil {
		t.Fatalf("wait for query alias editor: %v", err)
	}
	setAliasField(t, ctx, client, 0, shortcut)
	setAliasField(t, ctx, client, 1, query)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		disabled, found := automationdriver.Find(snapshot, "form-table-row-field-2")
		return found && !disabled.Checked
	}); err != nil {
		t.Fatalf("wait for enabled query alias checkbox: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save query alias: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		add, addFound := automationdriver.Find(snapshot, queryAliasFieldID+"-add")
		return !editorFound && addFound && add.Enabled
	}); err != nil {
		t.Fatalf("wait for saved query alias row: %v", err)
	}
}

func confirmQueryAlias(t *testing.T, ctx context.Context, client *automationdriver.Client, shortcut, query string) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after saving query alias: %v", err)
	}
	openQueryAliasSettings(t, ctx, client)
	rowIndex := waitForAliasRow(t, ctx, client, shortcut)
	editID := fmt.Sprintf("%s-row-%d-edit", queryAliasFieldID, rowIndex)
	if err := client.Perform(ctx, editID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("reopen query alias row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		shortcutField, shortcutFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		queryField, queryFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		disabled, disabledFound := automationdriver.Find(snapshot, "form-table-row-field-2")
		return shortcutFound && shortcutField.Value == shortcut && queryFound && queryField.Value == query && disabledFound && !disabled.Checked
	}); err != nil {
		t.Fatalf("confirm saved query alias %q: %v", shortcut, err)
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close query alias inspection: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-row-save")
		return !found
	}); err != nil {
		t.Fatalf("wait for query alias inspection to close: %v", err)
	}
}

func removeQueryAlias(t *testing.T, ctx context.Context, client *automationdriver.Client, shortcut string) {
	t.Helper()
	openQueryAliasSettings(t, ctx, client)
	closeAliasEditorIfOpen(t, ctx, client)
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read query alias rows: %v", err)
		}
		rowIndex, found := aliasRowIndex(snapshot, shortcut)
		if !found {
			break
		}
		smoke.RemoveApplicationTableRow(t, ctx, client, queryAliasFieldID, rowIndex)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after removing query alias: %v", err)
	}
}

func assertAliasQuery(t *testing.T, ctx context.Context, client *automationdriver.Client, wantResult bool) {
	t.Helper()
	// Settings stays the automation surface while it is open, so the launcher
	// query is invisible until that window closes.
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close settings before alias query: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, smokeQueryAlias)
	found := false
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
			found = true
			break
		}
	}
	if found != wantResult {
		t.Fatalf("alias query %q result=%t, want %t", smokeQueryAlias, found, wantResult)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func waitForAliasRow(t *testing.T, ctx context.Context, client *automationdriver.Client, shortcut string) int {
	t.Helper()
	var rowIndex int
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		index, found := aliasRowIndex(snapshot, shortcut)
		if found {
			rowIndex = index
		}
		return found
	}); err != nil {
		t.Fatalf("wait for query alias %q: %v", shortcut, err)
	}
	return rowIndex
}

func aliasRowIndex(snapshot woxwidget.AutomationSnapshot, shortcut string) (int, bool) {
	prefix := queryAliasFieldID + "-row-"
	const suffix = "-cell-0"
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, prefix) || !strings.HasSuffix(node.AutomationID, suffix) {
			continue
		}
		if node.Value != shortcut {
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

func setAliasField(t *testing.T, ctx context.Context, client *automationdriver.Client, index int, value string) {
	t.Helper()
	fieldID := "form-table-row-field-" + strconv.Itoa(index)
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set query alias field %d: %v", index, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, fieldID)
		return found && field.Value == value
	}); err != nil {
		t.Fatalf("confirm query alias field %d: %v", index, err)
	}
}

func closeAliasEditorIfOpen(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read query alias editor: %v", err)
	}
	if _, found := automationdriver.Find(snapshot, "form-table-row-cancel"); !found {
		return
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close query alias editor: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-row-save")
		return !found
	}); err != nil {
		t.Fatalf("wait for query alias editor to close: %v", err)
	}
}

// waitForQueryAlias polls General settings until the shortcut is present or gone.
func waitForQueryAlias(t *testing.T, ctx context.Context, shortcut string, present bool) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	var lastErr error
	for {
		value, err := readWoxSetting(t, "QueryAliases")
		if err == sql.ErrNoRows {
			value = ""
			err = nil
		}
		if err == nil && strings.Contains(value, shortcut) == present {
			return
		}
		last, lastErr = value, err
		select {
		case <-ctx.Done():
			t.Fatalf("wait for query alias %q present=%t: last=%q err=%v: %v", shortcut, present, last, lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}

func readWoxSetting(t *testing.T, key string) (string, error) {
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
	err = database.QueryRow("SELECT value FROM wox_settings WHERE key = ?", key).Scan(&value)
	return value, err
}
