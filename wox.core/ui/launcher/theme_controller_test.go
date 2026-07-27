package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	woxui "wox/ui/runtime"
)

// themeFakeBackend serves the /theme/store and /theme/installed routes from
// pre-populated payloads, with optional per-route errors.
type themeFakeBackend struct {
	mu       sync.Mutex
	themes   map[string][]json.RawMessage
	storeErr error
	instErr  error
}

func (f *themeFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch path {
	case "/theme/store":
		if f.storeErr != nil {
			return f.storeErr
		}
		if ptr, ok := out.(*[]json.RawMessage); ok {
			*ptr = append([]json.RawMessage(nil), f.themes["store"]...)
		}
	case "/theme/installed":
		if f.instErr != nil {
			return f.instErr
		}
		if ptr, ok := out.(*[]json.RawMessage); ok {
			*ptr = append([]json.RawMessage(nil), f.themes["installed"]...)
		}
	}
	return nil
}

func newThemeControllerDeps() (CommonDeps, *int) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &invalidateCalled
}

func TestThemeControllerThemes(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	c.SetThemes([]themeSettingsTheme{{ID: "a"}, {ID: "b"}})
	got := c.Themes()
	if len(got) != 2 {
		t.Fatalf("Themes len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("Themes = %+v, want [a, b]", got)
	}
}

func TestThemeControllerSelected(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	c.SetThemeSelected(3)
	if got := c.ThemeSelected(); got != 3 {
		t.Fatalf("ThemeSelected = %d, want 3", got)
	}
}

func TestThemeControllerSearchEditor(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	editor := woxui.NewTextEditor("query")
	c.SetThemeSearchEditor(editor)
	if got := c.ThemeSearchEditor(); got != editor {
		t.Fatalf("ThemeSearchEditor mismatch: got %v, want %v", got, editor)
	}
}

func TestThemeControllerWallpaper(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	c.SetThemeWallpaperPath("/path/to/wallpaper.jpg")
	if got := c.ThemeWallpaperPath(); got != "/path/to/wallpaper.jpg" {
		t.Fatalf("ThemeWallpaperPath = %q, want /path/to/wallpaper.jpg", got)
	}
}

func TestThemeControllerReloadThemesSuccess(t *testing.T) {
	deps, invalidateCalled := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	payloads := []json.RawMessage{
		json.RawMessage(`{"ThemeId":"t1","ThemeName":"Theme One","AppBackgroundColor":"#000000"}`),
		json.RawMessage(`{"ThemeId":"t2","ThemeName":"Theme Two","AppBackgroundColor":"#111111"}`),
	}
	client := &themeFakeBackend{themes: map[string][]json.RawMessage{"store": payloads}}
	err := c.ReloadThemes(context.Background(), client, "store", "", "")
	if err != nil {
		t.Fatalf("ReloadThemes error: %v", err)
	}
	snap := c.Snapshot()
	if !snap.ThemesLoaded {
		t.Fatalf("ThemesLoaded should be true after successful reload")
	}
	if snap.ThemesError != "" {
		t.Fatalf("ThemesError should be empty, got %q", snap.ThemesError)
	}
	if snap.ThemesLoading {
		t.Fatalf("ThemesLoading should be false after reload completes")
	}
	if len(snap.Themes) != 2 {
		t.Fatalf("Themes len = %d, want 2", len(snap.Themes))
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestThemeControllerReloadThemesError(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	client := &themeFakeBackend{storeErr: errors.New("network failed")}
	err := c.ReloadThemes(context.Background(), client, "store", "", "")
	if err == nil {
		t.Fatalf("ReloadThemes should return error on network failure")
	}
	snap := c.Snapshot()
	if snap.ThemesLoaded {
		t.Fatalf("ThemesLoaded should be false on error")
	}
	if snap.ThemesError == "" {
		t.Fatalf("ThemesError should be recorded, got empty")
	}
	if snap.ThemesLoading {
		t.Fatalf("ThemesLoading should be false after error")
	}
}

func TestThemeControllerThemesMode(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	c.SetThemesMode("store")
	if got := c.ThemesMode(); got != "store" {
		t.Fatalf("ThemesMode = %q, want store", got)
	}
}
