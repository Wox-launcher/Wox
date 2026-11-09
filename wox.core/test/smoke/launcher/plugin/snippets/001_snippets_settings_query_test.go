//go:build wox_ui_smoke

package snippets

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001SnippetsSettingsQuery verifies snippets are managed in settings and appear in launcher searches.
// Flow: create two snippets in plugin settings -> reopen settings -> search with snip -> delete both snippets.
// Evidence: the keyword retains its trailing space, both configured results appear, and deletion removes them.
func Test001SnippetsSettingsQuery(t *testing.T) {
	const title = "Snippets smoke address"
	const addressKeyword = ";addr "
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openSnippetSettings(t, ctx, client)
		if snapshot, err := client.Snapshot(ctx); err != nil {
			t.Fatal(err)
		} else if _, found := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-edit"); found {
			t.Fatal("snippet smoke requires an empty isolated table")
		}
		if err := client.Perform(ctx, "plugin-settings-field-2-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add snippet: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, a := automationdriver.Find(snapshot, "form-table-row-field-0")
			_, b := automationdriver.Find(snapshot, "form-table-row-field-1")
			_, c := automationdriver.Find(snapshot, "form-table-row-field-2")
			_, save := automationdriver.Find(snapshot, "form-table-row-save")
			return a && b && c && save
		}); err != nil {
			t.Fatalf("wait for snippet editor: %v", err)
		}
		for id, value := range map[string]string{"form-table-row-field-0": title, "form-table-row-field-1": addressKeyword, "form-table-row-field-2": "Example address"} {
			if err := client.Perform(ctx, id, woxui.AccessibilityActionSetValue, value); err != nil {
				t.Fatalf("set %s: %v", id, err)
			}
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save snippet: %v", err)
		}
		waitSnippetRow(t, ctx, client, true)
		created := true
		t.Cleanup(func() {
			if !created {
				return
			}
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := client.Hide(cleanupCtx); err != nil {
				t.Errorf("hide before snippet cleanup: %v", err)
				return
			}
			smoke.OpenInstalledPluginSettings(t, cleanupCtx, client, common.SnippetsPluginID)
			if _, err := client.WaitFor(cleanupCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
				_, ok := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-delete")
				return ok
			}); err != nil {
				t.Errorf("wait for snippet cleanup: %v", err)
				return
			}
			for {
				snapshot, err := client.Snapshot(cleanupCtx)
				if err != nil {
					t.Errorf("inspect snippet cleanup: %v", err)
					return
				}
				if _, found := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-delete"); !found {
					break
				}
				before := 0
				for _, node := range snapshot.Tree.Nodes {
					if strings.HasPrefix(node.AutomationID, "plugin-settings-field-2-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
						before++
					}
				}
				for range 2 {
					if err := client.Perform(cleanupCtx, "plugin-settings-field-2-row-0-delete", woxui.AccessibilityActionActivate, ""); err != nil {
						t.Errorf("delete snippet: %v", err)
						return
					}
				}
				if _, err := client.WaitFor(cleanupCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
					after := 0
					for _, node := range snapshot.Tree.Nodes {
						if strings.HasPrefix(node.AutomationID, "plugin-settings-field-2-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
							after++
						}
					}
					return after < before
				}); err != nil {
					t.Errorf("wait for snippet cleanup: %v", err)
					return
				}
			}
		})
		// Start another row as soon as the first row is visible, before relying on a catalog refresh.
		if err := client.Perform(ctx, "plugin-settings-field-2-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add second snippet: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(snapshot, "form-table-row-save")
			return ok
		}); err != nil {
			t.Fatalf("wait for second snippet editor: %v", err)
		}
		for id, value := range map[string]string{"form-table-row-field-0": "Snippets smoke phone", "form-table-row-field-1": ";phone", "form-table-row-field-2": "Example phone"} {
			if err := client.Perform(ctx, id, woxui.AccessibilityActionSetValue, value); err != nil {
				t.Fatalf("set second snippet %s: %v", id, err)
			}
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save second snippet: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			row, found := automationdriver.Find(snapshot, "plugin-settings-field-2-row-1-edit")
			return found && row.Enabled
		}); err != nil {
			t.Fatalf("wait for second snippet: %v", err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		openSnippetSettings(t, ctx, client)
		waitSnippetRow(t, ctx, client, true)
		addressRow := -1
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			for index := range 2 {
				name, hasName := automationdriver.Find(snapshot, fmt.Sprintf("plugin-settings-field-2-row-%d-cell-0", index))
				keyword, hasKeyword := automationdriver.Find(snapshot, fmt.Sprintf("plugin-settings-field-2-row-%d-cell-1", index))
				if hasName && hasKeyword && name.Value == title && keyword.Value == addressKeyword {
					addressRow = index
					return true
				}
			}
			return false
		}); err != nil {
			t.Fatalf("verify snippet table preserves keyword whitespace: %v", err)
		}
		if err := client.Perform(ctx, fmt.Sprintf("plugin-settings-field-2-row-%d-edit", addressRow), woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			field, found := automationdriver.Find(snapshot, "form-table-row-field-1")
			return found && field.Value == addressKeyword
		}); err != nil {
			t.Fatalf("verify trailing keyword space: %v", err)
		}
		if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatal(err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "snip ")
		if !smoke.HasLauncherResultLabel(snapshot, title) {
			t.Fatalf("configured snippet not found in results")
		}
		if !smoke.HasLauncherResultLabel(snapshot, "Snippets smoke phone") {
			t.Fatal("second configured snippet not found in results")
		}
		if smoke.HasLauncherResultLabel(snapshot, "New snippet") || smoke.HasLauncherResultLabel(snapshot, "Enable automatic expansion") {
			t.Fatal("settings actions appeared in query results")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		openSnippetSettings(t, ctx, client)
		waitSnippetRow(t, ctx, client, true)
		for range 2 {
			if err := client.Perform(ctx, "plugin-settings-field-2-row-1-delete", woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("delete second snippet: %v", err)
			}
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "plugin-settings-field-2-row-1-delete")
			return !found
		}); err != nil {
			t.Fatalf("wait for second deletion: %v", err)
		}
		for range 2 {
			if err := client.Perform(ctx, "plugin-settings-field-2-row-0-delete", woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("delete snippet: %v", err)
			}
		}
		waitSnippetRow(t, ctx, client, false)
		created = false
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		snapshot = smoke.ReplaceLauncherQuery(t, ctx, client, "snip "+title)
		if smoke.HasLauncherResultLabel(snapshot, title) {
			t.Fatal("deleted snippet still appears in results")
		}
	})
}

func openSnippetSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, common.SnippetsPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, "plugin-settings-field-2-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for snippet settings: %v", err)
	}
}

func waitSnippetRow(t *testing.T, ctx context.Context, client *automationdriver.Client, want bool) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		row, found := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-edit")
		_, editor := automationdriver.Find(snapshot, "form-table-row-save")
		return !editor && found == want && (!found || row.Enabled)
	}); err != nil {
		t.Fatalf("wait for snippet row present=%v: %v", want, err)
	}
}
