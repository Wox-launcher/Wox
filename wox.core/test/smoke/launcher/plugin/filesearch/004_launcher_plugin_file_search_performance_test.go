//go:build wox_ui_smoke

package filesearch

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokereport"
)

// Test004LauncherPluginFileSearchPerformance verifies a 10,000-file root becomes searchable.
// Flow: create fixtures -> configure the root -> wait for indexing -> query exact files.
// Evidence: completed Launcher results identify the fixtures; costs are reported only.
func Test004LauncherPluginFileSearchPerformance(t *testing.T) {
	count, timeout := 10000, smoke.CaseTimeout
	if raw := os.Getenv("WOX_FILESEARCH_SMOKE_FILES"); raw != "" {
		var err error
		count, err = strconv.Atoi(raw)
		if err != nil || count < 10000 || count > 1000000 || count%100 != 0 {
			t.Fatal("WOX_FILESEARCH_SMOKE_FILES must be a multiple of 100 between 10000 and 1000000")
		}
		timeout = 8 * time.Minute
	}
	smoke.CaseWithTimeout(t, timeout, func(ctx context.Context, client *automationdriver.Client) {
		root := newFileSearchRoot(t)
		paths := make([]string, count)
		for directory := range count / 100 {
			parent := filepath.Join(root, fmt.Sprintf("project-%03d", directory))
			mkdirFileSearchDir(t, parent)
			for file := range 100 {
				index := directory*100 + file
				name := fmt.Sprintf("report-%05d.bin", index)
				body := []byte(nil)
				if index%10 == 0 {
					name = fmt.Sprintf("项目报告-%05d.txt", index)
					body = []byte(fmt.Sprintf("Wox content fixture %05d. Deterministic searchable smoke document.\n", index))
				}
				paths[index] = filepath.Join(parent, name)
				if err := os.WriteFile(paths[index], body, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		report := smokereport.Report{Title: "File Search filename reference", Scope: fmt.Sprintf("Shared native Wox; %d directories / %d files (%d small text files). Query timings include automation and UI presentation; memory is sampled without forcing GC.", count/100, count, count/10)}
		defer func() { t.Log(report.Line()) }()
		reportFileSearchMemory(ctx, client, &report, "before indexing")
		reportFileSearchDisk(&report, "before indexing")
		started := time.Now()
		row := addFileSearchRoot(t, ctx, client, root)
		t.Cleanup(func() { removeFileSearchRoot(t, client, root, row) })
		waitForFileSearchRootIdle(t, ctx, root)
		report.Add("settings action to completed index", float64(time.Since(started))/float64(time.Millisecond), "ms")
		reportFileSearchMemory(ctx, client, &report, "after indexing")
		reportFileSearchDisk(&report, "after indexing")
		smoke.ShowLauncher(t, ctx, client)
		for _, index := range []int{0, count/2 + 1, count - 1} {
			queryFileSearchResult(t, ctx, client, paths[index])
		}
		for sample := range 3 {
			queryFileSearchResult(t, ctx, client, paths[101+sample])
		}
		samples := make([]float64, 20)
		for sample := range samples {
			started := time.Now()
			queryFileSearchResult(t, ctx, client, paths[123+sample*37])
			samples[sample] = float64(time.Since(started)) / float64(time.Millisecond)
		}
		sort.Float64s(samples)
		report.Add("completed launcher query p50", samples[9], "ms")
		report.Add("completed launcher query p95", samples[18], "ms")
		reportFileSearchMemory(ctx, client, &report, "after queries")
		// Status must describe the indexed fixture rather than timing out or
		// falling through to a filename search for the word "status".
		snapshot := queryFileSearchSnapshot(t, ctx, client, "f status ")
		statusFound, countsFound := false, false
		for _, node := range snapshot.Tree.Nodes {
			statusFound = statusFound || node.Label == "File Search Status"
			text := strings.ReplaceAll(node.Label+" "+node.Description+" "+node.Value, ",", "")
			countsFound = countsFound || strings.Contains(text, fmt.Sprintf("files=%d", count))
			if strings.Contains(text, "context deadline exceeded") {
				t.Fatalf("index status timed out: %s", automationdriver.DescribeSnapshot(snapshot))
			}
		}
		if !statusFound || !countsFound {
			t.Fatalf("diagnostic command omitted index status/counts: %s", automationdriver.DescribeSnapshot(snapshot))
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// waitForFileSearchRootIdle observes the persisted root lifecycle rather than
// any filename-storage table, so the same fixture works across index backends.
func waitForFileSearchRootIdle(t *testing.T, ctx context.Context, root string) {
	t.Helper()
	databasePath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "filesearch", "directory.db")
	database, err := sql.Open("sqlite3", databasePath+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ticker := time.NewTicker(fileSearchIndexPollInterval)
	defer ticker.Stop()
	for {
		var status string
		var completed int64
		var failure sql.NullString
		err := database.QueryRowContext(ctx, "SELECT status, last_full_scan_at, last_error FROM roots WHERE path = ?", root).Scan(&status, &completed, &failure)
		if err == nil && status == "idle" && completed > 0 {
			return
		}
		if err == nil && status == "error" {
			t.Fatalf("File Search fixture indexing failed: %s", failure.String)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("File Search root did not complete: status=%q error=%v: %v", status, err, ctx.Err())
		case <-ticker.C:
		}
	}
}

// reportFileSearchMemory records the Wox process rather than the test process.
func reportFileSearchMemory(ctx context.Context, client *automationdriver.Client, report *smokereport.Report, stage string) {
	stats, err := client.RuntimeStats(ctx)
	if err != nil {
		report.Missing(stage+" memory", err)
		return
	}
	report.Add(stage+" Go heap", float64(stats.HeapAlloc), "bytes")
	if stats.ProcessMemoryBytes == 0 {
		report.Missing(stage+" process memory", fmt.Errorf("platform memory reading unavailable"))
	} else {
		report.Add(stage+" platform process memory", float64(stats.ProcessMemoryBytes), "bytes")
	}
}

// reportFileSearchDisk includes metadata, columns and journals in the isolated
// filename index directory; contentsearch.db is measured by the content case.
func reportFileSearchDisk(report *smokereport.Report, stage string) {
	dir := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "filesearch")
	var bytes int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == "contentsearch.db" || entry.Name() == "contentsearch.db-wal" || entry.Name() == "contentsearch.db-shm" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		bytes += info.Size()
		return nil
	})
	if err != nil {
		report.Missing(stage+" filename index disk", err)
	} else {
		report.Add(stage+" filename index disk", float64(bytes), "bytes")
	}
}
