//go:build wox_ui_smoke

package wpm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	wpmPluginID                = "e2c5f005-6c73-43c8-bc53-ab04def265b2"
	wpmLocalDirectoriesFieldID = "plugin-settings-field-1"
	wpmLocalDirectoriesAddID   = wpmLocalDirectoriesFieldID + "-add"
	wpmLocalDirectoryPathField = "form-table-row-field-0"
	// A trailing space is required so Wox treats "dev.list" as a command, not search.
	wpmDevListQuery = "wpm dev.list "
)

type wpmDevPluginFixture struct {
	Directory string
	Name      string
}

func newWpmDevPluginFixture(t *testing.T) wpmDevPluginFixture {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory for WPM smoke plugin: %v", err)
	}
	baseDirectory := filepath.Join(workingDirectory, ".tmp-wpm-dev")
	if err := os.MkdirAll(baseDirectory, 0o755); err != nil {
		t.Fatalf("create WPM smoke plugin directory: %v", err)
	}
	directory, err := os.MkdirTemp(baseDirectory, "wpmdev-")
	if err != nil {
		t.Fatalf("create WPM smoke plugin: %v", err)
	}
	name := "WPM Dev Smoke " + filepath.Base(directory)
	manifest, err := json.Marshal(map[string]any{
		"Id":              "wox.smoke.wpm.dev." + filepath.Base(directory),
		"Name":            name,
		"Author":          "Wox Smoke",
		"Version":         "1.0.0",
		"MinWoxVersion":   "2.0.0",
		"Runtime":         "Go",
		"Description":     "Local plugin used by the WPM dev smoke case",
		"TriggerKeywords": []string{"wpmsmk"},
		"SupportedOS":     []string{"Windows", "Darwin", "Linux"},
	})
	if err != nil {
		t.Fatalf("encode WPM smoke plugin.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), append(manifest, '\n'), 0o644); err != nil {
		t.Fatalf("write WPM smoke plugin.json: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("remove WPM smoke plugin %q: %v", directory, err)
		}
		if err := os.Remove(baseDirectory); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Errorf("remove WPM smoke plugin directory %q: %v", baseDirectory, err)
		}
	})
	return wpmDevPluginFixture{Directory: filepath.Clean(directory), Name: name}
}

func openWpmSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, wpmPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, wpmLocalDirectoriesAddID)
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for WPM local plugin directories table: %v", err)
	}
}

func wpmLocalDirectoryCellID(rowIndex int) string {
	return fmt.Sprintf("%s-row-%d-cell-0", wpmLocalDirectoriesFieldID, rowIndex)
}

func wpmLocalDirectoryDeleteID(rowIndex int) string {
	return fmt.Sprintf("%s-row-%d-delete", wpmLocalDirectoriesFieldID, rowIndex)
}

func wpmLocalDirectoryRowCount(t *testing.T, ctx context.Context, client *automationdriver.Client) int {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, wpmLocalDirectoriesAddID)
		return found && add.Enabled
	})
	if err != nil {
		t.Fatalf("wait for WPM local plugin directories rows: %v", err)
	}
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, wpmLocalDirectoriesFieldID+"-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
			count++
		}
	}
	return count
}

func wpmPathsEqual(left, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func wpmNodeHasPath(node woxui.AccessibilityNode, path string) bool {
	return wpmPathsEqual(node.Value, path) || wpmPathsEqual(node.Label, path)
}

func waitForWpmLocalDirectoryCell(t *testing.T, ctx context.Context, client *automationdriver.Client, rowIndex int, path string) {
	t.Helper()
	cellID := wpmLocalDirectoryCellID(rowIndex)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		cell, found := automationdriver.Find(snapshot, cellID)
		add, addFound := automationdriver.Find(snapshot, wpmLocalDirectoriesAddID)
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return found && wpmNodeHasPath(cell, path) && addFound && add.Enabled && !editorFound
	}); err != nil {
		t.Fatalf("wait for WPM local plugin directory %q in row %d: %v", path, rowIndex, err)
	}
}

func addWpmLocalDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, directory string) int {
	t.Helper()
	openWpmSettings(t, ctx, client)
	rowIndex := wpmLocalDirectoryRowCount(t, ctx, client)
	if err := client.Perform(ctx, wpmLocalDirectoriesAddID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add WPM local plugin directory row: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, wpmLocalDirectoryPathField)
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for WPM local plugin directory editor: %v", err)
	}
	if err := client.Perform(ctx, wpmLocalDirectoryPathField, woxui.AccessibilityActionSetValue, directory); err != nil {
		t.Fatalf("set WPM local plugin directory %q: %v", directory, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, wpmLocalDirectoryPathField)
		return found && wpmNodeHasPath(field, directory)
	}); err != nil {
		t.Fatalf("confirm WPM local plugin directory field %q: %v", directory, err)
	}
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save WPM local plugin directory %q: %v", directory, err)
	}
	waitForWpmLocalDirectoryCell(t, ctx, client, rowIndex, directory)
	return rowIndex
}

func findWpmLocalDirectoryRow(t *testing.T, ctx context.Context, client *automationdriver.Client, directory string) (int, bool) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read WPM local plugin directories: %v", err)
	}
	for index := 0; ; index++ {
		cell, found := automationdriver.Find(snapshot, wpmLocalDirectoryCellID(index))
		if !found {
			return 0, false
		}
		if wpmNodeHasPath(cell, directory) {
			return index, true
		}
	}
}

func confirmWpmLocalDirectoryPersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, directory string, rowIndex int) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close WPM settings after save: %v", err)
	}
	openWpmSettings(t, ctx, client)
	waitForWpmLocalDirectoryCell(t, ctx, client, rowIndex, directory)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close WPM settings after inspecting local plugin directory: %v", err)
	}
}

func removeWpmLocalDirectory(t *testing.T, client *automationdriver.Client, directory string, rowIndex int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before removing WPM local plugin directory: %v", err)
		return
	}
	openWpmSettings(t, ctx, client)
	if foundIndex, found := findWpmLocalDirectoryRow(t, ctx, client, directory); found {
		rowIndex = foundIndex
	} else if wpmLocalDirectoryRowCount(t, ctx, client) <= rowIndex {
		if err := client.Hide(ctx); err != nil {
			t.Errorf("close WPM settings after missing local plugin directory: %v", err)
		}
		return
	}
	deleteID := wpmLocalDirectoryDeleteID(rowIndex)
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("delete WPM local plugin directory row %d: %v", rowIndex, err)
		return
	}
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("confirm WPM local plugin directory deletion: %v", err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, deleteID)
		add, addFound := automationdriver.Find(snapshot, wpmLocalDirectoriesAddID)
		return !rowFound && addFound && add.Enabled
	}); err != nil {
		t.Errorf("wait for WPM local plugin directory row removal: %v", err)
		return
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close WPM settings after removing local plugin directory: %v", err)
	}
}

func wpmLauncherResultLabels(snapshot woxwidget.AutomationSnapshot) []string {
	labels := make([]string, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			labels = append(labels, node.Label)
		}
	}
	return labels
}

func waitForWpmDevListPlugin(t *testing.T, ctx context.Context, client *automationdriver.Client, name string, present bool) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	deadline := time.Now().Add(automationdriver.ActionTimeout)
	var last woxwidget.AutomationSnapshot
	for {
		last = smoke.ReplaceLauncherQuery(t, ctx, client, wpmDevListQuery)
		_, found := smoke.FindLauncherResult(last, name)
		if found == present {
			smoke.AssertNoDiagnostics(t, last)
			return
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			t.Fatalf("WPM dev.list plugin %q present=%t (results=%v)", name, present, wpmLauncherResultLabels(last))
		}
	}
}
