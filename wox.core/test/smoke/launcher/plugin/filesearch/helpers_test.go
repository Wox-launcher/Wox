//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	fileSearchPluginID            = "979d6363-025a-4f51-88d3-0b04e9dc56bf"
	fileSearchInitialIndexTimeout = 30 * time.Second
	fileSearchIncrementalTimeout  = 8 * time.Second
	fileSearchIndexPollInterval   = 25 * time.Millisecond
)

var (
	fileSearchRootsFieldID          = fileSearchSettingFieldID(1)
	fileSearchIgnorePatternsFieldID = fileSearchSettingFieldID(4)
)

// fileSearchSettingFieldID accounts for the Windows-only file-index-service row.
func fileSearchSettingFieldID(index int) string {
	if runtime.GOOS == "windows" {
		index++
	}
	return fmt.Sprintf("plugin-settings-field-%d", index)
}

func newFileSearchRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory for File Search smoke root: %v", err)
	}
	baseDirectory := filepath.Join(workingDirectory, ".tmp-filesearch-roots")
	if err := os.MkdirAll(baseDirectory, 0o755); err != nil {
		t.Fatalf("create File Search smoke root directory: %v", err)
	}
	root, err := os.MkdirTemp(baseDirectory, "filesearch-smoke-root-")
	if err != nil {
		t.Fatalf("create File Search smoke root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Errorf("remove File Search smoke root %q: %v", root, err)
		}
		if err := os.Remove(baseDirectory); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("remove File Search smoke root directory %q: %v", baseDirectory, err)
		}
	})
	return root
}

func writeFileSearchFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("file search smoke fixture"), 0o644); err != nil {
		t.Fatalf("write File Search fixture %q: %v", path, err)
	}
}

func mkdirFileSearchDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create File Search directory %q: %v", path, err)
	}
}

// fileSearchNativeAbsolutePath returns the host's native absolute path for one fixture.
func fileSearchNativeAbsolutePath(t *testing.T, raw string) string {
	t.Helper()
	absolute, err := filepath.Abs(raw)
	if err != nil {
		t.Fatalf("resolve File Search absolute path %q: %v", raw, err)
	}
	return filepath.Clean(absolute)
}

// addFileSearchRoot adds one directory through the installed plugin's real Settings table.
func addFileSearchRoot(t *testing.T, ctx context.Context, client *automationdriver.Client, root string) int {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
	rowIndex := fileSearchSettingTableRowCount(t, ctx, client, fileSearchRootsFieldID)
	if err := client.Perform(ctx, fileSearchRootsFieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add File Search root row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for File Search root editor: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionSetValue, root); err != nil {
		t.Fatalf("set File Search root %q: %v", root, err)
	}
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save File Search root %q: %v", root, err)
	}
	waitForFileSearchDatabaseValue(t, ctx, "roots", root, true, fileSearchInitialIndexTimeout)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close File Search settings after adding root: %v", err)
	}
	return rowIndex
}

func fileSearchSettingTableRowCount(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string) int {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, fieldID+"-add")
		return found && add.Enabled
	})
	if err != nil {
		t.Fatalf("wait for File Search settings table %s: %v", fieldID, err)
	}
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, fieldID+"-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
			count++
		}
	}
	return count
}

func waitForFileSearchSettingTableRow(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string, rowIndex int) {
	t.Helper()
	editID := fmt.Sprintf("%s-row-%d-edit", fieldID, rowIndex)
	deleteID := fmt.Sprintf("%s-row-%d-delete", fieldID, rowIndex)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		edit, editFound := automationdriver.Find(snapshot, editID)
		remove, deleteFound := automationdriver.Find(snapshot, deleteID)
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return editFound && edit.Enabled && deleteFound && remove.Enabled && !editorFound
	}); err != nil {
		t.Fatalf("wait for File Search settings table %s row %d: %v", fieldID, rowIndex, err)
	}
}

// removeFileSearchRoot restores the plugin setting and waits until the index drops the root.
func removeFileSearchRoot(t *testing.T, client *automationdriver.Client, root string, rowIndex int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before removing File Search root: %v", err)
		return
	}
	smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
	waitForFileSearchSettingTableRow(t, ctx, client, fileSearchRootsFieldID, rowIndex)
	deleteID := fmt.Sprintf("%s-row-%d-delete", fileSearchRootsFieldID, rowIndex)
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("delete File Search root row %d: %v", rowIndex, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-delete-confirm")
		return found
	}); err != nil {
		t.Errorf("wait for File Search root delete confirmation: %v", err)
		return
	}
	if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("confirm File Search root deletion: %v", err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, deleteID)
		_, dialogFound := automationdriver.Find(snapshot, "form-table-delete-dialog")
		return !rowFound && !dialogFound
	}); err != nil {
		t.Errorf("wait for File Search root row removal: %v", err)
		return
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close File Search settings after cleanup: %v", err)
		return
	}
	waitForFileSearchDatabaseValue(t, ctx, "roots", root, false, fileSearchInitialIndexTimeout)
}

