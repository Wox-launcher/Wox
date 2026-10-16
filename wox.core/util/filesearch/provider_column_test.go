package filesearch

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"wox/util/filesearch/column"
)

// columnQueryFixture includes shared names, directory matches, polyphonic and
// mixed-language filenames, and files that only match by their parent path.
func columnQueryFixture(t *testing.T) column.View {
	t.Helper()
	builder := column.NewBuilder(t.TempDir(), 1)
	for _, record := range []column.Record{
		{ID: 2, ParentID: 1, Name: "Amiga", Flags: column.Directory},
		{ID: 3, ParentID: 1, Name: "desktop", Flags: column.Directory},
		{ID: 4, ParentID: 2, Name: "0.png"},
		{ID: 5, ParentID: 3, Name: "amiga_workbench.png"},
		{ID: 6, ParentID: 2, Name: "amiga_workbench.png"},
		{ID: 7, ParentID: 3, Name: "项目报告.txt"},
		{ID: 8, ParentID: 3, Name: "重庆火锅.md"},
		{ID: 9, ParentID: 3, Name: "Wox测试2026.txt"},
		{ID: 10, ParentID: 3, Name: "report.txt"},
		{ID: 11, ParentID: 3, Name: "plain_text_document.md"},
		{ID: 12, ParentID: 3, Name: "café_Über.txt"},
		{ID: 13, ParentID: 3, Name: "cafe_ascii.txt"},
		{ID: 14, ParentID: 3, Name: "e"},
		{ID: 15, ParentID: 3, Name: " a "},
		{ID: 16, ParentID: 3, Name: "s_S.txt"},
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
	return column.View{Base: base}
}

func TestColumnQueryMatchesExhaustiveScoring(t *testing.T) {
	view := columnQueryFixture(t)
	for _, raw := range []string{"*", "a a", "ss", "aa", "é", "e", "amiga", "amiga png", "png amiga", "amiga .png", `"amiga" .png`, `"amiga"`, "Amiga/", "Amiga/ png", "amiga*.png", "*.txt", ".txt", "rpt", "rp", "xiangmu", "xm", "项目", "chongqing", "zhongqing", "cq", "woxcs2026", "café", "cafe", "uber", "desktop", "absent"} {
		for _, disabled := range []bool{false, true} {
			for _, limit := range []int{1, 3, 50} {
				query := normalizeSearchQuery(SearchQuery{Raw: raw, DisablePinyin: disabled})
				var want []SearchResult
				for row := 1; row < view.Base.Len(); row++ {
					record := view.Base.Record(uint32(row))
					path, _ := view.Base.Path(uint32(row))
					if !columnExtensionEligible(query, record) {
						continue
					}
					if ok, score := scoreDocAgainstQuery(query, docRecord{Path: path, IsDir: record.Flags&column.Directory != 0}); ok {
						want = append(want, SearchResult{Path: path, Name: record.Name, ParentPath: filepath.Dir(path), IsDir: record.Flags&column.Directory != 0, Score: score})
					}
				}
				want = sortAndLimitResults(want, limit)
				got, err := searchColumnView(context.Background(), view, query, limit)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("%q disabled=%v limit=%d:\ngot %+v\nwant %+v\nerror %v", raw, disabled, limit, got, want, err)
				}
			}
		}
	}
}

func TestColumnShortQueriesAndPinyin(t *testing.T) {
	view := columnQueryFixture(t)
	for raw, name := range map[string]string{"rpt": "report.txt", "rp": "report.txt", "xiangmu": "项目报告.txt", "xm": "项目报告.txt", "chongqing": "重庆火锅.md", "zhongqing": "重庆火锅.md", "woxcs2026": "Wox测试2026.txt"} {
		results, err := searchColumnView(context.Background(), view, SearchQuery{Raw: raw}, 50)
		found := false
		for _, result := range results {
			found = found || result.Name == name
		}
		if err != nil || !found {
			t.Fatalf("%q missing %q: %+v %v", raw, name, results, err)
		}
	}
}

