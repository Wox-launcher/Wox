package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"wox/util/filesearch/column"
)

// TestColumnShortCJKNamesRemainSearchable covers the v2.4.7 short-query
// regression with explicit positive expectations for files and directories.
func TestColumnShortCJKNamesRemainSearchable(t *testing.T) {
	builder := column.NewBuilder(t.TempDir(), 1)
	for _, record := range []column.Record{
		{ID: 2, ParentID: 1, Name: "保", Flags: column.Directory},
		{ID: 3, ParentID: 1, Name: "保险", Flags: column.Directory},
		{ID: 4, ParentID: 1, Name: "保险资料", Flags: column.Directory},
		{ID: 5, ParentID: 1, Name: "保.txt"},
		{ID: 6, ParentID: 1, Name: "保险.txt"},
	} {
		if err := builder.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareColumnNames(base); err != nil {
		t.Fatal(err)
	}
	indexPath := filepath.Join(t.TempDir(), "short-cjk.columns")
	if err := column.Write(indexPath, base); err != nil {
		t.Fatal(err)
	}
	mapped, err := column.Open(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { mapped.Close() })
	view := column.View{Base: mapped}

	for _, tc := range []struct {
		query string
		name  string
		isDir bool
	}{
		{query: "保", name: "保", isDir: true},
		{query: "保", name: "保.txt", isDir: false},
		{query: "保险", name: "保险", isDir: true},
		{query: "保险", name: "保险.txt", isDir: false},
		{query: "保险资料", name: "保险资料", isDir: true},
	} {
		for _, disabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%s/pinyin-disabled-%t", tc.query, tc.name, disabled), func(t *testing.T) {
				results, err := searchColumnView(context.Background(), view, SearchQuery{
					Raw:           tc.query,
					DisablePinyin: disabled,
				}, 50)
				if err != nil {
					t.Fatal(err)
				}
				for _, result := range results {
					if result.Name == tc.name && result.IsDir == tc.isDir {
						return
					}
				}
				t.Fatalf("query %q did not return %q (isDir=%t): %+v", tc.query, tc.name, tc.isDir, results)
			})
		}
	}
}
