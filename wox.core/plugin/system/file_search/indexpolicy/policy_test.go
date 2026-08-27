package indexpolicy

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTraversalContextFallbackHandlesTrailingSpaceDirectory(t *testing.T) {
	if os.Getenv("WOX_INDEXPOLICY_TRAILING_SPACE_HELPER") == "1" {
		runTrailingSpaceFallbackHelper(t)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestTraversalContextFallbackHandlesTrailingSpaceDirectory")
	cmd.Env = append(os.Environ(), "WOX_INDEXPOLICY_TRAILING_SPACE_HELPER=1")
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("expected fallback policy check to finish without recursive re-rooting loop")
	}
	if err != nil {
		t.Fatalf("fallback helper failed: %v\n%s", err, output)
	}
}

func runTrailingSpaceFallbackHelper(t *testing.T) {
	t.Helper()

	rootPath := t.TempDir()
	spacedDir := filepath.Join(rootPath, "Research ")
	filePath := filepath.Join(spacedDir, "note.md")
	if err := os.MkdirAll(spacedDir, 0o755); err != nil {
		t.Fatalf("mkdir spaced directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("note"), 0o644); err != nil {
		t.Fatalf("write spaced file: %v", err)
	}

	policy := New()
	context := policy.NewTraversalContext(rootPath, rootPath, rootPath)
	if context == nil {
		t.Fatalf("expected traversal context")
	}
	if !context.ShouldIndexPath(filePath, false) {
		t.Fatalf("expected spaced file path to remain indexable")
	}
}

func TestConfiguredIgnoreMatchesAbsoluteFolderPaths(t *testing.T) {
	tests := []struct {
		name        string
		rootPath    string
		pattern     string
		ignoredDir  string
		ignoredFile string
		keptFile    string
		siblingFile string
		nestedFile  string
	}{
		{
			name:        "windows drive with backslashes",
			rootPath:    `D:/Game`,
			pattern:     `D:\Game\Cache`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
		{
			name:        "windows drive with slashes",
			rootPath:    `D:/Game`,
			pattern:     `D:/Game/Cache`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
		{
			name:        "windows drive root search",
			rootPath:    `D:/`,
			pattern:     `D:\Game\Cache`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
		{
			name:        "windows drive with trailing slash",
			rootPath:    `D:/Game`,
			pattern:     `D:\Game\Cache\`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
		{
			name:        "windows drive with recursive suffix",
			rootPath:    `D:/Game`,
			pattern:     `D:\Game\Cache\**`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
		{
			name:        "unix absolute folder",
			rootPath:    `/Users/demo`,
			pattern:     `/Users/demo/Downloads`,
			ignoredDir:  `/Users/demo/Downloads`,
			ignoredFile: `/Users/demo/Downloads/setup.dmg`,
			keptFile:    `/Users/demo/Documents/notes.txt`,
			siblingFile: `/Users/demo/DownloadsBackup/setup.dmg`,
			nestedFile:  `/Users/demo/Downloads/apps/setup.dmg`,
		},
		{
			name:        "relative folder path",
			rootPath:    `D:/`,
			pattern:     `Game/Cache`,
			ignoredDir:  `D:/Game/Cache`,
			ignoredFile: `D:/Game/Cache/save.dat`,
			keptFile:    `D:/Game/Notes/readme.txt`,
			siblingFile: `D:/Game/CacheBackup/save.dat`,
			nestedFile:  `D:/Game/Cache/sub/slot1.dat`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := New()
			policy.SetIgnorePatterns([]string{test.pattern})
			context := policy.NewTraversalContext(test.rootPath, test.rootPath, test.rootPath)
			if context == nil {
				t.Fatal("expected traversal context")
			}

			assertFileSearchIgnored(t, context, test.ignoredDir, true)
			assertFileSearchIgnored(t, context, test.ignoredFile, false)
			assertFileSearchIgnored(t, context, test.nestedFile, false)
			assertFileSearchKept(t, context, test.keptFile, false)
			assertFileSearchKept(t, context, test.siblingFile, false)
		})
	}
}

func TestConfiguredIgnoreWindowsAbsolutePathIsCaseInsensitive(t *testing.T) {
	policy := New()
	policy.SetIgnorePatterns([]string{`d:\game\cache`})
	context := policy.NewTraversalContext(`D:/Game`, `D:/Game`, `D:/Game`)
	if context == nil {
		t.Fatal("expected traversal context")
	}

	assertFileSearchIgnored(t, context, `D:/Game/Cache/save.dat`, false)
	assertFileSearchKept(t, context, `D:/Game/Notes/readme.txt`, false)
}

func TestConfiguredIgnoreSegmentRuleDoesNotUseAbsoluteAncestors(t *testing.T) {
	rootPath := `C:/Users/demo/AppData/Local/Temp/MyProject`
	policy := New()
	policy.SetIgnorePatterns([]string{"temp", "**/temp/**"})
	context := policy.NewTraversalContext(rootPath, rootPath, rootPath)
	if context == nil {
		t.Fatal("expected traversal context")
	}

	// Default-style segment rules must stay scoped to the search root. A project
	// that happens to live under %TEMP% should still be indexed.
	assertFileSearchKept(t, context, rootPath+`/note.txt`, false)
	assertFileSearchIgnored(t, context, rootPath+`/temp/cache.dat`, false)
}

func assertFileSearchIgnored(t *testing.T, context *TraversalContext, path string, isDir bool) {
	t.Helper()
	if context.ShouldIndexPath(path, isDir) {
		t.Fatalf("expected ignore rule to drop %q", path)
	}
}

func assertFileSearchKept(t *testing.T, context *TraversalContext, path string, isDir bool) {
	t.Helper()
	if !context.ShouldIndexPath(path, isDir) {
		t.Fatalf("expected %q to stay indexable", path)
	}
}
