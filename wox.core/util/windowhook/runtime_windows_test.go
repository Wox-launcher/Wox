//go:build windows

package windowhook

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"golang.org/x/sys/windows"
)

// TestRuntimeCleanup uses a real mapped DLL to exercise the Windows image lock
// as well as the directory lease held by another Wox instance.
func TestRuntimeCleanup(t *testing.T) {
	dataRoot := t.TempDir()
	root := filepath.Join(dataRoot, "runtime", runtimeName)
	legacyRoot := filepath.Join(dataRoot, "others")
	systemDir, err := windows.GetSystemDirectory()
	if err != nil {
		t.Fatal(err)
	}
	dllData, err := os.ReadFile(filepath.Join(systemDir, "version.dll"))
	if err != nil {
		t.Fatal(err)
	}
	assetRoot := "others/" + runtimeName + "/"
	assets := fstest.MapFS{
		assetRoot + hookDLL64:    {Data: dllData},
		assetRoot + hookDLL32:    {Data: []byte("32-bit DLL")},
		assetRoot + hookHelper32: {Data: []byte("old helper")},
	}
	old, err := installRuntime(root, assets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if old.lease != 0 {
			_ = windows.CloseHandle(old.lease)
		}
	})
	loadedDLL, err := windows.LoadDLL(filepath.Join(old.directory, hookDLL64))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if loadedDLL != nil {
			_ = loadedDLL.Release()
		}
	})

	reused, err := installRuntime(root, assets)
	if err != nil {
		t.Fatalf("reuse mapped runtime: %v", err)
	}
	if err := windows.CloseHandle(reused.lease); err != nil {
		t.Fatal(err)
	}
	if reused.directory != old.directory {
		t.Fatalf("identical assets used different directories: %q and %q", old.directory, reused.directory)
	}

	assets[assetRoot+hookHelper32] = &fstest.MapFile{Data: []byte("new helper")}
	current, err := installRuntime(root, assets)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(current.lease) })
	if current.directory == old.directory {
		t.Fatal("changing the helper must select a new bundle")
	}
	legacy := filepath.Join(legacyRoot, runtimeName+".stale-134332")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	legacyDLLPath := filepath.Join(legacy, hookDLL64)
	if err := os.WriteFile(legacyDLLPath, dllData, 0644); err != nil {
		t.Fatal(err)
	}
	legacyDLL, err := windows.LoadDLL(legacyDLLPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if legacyDLL != nil {
			_ = legacyDLL.Release()
		}
	})

	if failures := cleanupRuntime(root, legacyRoot, current.directory); len(failures) != 2 ||
		!errors.Is(failures[0], windows.ERROR_SHARING_VIOLATION) || !errors.Is(failures[1], windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("expected the active lease and mapped legacy DLL to defer removal, got %v", failures)
	}
	if _, err := os.Stat(filepath.Join(old.directory, hookHelper32)); err != nil {
		t.Fatalf("cleanup removed part of an active runtime: %v", err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy runtime was not moved outside others: %v", err)
	}

	if err := windows.CloseHandle(old.lease); err != nil {
		t.Fatal(err)
	}
	old.lease = 0
	if failures := cleanupRuntime(root, legacyRoot, current.directory); len(failures) != 2 ||
		!errors.Is(failures[0], windows.ERROR_ACCESS_DENIED) || !errors.Is(failures[1], windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("expected both mapped DLLs to defer removal, got %v", failures)
	}
	if _, err := os.Stat(old.directory); !os.IsNotExist(err) {
		t.Fatalf("obsolete hash directory was not retired: %v", err)
	}
	if pending, err := filepath.Glob(filepath.Join(root, retiredPrefix+"*")); err != nil || len(pending) != 2 {
		t.Fatalf("expected two retired directories to retry, got %v, %v", pending, err)
	}

	if err := loadedDLL.Release(); err != nil {
		t.Fatal(err)
	}
	loadedDLL = nil
	if err := legacyDLL.Release(); err != nil {
		t.Fatal(err)
	}
	legacyDLL = nil
	if failures := cleanupRuntime(root, legacyRoot, current.directory); len(failures) != 0 {
		t.Fatalf("cleanup after unload: %v", failures)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(current.directory) {
		t.Fatalf("expected only the current bundle, got %v, %v", entries, err)
	}
}
