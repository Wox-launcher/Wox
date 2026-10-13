//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
	"unicode/utf8"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokereport"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test005LauncherPluginFileSearchContentPerformance verifies content-only words
// are searchable from a separately configured content directory.
// Flow: create 1,000 text files -> enable content search and add its root -> search.
// Evidence: completed content queries identify exact files; costs are reported only.
func Test005LauncherPluginFileSearchContentPerformance(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		root := newFileSearchRoot(t)
		paths := make([]string, 1000)
		for i := range paths {
			paths[i] = filepath.Join(root, fmt.Sprintf("document-%04d.txt", i))
			if err := os.WriteFile(paths[i], []byte(fmt.Sprintf("Fixed Wox content fixture. woxcontentsentinel%04d appears inside this document only.\n", i)), 0600); err != nil {
				t.Fatal(err)
			}
		}
		report := smokereport.Report{Title: "File Search content reference", Scope: "Shared native Wox; 1,000 temporary text files; content root configured independently. Query timings include automation and UI presentation; no forced GC."}
		defer func() { t.Log(report.Line()) }()
		reportFileSearchMemory(ctx, client, &report, "before content indexing")
		smoke.OpenInstalledPluginSettings(t, ctx, client, fileSearchPluginID)
		fieldID := fileSearchSettingFieldID(6)
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, fieldID)
			return found
		})
		if err != nil {
			t.Fatal(err)
		}
		control, _ := automationdriver.Find(snapshot, fieldID)
		original := control.Checked
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), fileSearchInitialIndexTimeout)
			defer cancel()
			smoke.OpenInstalledPluginSettings(t, cleanupCtx, client, fileSearchPluginID)
			setFileSearchContentEnabled(t, cleanupCtx, client, fieldID, original)
			if err := client.Hide(cleanupCtx); err != nil {
				t.Error(err)
			}
		})
		started := time.Now()
		setFileSearchContentEnabled(t, ctx, client, fieldID, true)
		rootField := fileSearchSettingFieldID(7)
		row := addFileSearchRootField(t, ctx, client, root, rootField)
		t.Cleanup(func() { removeFileSearchRootField(t, client, root, row, rootField) })
		waitForFileSearchContentReady(t, ctx, root, len(paths))
		report.Add("settings action to completed content index", float64(time.Since(started))/float64(time.Millisecond), "ms")
		reportFileSearchMemory(ctx, client, &report, "after content indexing")
		reportFileSearchContentDisk(&report)
		smoke.ShowLauncher(t, ctx, client)
		query := func(index int) {
			snapshot := queryFileSearchSnapshot(t, ctx, client, fmt.Sprintf("f woxcontentsentinel%04d", index))
			if _, found := fileSearchResultByPath(snapshot, paths[index]); !found {
				t.Fatalf("content-only query omitted %q", paths[index])
			}
			smoke.AssertNoDiagnostics(t, snapshot)
		}
		for i := range 3 {
			query(i)
		}
		samples := make([]float64, 20)
		for i := range samples {
			started := time.Now()
			query(23 + i*37)
			samples[i] = float64(time.Since(started)) / float64(time.Millisecond)
		}
		sort.Float64s(samples)
		report.Add("completed content query p50", samples[9], "ms")
		report.Add("completed content query p95", samples[18], "ms")
		reportFileSearchMemory(ctx, client, &report, "after content queries")
	})
}

// setFileSearchContentEnabled uses the plugin form's checkbox and waits for its
// persisted value so a delayed save cannot race root edits or cleanup.
func setFileSearchContentEnabled(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string, enabled bool) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	control, found := automationdriver.Find(snapshot, fieldID)
	if !found {
		t.Fatalf("content checkbox %s missing", fieldID)
	}
	if control.Checked != enabled {
		if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, fieldID)
		return found && control.Checked == enabled
	}); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment), "wox.db")
	ticker := time.NewTicker(fileSearchIndexPollInterval)
	defer ticker.Stop()
	for {
		value, err := fileSearchPluginSettingValue(databasePath, "contentSearchEnabled")
		if err == nil && value == fmt.Sprint(enabled) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("content setting was not persisted: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// waitForFileSearchContentReady observes the content database's own crawl state
// and exact fixture count; it has no dependency on filename storage internals.
func waitForFileSearchContentReady(t *testing.T, ctx context.Context, root string, count int) {
	t.Helper()
	databasePath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "filesearch", "contentsearch.db")
	database, err := sql.Open("sqlite3", databasePath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ticker := time.NewTicker(fileSearchIndexPollInterval)
	defer ticker.Stop()
	prefix := root + string(filepath.Separator)
	for {
		var state string
		var indexed int
		err := database.QueryRowContext(ctx, "SELECT value FROM meta WHERE key = 'content_crawl_state'").Scan(&state)
		if err == nil && state == "complete" {
			err = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM content_entries WHERE substr(path,1,?) = ? COLLATE NOCASE", utf8.RuneCountInString(prefix), prefix).Scan(&indexed)
			if err == nil && ((count < 0 && indexed > 0) || indexed == count) {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("content index not ready: state=%q count=%d error=%v: %v", state, indexed, err, ctx.Err())
		case <-ticker.C:
		}
	}
}

// reportFileSearchContentDisk includes the database and any live WAL pages.
func reportFileSearchContentDisk(report *smokereport.Report) {
	var diskBytes int64
	var diskErr error
	for _, suffix := range []string{"", "-wal", "-shm"} {
		path := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "filesearch", "contentsearch.db"+suffix)
		if info, err := os.Stat(path); err == nil {
			diskBytes += info.Size()
		} else if !os.IsNotExist(err) {
			diskErr = err
		}
	}
	if diskErr != nil {
		report.Missing("content index disk", diskErr)
	} else {
		report.Add("content index disk", float64(diskBytes), "bytes")
	}
}
