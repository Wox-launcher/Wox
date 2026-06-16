package system

import (
	"os"
	"path/filepath"
	"testing"
	"wox/util/filesearch"
)

func TestFileSearchPolicyUsesPolicyRootPathForDynamicRootGitIgnore(t *testing.T) {
	parentRoot := filepath.Join(t.TempDir(), "root-policy-parent")
	dynamicRoot := filepath.Join(parentRoot, "workspace", "content")
	ignoredFile := filepath.Join(dynamicRoot, "ignored.log")
	keptFile := filepath.Join(dynamicRoot, "kept.txt")

	mustWritePolicyTestFile(t, filepath.Join(parentRoot, ".gitignore"), "*.log\n")
	mustWritePolicyTestFile(t, ignoredFile, "ignored")
	mustWritePolicyTestFile(t, keptFile, "kept")

	policy := newFileSearchIndexPolicy()
	root := filesearch.RootRecord{
		ID:             "root-policy-dynamic",
		Path:           dynamicRoot,
		Kind:           filesearch.RootKindDynamic,
		PolicyRootPath: parentRoot,
	}

	ignoredContext := policy.newTraversalContext(root, filepath.Dir(ignoredFile))
	if ignoredContext.ShouldIndexPath(ignoredFile, false) {
		t.Fatalf("expected dynamic root to inherit parent gitignore for %q", ignoredFile)
	}
	keptContext := policy.newTraversalContext(root, filepath.Dir(keptFile))
	if !keptContext.ShouldIndexPath(keptFile, false) {
		t.Fatalf("expected dynamic root to keep non-ignored file %q", keptFile)
	}
}

func TestFileSearchPolicyCachesChangeTraversalContext(t *testing.T) {
	rootPath := t.TempDir()
	firstFile := filepath.Join(rootPath, "workspace", "first.txt")
	secondFile := filepath.Join(rootPath, "workspace", "second.txt")
	mustWritePolicyTestFile(t, firstFile, "first")
	mustWritePolicyTestFile(t, secondFile, "second")

	policy := newFileSearchIndexPolicy()
	root := filesearch.RootRecord{
		ID:   "root-policy-cache",
		Path: rootPath,
		Kind: filesearch.RootKindUser,
	}

	for _, path := range []string{firstFile, secondFile} {
		if !policy.shouldProcessChange(root, filesearch.ChangeSignal{
			Kind:          filesearch.ChangeSignalKindDirtyPath,
			RootID:        root.ID,
			Path:          path,
			PathIsDir:     false,
			PathTypeKnown: true,
		}) {
			t.Fatalf("expected policy to keep %q", path)
		}
	}

	if got := policyChangeContextCacheSize(policy); got != 1 {
		t.Fatalf("expected same-directory change signals to reuse one traversal context, got cache size %d", got)
	}
	policy.SetIgnorePatterns([]string{"*.txt"})
	if got := policyChangeContextCacheSize(policy); got != 0 {
		t.Fatalf("expected ignore pattern changes to clear traversal cache, got cache size %d", got)
	}
}

func TestFileSearchPolicyGitIgnoreChangeClearsChangeTraversalCache(t *testing.T) {
	rootPath := t.TempDir()
	ignoredFile := filepath.Join(rootPath, "workspace", "ignored.log")
	gitIgnorePath := filepath.Join(rootPath, ".gitignore")
	mustWritePolicyTestFile(t, gitIgnorePath, "*.log\n")
	mustWritePolicyTestFile(t, ignoredFile, "ignored")

	policy := newFileSearchIndexPolicy()
	root := filesearch.RootRecord{
		ID:   "root-policy-gitignore-cache",
		Path: rootPath,
		Kind: filesearch.RootKindUser,
	}
	change := filesearch.ChangeSignal{
		Kind:          filesearch.ChangeSignalKindDirtyPath,
		RootID:        root.ID,
		Path:          ignoredFile,
		PathIsDir:     false,
		PathTypeKnown: true,
	}
	if policy.shouldProcessChange(root, change) {
		t.Fatalf("expected initial .gitignore to drop %q", ignoredFile)
	}

	mustWritePolicyTestFile(t, gitIgnorePath, "")
	policy.shouldProcessChange(root, filesearch.ChangeSignal{
		Kind:          filesearch.ChangeSignalKindDirtyPath,
		RootID:        root.ID,
		Path:          gitIgnorePath,
		PathIsDir:     false,
		PathTypeKnown: true,
	})
	if !policy.shouldProcessChange(root, change) {
		t.Fatalf("expected .gitignore cache invalidation to keep %q after rule removal", ignoredFile)
	}
}

func policyChangeContextCacheSize(policy *fileSearchIndexPolicy) int {
	policy.changeContextCacheMu.RLock()
	defer policy.changeContextCacheMu.RUnlock()
	return len(policy.changeContextCache)
}

func mustWritePolicyTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
