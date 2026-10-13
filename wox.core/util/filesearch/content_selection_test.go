package filesearch

import (
	"os"
	"path/filepath"
	"testing"
)

type contentTestTraversalPolicy struct{}

func (contentTestTraversalPolicy) ShouldIndexPath(path string, _ bool) bool {
	return filepath.Base(path) != "ignored.txt"
}
func (p contentTestTraversalPolicy) Descend(string) TraversalPolicyContext { return p }

// TestContentCandidatesRespectIndependentRoots verifies selection before reads
// and cleanup: a broad filename index cannot expand the content index's scope.
func TestContentCandidatesRespectIndependentRoots(t *testing.T) {
	nameDB, ctx := openTestFileSearchDB(t)
	contentDB := newTestContentSearchDB(t)
	defer contentDB.Close()
	root := RootRecord{ID: "filename-root", Path: t.TempDir(), Kind: RootKindUser}
	mustInsertRoot(t, ctx, nameDB, root)
	contentRoot := RootRecord{ID: "content-root", Path: filepath.Join(root.Path, "content")}
	if err := os.MkdirAll(contentRoot.Path, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(contentRoot.Path, "allowed.txt"), filepath.Join(contentRoot.Path, "ignored.txt"), filepath.Join(root.Path, "outside.txt")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("uniquescopetoken"), 0600); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		mustInsertEntrySnapshots(t, ctx, nameDB, EntryRecord{RootID: root.ID, Path: path, Name: filepath.Base(path), ParentPath: filepath.Dir(path), Size: info.Size(), Mtime: info.ModTime().UnixMilli()})
		if _, err := contentDB.IndexContent(ctx, path, 0, 0, "txt", "oldscopecontent"); err != nil {
			t.Fatal(err)
		}
	}
	policy := Policy{NewTraversalContext: func(RootRecord, string) TraversalPolicyContext { return contentTestTraversalPolicy{} }}
	extensions := ContentExtensionsFromList([]string{"txt"})
	crawler := NewContentCrawler(contentDB, []RootRecord{contentRoot}, policy, extensions, ContentDefaultMaxReadBytes, nil)
	crawler.setNameDB(nameDB)
	if err := crawler.Run(ctx); err != nil {
		t.Fatal(err)
	}
	entries, err := contentDB.ListContentEntryMetadata(ctx)
	if err != nil || len(entries) != 1 {
		t.Fatalf("content entries=%v error=%v", entries, err)
	}
	if _, found := entries[paths[0]]; !found {
		t.Fatal("allowed content missing")
	}
	hook := NewContentIndexHook(contentDB, nameDB, []RootRecord{contentRoot}, policy, extensions, ContentDefaultMaxReadBytes)
	defer hook.Close()
	for _, path := range paths {
		hook.processUpsert(ctx, path)
	}
	entries, err = contentDB.ListContentEntryMetadata(ctx)
	if err != nil || len(entries) != 1 {
		t.Fatalf("incremental hook expanded content scope: %v %v", entries, err)
	}
}
