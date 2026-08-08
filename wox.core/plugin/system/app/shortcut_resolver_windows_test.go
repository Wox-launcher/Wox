//go:build windows

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShortcutTargetResolverCachesUntilShortcutChanges(t *testing.T) {
	shortcutPath := filepath.Join(t.TempDir(), "app.lnk")
	if err := os.WriteFile(shortcutPath, []byte("first"), 0644); err != nil {
		t.Fatalf("write shortcut: %v", err)
	}

	resolver := newShortcutTargetResolver()
	loadCount := 0
	load := func(context.Context, string) (string, error) {
		loadCount++
		return `C:\Apps\Wox.exe`, nil
	}

	for range 2 {
		targetPath, err := resolver.resolve(context.Background(), shortcutPath, load)
		if err != nil {
			t.Fatalf("resolve shortcut: %v", err)
		}
		if targetPath != `C:\Apps\Wox.exe` {
			t.Fatalf("target path = %q", targetPath)
		}
	}
	if loadCount != 1 {
		t.Fatalf("load count before change = %d, want 1", loadCount)
	}

	changedAt := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(shortcutPath, changedAt, changedAt); err != nil {
		t.Fatalf("change shortcut timestamp: %v", err)
	}
	if _, err := resolver.resolve(context.Background(), shortcutPath, load); err != nil {
		t.Fatalf("resolve changed shortcut: %v", err)
	}
	if loadCount != 2 {
		t.Fatalf("load count after change = %d, want 2", loadCount)
	}
}

func TestShortcutTargetResolverDoesNotCacheFailures(t *testing.T) {
	shortcutPath := filepath.Join(t.TempDir(), "app.lnk")
	if err := os.WriteFile(shortcutPath, []byte("shortcut"), 0644); err != nil {
		t.Fatalf("write shortcut: %v", err)
	}

	resolver := newShortcutTargetResolver()
	loadCount := 0
	load := func(context.Context, string) (string, error) {
		loadCount++
		if loadCount == 1 {
			return "", errors.New("temporary COM failure")
		}
		return `C:\Apps\Wox.exe`, nil
	}

	if _, err := resolver.resolve(context.Background(), shortcutPath, load); err == nil {
		t.Fatal("expected first resolution to fail")
	}
	if _, err := resolver.resolve(context.Background(), shortcutPath, load); err != nil {
		t.Fatalf("retry shortcut resolution: %v", err)
	}
	if loadCount != 2 {
		t.Fatalf("load count = %d, want 2", loadCount)
	}
}
