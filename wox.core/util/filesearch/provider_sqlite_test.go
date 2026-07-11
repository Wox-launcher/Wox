package filesearch

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteSearchProviderOneCharacterSearchMatchesNamePrefixOnly(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-one-char")
	root := RootRecord{
		ID:        "root-sqlite-provider-one-char",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		{
			Path:           filepath.Join(rootPath, "alpha.txt"),
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "alpha.txt",
			NormalizedName: "alpha.txt",
			NormalizedPath: filepath.Join(rootPath, "alpha.txt"),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
		{
			Path:           filepath.Join(rootPath, "nested", "report.txt"),
			RootID:         root.ID,
			ParentPath:     filepath.Join(rootPath, "nested"),
			Name:           "report.txt",
			NormalizedName: "report.txt",
			NormalizedPath: filepath.Join(rootPath, "nested", "report.txt"),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
	}, nil); err != nil {
		t.Fatalf("seed sqlite provider entries: %v", err)
	}

	results, err := provider.Search(context.Background(), SearchQuery{Raw: "a"}, 10)
	if err != nil {
		t.Fatalf("search one-char prefix: %v", err)
	}
	if len(results) != 1 || results[0].Name != "alpha.txt" {
		t.Fatalf("expected one-char search to return only alpha.txt, got %#v", results)
	}
}

func TestSQLiteSearchProviderTwoCharacterSearchUsesNamePrefixOnly(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-two-char")
	root := RootRecord{
		ID:        "root-sqlite-provider-two-char",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		{
			Path:           filepath.Join(rootPath, "readme.md"),
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "readme.md",
			NormalizedName: "readme.md",
			NormalizedPath: filepath.Join(rootPath, "readme.md"),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
	}, nil); err != nil {
		t.Fatalf("seed sqlite provider two-char entry: %v", err)
	}

	results, err := provider.Search(context.Background(), SearchQuery{Raw: "re"}, 10)
	if err != nil {
		t.Fatalf("search two-char prefix: %v", err)
	}
	if len(results) != 1 || results[0].Name != "readme.md" {
		t.Fatalf("expected two-char prefix search to return readme.md, got %#v", results)
	}

	substringResults, err := provider.Search(context.Background(), SearchQuery{Raw: "ea"}, 10)
	if err != nil {
		t.Fatalf("search two-char substring: %v", err)
	}
	if len(substringResults) != 0 {
		t.Fatalf("expected two-char substring search to stop matching readme.md, got %#v", substringResults)
	}
}

func TestSQLiteSearchProviderRenameDoesNotLeaveGhostResult(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-rename")
	root := RootRecord{
		ID:        "root-sqlite-provider-rename",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	oldPath := filepath.Join(rootPath, "legacy-report.txt")
	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{{
		Path:           oldPath,
		RootID:         root.ID,
		ParentPath:     rootPath,
		Name:           "legacy-report.txt",
		NormalizedName: "legacy-report.txt",
		NormalizedPath: oldPath,
		IsDir:          false,
		Mtime:          now,
		Size:           1,
		UpdatedAt:      now,
	}}, nil); err != nil {
		t.Fatalf("seed legacy entry: %v", err)
	}

	newPath := filepath.Join(rootPath, "fresh-report.txt")
	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{{
		Path:           newPath,
		RootID:         root.ID,
		ParentPath:     rootPath,
		Name:           "fresh-report.txt",
		NormalizedName: "fresh-report.txt",
		NormalizedPath: newPath,
		IsDir:          false,
		Mtime:          now + 1,
		Size:           2,
		UpdatedAt:      now + 1,
	}}, nil); err != nil {
		t.Fatalf("replace renamed entry: %v", err)
	}

	oldResults, err := provider.Search(context.Background(), SearchQuery{Raw: "legacy-report"}, 10)
	if err != nil {
		t.Fatalf("search old name after rename: %v", err)
	}
	if len(oldResults) != 0 {
		t.Fatalf("expected renamed old name to disappear, got %#v", oldResults)
	}

	newResults, err := provider.Search(context.Background(), SearchQuery{Raw: "fresh-report"}, 10)
	if err != nil {
		t.Fatalf("search new name after rename: %v", err)
	}
	if len(newResults) != 1 || newResults[0].Path != newPath {
		t.Fatalf("expected renamed new name to be searchable, got %#v", newResults)
	}
}

