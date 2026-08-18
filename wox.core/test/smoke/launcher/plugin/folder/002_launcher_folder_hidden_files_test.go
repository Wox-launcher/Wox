//go:build wox_ui_smoke

package folder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test002LauncherFolderHiddenFiles verifies that the folder action changes hidden-child visibility in place.
// Flow: browse a folder with visible and hidden children -> show hidden files -> hide them again.
// Evidence: the hidden child appears after the first refresh and disappears after the cleanup refresh.
func Test002LauncherFolderHiddenFiles(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := t.TempDir()
		visiblePath := filepath.Join(root, "visible-folder")
		hiddenPath := filepath.Join(root, ".hidden-folder")
		mustCreateFolder(t, visiblePath)
		mustCreateFolder(t, hiddenPath)

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, root+string(os.PathSeparator))
		if _, found := folderResultByPath(snapshot, hiddenPath); found {
			t.Fatal("hidden folder was visible before enabling hidden files")
		}
		visible, found := folderResultByPath(snapshot, visiblePath)
		if !found {
			t.Fatalf("visible folder result for %q was not exposed", visiblePath)
		}
		if !visible.Selected {
			smoke.SelectLauncherResult(t, ctx, client, visible.AutomationID)
		}

		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-toggle_hidden_files-")
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, hiddenFound := folderResultByPath(snapshot, hiddenPath)
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return hiddenFound && resultsFound && results.Value == "complete"
		})
		if err != nil {
			t.Fatalf("wait for hidden folder to appear: %v", err)
		}

		visible, found = folderResultByPath(snapshot, visiblePath)
		if !found {
			t.Fatalf("visible folder disappeared after showing hidden files: %q", visiblePath)
		}
		if !visible.Selected {
			smoke.SelectLauncherResult(t, ctx, client, visible.AutomationID)
		}
		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-toggle_hidden_files-")
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, hiddenFound := folderResultByPath(snapshot, hiddenPath)
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return !hiddenFound && resultsFound && results.Value == "complete"
		})
		if err != nil {
			t.Fatalf("wait for hidden folder cleanup: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
