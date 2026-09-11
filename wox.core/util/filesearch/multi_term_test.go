package filesearch

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// TestSQLiteSearchProviderMultiTermAND covers recall and scoring together, including
// a matching entry beyond the normal candidate cap for a common extension.
func TestSQLiteSearchProviderMultiTermAND(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "search-root")
	root := RootRecord{
		ID: "multi-term", Path: rootPath, Kind: RootKindUser,
		Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)
	var entries []EntryRecord
	add := func(relative string, isDir bool) {
		fullPath := filepath.Join(rootPath, relative)
		entries = append(entries, EntryRecord{
			Path:           fullPath,
			RootID:         root.ID,
			ParentPath:     filepath.Dir(fullPath),
			Name:           filepath.Base(fullPath),
			NormalizedName: normalizeIndexText(filepath.Base(fullPath)),
			NormalizedPath: normalizeIndexText(fullPath),
			IsDir:          isDir,
			Mtime:          now,
			UpdatedAt:      now,
		})
	}
	for i := 0; i < defaultPreRerankLimit+1; i++ {
		add(fmt.Sprintf("common_%04d.txt", i), false)
	}
	add("amiga_workbench.txt", false)
	add("amiga_workbench.png", false)
	add("amiga.txt.bak", false)
	add("amiga notes.txt", false)
	add("design", true)
	add("design/README.md", false)
	add("amiga_folder.txt", true)
	add("测试报告.txt", false)
	entries[len(entries)-1].PinyinFull = "ceshibaogao"
	entries[len(entries)-1].PinyinInitials = "csbg"
	if err := db.ReplaceRootEntries(ctx, root, entries, nil); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query string
		names []string
	}{
		{"amiga .txt", []string{"amiga_workbench.txt", "amiga notes.txt"}},
		{".txt amiga", []string{"amiga_workbench.txt", "amiga notes.txt"}},
		{"amiga workbench .txt", []string{"amiga_workbench.txt"}},
		{"workbench amiga", []string{"amiga_workbench.txt", "amiga_workbench.png"}},
		{"amiga\t workbench  .TXT", []string{"amiga_workbench.txt"}},
		{"amiga bench .txt", []string{"amiga_workbench.txt"}},
		{"amiga nc .txt", []string{"amiga_workbench.txt"}},
		{"amiga missing .txt", nil},
		{"amiga .txt .png", nil},
		{`"amiga workbench" .txt`, nil},
		{`"amiga notes" .txt`, []string{"amiga notes.txt"}},
		{`amiga "workbench" .txt`, []string{"amiga_workbench.txt"}},
		{"design README", []string{"README.md"}},
		{"README design", []string{"README.md"}},
		{"design/ README", []string{"README.md"}},
		{"测试 报告 .txt", []string{"测试报告.txt"}},
		{"amiga*.txt", []string{"amiga_workbench.txt", "amiga notes.txt", "amiga_folder.txt"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			results, err := provider.Search(ctx, SearchQuery{Raw: tc.query, DisablePinyin: true}, 100)
			if err != nil {
				t.Fatal(err)
			}
			// Exercise stale-index fallback recall without corrupting the live FTS
			// tables or racing the provider's asynchronous repair goroutine.
			query := normalizeSearchQuery(SearchQuery{Raw: tc.query, DisablePinyin: true})
			if len(query.plan.andTerms) > 0 {
				statement, args := buildANDCandidateSQL(query.plan, 100, false)
				ids, err := provider.queryIDs(ctx, statement, args...)
				if err != nil {
					t.Fatal(err)
				}
				rows, err := provider.listEntriesByIDs(ctx, ids)
				if err != nil {
					t.Fatal(err)
				}
				var fallbackNames []string
				for _, row := range rows {
					if matched, _ := scoreDocAgainstQuery(query, docRecord{Path: row.Path, IsDir: row.IsDir}); matched {
						fallbackNames = append(fallbackNames, row.Name)
					}
				}
				if len(fallbackNames) != len(tc.names) {
					t.Fatalf("fallback expected %v, got %v", tc.names, fallbackNames)
				}
				for _, name := range fallbackNames {
					found := false
					for _, expected := range tc.names {
						found = found || name == expected
					}
					if !found {
						t.Fatalf("unexpected fallback result %q", name)
					}
				}
			}
			got := map[string]bool{}
			for _, result := range results {
				got[result.Name] = true
			}
			if len(results) != len(tc.names) {
				t.Fatalf("expected %v, got %v", tc.names, got)
			}
			for _, name := range tc.names {
				if !got[name] {
					t.Fatalf("missing %s in %v", name, got)
				}
			}
		})
	}
	for _, disabled := range []bool{false, true} {
		for _, raw := range []string{"ceshi .txt", "csbg .txt"} {
			results, err := provider.Search(ctx, SearchQuery{Raw: raw, DisablePinyin: disabled}, 100)
			if err != nil {
				t.Fatal(err)
			}
			if disabled && len(results) != 0 {
				t.Fatalf("disabled pinyin matched %q: %v", raw, results)
			}
			if !disabled && (len(results) != 1 || results[0].Name != "测试报告.txt") {
				t.Fatalf("pinyin query %q: %v", raw, results)
			}
		}
	}
}
