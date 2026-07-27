package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"

	woxui "wox/ui/runtime"
)

// searchFakeBackend serves /plugin/installed from a pre-populated payload, with an
// optional error. Matches the core endpoint signature that decodes directly into
// []pluginSettingsPlugin (not json.RawMessage).
type searchFakeBackend struct {
	mu           sync.Mutex
	plugins      []pluginSettingsPlugin
	installedErr error
}

func (f *searchFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch path {
	case "/plugin/installed":
		if f.installedErr != nil {
			return f.installedErr
		}
		if ptr, ok := out.(*[]pluginSettingsPlugin); ok {
			*ptr = append([]pluginSettingsPlugin(nil), f.plugins...)
		}
	}
	return nil
}

// mockSearchable is a minimal Searchable source for Run-aggregation tests.
type mockSearchable struct {
	results []searchResult
}

func (m mockSearchable) Search(_ string) []searchResult {
	return append([]searchResult(nil), m.results...)
}

func newSearchControllerDeps() (CommonDeps, *int) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &invalidateCalled
}

func TestSettingsSearchRunAggregates(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	sources := []Searchable{
		mockSearchable{results: []searchResult{{ID: "a", Title: "A", Tab: "general", Row: 0}}},
		mockSearchable{results: []searchResult{{ID: "b", Title: "B", Tab: "ai", Row: 1}}},
	}
	got := c.Run("hot", sources)
	if len(got) != 2 {
		t.Fatalf("Run len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("Run = %+v, want [a, b]", got)
	}
}

func TestSettingsSearchRunEmptyQueryReturnsNil(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	got := c.Run("   ", []Searchable{mockSearchable{results: []searchResult{{ID: "x"}}}})
	if got != nil {
		t.Fatalf("Run with blank query should return nil, got %+v", got)
	}
}

func TestSettingsSearchEnterExit(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	c.SetEditor(woxui.NewTextEditor("hotkey"))
	c.SetFocused(true)
	c.SetPanel(true)

	if !c.Panel() {
		t.Fatal("Panel should be true after enter")
	}
	if !c.Focused() {
		t.Fatal("Focused should be true after enter")
	}
	editor := c.Editor()
	if editor == nil || editor.State().Text != "hotkey" {
		got := ""
		if editor != nil {
			got = editor.State().Text
		}
		t.Fatalf("Editor text = %q, want hotkey", got)
	}

	c.SetFocused(false)
	c.SetPanel(false)
	if c.Panel() {
		t.Fatal("Panel should be false after exit")
	}
	if c.Focused() {
		t.Fatal("Focused should be false after exit")
	}
}

func TestSettingsSearchReloadPluginsSuccess(t *testing.T) {
	deps, invalidateCalled := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	plugins := []pluginSettingsPlugin{
		{ID: "p1", Name: "Plugin One"},
		{ID: "p2", Name: "Plugin Two"},
	}
	client := &searchFakeBackend{plugins: plugins}
	if err := c.ReloadPlugins(context.Background(), client); err != nil {
		t.Fatalf("ReloadPlugins error: %v", err)
	}
	if !c.Loaded() {
		t.Fatal("Loaded should be true after successful reload")
	}
	if c.Error() != "" {
		t.Fatalf("Error should be empty, got %q", c.Error())
	}
	if c.Loading() {
		t.Fatal("Loading should be false after reload completes")
	}
	got := c.Plugins()
	if len(got) != 2 {
		t.Fatalf("Plugins len = %d, want 2", len(got))
	}
	if got[0].ID != "p1" || got[1].ID != "p2" {
		t.Fatalf("Plugins = %+v, want [p1, p2]", got)
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestSettingsSearchReloadPluginsError(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	client := &searchFakeBackend{installedErr: errors.New("network failed")}
	if err := c.ReloadPlugins(context.Background(), client); err == nil {
		t.Fatal("ReloadPlugins should return error on network failure")
	}
	if c.Loaded() {
		t.Fatal("Loaded should be false on error")
	}
	if c.Error() == "" {
		t.Fatal("Error should be recorded, got empty")
	}
	if c.Loading() {
		t.Fatal("Loading should be false after error")
	}
}

func TestSettingsSearchReloadPluginsSkipsWhenLoaded(t *testing.T) {
	deps, invalidateCalled := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	c.SetLoaded(true)
	c.SetPlugins([]pluginSettingsPlugin{{ID: "existing"}})
	before := *invalidateCalled
	client := &searchFakeBackend{plugins: []pluginSettingsPlugin{{ID: "new"}}}
	if err := c.ReloadPlugins(context.Background(), client); err != nil {
		t.Fatalf("ReloadPlugins error: %v", err)
	}
	if c.Plugins()[0].ID != "existing" {
		t.Fatalf("ReloadPlugins should be a no-op when already loaded, got %q", c.Plugins()[0].ID)
	}
	if *invalidateCalled != before {
		t.Fatalf("Invalidate should not be called when already loaded, got %d", *invalidateCalled-before)
	}
}

func TestSettingsSearchSelected(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	c.SetSelected(3)
	if got := c.Selected(); got != 3 {
		t.Fatalf("Selected = %d, want 3", got)
	}
}

func TestSettingsSearchSetError(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	c.SetError("oops")
	if got := c.Error(); got != "oops" {
		t.Fatalf("Error = %q, want oops", got)
	}
}

func TestSettingsSearchSnapshotCopiesPlugins(t *testing.T) {
	deps, _ := newSearchControllerDeps()
	c := newSettingsSearchController(deps)
	c.SetPlugins([]pluginSettingsPlugin{{ID: "a"}, {ID: "b"}})
	snap := c.Snapshot()
	if len(snap.Plugins) != 2 {
		t.Fatalf("Snapshot Plugins len = %d, want 2", len(snap.Plugins))
	}
	c.SetPlugins([]pluginSettingsPlugin{{ID: "c"}})
	if snap.Plugins[0].ID != "a" {
		t.Fatalf("Snapshot should be a copy, got %q after mutation", snap.Plugins[0].ID)
	}
}