func TestSQLiteSearchProviderQuotedNamePhraseMatchesLiteralSubstring(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-quoted-name")
	root := RootRecord{
		ID:        "root-sqlite-provider-quoted-name",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	phrasePath := filepath.Join(rootPath, "Initialize content index if enabled.md")
	scrambledPath := filepath.Join(rootPath, "Initialize index content if enabled.md")
	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		{
			Path:           phrasePath,
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "Initialize content index if enabled.md",
			NormalizedName: normalizeIndexText("Initialize content index if enabled.md"),
			NormalizedPath: normalizeIndexText(phrasePath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
		{
			Path:           scrambledPath,
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "Initialize index content if enabled.md",
			NormalizedName: normalizeIndexText("Initialize index content if enabled.md"),
			NormalizedPath: normalizeIndexText(scrambledPath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
	}, nil); err != nil {
		t.Fatalf("seed quoted name entries: %v", err)
	}

	results, err := provider.Search(context.Background(), SearchQuery{Raw: `"content index"`}, 10)
	if err != nil {
		t.Fatalf("search quoted name phrase: %v", err)
	}
	if len(results) != 1 || results[0].Path != phrasePath {
		t.Fatalf("expected only literal quoted name phrase match, got %#v", results)
	}
}

func TestSQLiteSearchProviderQuotedPathPhraseMatchesLiteralSubstring(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-quoted-path")
	root := RootRecord{
		ID:        "root-sqlite-provider-quoted-path",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	phraseDir := filepath.Join(rootPath, "release notes")
	scrambledDir := filepath.Join(rootPath, "release internal notes")
	phrasePath := filepath.Join(phraseDir, "report.txt")
	scrambledPath := filepath.Join(scrambledDir, "report.txt")
	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		{
			Path:           phrasePath,
			RootID:         root.ID,
			ParentPath:     phraseDir,
			Name:           "report.txt",
			NormalizedName: "report.txt",
			NormalizedPath: normalizeIndexText(phrasePath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
		{
			Path:           scrambledPath,
			RootID:         root.ID,
			ParentPath:     scrambledDir,
			Name:           "report.txt",
			NormalizedName: "report.txt",
			NormalizedPath: normalizeIndexText(scrambledPath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
	}, nil); err != nil {
		t.Fatalf("seed quoted path entries: %v", err)
	}

	results, err := provider.Search(context.Background(), SearchQuery{Raw: `"release notes"`}, 10)
	if err != nil {
		t.Fatalf("search quoted path phrase: %v", err)
	}
	if len(results) != 1 || results[0].Path != phrasePath {
		t.Fatalf("expected only literal quoted path phrase match, got %#v", results)
	}
}

func TestSQLiteSearchProviderWildcardWithQuotedPhraseRequiresBoth(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	provider := NewSQLiteSearchProvider(db)
	now := time.Now().UnixMilli()
	rootPath := filepath.Join(t.TempDir(), "root-sqlite-provider-wildcard-quoted")
	root := RootRecord{
		ID:        "root-sqlite-provider-wildcard-quoted",
		Path:      rootPath,
		Kind:      RootKindUser,
		Status:    RootStatusIdle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	mustInsertRoot(t, ctx, db, root)

	matchingPath := filepath.Join(rootPath, "task content index.md")
	wildcardOnlyPath := filepath.Join(rootPath, "task index content.md")
	if err := db.ReplaceRootEntries(ctx, root, []EntryRecord{
		{
			Path:           matchingPath,
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "task content index.md",
			NormalizedName: "task content index.md",
			NormalizedPath: normalizeIndexText(matchingPath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
		{
			Path:           wildcardOnlyPath,
			RootID:         root.ID,
			ParentPath:     rootPath,
			Name:           "task index content.md",
			NormalizedName: "task index content.md",
			NormalizedPath: normalizeIndexText(wildcardOnlyPath),
			IsDir:          false,
			Mtime:          now,
			Size:           1,
			UpdatedAt:      now,
		},
	}, nil); err != nil {
		t.Fatalf("seed wildcard quoted entries: %v", err)
	}

	results, err := provider.Search(context.Background(), SearchQuery{Raw: `task* "content index"`}, 10)
	if err != nil {
		t.Fatalf("search wildcard with quoted phrase: %v", err)
	}
	if len(results) != 1 || results[0].Path != matchingPath {
		t.Fatalf("expected wildcard result to also satisfy quoted phrase, got %#v", results)
	}
}

func TestScoreDocAgainstQueryWildcardWithQuotedPhraseRequiresBoth(t *testing.T) {
	query := normalizeSearchQuery(SearchQuery{Raw: `task* "content index"`})
	matched, _ := scoreDocAgainstQuery(query, docRecord{
		Path:  "/tmp/task index content.md",
		IsDir: false,
	})
	if matched {
		t.Fatal("wildcard match should still be rejected when quoted phrase is missing")
	}

	matched, _ = scoreDocAgainstQuery(query, docRecord{
		Path:  "/tmp/task content index.md",
		IsDir: false,
	})
	if !matched {
		t.Fatal("wildcard match should pass when quoted phrase is present")
	}
}
