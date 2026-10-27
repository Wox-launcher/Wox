package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/common"
	"wox/setting"
	"wox/ui/omarchytheme"
	"wox/util"

	"github.com/fsnotify/fsnotify"
)

func TestReplaceOmarchyAutoThemeUpdatesPaletteOnce(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	hackerman, err := omarchytheme.ParsePalette([]byte(hackermanPaletteFixture))
	if err != nil {
		t.Fatal(err)
	}
	first, err := omarchytheme.ThemeFromPalette(hackerman, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	manager.replaceOmarchyAutoTheme(context.Background(), first, false)
	if !manager.IsSystemTheme(omarchytheme.ThemeID) {
		t.Fatal("Omarchy Auto was not registered as a system theme")
	}
	manager.replaceOmarchyAutoTheme(context.Background(), first, false)
	if len(manager.systemThemeIds) != 1 {
		t.Fatalf("system ids = %v", manager.systemThemeIds)
	}

	light, err := omarchytheme.ParsePalette([]byte(flexokiPaletteFixture))
	if err != nil {
		t.Fatal(err)
	}
	second, err := omarchytheme.ThemeFromPalette(light, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	manager.replaceOmarchyAutoTheme(context.Background(), second, false)
	stored, ok := manager.themes.Load(omarchytheme.ThemeID)
	if !ok || stored.ResultItemActiveIndicatorColor != second.ResultItemActiveIndicatorColor || stored.ThemeName != omarchytheme.ThemeName {
		t.Fatalf("stored = %+v", stored.ThemeName)
	}
	listed := false
	for _, theme := range manager.GetAllThemes(context.Background()) {
		if theme.ThemeId == omarchytheme.ThemeID {
			listed = true
		}
	}
	if listed != omarchytheme.Available() {
		t.Fatalf("listed=%v available=%v", listed, omarchytheme.Available())
	}
}

func TestReplaceOmarchyAutoThemeTracksBorderColor(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	firstPalette, err := omarchytheme.ParsePalette([]byte(hackermanPaletteFixture + "hyprland_active_border = \"rgba(26a269ee) rgba(2ec27eee) 45deg\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := omarchytheme.ThemeFromPalette(firstPalette, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	manager.replaceOmarchyAutoTheme(context.Background(), first, false)

	secondPalette, err := omarchytheme.ParsePalette([]byte(hackermanPaletteFixture + "hyprland_active_border = \"rgb(ff00aa)\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := omarchytheme.ThemeFromPalette(secondPalette, "2.4.8")
	if err != nil {
		t.Fatal(err)
	}
	manager.replaceOmarchyAutoTheme(context.Background(), second, false)
	stored, ok := manager.themes.Load(omarchytheme.ThemeID)
	if !ok || stored.ResolvedColors()["AppBorderColor"] != "#FF00AAFF" {
		t.Fatalf("border = %s", stored.ResolvedColors()["AppBorderColor"])
	}
}

func TestOmarchyAutoThemeHiddenWhenUnavailable(t *testing.T) {
	if showOmarchyAutoTheme(omarchytheme.ThemeID, false) {
		t.Fatal("Omarchy Auto should be hidden when Omarchy is absent")
	}
	if !showOmarchyAutoTheme(omarchytheme.ThemeID, true) || !showOmarchyAutoTheme(setting.DefaultThemeId, false) {
		t.Fatal("Omarchy Auto should stay visible on Omarchy, and other themes stay visible everywhere")
	}
}

func TestOmarchyThemeEventIgnoresChmod(t *testing.T) {
	if omarchyThemeEvent(fsnotify.Event{Op: fsnotify.Chmod}) {
		t.Fatal("chmod should not reload the palette")
	}
	if !omarchyThemeEvent(fsnotify.Event{Op: fsnotify.Write}) {
		t.Fatal("write should reload the palette")
	}
}

func TestOmarchyThemeWatchSeesDirectoryReplace(t *testing.T) {
	dir := t.TempDir()
	themeDir := filepath.Join(dir, "theme")
	if err := os.Mkdir(themeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	events := make(chan fsnotify.Event, 8)
	watch, err := util.WatchDirectory(dir, func(event fsnotify.Event) {
		if omarchyThemeEvent(event) {
			select {
			case events <- event:
			default:
			}
		}
	}, func(watchErr error) {
		t.Errorf("watch: %v", watchErr)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()

	replacement := filepath.Join(dir, "next-theme")
	if err := os.Mkdir(replacement, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacement, "colors.toml"), []byte("background = \"#222222\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(themeDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, themeDir); err != nil {
		t.Fatal(err)
	}

	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Fatal("replacing the theme directory produced no watch event")
	}
}

const hackermanPaletteFixture = `
mode = "dark"
accent = "#82FB9C"
selection = "#1f253a"
background = "#0B0C16"
dark_background = "#080910"
darker_background = "#06060c"
lighter_background = "#151828"
foreground = "#ddf7ff"
dark_foreground = "#6a6e95"
`

const flexokiPaletteFixture = `
mode = "light"
accent = "#205EA6"
selection = "#CECDC3"
background = "#FFFCF0"
dark_background = "#f2efe4"
darker_background = "#e5e2d8"
lighter_background = "#E6E4D9"
foreground = "#100F0F"
dark_foreground = "#878580"
`
