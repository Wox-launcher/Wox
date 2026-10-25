package launcher

import (
	"context"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// bindSettingsSearch wires the search panel to Core, the Settings window and the pages
// whose fields it indexes and opens.
func (a *App) bindSettingsSearch() {
	c := a.settingsSearch
	if c == nil {
		return
	}
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.language = &a.language
	c.settings = &a.settingsWindow

	c.aiSettings = func() *aiSettingsController { return a.aiSettings }
	c.generalSettings = func() *generalSettingsController { return a.generalSettings }
	c.hotkeySettings = func() *hotkeySettingsController { return a.hotkeySettings }
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.themeSettings = func() *themeSettingsController { return a.themeSettings }

	c.settingsSnapshot = a.settingsSnapshot
	c.localizedSettingItem = func(item settingItem) settingItem { return a.generalSettings.localizedSettingItem(item) }
	c.settingNavLabel = a.settingNavLabel
	c.selectSettingsNavItem = a.selectSettingsNavItem
	c.selectSettingTab = a.selectSettingTab
}

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
// It also builds results from the Settings snapshot, moves focus between the search
// field and other Settings pages, and opens the chosen page, plugin or field. Those
// pages are reached through the getters and callbacks wired by bindSettingsSearch.
// Run() below provides the narrow Searchable-aggregation path exercised by tests and by
// the Searchable contract.
type settingsSearchController struct {
	deps CommonDeps

	services     contract.PluginCatalogSettingsServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	language     *languageCatalog
	settings     *settingsWindowController

	aiSettings      func() *aiSettingsController
	generalSettings func() *generalSettingsController
	hotkeySettings  func() *hotkeySettingsController
	pluginSettings  func() *pluginSettingsController
	themeSettings   func() *themeSettingsController

	settingsSnapshot      func() settingsSnapshot
	localizedSettingItem  func(item settingItem) settingItem
	settingNavLabel       func(spec settingNavSpec) string
	selectSettingsNavItem func(item settingNavSpec)
	selectSettingTab      func(tab string)

	editor   *woxwidget.TextEditingController
	focused  bool
	panel    bool
	selected int
	plugins  []pluginSettingsPlugin
	loading  bool
	loaded   bool
	errMsg   string
	revision uint64
}

func newSettingsSearchController(deps CommonDeps) *settingsSearchController {
	return &settingsSearchController{deps: deps}
}

// Editor returns the floating search editor. May be nil before the settings window is opened.
func (c *settingsSearchController) Editor() *woxwidget.TextEditingController {
	return c.editor
}

// SetEditor installs the floating search editor. Passing nil clears it (used on window close).
func (c *settingsSearchController) SetEditor(editor *woxwidget.TextEditingController) {
	c.editor = editor
}

func (c *settingsSearchController) Focused() bool {
	return c.focused
}

func (c *settingsSearchController) SetFocused(focused bool) {
	c.focused = focused
}

func (c *settingsSearchController) Panel() bool {
	return c.panel
}

func (c *settingsSearchController) SetPanel(panel bool) {
	c.panel = panel
}

func (c *settingsSearchController) Selected() int {
	return c.selected
}

func (c *settingsSearchController) SetSelected(index int) {
	c.selected = index
}

// Plugins returns a copy of the installed-plugin mirror used as the search index.
func (c *settingsSearchController) Plugins() []pluginSettingsPlugin {
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

// SetPlugins installs the installed-plugin mirror. The slice is copied so the controller
// owns its own storage and later App-side mutation cannot leak into the snapshot.
func (c *settingsSearchController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
}

func (c *settingsSearchController) Loading() bool {
	return c.loading
}

func (c *settingsSearchController) SetLoading(loading bool) {
	c.loading = loading
}

func (c *settingsSearchController) Loaded() bool {
	return c.loaded
}

func (c *settingsSearchController) SetLoaded(loaded bool) {
	c.loaded = loaded
}

func (c *settingsSearchController) Error() string {
	return c.errMsg
}

func (c *settingsSearchController) SetError(msg string) {
	c.errMsg = msg
}

// ReleaseWindowMemory drops the plugin search index and invalidates an in-flight reload.
func (c *settingsSearchController) ReleaseWindowMemory() {
	c.revision++
	c.editor = nil
	c.focused = false
	c.panel = false
	c.selected = 0
	c.plugins = nil
	c.loading = false
	c.loaded = false
	c.errMsg = ""
}

// ReloadPlugins fetches the installed-plugin list that backs the search index. It is a no-op
// if a load is already in flight or has already completed successfully, mirroring the original
// load guard in loadSettingsSearchPlugins. On success Loaded becomes true, Loading false, and
// Plugins holds the sorted catalog; on failure Loading false, Loaded false, Error records the
// message. The caller is responsible for any post-load invalidation; this method only invalidates
// around the loading-state transitions.
func (c *settingsSearchController) ReloadPlugins(ctx context.Context, service contract.PluginCatalogSettingsServices, sessionID string) error {
	shouldLoad := false
	var revision uint64
	if !c.deps.OnUI("start loading settings search plugins", func() {
		if c.loading || c.loaded {
			return
		}
		c.revision++
		revision = c.revision
		c.loading = true
		c.errMsg = ""
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	loaded, err := service.Plugins(timeoutCtx, sessionID, contract.PluginCatalogInstalled)
	var plugins []pluginSettingsPlugin
	if err == nil {
		plugins, err = pluginSettingsPluginsFromContract(loaded)
	}
	if err == nil {
		sort.SliceStable(plugins, func(i, j int) bool {
			if plugins[i].IsSystem != plugins[j].IsSystem {
				return plugins[i].IsSystem
			}
			return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
		})
	}

	c.deps.OnUI("apply settings search plugins", func() {
		if revision != c.revision {
			return
		}
		c.loading = false
		c.loaded = err == nil
		if err != nil {
			c.errMsg = err.Error()
		} else {
			c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
			c.errMsg = ""
		}
		c.deps.Invalidate()
	})
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
