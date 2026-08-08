package launcher

import (
	"context"
	"errors"
	"testing"

	"wox/common"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type themeFakeService struct {
	themes   map[contract.ThemeCatalog][]contract.ThemeCatalogItem
	storeErr error
	instErr  error
}

func (f *themeFakeService) Themes(_ context.Context, _ string, catalog contract.ThemeCatalog) ([]contract.ThemeCatalogItem, error) {
	switch catalog {
	case contract.ThemeCatalogStore:
		if f.storeErr != nil {
			return nil, f.storeErr
		}
	case contract.ThemeCatalogInstalled:
		if f.instErr != nil {
			return nil, f.instErr
		}
	}
	return append([]contract.ThemeCatalogItem(nil), f.themes[catalog]...), nil
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
	items := []contract.ThemeCatalogItem{
		{Theme: common.Theme{ThemeId: "t1", ThemeName: "Theme One", AppBackgroundColor: "#000000", ResultItemActiveSubTitleColor: "#20202A"}},
		{Theme: common.Theme{ThemeId: "t2", ThemeName: "Theme Two", AppBackgroundColor: "#111111"}},
	}
	service := &themeFakeService{themes: map[contract.ThemeCatalog][]contract.ThemeCatalogItem{contract.ThemeCatalogStore: items}}
	err := c.ReloadThemes(context.Background(), service, "session", "store", "", "")
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
	if got := themeCatalogItem(snap.Themes[0], 0, settingsSnapshot{}).PreviewTheme.SelectedSubtitle; got != (woxui.Color{R: 32, G: 32, B: 42, A: 255}) {
		t.Fatalf("preview selected subtitle = %#v, want actual theme color", got)
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestThemeControllerRetainsAutoAppearanceVariantIDs(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	service := &themeFakeService{themes: map[contract.ThemeCatalog][]contract.ThemeCatalogItem{
		contract.ThemeCatalogInstalled: {{Theme: common.Theme{ThemeId: "auto", IsAutoAppearance: true, LightThemeId: "light", DarkThemeId: "dark"}}},
	}}

	if err := c.ReloadThemes(context.Background(), service, "session", "installed", "", ""); err != nil {
		t.Fatalf("ReloadThemes error: %v", err)
	}
	got := c.Snapshot().Themes[0]
	if got.LightThemeID != "light" || got.DarkThemeID != "dark" {
		t.Fatalf("AUTO variant IDs = %q/%q, want light/dark", got.LightThemeID, got.DarkThemeID)
	}
}

func TestThemeControllerReloadThemesError(t *testing.T) {
	deps, _ := newThemeControllerDeps()
	c := newThemeSettingsController(deps)
	service := &themeFakeService{storeErr: errors.New("network failed")}
	err := c.ReloadThemes(context.Background(), service, "session", "store", "", "")
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
