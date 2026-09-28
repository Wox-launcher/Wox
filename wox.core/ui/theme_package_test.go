package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"wox/common"
)

// TestPersistThemePackage preserves an existing installation on failed validation and reloads resources.
func TestPersistThemePackage(t *testing.T) {
	manager := &Manager{}
	var theme common.Theme
	err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"package-test","ThemeName":"Package","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ff0000","Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}}`), &theme)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	theme.AssetFiles = map[string][]byte{"assets/bg.png": data.Bytes()}
	directory := t.TempDir()
	if err := manager.persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, theme.ThemeId, "theme.json")
	before, _ := os.ReadFile(manifest)
	bad := theme
	bad.AssetFiles = map[string][]byte{"../escape.png": data.Bytes()}
	if manager.persistThemePackage(directory, bad) == nil {
		t.Fatal("accepted traversal")
	}
	after, _ := os.ReadFile(manifest)
	if !bytes.Equal(before, after) {
		t.Fatal("failed update modified installed theme")
	}
	var loaded common.Theme
	if err := json.Unmarshal(after, &loaded); err != nil {
		t.Fatal(err)
	}
	if err := loaded.LoadThemeAssets(filepath.Dir(manifest)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded.AssetFiles["assets/bg.png"], data.Bytes()) {
		t.Fatal("reload lost image")
	}
	theme.ThemeName = "Updated"
	if err := manager.persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(directory)
	if len(entries) != 1 {
		t.Fatal("staging files leaked")
	}
	other := theme
	other.ThemeId = "shared-assets"
	if err := os.Mkdir(filepath.Join(directory, other.ThemeId), 0755); err != nil {
		t.Fatal(err)
	}
	if manager.persistThemePackage(directory, other) == nil {
		t.Fatal("replaced an unrelated directory")
	}
}

// TestPersistThemePackageWithMonitoring covers Windows rename locks and watch restoration on errors.
func TestPersistThemePackageWithMonitoring(t *testing.T) {
	manager := &Manager{}
	var theme common.Theme
	if err := json.Unmarshal([]byte(testThemeJSON), &theme); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	theme.AssetFiles = map[string][]byte{"assets/bg.png": data.Bytes()}
	directory := t.TempDir()
	if err := manager.persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, theme.ThemeId, "theme.json")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		manager.startUserThemeMonitoring(ctx, directory)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
		manager.themeReloadTimers.Range(func(key string, timer *time.Timer) bool {
			timer.Stop()
			return true
		})
	})
	deadline := time.Now().Add(3 * time.Second)
	for {
		manager.themeWatchMu.Lock()
		ready := manager.themeWatchSuspend != nil
		manager.themeWatchMu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("theme monitoring did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	assertAssetsWatched := func() {
		t.Helper()
		if runtime.GOOS == "windows" {
			// A child watch blocks this rename; the package's own watch alone does not.
			target := filepath.Dir(manifest)
			probe := filepath.Join(directory, ".watch-probe")
			if err := os.Rename(target, probe); err == nil {
				if restoreErr := os.Rename(probe, target); restoreErr != nil {
					t.Fatal(restoreErr)
				}
				t.Fatal("package assets directory is no longer watched")
			} else if !os.IsPermission(err) {
				t.Fatal(err)
			}
		}
		key := themeWatchKey(manifest)
		// Observe a reload being scheduled without applying a theme to the global UI manager.
		manager.themeWatchIgnored.Store(key, time.Now().Add(time.Minute).UnixMilli())
		manager.themeReloadTimers.Delete(key)
		if err := os.WriteFile(filepath.Join(directory, theme.ThemeId, "assets", "bg.png"), data.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if _, ok := manager.themeReloadTimers.Load(key); ok {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("asset edits no longer schedule a theme reload")
	}
	assertAssetsWatched()
	theme.ThemeName = "Updated while watched"
	if err := manager.persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}
	assertAssetsWatched()
	if err := os.WriteFile(manifest, []byte(`{"ThemeId":"another-resource"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := manager.persistThemePackage(directory, theme); err == nil {
		t.Fatal("replaced a directory belonging to another resource")
	}
	assertAssetsWatched()
}

// TestStoredThemeReloadsAssetsOnDemand keeps packaged bytes out of the long-lived theme map.
func TestStoredThemeReloadsAssetsOnDemand(t *testing.T) {
	manager := &Manager{}
	var theme common.Theme
	err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"lazy-assets","ThemeName":"Lazy","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ff0000","Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}}`), &theme)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	theme.AssetFiles = map[string][]byte{"assets/bg.png": data.Bytes()}
	directory := t.TempDir()
	if err := manager.persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}

	manager.ensureThemeWatchMaps()
	loaded, err := manager.upsertUserThemeFromFile(context.Background(), filepath.Join(directory, theme.ThemeId, "theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.AssetFiles) == 0 {
		t.Fatal("upsert must validate the package with its assets loaded")
	}
	stored, ok := manager.themes.Load(theme.ThemeId)
	if !ok || stored.AssetFiles != nil {
		t.Fatalf("stored theme keeps %d asset files, want none", len(stored.AssetFiles))
	}
	withAssets := manager.ThemeWithAssets(context.Background(), stored)
	if !bytes.Equal(withAssets.AssetFiles["assets/bg.png"], data.Bytes()) {
		t.Fatal("ThemeWithAssets did not reload the package image")
	}
	if again, _ := manager.themes.Load(theme.ThemeId); again.AssetFiles != nil {
		t.Fatal("on-demand load must not write bytes back into the stored theme")
	}
}
