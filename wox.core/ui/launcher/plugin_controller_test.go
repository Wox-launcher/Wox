package launcher

import (
	"context"
	"testing"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type pluginFakeService struct {
	plugins  map[contract.PluginCatalog][]contract.PluginCatalogItem
	storeErr error
	instErr  error
}

func (f *pluginFakeService) Plugins(_ context.Context, _ string, catalog contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	switch catalog {
	case contract.PluginCatalogStore:
		if f.storeErr != nil {
			return nil, f.storeErr
		}
	case contract.PluginCatalogInstalled:
		if f.instErr != nil {
			return nil, f.instErr
		}
	}
	return append([]contract.PluginCatalogItem(nil), f.plugins[catalog]...), nil
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
	plugins := []contract.PluginCatalogItem{{ID: "t1", Name: "Plugin One"}, {ID: "t2", Name: "Plugin Two"}}
	service := &pluginFakeService{plugins: map[contract.PluginCatalog][]contract.PluginCatalogItem{contract.PluginCatalogInstalled: plugins}}
	if err := c.ReloadPlugins(context.Background(), service, "session", false, ""); err != nil {
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
