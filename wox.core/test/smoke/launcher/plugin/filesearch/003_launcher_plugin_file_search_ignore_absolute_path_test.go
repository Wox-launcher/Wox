//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test003LauncherPluginFileSearchIgnoreAbsolutePath verifies an absolute folder ignore rule drops already-indexed files.
// Flow: index a search root with kept and ignored files -> add the ignored folder's host-native absolute path -> query Launcher.
// Evidence: the live index and completed File Search results omit the ignored path while the sibling file stays searchable.
func Test003LauncherPluginFileSearchIgnoreAbsolutePath(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		stamp := time.Now().UnixNano()
		root := fileSearchNativeAbsolutePath(t, newFileSearchRoot(t))
		ignoredDir := fileSearchNativeAbsolutePath(t, filepath.Join(root, fmt.Sprintf("filesearch-ignored-dir-%d", stamp)))
		keptDir := fileSearchNativeAbsolutePath(t, filepath.Join(root, fmt.Sprintf("filesearch-kept-dir-%d", stamp)))
		mkdirFileSearchDir(t, filepath.Join(ignoredDir, "nested"))
		mkdirFileSearchDir(t, keptDir)

		ignoredPath := filepath.Join(ignoredDir, "nested", fmt.Sprintf("filesearch-ignored-%d.txt", stamp))
		keptPath := filepath.Join(keptDir, fmt.Sprintf("filesearch-kept-%d.txt", stamp))
		writeFileSearchFixture(t, ignoredPath)
		writeFileSearchFixture(t, keptPath)
		// Keep native separators so Windows exercises literal backslashes through the text control and persistence.
		ignorePattern := ignoredDir
		t.Logf("File Search ignore pattern %q on %s", ignorePattern, runtime.GOOS)

		rootRow := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, rootRow) })
		waitForFileSearchDatabaseValue(t, ctx, "entries", ignoredPath, true, fileSearchInitialIndexTimeout)
		waitForFileSearchDatabaseValue(t, ctx, "entries", keptPath, true, fileSearchInitialIndexTimeout)

		addFileSearchIgnorePattern(t, ctx, client, ignorePattern)
		indexCtx, indexCancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
		defer indexCancel()
		droppedLatency := waitForFileSearchDatabaseValue(t, indexCtx, "entries", ignoredPath, false, fileSearchInitialIndexTimeout)
		t.Logf("File Search dropped ignored path %q after %s", ignoredPath, droppedLatency)
		waitForFileSearchDatabaseValue(t, indexCtx, "entries", keptPath, true, fileSearchIncrementalTimeout)

		smoke.ShowLauncher(t, ctx, client)
		queryFileSearchResultAbsent(t, ctx, client, ignoredPath)
		queryFileSearchResult(t, ctx, client, keptPath)
	})
}
