//go:build wox_ui_smoke

package folder

import (
	"context"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test003LauncherFolderFavorite verifies the result action form persists a named folder favorite for global lookup.
// Flow: query a folder -> add it as a renamed favorite -> query the favorite name -> delete it from its result actions.
// Evidence: the name resolves to the saved path before deletion and no longer resolves after cleanup.
func Test003LauncherFolderFavorite(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := t.TempDir()
		favoriteName := "wox-folder-smoke-" + filepath.Base(root)

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, root)
		result, found := folderResultByPath(snapshot, root)
		if !found {
			t.Fatalf("exact folder result for %q was not exposed", root)
		}
		if !result.Selected {
			smoke.SelectLauncherResult(t, ctx, client, result.AutomationID)
		}
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-add_folder_favorite-")

		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			name, nameFound := automationdriver.Find(snapshot, "action-form-field-0")
			path, pathFound := automationdriver.Find(snapshot, "action-form-field-1")
			_, saveFound := automationdriver.Find(snapshot, "form-save")
			return nameFound && pathFound && saveFound && name.Value != "" && filepath.Clean(path.Value) == filepath.Clean(root)
		}); err != nil {
			t.Fatalf("wait for add-folder-favorite form: %v", err)
		}
		if err := client.Perform(ctx, "action-form-field-0", woxui.AccessibilityActionSetValue, favoriteName); err != nil {
			t.Fatalf("set folder favorite name: %v", err)
		}
		if err := client.Perform(ctx, "form-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save folder favorite: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, formOpen := automationdriver.Find(snapshot, "form-save")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return !formOpen && resultsFound && results.Value == "complete"
		}); err != nil {
			t.Fatalf("wait for folder favorite save: %v", err)
		}

		snapshot = smoke.ReplaceLauncherQuery(t, ctx, client, favoriteName)
		favorite, found := folderResultByPath(snapshot, root)
		if !found || favorite.Label != favoriteName {
			t.Fatalf("folder favorite result = %+v, found=%v", favorite, found)
		}
		if !favorite.Selected {
			smoke.SelectLauncherResult(t, ctx, client, favorite.AutomationID)
		}
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-delete_folder_favorite-")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, favoriteFound := folderResultByPath(snapshot, root)
			_, panelOpen := automationdriver.Find(snapshot, "action-search")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return !favoriteFound && !panelOpen && (!resultsFound || results.Value == "complete")
		})
		if err != nil {
			t.Fatalf("wait for folder favorite deletion: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
