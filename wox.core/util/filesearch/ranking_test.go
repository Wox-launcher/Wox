package filesearch

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// TestFileSearchNameMatchesRankBeforeDirectoryMatches covers the issue's query
// variants and ensures a directory cannot boost an otherwise identical filename.
func TestFileSearchNameMatchesRankBeforeDirectoryMatches(t *testing.T) {
	for _, raw := range []string{"amiga", "amiga png", "png amiga", "amiga .png", `"amiga" .png`, `"amiga"`} {
		t.Run(raw, func(t *testing.T) {
			query := normalizeSearchQuery(SearchQuery{Raw: raw, DisablePinyin: true})
			namePath := filepath.Join("/", "home", "desktop", "amiga_workbench.png")
			ok, nameScore := scoreDocAgainstQuery(query, docRecord{Path: namePath})
			if !ok {
				t.Fatal("filename should match")
			}
			for _, path := range []string{
				filepath.Join("/", "home", "Amiga", "0.png"),
				filepath.Join("/", "home", "amigasrc", "DariusZendehRegs.png"),
			} {
				matched, score := scoreDocAgainstQuery(query, docRecord{Path: path})
				if !matched || score >= nameScore {
					t.Fatalf("filename score=%d, directory match=%v score=%d path=%s", nameScore, matched, score, path)
				}
			}
			_, sameNameScore := scoreDocAgainstQuery(query, docRecord{Path: filepath.Join("/", "home", "Amiga", "amiga_workbench.png")})
			if sameNameScore != nameScore {
				t.Fatalf("directory boosted filename: %d != %d", sameNameScore, nameScore)
			}
		})
	}
}

// TestFileSearchNamePrioritySurvivesCandidateLimit seeds path matches first so
// ranking cannot accidentally pass just because the filename had an earlier ID.
func TestFileSearchNamePrioritySurvivesCandidateLimit(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root")
	root := RootRecord{ID: "ranking", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle, CreatedAt: now, UpdatedAt: now}
	mustInsertRoot(t, ctx, db, root)
	var entries []EntryRecord
	add := func(path string, dir bool) {
		entries = append(entries, EntryRecord{Path: path, RootID: root.ID, ParentPath: filepath.Dir(path), Name: filepath.Base(path), IsDir: dir, Mtime: now, UpdatedAt: now})
	}
	dir := filepath.Join(rootPath, "Amiga")
	add(dir, true)
	for i := 0; i < defaultPreRerankLimit+1; i++ {
		add(filepath.Join(dir, fmt.Sprintf("%05d.png", i)), false)
	}
	target := filepath.Join(rootPath, "amiga_workbench.png")
	add(target, false)
	if err := db.ReplaceRootEntries(ctx, root, entries, nil); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"amiga png", "amiga .png", `"amiga" .png`} {
		results, err := NewSQLiteSearchProvider(db).Search(ctx, SearchQuery{Raw: raw, DisablePinyin: true}, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 || results[0].Path != target {
			t.Fatalf("%q: filename did not rank first: %v", raw, results)
		}
		if len(results) < 2 {
			t.Fatalf("%q: directory matches should remain available", raw)
		}
	}
}

// TestFileSearchNamePriorityPreservesOtherMatchModes keeps explicit path and
// wildcard ranking unchanged and treats enabled filename pinyin as a name match.
func TestFileSearchNamePriorityPreservesOtherMatchModes(t *testing.T) {
	record := docRecord{Path: filepath.Join("/", "home", "Amiga", "amiga_workbench.png")}
	for _, raw := range []string{"Amiga/", "Amiga/ png", "amiga*.png"} {
		query := normalizeSearchQuery(SearchQuery{Raw: raw})
		matched, score := scoreDocAgainstQuery(query, record)
		oldMatched, oldScore := scoreDocMatch(query, record, false)
		if !matched || matched != oldMatched || score != oldScore {
			t.Fatalf("%q changed path/wildcard ranking: %v %d vs %v %d", raw, matched, score, oldMatched, oldScore)
		}
	}
	chinese := docRecord{Path: filepath.Join("/", "home", "测试报告.png"), PinyinFull: "ceshibaogao", PinyinInitials: "csbg"}
	directory := docRecord{Path: filepath.Join("/", "home", "ceshi", "0.png")}
	for _, disabled := range []bool{false, true} {
		query := normalizeSearchQuery(SearchQuery{Raw: "ceshi png", DisablePinyin: disabled})
		nameMatched, nameScore := scoreDocAgainstQuery(query, chinese)
		pathMatched, pathScore := scoreDocAgainstQuery(query, directory)
		if !pathMatched {
			t.Fatal("directory match lost")
		}
		if disabled && nameMatched {
			t.Fatal("disabled pinyin still matched")
		}
		if !disabled && (!nameMatched || nameScore <= pathScore) {
			t.Fatalf("filename pinyin should rank first: %v %d vs %d", nameMatched, nameScore, pathScore)
		}
	}
}