// waitForFileSearchDatabaseValue polls the live SQLite index for one exact path.
func waitForFileSearchDatabaseValue(t *testing.T, ctx context.Context, table, path string, present bool, timeout time.Duration) time.Duration {
	t.Helper()
	started := time.Now()
	deadline := started.Add(timeout)
	databasePath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "filesearch", "filesearch.db")
	var lastErr error
	ticker := time.NewTicker(fileSearchIndexPollInterval)
	defer ticker.Stop()
	for {
		exists, err := fileSearchDatabasePathExists(databasePath, table, path)
		if err == nil && exists == present {
			return time.Since(started)
		}
		if err != nil {
			lastErr = err
		}
		if time.Now().After(deadline) {
			t.Fatalf("File Search index table %s path %q present=%t after %s (last error: %v)", table, path, exists, time.Since(started), lastErr)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for File Search index table %s path %q present=%t: %v", table, path, present, ctx.Err())
		case <-ticker.C:
		}
	}
}

func fileSearchDatabasePathExists(databasePath, table, path string) (bool, error) {
	if table != "entries" && table != "roots" {
		return false, fmt.Errorf("unsupported File Search index table %q", table)
	}
	database, err := sql.Open("sqlite3", databasePath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return false, err
	}
	defer database.Close()
	if runtime.GOOS != "windows" {
		var count int
		err = database.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE path = ?", filepath.Clean(path)).Scan(&count)
		if err != nil {
			return false, err
		}
		return count > 0, nil
	}

	// SQLite NOCASE only handles ASCII, so compare Windows paths in Go to preserve Unicode case folding.
	rows, err := database.Query("SELECT path FROM " + table)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var storedPath string
		if err := rows.Scan(&storedPath); err != nil {
			return false, err
		}
		if fileSearchPathsEqual(storedPath, path) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func queryFileSearchResult(t *testing.T, ctx context.Context, client *automationdriver.Client, path string) woxwidget.AutomationSnapshot {
	t.Helper()
	query := "f " + filepath.Base(path)
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, query)
	if _, found := fileSearchResultByPath(snapshot, path); !found {
		t.Fatalf("File Search query %q did not expose %q", query, path)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	return snapshot
}

func queryFileSearchResultAbsent(t *testing.T, ctx context.Context, client *automationdriver.Client, path string) woxwidget.AutomationSnapshot {
	t.Helper()
	query := "f " + filepath.Base(path)
	smoke.ReplaceLauncherQuery(t, ctx, client, "")
	before, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read launcher before absent File Search query: %v", err)
	}
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("set absent File Search query %q: %v", query, err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		_, loadingFound := automationdriver.Find(snapshot, "launcher.query.loading")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		settledResults := !resultsFound || results.Value == "complete"
		return snapshot.Tree.Generation > before.Tree.Generation && inputFound && input.Value == query && !loadingFound && settledResults
	})
	if err != nil {
		t.Fatalf("wait for absent File Search query %q: %v", query, err)
	}
	if _, found := fileSearchResultByPath(snapshot, path); found {
		t.Fatalf("deleted File Search result %q remained visible", path)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	return snapshot
}

func fileSearchResultByPath(snapshot woxwidget.AutomationSnapshot, path string) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && fileSearchPathsEqual(node.Description, path) {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

// fileSearchPathsEqual follows the host platform's path case-sensitivity rules.
func fileSearchPathsEqual(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func removeFileSearchFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("remove File Search fixture %q: %v", path, err)
	}
}

