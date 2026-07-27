package launcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	woxui "wox/ui/runtime"
)

// settingsSearchSnapshot is the immutable Settings-search state consumed by the view layer.
// Mirrors the flat fields that previously lived on settingsSnapshot (searchQuery, searchFocused,
// searchPanel, searchSelected, searchPlugins, searchLoading, searchError). searchLoaded is
// intentionally excluded: it is only read by the load guard in ReloadPlugins (via Loaded()),
// never by the view.
type settingsSearchSnapshot struct {
	Query    woxui.TextEditingState
	Focused  bool
	Panel    bool
	Selected int
	Plugins  []pluginSettingsPlugin
	Loading  bool
	Error    string
}

// settingsSearchController owns the Settings-search panel state: the floating search editor,
// focus/panel flags, selection within the result palette, the installed-plugin mirror used
// as the search index, and loading/loaded/error flags for the background plugin fetch.
//
// All 8 fields that used to live on App are held here. App methods became thin wrappers that
// call the controller's getters/setters while still coordinating cross-domain focus routing
// under a.mu before delegating. The rich settingsSearchResults aggregation stays on App
// because it reads from every settings controller snapshot; Run() below provides the narrow
// Searchable-aggregation path exercised by tests and by the Searchable contract.
type settingsSearchController struct {
	deps CommonDeps
	mu   sync.RWMutex

	editor   *woxui.TextEditor
	focused  bool
	panel    bool
	selected int
	plugins  []pluginSettingsPlugin
	loading  bool
	loaded   bool
	errMsg   string
}

func newSettingsSearchController(deps CommonDeps) *settingsSearchController {
	return &settingsSearchController{deps: deps}
}

// Editor returns the floating search editor. May be nil before the settings window is opened.
func (c *settingsSearchController) Editor() *woxui.TextEditor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.editor
}

// SetEditor installs the floating search editor. Passing nil clears it (used on window close).
func (c *settingsSearchController) SetEditor(editor *woxui.TextEditor) {
	c.mu.Lock()
	c.editor = editor
	c.mu.Unlock()
}

func (c *settingsSearchController) Focused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.focused
}

func (c *settingsSearchController) SetFocused(focused bool) {
	c.mu.Lock()
	c.focused = focused
	c.mu.Unlock()
}

func (c *settingsSearchController) Panel() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.panel
}

func (c *settingsSearchController) SetPanel(panel bool) {
	c.mu.Lock()
	c.panel = panel
	c.mu.Unlock()
}

func (c *settingsSearchController) Selected() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.selected
}

func (c *settingsSearchController) SetSelected(index int) {
	c.mu.Lock()
	c.selected = index
	c.mu.Unlock()
}

// Plugins returns a copy of the installed-plugin mirror used as the search index.
func (c *settingsSearchController) Plugins() []pluginSettingsPlugin {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

// SetPlugins installs the installed-plugin mirror. The slice is copied so the controller
// owns its own storage and later App-side mutation cannot leak into the snapshot.
func (c *settingsSearchController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.mu.Lock()
	c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
	c.mu.Unlock()
}

func (c *settingsSearchController) Loading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loading
}

func (c *settingsSearchController) SetLoading(loading bool) {
	c.mu.Lock()
	c.loading = loading
	c.mu.Unlock()
}

func (c *settingsSearchController) Loaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

func (c *settingsSearchController) SetLoaded(loaded bool) {
	c.mu.Lock()
	c.loaded = loaded
	c.mu.Unlock()
}

func (c *settingsSearchController) Error() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.errMsg
}

func (c *settingsSearchController) SetError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
}

// ReloadPlugins fetches the installed-plugin list that backs the search index. It is a no-op
// if a load is already in flight or has already completed successfully, mirroring the original
// load guard in loadSettingsSearchPlugins. On success Loaded becomes true, Loading false, and
// Plugins holds the sorted catalog; on failure Loading false, Loaded false, Error records the
// message. The caller is responsible for any post-load invalidation; this method only invalidates
// around the loading-state transitions.
func (c *settingsSearchController) ReloadPlugins(ctx context.Context, client backendClient) error {
	c.mu.Lock()
	if c.loading || c.loaded {
		c.mu.Unlock()
		return nil
	}
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var plugins []pluginSettingsPlugin
	err := client.Post(timeoutCtx, "/plugin/installed", map[string]any{}, &plugins)
	if err == nil {
		sort.SliceStable(plugins, func(i, j int) bool {
			if plugins[i].IsSystem != plugins[j].IsSystem {
				return plugins[i].IsSystem
			}
			return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
		})
	}

	c.mu.Lock()
	c.loading = false
	c.loaded = err == nil
	if err != nil {
		c.errMsg = err.Error()
	} else {
		c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
		c.errMsg = ""
	}
	c.mu.Unlock()
	c.deps.Invalidate()
	return err
}

// Run aggregates matches from every Searchable source for the given query. Each source
// contributes its own ranked results; Run concatenates them in source order. The rich
// cross-domain aggregation in App.settingsSearchResults reads from every settings snapshot
// and is kept separate; Run exists so the Searchable contract and tests can exercise the
// controller's aggregation path without re-implementing the full settings-catalog join.
func (c *settingsSearchController) Run(query string, sources []Searchable) []searchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	combined := make([]searchResult, 0)
	for _, source := range sources {
		if source == nil {
			continue
		}
		combined = append(combined, source.Search(query)...)
	}
	return combined
}

// Snapshot returns a copy of the Settings-search state for the view layer. Plugins is copied
// so the snapshot is safe to read outside the lock.
func (c *settingsSearchController) Snapshot() settingsSearchSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var query woxui.TextEditingState
	if c.editor != nil {
		query = c.editor.State()
	}
	return settingsSearchSnapshot{
		Query:    query,
		Focused:  c.focused,
		Panel:    c.panel,
		Selected: c.selected,
		Plugins:  append([]pluginSettingsPlugin(nil), c.plugins...),
		Loading:  c.loading,
		Error:    c.errMsg,
	}
}
