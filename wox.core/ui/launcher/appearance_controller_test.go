package launcher

import (
	"context"
	"errors"
	"testing"

	"wox/ui/contract"
)

type appearanceFakeService struct {
	fonts   []string
	catalog []contract.GlanceCatalogItem

	fontsErr   error
	catalogErr error
}

func (f *appearanceFakeService) SystemFontFamilies(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), f.fonts...), f.fontsErr
}

func (f *appearanceFakeService) GlanceCatalog(_ context.Context, _ string) ([]contract.GlanceCatalogItem, error) {
	return append([]contract.GlanceCatalogItem(nil), f.catalog...), f.catalogErr
}

func TestAppearanceControllerReloadFontsSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newAppearanceSettingsController(deps)
	service := &appearanceFakeService{fonts: []string{"Helvetica", "Arial"}}
	c.ReloadFonts(context.Background(), service, "session")

	snap := c.Snapshot()
	if len(snap.FontFamilies) != 2 {
		t.Fatalf("FontFamilies len = %d, want 2", len(snap.FontFamilies))
	}
	if snap.FontsLoading {
		t.Fatalf("FontsLoading should be false after reload")
	}
	if !snap.FontsLoaded {
		t.Fatalf("FontsLoaded should be true after successful reload")
	}
	if snap.FontsError != "" {
		t.Fatalf("FontsError should be empty, got %q", snap.FontsError)
	}
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestAppearanceControllerReloadFontsError(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newAppearanceSettingsController(deps)
	service := &appearanceFakeService{fontsErr: errors.New("font enumeration failed")}
	c.ReloadFonts(context.Background(), service, "session")

	snap := c.Snapshot()
	if snap.FontsError == "" {
		t.Fatalf("FontsError should be recorded, got empty")
	}
	if snap.FontsLoaded {
		t.Fatalf("FontsLoaded should be false on error")
	}
	if snap.FontsLoading {
		t.Fatalf("FontsLoading should be false after reload completes")
	}
}

func TestAppearanceControllerReloadFontsSkipsWhenLoaded(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newAppearanceSettingsController(deps)
	service := &appearanceFakeService{fonts: []string{"Helvetica"}}
	c.ReloadFonts(context.Background(), service, "session")

	first := c.Snapshot()
	if !first.FontsLoaded || len(first.FontFamilies) != 1 {
		t.Fatalf("first reload: %+v", first)
	}
	// Second call should be a no-op; even if the fake returns different data, the
	// cached state must remain unchanged.
	service.fonts = []string{"Changed"}
	c.ReloadFonts(context.Background(), service, "session")
	second := c.Snapshot()
	if !second.FontsLoaded {
		t.Fatalf("FontsLoaded should remain true")
	}
	if len(second.FontFamilies) != 1 || second.FontFamilies[0] != "Helvetica" {
		t.Fatalf("FontFamilies should stay cached, got %+v", second.FontFamilies)
	}
}

func TestAppearanceControllerResetGlanceCatalog(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newAppearanceSettingsController(deps)
	service := &appearanceFakeService{catalog: []contract.GlanceCatalogItem{
		{PluginID: "p1", PluginName: "Plugin One", GlanceID: "g1", Name: "Glance One"},
	}}
	c.ReloadGlanceCatalog(context.Background(), service, "session", nil)

	loaded := c.Snapshot()
	if !loaded.GlanceCatalogLoaded || len(loaded.GlanceCatalog) != 1 {
		t.Fatalf("glance catalog should be loaded: %+v", loaded)
	}

	c.ResetGlanceCatalog()
	reset := c.Snapshot()
	if reset.GlanceCatalogLoaded {
		t.Fatalf("GlanceCatalogLoaded should be false after reset")
	}
	if reset.GlanceCatalog != nil {
		t.Fatalf("GlanceCatalog should be nil after reset, got %+v", reset.GlanceCatalog)
	}
	if reset.GlanceCatalogError != "" {
		t.Fatalf("GlanceCatalogError should be cleared, got %q", reset.GlanceCatalogError)
	}
}
