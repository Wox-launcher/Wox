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

// Test001LauncherFolderBrowse verifies that an exact folder result can enter one-level browsing.
// Flow: query an exact temporary folder -> invoke Enter folder -> wait for its immediate children.
// Evidence: the query gains a trailing separator and exposes the folder and two files while omitting the hidden file.
func Test001LauncherFolderBrowse(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := t.TempDir()
		mustCreateFolder(t, filepath.Join(root, "child-folder"))
		mustWriteFolderFile(t, filepath.Join(root, "alpha.txt"))
		mustWriteFolderFile(t, filepath.Join(root, "zeta.txt"))
		mustWriteFolderFile(t, filepath.Join(root, ".hidden.txt"))

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, root)
		result, found := folderResultByPath(snapshot, root)
		if !found {
			t.Fatalf("exact folder result for %q was not exposed", root)
		}
		if !result.Selected {
			smoke.SelectLauncherResult(t, ctx, client, result.AutomationID)
		}

		smoke.ActivateSelectedResultAction(t, ctx, client, "action-result-enter_folder-")
		expectedQuery := root + string(os.PathSeparator)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			return inputFound && input.Value == expectedQuery && resultsFound && results.Value == "complete" &&
				len(folderChildren(snapshot, root)) == 3
		})
		if err != nil {
			t.Fatalf("wait for folder children: %v", err)
		}

		for _, path := range []string{
			filepath.Join(root, "child-folder"),
			filepath.Join(root, "alpha.txt"),
			filepath.Join(root, "zeta.txt"),
		} {
			if _, found := folderResultByPath(snapshot, path); !found {
				t.Fatalf("folder child result for %q was not exposed", path)
			}
		}
		if _, found := folderResultByPath(snapshot, filepath.Join(root, ".hidden.txt")); found {
			t.Fatal("hidden file was visible before enabling hidden files")
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func mustCreateFolder(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("create folder fixture %q: %v", path, err)
	}
}

func mustWriteFolderFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("folder smoke"), 0o644); err != nil {
		t.Fatalf("write folder fixture %q: %v", path, err)
	}
}
