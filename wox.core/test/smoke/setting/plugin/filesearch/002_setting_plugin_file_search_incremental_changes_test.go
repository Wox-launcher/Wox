//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test002SettingPluginFileSearchIncrementalChanges verifies watched file additions and deletions reach File Search quickly.
// Flow: index a configured directory -> create and query a file -> delete it -> repeat the same Launcher query.
// Evidence: the live index and Launcher add then remove the exact path within the incremental latency budget.
func Test002SettingPluginFileSearchIncrementalChanges(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := newFileSearchRoot(t)
		baselinePath := filepath.Join(root, fmt.Sprintf("filesearch-baseline-%d.txt", time.Now().UnixNano()))
		writeFileSearchFixture(t, baselinePath)

		rowIndex := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, rowIndex) })
		waitForFileSearchDatabaseValue(t, ctx, "entries", baselinePath, true, fileSearchInitialIndexTimeout)

		smoke.ShowLauncher(t, ctx, client)
		queryFileSearchResult(t, ctx, client, baselinePath)

		changedPath := filepath.Join(root, fmt.Sprintf("filesearch-incremental-%d.txt", time.Now().UnixNano()))
		writeFileSearchFixture(t, changedPath)
		addedLatency := waitForFileSearchDatabaseValue(t, ctx, "entries", changedPath, true, fileSearchIncrementalTimeout)
		t.Logf("File Search indexed new file in %s", addedLatency)
		queryFileSearchResult(t, ctx, client, changedPath)

		removeFileSearchFixture(t, changedPath)
		deletedLatency := waitForFileSearchDatabaseValue(t, ctx, "entries", changedPath, false, fileSearchIncrementalTimeout)
		t.Logf("File Search removed deleted file in %s", deletedLatency)
		queryFileSearchResultAbsent(t, ctx, client, changedPath)
	})
}
