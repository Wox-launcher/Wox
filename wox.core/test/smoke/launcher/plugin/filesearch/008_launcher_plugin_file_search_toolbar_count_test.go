//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test008LauncherPluginFileSearchToolbarCount verifies the indexed file count belongs to the active File Search query.
// Flow: index a local file -> search for it -> inspect the count -> leave File Search.
// Evidence: the file result and localized indexed count are visible, and the count clears after leaving the plugin.
func Test008LauncherPluginFileSearchToolbarCount(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := newFileSearchRoot(t)
		file := filepath.Join(root, "toolbar-count-fixture.txt")
		writeFileSearchFixture(t, file)
		row := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, row) })
		waitForFileSearchResultPath(t, ctx, client, file, true, fileSearchInitialIndexTimeout)
		smoke.ShowLauncher(t, ctx, client)
		queryFileSearchResult(t, ctx, client, file)
		format := smoke.LocalizedText(t, ctx, client, "plugin_file_status_indexed_files")
		prefix, suffix, ok := strings.Cut(format, "%s")
		if !ok {
			t.Fatalf("indexed count format %q has no count placeholder", format)
		}
		isCount := func(value string) bool {
			count, ok := strings.CutPrefix(value, prefix)
			if !ok {
				return false
			}
			count, ok = strings.CutSuffix(count, suffix)
			return ok && strings.IndexFunc(count, unicode.IsDigit) >= 0
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return found && isCount(status.Value)
		})
		if err != nil {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			t.Fatalf("wait for indexed count: found=%t value=%q: %v", found, status.Value, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			status, found := automationdriver.Find(snapshot, "launcher.toolbar.status")
			return !found || !isCount(status.Value)
		}); err != nil {
			t.Fatalf("file count survived leaving File Search: %v", err)
		}
	})
}
