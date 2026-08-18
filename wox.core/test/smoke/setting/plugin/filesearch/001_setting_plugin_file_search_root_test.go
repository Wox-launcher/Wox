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

// Test001SettingPluginFileSearchRoot verifies a directory added in File Search settings becomes searchable.
// Flow: create a unique file -> add its directory through plugin settings -> wait for the live index -> query Launcher.
// Evidence: the completed File Search result identifies the exact configured file path.
func Test001SettingPluginFileSearchRoot(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := newFileSearchRoot(t)
		filePath := filepath.Join(root, fmt.Sprintf("filesearch-root-%d.txt", time.Now().UnixNano()))
		writeFileSearchFixture(t, filePath)

		rowIndex := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, rowIndex) })
		waitForFileSearchDatabaseValue(t, ctx, "entries", filePath, true, fileSearchInitialIndexTimeout)

		smoke.ShowLauncher(t, ctx, client)
		queryFileSearchResult(t, ctx, client, filePath)
	})
}