// addFileSearchIgnorePattern adds one host-native absolute path through the ignore-rules table.
func addFileSearchIgnorePattern(t *testing.T, ctx context.Context, client *automationdriver.Client, pattern string) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
	rowIndex := fileSearchSettingTableRowCount(t, ctx, client, fileSearchIgnorePatternsFieldID)
	if err := client.Perform(ctx, fileSearchIgnorePatternsFieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add File Search ignore pattern row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for File Search ignore pattern editor: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionSetValue, pattern); err != nil {
		t.Fatalf("set File Search ignore pattern %q: %v", pattern, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, "form-table-row-field-0")
		return found && field.Value == pattern
	}); err != nil {
		t.Fatalf("confirm File Search ignore pattern field %q: %v", pattern, err)
	}
	// Saving can succeed even when its response or a later verification fails.
	// Register cleanup first so either outcome restores the shared settings.
	t.Cleanup(func() { removeFileSearchIgnorePattern(t, client, rowIndex) })
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save File Search ignore pattern %q: %v", pattern, err)
	}
	waitForFileSearchSettingTableRow(t, ctx, client, fileSearchIgnorePatternsFieldID, rowIndex)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close File Search settings after adding ignore pattern: %v", err)
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
	defer cancel()
	waitForFileSearchIgnorePatternPersisted(t, persistCtx, pattern)
}

// removeFileSearchIgnorePattern restores the ignore-rules table after a smoke case.
func removeFileSearchIgnorePattern(t *testing.T, client *automationdriver.Client, rowIndex int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before removing File Search ignore pattern: %v", err)
		return
	}
	smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
	// A failed save may never append the row; existing rows must stay untouched.
	if fileSearchSettingTableRowCount(t, ctx, client, fileSearchIgnorePatternsFieldID) <= rowIndex {
		if err := client.Hide(ctx); err != nil {
			t.Errorf("close File Search settings after unsaved ignore pattern: %v", err)
		}
		return
	}
	waitForFileSearchSettingTableRow(t, ctx, client, fileSearchIgnorePatternsFieldID, rowIndex)
	deleteID := fmt.Sprintf("%s-row-%d-delete", fileSearchIgnorePatternsFieldID, rowIndex)
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("delete File Search ignore pattern row %d: %v", rowIndex, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "form-table-delete-confirm")
		return found
	}); err != nil {
		t.Errorf("wait for File Search ignore pattern delete confirmation: %v", err)
		return
	}
	if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("confirm File Search ignore pattern deletion: %v", err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, deleteID)
		_, dialogFound := automationdriver.Find(snapshot, "form-table-delete-dialog")
		return !rowFound && !dialogFound
	}); err != nil {
		t.Errorf("wait for File Search ignore pattern row removal: %v", err)
		return
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close File Search settings after ignore pattern cleanup: %v", err)
	}
}

// fileSearchIgnorePatternsEqual compares user-entered ignore paths across separator and case differences.
func fileSearchIgnorePatternsEqual(left, right string) bool {
	left = path.Clean(strings.TrimSpace(strings.ReplaceAll(left, "\\", "/")))
	right = path.Clean(strings.TrimSpace(strings.ReplaceAll(right, "\\", "/")))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func fileSearchIgnorePatternsSettingKey() string {
	return "ignorePatterns@" + strings.ToLower(runtime.GOOS)
}

// waitForFileSearchIgnorePatternPersisted polls the platform-specific ignorePatterns JSON.
func waitForFileSearchIgnorePatternPersisted(t *testing.T, ctx context.Context, pattern string) {
	t.Helper()
	databasePath := filepath.Join(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment), "wox.db")
	key := fileSearchIgnorePatternsSettingKey()
	started := time.Now()
	deadline := started.Add(fileSearchInitialIndexTimeout)
	ticker := time.NewTicker(fileSearchIndexPollInterval)
	defer ticker.Stop()
	var lastValue string
	var lastErr error
	for {
		value, err := fileSearchPluginSettingValue(databasePath, key)
		if err == nil && fileSearchSettingContainsIgnorePattern(value, pattern) {
			return
		}
		lastValue = value
		lastErr = err
		if time.Now().After(deadline) {
			t.Fatalf("File Search ignore pattern %q not persisted in %s after %s (value=%q last error: %v)", pattern, key, time.Since(started), lastValue, lastErr)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for persisted File Search ignore pattern %q: %v", pattern, ctx.Err())
		case <-ticker.C:
		}
	}
}

func fileSearchPluginSettingValue(databasePath, key string) (string, error) {
	database, err := sql.Open("sqlite3", databasePath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return "", err
	}
	defer database.Close()
	var value string
	err = database.QueryRow("SELECT value FROM plugin_settings WHERE plugin_id = ? AND key = ?", fileSearchPluginID, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func fileSearchSettingContainsIgnorePattern(raw, pattern string) bool {
	var rows []struct {
		Pattern string `json:"Pattern"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return strings.Contains(raw, pattern) || strings.Contains(raw, filepath.FromSlash(pattern))
	}
	for _, row := range rows {
		if fileSearchIgnorePatternsEqual(row.Pattern, pattern) {
			return true
		}
	}
	return false
}
