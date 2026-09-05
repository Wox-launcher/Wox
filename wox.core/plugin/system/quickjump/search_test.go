package quickjump

import (
	"path/filepath"
	"testing"
	"wox/plugin"
	"wox/util"
)

func TestIsDirectChildPath(t *testing.T) {
	quickJumpPlugin := &QuickJumpPlugin{}
	root := t.TempDir()
	child := filepath.Join(root, "readme.md")
	nested := filepath.Join(root, "docs", "readme.md")

	if !isDirectChildPath(child, root, quickJumpPlugin.normalizePathKey) {
		t.Fatalf("expected %q to be a direct child of %q", child, root)
	}
	if isDirectChildPath(nested, root, quickJumpPlugin.normalizePathKey) {
		t.Fatalf("nested path %q should not count as a current-folder hit", nested)
	}
	if isDirectChildPath(root, root, quickJumpPlugin.normalizePathKey) {
		t.Fatal("a folder is not a child of itself")
	}
	if isDirectChildPath(child, "", quickJumpPlugin.normalizePathKey) {
		t.Fatal("empty current folder should not match")
	}
	if util.IsWindows() && !isDirectChildPath(`C:\Docs\readme.md`, `C:\docs`, quickJumpPlugin.normalizePathKey) {
		t.Fatal("windows current-folder matching should ignore case")
	}
}

func TestMergeCurrentDirectoryResultsBoostsAndBackfills(t *testing.T) {
	quickJumpPlugin := &QuickJumpPlugin{}
	root := t.TempDir()
	localFile := filepath.Join(root, "notes.txt")
	localOnly := filepath.Join(root, "local-only.txt")
	nestedFile := filepath.Join(root, "archive", "notes.txt")
	elsewhere := filepath.Join(filepath.Dir(root), "elsewhere", "notes.txt")

	indexed := []plugin.QueryResult{
		{Title: "notes.txt", SubTitle: elsewhere, Score: 8000},
		{Title: "notes.txt", SubTitle: localFile, Score: 4200},
		{Title: "notes.txt", SubTitle: nestedFile, Score: 7600},
	}
	local := []plugin.QueryResult{
		{Title: "notes.txt", SubTitle: localFile, Score: 180},
		{Title: "local-only.txt", SubTitle: localOnly, Score: 90},
	}

	merged := mergeCurrentDirectoryResults(indexed, local, root, quickJumpPlugin.normalizePathKey)
	if len(merged) != 4 {
		t.Fatalf("merged count = %d, want 4", len(merged))
	}

	byPath := map[string]plugin.QueryResult{}
	for _, item := range merged {
		byPath[item.SubTitle] = item
	}

	if got := byPath[localFile].Score; got != 4200+currentDirectoryScoreBoost {
		t.Fatalf("current-folder score = %d, want %d", got, 4200+currentDirectoryScoreBoost)
	}
	if got := byPath[elsewhere].Score; got != 8000 {
		t.Fatalf("other-folder score = %d, want 8000", got)
	}
	if got := byPath[nestedFile].Score; got != 7600 {
		t.Fatalf("nested score = %d, want 7600", got)
	}
	if got := byPath[localOnly].Score; got != 90+currentDirectoryScoreBoost {
		t.Fatalf("local-only score = %d, want %d", got, 90+currentDirectoryScoreBoost)
	}
	if byPath[localFile].Score <= byPath[elsewhere].Score {
		t.Fatal("current-folder file should outrank the same name elsewhere")
	}
	assertCurrentDirectoryTail(t, byPath[localFile], true)
	assertCurrentDirectoryTail(t, byPath[localOnly], true)
	assertCurrentDirectoryTail(t, byPath[elsewhere], false)
	assertCurrentDirectoryTail(t, byPath[nestedFile], false)
}

func TestMergeCurrentDirectoryResultsLeavesIndexedUntouchedWithoutCurrentDir(t *testing.T) {
	quickJumpPlugin := &QuickJumpPlugin{}
	indexed := []plugin.QueryResult{
		{Title: "notes.txt", SubTitle: filepath.Join("C:", "docs", "notes.txt"), Score: 5000},
	}

	merged := mergeCurrentDirectoryResults(indexed, nil, "", quickJumpPlugin.normalizePathKey)
	if len(merged) != 1 || merged[0].Score != 5000 {
		t.Fatalf("empty current dir should keep indexed results: %#v", merged)
	}
	assertCurrentDirectoryTail(t, merged[0], false)
}

func assertCurrentDirectoryTail(t *testing.T, result plugin.QueryResult, want bool) {
	t.Helper()
	hasTail := false
	for _, tail := range result.Tails {
		if tail.Type == plugin.QueryResultTailTypeText && tail.Text == "i18n:plugin_quickjump_result_tail_current_folder" {
			hasTail = true
			break
		}
	}
	if hasTail != want {
		t.Fatalf("%q current-folder tail = %v, want %v", result.SubTitle, hasTail, want)
	}
}
