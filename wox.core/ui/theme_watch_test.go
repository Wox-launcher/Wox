package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/common"
	"wox/util"
)

const testThemeJSON = `{"SchemaVersion":2,"ThemeId":"theme-watch-new","ThemeName":"Watch New","BaseBackgroundColor":"#FCEEE0","BaseTextColor":"#2A241C","BaseAccentColor":"#81C7C7"}`

func TestShouldIgnoreThemeWatchName(t *testing.T) {
	if !shouldIgnoreThemeWatchName(".hidden.json") || !shouldIgnoreThemeWatchName("theme.json~") || !shouldIgnoreThemeWatchName("theme.json.tmp") || !shouldIgnoreThemeWatchName("readme.txt") {
		t.Fatal("temp and non-json names should be ignored")
	}
	if shouldIgnoreThemeWatchName("8f8591d8-28fd-476f-aae8-7c7099641777.json") {
		t.Fatal("user theme json should be watched")
	}
}

func TestUpsertUserThemeFromFileAddsNewTheme(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	path := filepath.Join(t.TempDir(), "theme-watch-new.json")
	if err := os.WriteFile(path, []byte(testThemeJSON), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := manager.upsertUserThemeFromFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if theme.ThemeId != "theme-watch-new" || !theme.IsInstalled || theme.IsSystem {
		t.Fatalf("loaded %+v", theme)
	}
	stored, ok := manager.themes.Load("theme-watch-new")
	if !ok || stored.ThemeName != "Watch New" {
		t.Fatal("new theme was not stored")
	}
}

func TestUpsertUserThemeFromFileUpdatesExisting(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	path := filepath.Join(t.TempDir(), "theme-watch-new.json")
	if err := os.WriteFile(path, []byte(testThemeJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.upsertUserThemeFromFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	updated := `{"SchemaVersion":2,"ThemeId":"theme-watch-new","ThemeName":"Watch Updated","BaseBackgroundColor":"#FCEEE0","BaseTextColor":"#2A241C","BaseAccentColor":"#81C7C7"}`
	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		t.Fatal(err)
	}
	theme, err := manager.upsertUserThemeFromFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if theme.ThemeName != "Watch Updated" {
		t.Fatalf("name=%s", theme.ThemeName)
	}
}

func TestUpsertUserThemeFromFileSkipsSystemTheme(t *testing.T) {
	manager := &Manager{
		themes:         util.NewHashMap[string, common.Theme](),
		systemThemeIds: []string{"theme-watch-new"},
	}
	manager.themes.Store("theme-watch-new", common.Theme{ThemeId: "theme-watch-new", ThemeName: "System", IsSystem: true})
	path := filepath.Join(t.TempDir(), "theme-watch-new.json")
	if err := os.WriteFile(path, []byte(testThemeJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.upsertUserThemeFromFile(context.Background(), path); err == nil {
		t.Fatal("system theme overwrite should fail")
	}
	stored, _ := manager.themes.Load("theme-watch-new")
	if stored.ThemeName != "System" {
		t.Fatalf("system theme was replaced with %s", stored.ThemeName)
	}
}

func TestRemoveUserThemeByPath(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	path := filepath.Join(t.TempDir(), "theme-watch-new.json")
	if err := os.WriteFile(path, []byte(testThemeJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.upsertUserThemeFromFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	removedID, ok := manager.removeUserThemeByPath(path)
	if !ok || removedID != "theme-watch-new" {
		t.Fatalf("removed=%s ok=%v", removedID, ok)
	}
	if _, exists := manager.themes.Load("theme-watch-new"); exists {
		t.Fatal("theme remained after file removal")
	}
}

func TestIgnoreThemeWatchExpires(t *testing.T) {
	manager := &Manager{}
	path := filepath.Join(t.TempDir(), "theme.json")
	manager.IgnoreThemeWatch(path)
	if !manager.isThemeWatchIgnored(path) {
		t.Fatal("managed write should ignore watch events")
	}
	manager.themeWatchIgnored.Store(themeWatchKey(path), time.Now().Add(-time.Millisecond).UnixMilli())
	if manager.isThemeWatchIgnored(path) {
		t.Fatal("expired ignore window should not suppress reload")
	}
}
