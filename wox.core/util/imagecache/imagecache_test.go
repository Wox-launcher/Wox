package imagecache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wox/util"
)

func TestPluginDirectoryCleanup(t *testing.T) {
	data := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, data)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(data, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	root := util.GetLocation().GetImageCacheDirectory()
	first := filepath.Join(root, "plugins", "first", "1", "icon.png")
	second := filepath.Join(root, "plugins", "first-sibling", "1", "icon.png")
	for _, filename := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte("image"), 0644); err != nil {
			t.Fatal(err)
		}
		RememberDerivedPathExists(filename)
		Touch(context.Background(), filename, nil)
	}
	if err := RemoveDirectory(root); err == nil {
		t.Fatal("must reject the entire cache root")
	}
	if err := RemoveDirectory(filepath.Dir(root)); err == nil {
		t.Fatal("must reject paths outside image cache")
	}
	if err := RemoveDirectory(filepath.Dir(first)); err != nil {
		t.Fatal(err)
	}
	if IsKnownExistingDerivedPath(first) {
		t.Fatal("deleted path still remembered")
	}
	if !IsKnownExistingDerivedPath(second) {
		t.Fatal("sibling positive cache removed")
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatal(err)
	}
	touchState.mu.Lock()
	_, remembered := touchState.attempts[first]
	touchState.mu.Unlock()
	if remembered {
		t.Fatal("deleted path touch state retained")
	}
	old := time.Now().Add(-retentionAge - time.Hour)
	if err := os.Chtimes(second, old, old); err != nil {
		t.Fatal(err)
	}
	removed, err := CleanupExpired(context.Background())
	if err != nil || removed != 1 {
		t.Fatalf("nested expiry: removed=%d err=%v", removed, err)
	}
	if IsKnownExistingDerivedPath(second) {
		t.Fatal("expired path still remembered")
	}
}