// TestColumnRecycleBinRanksBelowOrdinaryFiles keeps short $I names from filling
// the top K ahead of ordinary extension matches, without dropping them entirely.
func TestColumnRecycleBinRanksBelowOrdinaryFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("recycle bin ranking follows the Windows drive-root layout")
	}
	view := recycleBinColumnFixture(t, true)
	limited, err := searchColumnView(context.Background(), view, SearchQuery{Raw: ".wox", DisablePinyin: true}, 1)
	if err != nil || len(limited) != 1 || limited[0].Name != "Wox.Plugin.Spotify.wox" {
		t.Fatalf("ordinary file lost the only slot: %+v %v", limited, err)
	}
	ranked, err := searchColumnView(context.Background(), view, SearchQuery{Raw: ".wox", DisablePinyin: true}, 8)
	if err != nil || len(ranked) != 8 || ranked[0].Name != "Wox.Plugin.Spotify.wox" {
		t.Fatalf("ordinary file should lead: %+v %v", ranked, err)
	}
	for _, result := range ranked[1:] {
		if !pathInWindowsRecycleBin(result.Path) || result.Score >= ranked[0].Score {
			t.Fatalf("recycle bin result out of order: %+v", ranked)
		}
	}
	exact, err := searchColumnView(context.Background(), view, SearchQuery{Raw: "$I00", DisablePinyin: true}, 5)
	if err != nil || len(exact) != 1 || exact[0].Name != "$I00.wox" {
		t.Fatalf("recycle bin file should remain searchable: %+v %v", exact, err)
	}

	nested := recycleBinColumnFixture(t, false)
	nestedRanked, err := searchColumnView(context.Background(), nested, SearchQuery{Raw: ".wox", DisablePinyin: true}, 1)
	if err != nil || len(nestedRanked) != 1 || nestedRanked[0].Name != "Wox.Plugin.Nested.wox" || pathInWindowsRecycleBin(nestedRanked[0].Path) {
		t.Fatalf("same-named folder outside the drive root should keep its score: %+v %v", nestedRanked, err)
	}
}

func recycleBinColumnFixture(t *testing.T, includePlugin bool) column.View {
	t.Helper()
	builder := column.NewBuilder(`C:\`, 1)
	add := func(record column.Record) {
		t.Helper()
		if err := builder.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	add(column.Record{ID: 2, ParentID: 1, Name: "$Recycle.Bin", Flags: column.Directory})
	add(column.Record{ID: 3, ParentID: 2, Name: "S-1-5-21", Flags: column.Directory})
	for i := 0; i < 30; i++ {
		add(column.Record{ID: uint64(10 + i), ParentID: 3, Name: fmt.Sprintf("$I%02d.wox", i)})
	}
	if includePlugin {
		add(column.Record{ID: 4, ParentID: 1, Name: "dev", Flags: column.Directory})
		add(column.Record{ID: 5, ParentID: 4, Name: "Wox.Plugin.Spotify.wox"})
	} else {
		add(column.Record{ID: 4, ParentID: 1, Name: "projects", Flags: column.Directory})
		add(column.Record{ID: 5, ParentID: 4, Name: "$Recycle.Bin", Flags: column.Directory})
		add(column.Record{ID: 6, ParentID: 5, Name: "Wox.Plugin.Nested.wox"})
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := prepareColumnNames(base); err != nil {
		t.Fatal(err)
	}
	return column.View{Base: base}
}

func TestColumnQueryHonorsDirectoryOverlay(t *testing.T) {
	view := columnQueryFixture(t)
	view.Delta = map[uint64]column.Change{3: {Record: column.Record{ID: 3, ParentID: 1, Name: "renamed", Flags: column.Directory}}}
	results, err := searchColumnView(context.Background(), view, SearchQuery{Raw: "report"}, 50)
	if err != nil || len(results) != 1 || filepath.Base(results[0].ParentPath) != "renamed" {
		t.Fatalf("renamed parent: %+v %v", results, err)
	}
	view.Delta[3] = column.Change{Record: column.Record{ID: 3}, Deleted: true}
	results, err = searchColumnView(context.Background(), view, SearchQuery{Raw: "report"}, 50)
	if err != nil || len(results) != 0 {
		t.Fatalf("deleted subtree: %+v %v", results, err)
	}
}
