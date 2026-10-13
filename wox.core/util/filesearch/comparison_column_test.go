//go:build filesearch_comparison

package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"wox/test/filesearchbench"
	"wox/util/filesearch/column"
)

// TestColumnComparison uses exactly the legacy fixture and query sequence. The
// report describes index construction, excluding filesystem enumeration and UI.
func TestColumnComparison(t *testing.T) {
	dir := t.TempDir()
	store, err := column.OpenStore(dir, prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	report := filesearchbench.Report{Implementation: "shared-column", Files: 100000}
	report.HeapBefore, report.ResidentBefore = filesearchbench.Memory()
	started := time.Now()
	root := "/wox-comparison"
	if runtime.GOOS == "windows" {
		root = `C:\wox-comparison`
	}
	builder := column.NewBuilder(root, 5)
	for _, entry := range filesearchbench.Entries(report.Files)[1:] {
		flags := byte(0)
		if entry.Directory {
			flags = column.Directory
		}
		if err := builder.Add(column.Record{ID: entry.ID, ParentID: entry.ParentID, Name: entry.Name, Flags: flags}); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(base, "fixture", "", 0); err != nil {
		t.Fatal(err)
	}
	base, builder = nil, nil
	report.BuildMillis = float64(time.Since(started).Nanoseconds()) / 1e6
	if err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			stat, err := entry.Info()
			if err != nil {
				return err
			}
			report.DiskBytes += stat.Size()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	report.HeapIndexed, report.ResidentIndexed = filesearchbench.Memory()
	filesearchbench.Measure(t, report, func(query string) (int, error) {
		count := 0
		err := store.Read(func(view column.View) error {
			results, err := searchColumnView(context.Background(), view, SearchQuery{Raw: query}, 50)
			count = len(results)
			return err
		})
		return count, err
	})
}
