package launcher

import (
	"context"
	"sync"
	"testing"

	woxui "wox/ui/runtime"
)

// pluginFakeBackend serves /plugin/store and /plugin/installed from pre-populated
// payloads, with optional per-route errors. Matches the core endpoint signature
// which decodes directly into []pluginSettingsPlugin (not json.RawMessage).
type pluginFakeBackend struct {
	mu       sync.Mutex
	plugins  map[string][]pluginSettingsPlugin
	storeErr error
	instErr  error
}

func (f *pluginFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch path {
	case "/plugin/store":
		if f.storeErr != nil {
			return f.storeErr
		}
		if ptr, ok := out.(*[]pluginSettingsPlugin); ok {
			*ptr = append([]pluginSettingsPlugin(nil), f.plugins["store"]...)
		}
	case "/plugin/installed":
		if f.instErr != nil {
			return f.instErr
		}
		if ptr, ok := out.(*[]pluginSettingsPlugin); ok {
			*ptr = append([]pluginSettingsPlugin(nil), f.plugins["installed"]...)
		}
	}
	return nil
}

func newPluginControllerDeps() (CommonDeps, *int) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &invalidateCalled
}

func TestPluginControllerPlugins(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetPlugins([]pluginSettingsPlugin{{ID: "a"}, {ID: "b"}})
	got := c.Plugins()
	if len(got) != 2 {
		t.Fatalf("Plugins len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("Plugins = %+v, want [a, b]", got)
	}
}

func TestPluginControllerSelected(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetSelected(3)
	if got := c.Selected(); got != 3 {
		t.Fatalf("Selected = %d, want 3", got)
	}
}

func TestPluginControllerSearchEditor(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	editor := woxui.NewTextEditor("query")
	c.SetSearchEditor(editor)
	if got := c.SearchEditor(); got != editor {
		t.Fatalf("SearchEditor mismatch: got %v, want %v", got, editor)
	}
}

func TestPluginControllerForm(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	form := &pluginSettingsFormState{pluginID: "p1"}
	c.SetForm(form)
	if got := c.Form(); got != form {
		t.Fatalf("Form mismatch: got %v, want %v", got, form)
	}
}

func TestPluginControllerReloadPluginsSuccess(t *testing.T) {
	deps, invalidateCalled := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	plugins := []pluginSettingsPlugin{{ID: "t1", Name: "Plugin One"}, {ID: "t2", Name: "Plugin Two"}}
	client := &pluginFakeBackend{plugins: map[string][]pluginSettingsPlugin{"installed": plugins}}
	if err := c.ReloadPlugins(context.Background(), client, false, ""); err != nil {
		t.Fatalf("ReloadPlugins error: %v", err)
	}
	if !c.PluginsLoaded() {
		t.Fatalf("PluginsLoaded should be true after successful reload")
	}
	if c.PluginsError() != "" {
		t.Fatalf("PluginsError should be empty, got %q", c.PluginsError())
	}
	if c.PluginsLoading() {
		t.Fatalf("PluginsLoading should be false after reload completes")
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestPluginControllerPluginsStore(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetPluginsStore(true)
	if got := c.PluginsStore(); got != true {
		t.Fatalf("PluginsStore = %v, want true", got)
	}
}
