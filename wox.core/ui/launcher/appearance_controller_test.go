package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// appearanceFakeBackend serves the two appearance routes (/setting/ui/fonts and
// /plugin/installed) from pre-populated values, with optional per-route errors and a
// blocking handshake for mid-flight assertions.
type appearanceFakeBackend struct {
	mu      sync.Mutex
	fonts   []string
	plugins []struct {
		ID      string         `json:"Id"`
		Name    string         `json:"Name"`
		Glances []pluginGlance `json:"Glances"`
	}
	fontsErr   error
	pluginsErr error

	pathSel string
	entered chan<- struct{}
	release <-chan struct{}
}

func (f *appearanceFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
	if f.pathSel != "" && path == f.pathSel && f.entered != nil {
		close(f.entered)
		<-f.release
	}
	switch path {
	case "/setting/ui/fonts":
		if f.fontsErr != nil {
			return f.fontsErr
		}
		if ptr, ok := out.(*[]string); ok {
			*ptr = append([]string(nil), f.fonts...)
		}
	case "/plugin/installed":
		if f.pluginsErr != nil {
			return f.pluginsErr
		}
		if ptr, ok := out.(*[]struct {
			ID      string         `json:"Id"`
			Name    string         `json:"Name"`
			Glances []pluginGlance `json:"Glances"`
		}); ok {
			*ptr = append([]struct {
				ID      string         `json:"Id"`
				Name    string         `json:"Name"`
				Glances []pluginGlance `json:"Glances"`
			}(nil), f.plugins...)
		}
	}
	return nil
}

func TestAppearanceControllerReloadFontsSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newAppearanceSettingsController(deps)
	client := &appearanceFakeBackend{fonts: []string{"Helvetica", "Arial"}}
	c.ReloadFonts(context.Background(), client)

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
	client := &appearanceFakeBackend{fontsErr: errors.New("font enumeration failed")}
	c.ReloadFonts(context.Background(), client)

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
	client := &appearanceFakeBackend{fonts: []string{"Helvetica"}}
	c.ReloadFonts(context.Background(), client)

	first := c.Snapshot()
	if !first.FontsLoaded || len(first.FontFamilies) != 1 {
		t.Fatalf("first reload: %+v", first)
	}
	// Second call should be a no-op; even if the fake returns different data, the
	// cached state must remain unchanged.
	client.fonts = []string{"Changed"}
	c.ReloadFonts(context.Background(), client)
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
	client := &appearanceFakeBackend{plugins: []struct {
		ID      string         `json:"Id"`
		Name    string         `json:"Name"`
		Glances []pluginGlance `json:"Glances"`
	}{
		{ID: "p1", Name: "Plugin One", Glances: []pluginGlance{{ID: "g1", Name: "Glance One"}}},
	}}
	c.ReloadGlanceCatalog(context.Background(), client, nil)

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
