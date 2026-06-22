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
