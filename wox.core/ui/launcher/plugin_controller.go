package launcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	woxui "wox/ui/runtime"
)

// pluginSettingsSnapshot is the immutable Plugin tab state consumed by the view layer.
// Mirrors the flat fields that previously lived on settingsSnapshot (plugins,
// pluginsLoading, pluginsError, pluginSelected, pluginSearch, pluginSearchFocused,
// pluginFilters, pluginFilterOpen, pluginDetailTab, pluginForm, pluginsStore,
// pluginOperation, pluginOperationError, pluginUninstallArmed) and adds pluginsLoaded
// which the load decisions in selectSettingTab need.
type pluginSettingsSnapshot struct {
	Plugins              []pluginSettingsPlugin
	PluginsLoading       bool
	PluginsLoaded        bool
	PluginsError         string
	PluginSelected       int
	PluginSearch         woxui.TextEditingState
	PluginSearchFocused  bool
	PluginFilters        pluginFilterState
	PluginFilterOpen     bool
	PluginDetailTab      string
	PluginForm           *pluginSettingsFormSnapshot
	PluginsStore         bool
	PluginOperation      string
	PluginOperationError string
	PluginUninstallArmed string
}

// pluginSettingsController owns the Plugin tab state: installed/store catalog,
// selection, search, advanced filters, detail tab, install/uninstall/enable/disable
// operation progress, and the plugin settings inline form. All 15 fields that used to
// live on App are held here; App methods became thin wrappers that call the controller's
// getters/setters while still coordinating cross-domain state (focus routing, shared
// setting note, AI model loading) under a.mu before delegating.
//
// The controller's mu only guards pointer swaps and scalar stores. Form mutation by
// cross-domain code (model_manager, settings_search, requirement_preview) happens under
// a.mu — same convention as hotkeySettings.Form() and aiSettings.Form(). The Form()
// getter returns the live *pluginSettingsFormState pointer for that purpose.
type pluginSettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

	plugins              []pluginSettingsPlugin
	pluginsLoading       bool
	pluginsLoaded        bool
	pluginsError         string
	pluginSelected       int
	pluginSearchEditor   *woxui.TextEditor
	pluginSearchFocused  bool
	pluginFilters        pluginFilterState
	pluginFilterOpen     bool
	pluginDetailTab      string
	pluginForm           *pluginSettingsFormState
	pluginsStore         bool
	pluginOperation      string
	pluginOperationError string
	pluginUninstallArmed string
}

func newPluginSettingsController(deps CommonDeps) *pluginSettingsController {
	return &pluginSettingsController{deps: deps}
}

func (c *pluginSettingsController) Plugins() []pluginSettingsPlugin {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

func (c *pluginSettingsController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.mu.Lock()
	c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
	c.mu.Unlock()
}

func (c *pluginSettingsController) PluginsLoading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginsLoading
}

func (c *pluginSettingsController) SetPluginsLoading(loading bool) {
	c.mu.Lock()
	c.pluginsLoading = loading
	c.mu.Unlock()
}

func (c *pluginSettingsController) PluginsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginsLoaded
}

func (c *pluginSettingsController) SetPluginsLoaded(loaded bool) {
	c.mu.Lock()
	c.pluginsLoaded = loaded
	c.mu.Unlock()
}

func (c *pluginSettingsController) PluginsError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginsError
}

func (c *pluginSettingsController) SetPluginsError(msg string) {
	c.mu.Lock()
	c.pluginsError = msg
	c.mu.Unlock()
}

func (c *pluginSettingsController) Selected() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginSelected
}

func (c *pluginSettingsController) SetSelected(index int) {
	c.mu.Lock()
	c.pluginSelected = index
	c.mu.Unlock()
}

func (c *pluginSettingsController) SearchEditor() *woxui.TextEditor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginSearchEditor
}

func (c *pluginSettingsController) SetSearchEditor(editor *woxui.TextEditor) {
	c.mu.Lock()
	c.pluginSearchEditor = editor
	c.mu.Unlock()
}

func (c *pluginSettingsController) SearchFocused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginSearchFocused
}

func (c *pluginSettingsController) SetSearchFocused(focused bool) {
	c.mu.Lock()
	c.pluginSearchFocused = focused
	c.mu.Unlock()
}

func (c *pluginSettingsController) Filters() pluginFilterState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginFilters
}

func (c *pluginSettingsController) SetFilters(filters pluginFilterState) {
	c.mu.Lock()
	c.pluginFilters = filters
	c.mu.Unlock()
}

func (c *pluginSettingsController) FilterOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginFilterOpen
}

func (c *pluginSettingsController) SetFilterOpen(open bool) {
	c.mu.Lock()
	c.pluginFilterOpen = open
	c.mu.Unlock()
}

func (c *pluginSettingsController) DetailTab() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginDetailTab
}

func (c *pluginSettingsController) SetDetailTab(tab string) {
	c.mu.Lock()
	c.pluginDetailTab = tab
	c.mu.Unlock()
}

// Form returns the live plugin settings form pointer. Cross-domain callers
// (model_manager, settings_search, requirement_preview, form_table) compare table-editor
// and recording targets against &Form().formFieldsState and mutate the form in place
// under a.mu. The controller's mu only guards the pointer swap, not the form's fields —
// same convention as hotkeySettings.Form() and aiSettings.Form().
func (c *pluginSettingsController) Form() *pluginSettingsFormState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginForm
}

