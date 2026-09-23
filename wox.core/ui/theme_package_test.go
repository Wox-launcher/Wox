package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
)

// TestPersistThemePackage preserves an existing installation on failed validation and reloads resources.
func TestPersistThemePackage(t *testing.T) {
	var theme common.Theme
	err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"package-test","ThemeName":"Package","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ff0000","Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}}`), &theme)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	theme.AssetFiles = map[string][]byte{"assets/bg.png": data.Bytes()}
	directory := t.TempDir()
	if err := persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(directory, theme.ThemeId, "theme.json")
	before, _ := os.ReadFile(manifest)
	bad := theme
	bad.AssetFiles = map[string][]byte{"../escape.png": data.Bytes()}
	if persistThemePackage(directory, bad) == nil {
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
	if err := persistThemePackage(directory, theme); err != nil {
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
	if persistThemePackage(directory, other) == nil {
		t.Fatal("replaced an unrelated directory")
	}
}

// TestStoredThemeReloadsAssetsOnDemand keeps packaged bytes out of the long-lived theme map.
func TestStoredThemeReloadsAssetsOnDemand(t *testing.T) {
	var theme common.Theme
	err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"lazy-assets","ThemeName":"Lazy","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ff0000","Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}}`), &theme)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	theme.AssetFiles = map[string][]byte{"assets/bg.png": data.Bytes()}
	directory := t.TempDir()
	if err := persistThemePackage(directory, theme); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{}
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
