//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"wox/i18n"
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
		var patterns [][]string
		for _, lang := range i18n.GetSupportedLanguages() {
			values, err := i18n.GetI18nManager().GetLangMap(ctx, lang.Code)
			if err != nil {
				t.Fatal(err)
			}
			if parts := strings.Split(values["plugin_file_status_indexed_files"], "%s"); len(parts) == 2 {
				patterns = append(patterns, parts)
			}
		}
		isCount := func(value string) bool {
			for _, parts := range patterns {
				if strings.HasPrefix(value, parts[0]) && strings.HasSuffix(value, parts[1]) {
					count := strings.TrimSuffix(strings.TrimPrefix(value, parts[0]), parts[1])
					return strings.IndexFunc(count, unicode.IsDigit) >= 0
				}
			}
			return false
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
