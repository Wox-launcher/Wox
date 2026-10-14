package migration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"
	"wox/util"
)

// The process logger keeps its files open. Bind it outside the test temp directory
// before Init, or t.TempDir cannot remove the Wox data directory on Windows.
var screenshotMigrationLoggerOnce sync.Once

func ensureScreenshotMigrationLogger() {
	screenshotMigrationLoggerOnce.Do(func() {
		dir, err := os.MkdirTemp("", "wox-screenshot-migration-log")
		if err != nil {
			panic(err)
		}
		os.Setenv(util.TestWoxDataDirEnv, dir)
		os.Setenv(util.TestUserDataDirEnv, filepath.Join(dir, "user"))
		if err := util.GetLocation().Init(); err != nil {
			panic(err)
		}
		util.GetLogger()
		os.Unsetenv(util.TestWoxDataDirEnv)
		os.Unsetenv(util.TestUserDataDirEnv)
	})
}

func initScreenshotMigrationLocation(t *testing.T) (legacy string, dest string) {
	t.Helper()
	ensureScreenshotMigrationLogger()
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(root, "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	legacy = util.GetLocation().GetLegacyScreenshotDirectory()
	dest = util.GetLocation().GetScreenshotDirectory()
	if !util.IsDirExists(dest) {
		t.Fatal("expected startup to create the user screenshot directory")
	}
	return legacy, dest
}

func writeScreenshotMigrationFile(t *testing.T, path, content string, modified time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if !modified.IsZero() {
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}

func readScreenshotMigrationFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMoveScreenshotsMigrationMovesLegacyDirectory(t *testing.T) {
	legacy, dest := initScreenshotMigrationLocation(t)
	modified := time.UnixMilli(1_700_000_000_000)
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png"), "image", modified)
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png.ocr.json"), "ocr", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "nested", "extra.png"), "extra", time.Time{})

	if err := (&moveScreenshotsMigration{}).Up(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot.png")); got != "image" {
		t.Fatalf("moved image = %q", got)
	}
	info, err := os.Stat(filepath.Join(dest, "shot.png"))
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().UnixMilli() != modified.UnixMilli() {
		t.Fatalf("moved image mtime = %d, want %d", info.ModTime().UnixMilli(), modified.UnixMilli())
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot.png.ocr.json")); got != "ocr" {
		t.Fatalf("moved sidecar = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "nested", "extra.png")); got != "extra" {
		t.Fatalf("moved nested image = %q", got)
	}
	if util.IsDirExists(legacy) {
		t.Fatal("expected the legacy screenshot directory to be removed")
	}

	if err := (&moveScreenshotsMigration{}).Up(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestMoveScreenshotsMigrationMergesAndRenamesCollisions(t *testing.T) {
	legacy, dest := initScreenshotMigrationLocation(t)
	writeScreenshotMigrationFile(t, filepath.Join(dest, "keep.png"), "keep", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(dest, "shot.png"), "current", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "moved.png"), "moved", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "nested", "extra.png"), "extra", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png"), "legacy", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png.ocr.json"), "legacy-ocr", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png.edit.zip"), "legacy-edit", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "same.png"), "same", time.Time{})
	writeScreenshotMigrationFile(t, filepath.Join(dest, "same.png"), "same", time.Time{})

	if err := moveScreenshotDirectory(context.Background(), legacy, dest); err != nil {
		t.Fatal(err)
	}

	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "keep.png")); got != "keep" {
		t.Fatalf("existing file = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "moved.png")); got != "moved" {
		t.Fatalf("merged file = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "nested", "extra.png")); got != "extra" {
		t.Fatalf("merged nested file = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot.png")); got != "current" {
		t.Fatalf("current collision winner = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot_legacy_01.png")); got != "legacy" {
		t.Fatalf("renamed collision = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot_legacy_01.png.ocr.json")); got != "legacy-ocr" {
		t.Fatalf("renamed sidecar = %q", got)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "shot_legacy_01.png.edit.zip")); got != "legacy-edit" {
		t.Fatalf("renamed edit archive = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "shot.png.ocr.json")); !os.IsNotExist(err) {
		t.Fatalf("sidecar attached to the wrong image: %v", err)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(dest, "same.png")); got != "same" {
		t.Fatalf("identical file = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "same_legacy_01.png")); !os.IsNotExist(err) {
		t.Fatalf("identical file was duplicated: %v", err)
	}
	if util.IsDirExists(legacy) {
		t.Fatal("expected the legacy screenshot directory to be removed")
	}
}

func TestMoveScreenshotsMigrationSkipsOverlappingPaths(t *testing.T) {
	initScreenshotMigrationLocation(t)
	root := t.TempDir()
	legacy := filepath.Join(root, "screenshots")
	nested := filepath.Join(legacy, "screenshots")
	writeScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png"), "image", time.Time{})

	if err := moveScreenshotDirectory(context.Background(), legacy, nested); err != nil {
		t.Fatal(err)
	}
	if err := moveScreenshotDirectory(context.Background(), legacy, legacy); err != nil {
		t.Fatal(err)
	}
	if got := readScreenshotMigrationFile(t, filepath.Join(legacy, "shot.png")); got != "image" {
		t.Fatalf("overlapping move changed the file: %q", got)
	}
	if util.IsDirExists(nested) {
		t.Fatal("expected overlapping paths to stay untouched")
	}
}

func TestMoveScreenshotsMigrationIgnoresMissingLegacyDirectory(t *testing.T) {
	legacy, dest := initScreenshotMigrationLocation(t)
	if err := moveScreenshotDirectory(context.Background(), legacy, dest); err != nil {
		t.Fatal(err)
	}
	if !util.IsDirExists(dest) {
		t.Fatal("expected the user screenshot directory to remain")
	}
}

func TestCopyScreenshotFilePreservesModifiedTime(t *testing.T) {
	dir := t.TempDir()
	modified := time.UnixMilli(1_700_000_123_000)
	src := filepath.Join(dir, "shot.png")
	dst := filepath.Join(dir, "out", "shot.png")
	writeScreenshotMigrationFile(t, src, "image", modified)
	info, err := os.Stat(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := copyScreenshotFile(src, dst, info); err != nil {
		t.Fatal(err)
	}
	copied, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if copied.ModTime().UnixMilli() != modified.UnixMilli() {
		t.Fatalf("copied mtime = %d, want %d", copied.ModTime().UnixMilli(), modified.UnixMilli())
	}
	if _, err := os.Stat(src + ".migrating"); !os.IsNotExist(err) {
		t.Fatal("expected the temporary copy to be renamed into place")
	}
}

func TestIsCrossDeviceRename(t *testing.T) {
	if !isCrossDeviceRename(&os.LinkError{Op: "rename", Err: syscall.EXDEV}) {
		t.Fatal("expected EXDEV to be a cross-device rename")
	}
	if runtime.GOOS == "windows" && !isCrossDeviceRename(&os.LinkError{Op: "rename", Err: syscall.Errno(17)}) {
		t.Fatal("expected ERROR_NOT_SAME_DEVICE to be a cross-device rename")
	}
	if isCrossDeviceRename(&os.LinkError{Op: "rename", Err: syscall.EACCES}) {
		t.Fatal("expected permission errors to stay retryable rename failures")
	}
}
