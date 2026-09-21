package ui

import (
	"bytes"
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