// SetForm installs the plugin settings inline form. Passing nil clears it. The form is
// built by the App from the selected plugin's SettingDefinitions + persisted values and
// then handed to the controller so the view layer can read it through the snapshot.
func (c *pluginSettingsController) SetForm(form *pluginSettingsFormState) {
	c.mu.Lock()
	c.pluginForm = form
	c.mu.Unlock()
}

func (c *pluginSettingsController) PluginsStore() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginsStore
}

func (c *pluginSettingsController) SetPluginsStore(store bool) {
	c.mu.Lock()
	c.pluginsStore = store
	c.mu.Unlock()
}

func (c *pluginSettingsController) Operation() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginOperation
}

func (c *pluginSettingsController) SetOperation(op string) {
	c.mu.Lock()
	c.pluginOperation = op
	c.mu.Unlock()
}

func (c *pluginSettingsController) OperationError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginOperationError
}

func (c *pluginSettingsController) SetOperationError(msg string) {
	c.mu.Lock()
	c.pluginOperationError = msg
	c.mu.Unlock()
}

func (c *pluginSettingsController) UninstallArmed() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pluginUninstallArmed
}

func (c *pluginSettingsController) SetUninstallArmed(id string) {
	c.mu.Lock()
	c.pluginUninstallArmed = id
	c.mu.Unlock()
}

// ReloadPlugins fetches either store or installed entries through the same core DTO as
// the retired Flutter catalog. store selects /plugin/store vs /plugin/installed.
// preferredID, when non-empty, selects which plugin becomes Selected after the load;
// when empty and the current selection is valid, the previously selected plugin's ID
// is retained. On success PluginsLoaded becomes true and PluginsError is cleared; on
// failure PluginsLoaded becomes false and PluginsError records the message.
func (c *pluginSettingsController) ReloadPlugins(ctx context.Context, client backendClient, store bool, preferredID string) error {
	c.mu.Lock()
	c.pluginsLoading = true
	c.pluginsError = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var plugins []pluginSettingsPlugin
	path := "/plugin/installed"
	if store {
		path = "/plugin/store"
	}
	if err := client.Post(timeoutCtx, path, map[string]any{}, &plugins); err != nil {
		c.mu.Lock()
		c.pluginsLoading = false
		c.pluginsLoaded = false
		c.pluginsError = err.Error()
		c.mu.Unlock()
		c.deps.Invalidate()
		return err
	}
	sort.SliceStable(plugins, func(i, j int) bool {
		if !store && plugins[i].IsSystem != plugins[j].IsSystem {
			return plugins[i].IsSystem
		}
		return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
	})

	c.mu.Lock()
	if preferredID == "" && c.pluginSelected >= 0 && c.pluginSelected < len(c.plugins) {
		preferredID = c.plugins[c.pluginSelected].ID
	}
	c.plugins = plugins
	c.pluginsLoading = false
	c.pluginsLoaded = true
	c.pluginsError = ""
	c.pluginOperationError = ""
	if c.pluginSearchEditor == nil {
		c.pluginSearchEditor = woxui.NewTextEditor("")
	}
	if c.pluginDetailTab == "" {
		c.pluginDetailTab = "settings"
	}
	selected := 0
	for index, plugin := range plugins {
		if plugin.ID == preferredID {
			selected = index
			break
		}
	}
	if len(plugins) == 0 {
		c.pluginSelected = -1
		c.pluginForm = nil
	} else {
		c.pluginSelected = selected
	}
	c.mu.Unlock()
	c.deps.Invalidate()
	return nil
}

// finishPluginLoadError is the shared error path when the App needs to mark a
// reload failed after the controller's ReloadPlugins already ran (e.g. when
// the App's post-reload form rebuild fails). Kept here so all plugin load
// error state stays in the controller.
func (c *pluginSettingsController) finishPluginLoadError(err error) {
	c.mu.Lock()
	c.pluginsLoading = false
	c.pluginsLoaded = false
	c.pluginsError = err.Error()
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Plugin tab state for the view layer. Form is
// deep-copied via snapshotPluginSettingsFormLocked so the snapshot is safe to
// read outside the lock; all other fields are value types or copied slices.
func (c *pluginSettingsController) Snapshot() pluginSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var pluginSearch woxui.TextEditingState
	if c.pluginSearchEditor != nil {
		pluginSearch = c.pluginSearchEditor.State()
	}
	return pluginSettingsSnapshot{
		Plugins:              append([]pluginSettingsPlugin(nil), c.plugins...),
		PluginsLoading:       c.pluginsLoading,
		PluginsLoaded:        c.pluginsLoaded,
		PluginsError:         c.pluginsError,
		PluginSelected:       c.pluginSelected,
		PluginSearch:         pluginSearch,
		PluginSearchFocused:  c.pluginSearchFocused,
		PluginFilters:        c.pluginFilters,
		PluginFilterOpen:     c.pluginFilterOpen,
		PluginDetailTab:      c.pluginDetailTab,
		PluginForm:           snapshotPluginSettingsFormLocked(c.pluginForm),
		PluginsStore:         c.pluginsStore,
		PluginOperation:      c.pluginOperation,
		PluginOperationError: c.pluginOperationError,
		PluginUninstallArmed: c.pluginUninstallArmed,
	}
}
