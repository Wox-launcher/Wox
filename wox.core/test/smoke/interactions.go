package smoke

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// OpenInstalledPluginSettings opens one installed plugin through the shared settings route.
func OpenInstalledPluginSettings(t *testing.T, ctx context.Context, client *automationdriver.Client, pluginID string) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/plugins"); err != nil {
		t.Fatalf("open plugin settings: %v", err)
	}
	listID := "plugin-list-" + pluginID
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, listID)
		return found
	}); err != nil {
		t.Fatalf("wait for installed plugin %q: %v", pluginID, err)
	}
	if err := client.Perform(ctx, listID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select installed plugin %q: %v", pluginID, err)
	}
}

// SelectSettingChoiceByLabel chooses one shared dropdown option and waits for its committed value.
func SelectSettingChoiceByLabel(t *testing.T, ctx context.Context, client *automationdriver.Client, controlID, expected string) {
	t.Helper()
	if err := client.Perform(ctx, controlID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open setting choices for %q: %v", controlID, err)
	}
	var choiceID string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "setting-choice-") && node.Label == expected {
				choiceID = node.AutomationID
				return true
			}
		}
		return false
	}); err != nil {
		t.Fatalf("wait for setting choice %q: %v", expected, err)
	}
	if err := client.Perform(ctx, choiceID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select setting choice %q: %v", expected, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, controlID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		return found && control.Value == expected && !menuFound
	}); err != nil {
		t.Fatalf("wait for setting choice %q to commit: %v", expected, err)
	}
}

// SetLauncherQueryAndWaitComplete changes the query and waits for the matching result generation.
func SetLauncherQueryAndWaitComplete(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("set launcher query %q: %v", query, err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return inputFound && input.Value == query && resultsFound && results.Value == "complete"
	})
	if err != nil {
		t.Fatalf("wait for launcher query %q: %v", query, err)
	}
	return snapshot
}

// ReplaceLauncherQuery clears retained results before submitting a fresh query generation.
func ReplaceLauncherQuery(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read launcher query before replacing it: %v", err)
	}
	current, found := automationdriver.Find(snapshot, "launcher.query.input")
	if found && current.Value != "" {
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
			t.Fatalf("clear retained launcher query %q: %v", current.Value, err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			if !inputFound || input.Value != "" {
				return false
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") {
					return false
				}
			}
			return true
		})
		if err != nil {
			t.Fatalf("wait for retained launcher query %q to clear: %v", current.Value, err)
		}
	}
	if query == "" {
		return snapshot
	}
	return SetLauncherQueryAndWaitComplete(t, ctx, client, query)
}

// WaitForFile polls one real artifact independently of UI generation changes.
func WaitForFile(t *testing.T, ctx context.Context, path string, matches func([]byte) bool) []byte {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil && matches(data) {
			return data
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read artifact %q: %v", path, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for artifact %q: %v", path, ctx.Err())
		case <-ticker.C:
		}
	}
}

// AssertNoDiagnostics fails when the semantics tree reports accessibility defects.
func AssertNoDiagnostics(t *testing.T, snapshot woxwidget.AutomationSnapshot) {
	t.Helper()
	if len(snapshot.Diagnostics) > 0 {
		t.Fatalf("semantics diagnostics: %v", snapshot.Diagnostics)
	}
}
