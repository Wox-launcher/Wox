package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
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
// getters/setters while coordinating cross-domain state in UI-thread transactions.
type pluginSettingsController struct {
	deps CommonDeps

	plugins              []pluginSettingsPlugin
	installedPlugins     []pluginSettingsPlugin
	storePlugins         []pluginSettingsPlugin
	installedLoaded      bool
	storeLoaded          bool
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
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

func (c *pluginSettingsController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
}

// CachedPlugins returns the previously loaded catalog for immediate list switching.
func (c *pluginSettingsController) CachedPlugins(store bool) ([]pluginSettingsPlugin, bool) {
	if store {
		return append([]pluginSettingsPlugin(nil), c.storePlugins...), c.storeLoaded
	}
	return append([]pluginSettingsPlugin(nil), c.installedPlugins...), c.installedLoaded
}

// cachePlugins stores one catalog independently from the currently visible list.
func (c *pluginSettingsController) cachePlugins(store bool, plugins []pluginSettingsPlugin) {
	if store {
		c.storePlugins = append([]pluginSettingsPlugin(nil), plugins...)
		c.storeLoaded = true
		return
	}
	c.installedPlugins = append([]pluginSettingsPlugin(nil), plugins...)
	c.installedLoaded = true
}

// invalidateCachedPlugins prevents lifecycle changes from exposing stale related-catalog data.
func (c *pluginSettingsController) invalidateCachedPlugins(store bool) {
	if store {
		c.storeLoaded = false
		return
	}
	c.installedLoaded = false
}

func (c *pluginSettingsController) PluginsLoading() bool {
	return c.pluginsLoading
}

func (c *pluginSettingsController) SetPluginsLoading(loading bool) {
	c.pluginsLoading = loading
}

func (c *pluginSettingsController) PluginsLoaded() bool {
	return c.pluginsLoaded
}

func (c *pluginSettingsController) SetPluginsLoaded(loaded bool) {
	c.pluginsLoaded = loaded
}

func (c *pluginSettingsController) PluginsError() string {
	return c.pluginsError
}

func (c *pluginSettingsController) SetPluginsError(msg string) {
	c.pluginsError = msg
}

func (c *pluginSettingsController) Selected() int {
	return c.pluginSelected
}

func (c *pluginSettingsController) SetSelected(index int) {
	c.pluginSelected = index
}

func (c *pluginSettingsController) SearchEditor() *woxui.TextEditor {
	return c.pluginSearchEditor
}

func (c *pluginSettingsController) SetSearchEditor(editor *woxui.TextEditor) {
	c.pluginSearchEditor = editor
}

func (c *pluginSettingsController) SearchFocused() bool {
	return c.pluginSearchFocused
}

func (c *pluginSettingsController) SetSearchFocused(focused bool) {
	c.pluginSearchFocused = focused
}

func (c *pluginSettingsController) Filters() pluginFilterState {
	return c.pluginFilters
}

func (c *pluginSettingsController) SetFilters(filters pluginFilterState) {
	c.pluginFilters = filters
}

func (c *pluginSettingsController) FilterOpen() bool {
	return c.pluginFilterOpen
}

func (c *pluginSettingsController) SetFilterOpen(open bool) {
	c.pluginFilterOpen = open
}

func (c *pluginSettingsController) DetailTab() string {
	return c.pluginDetailTab
}

func (c *pluginSettingsController) SetDetailTab(tab string) {
	c.pluginDetailTab = tab
}

// Form returns the live plugin settings form pointer. Cross-domain callers
// (model_manager, settings_search, requirement_preview, form_table) compare table-editor
// and recording targets against &Form().formFieldsState and mutate the form on the UI thread.
func (c *pluginSettingsController) Form() *pluginSettingsFormState {
	return c.pluginForm
}

// SetForm installs the plugin settings inline form. Passing nil clears it. The form is
// built by the App from the selected plugin's SettingDefinitions + persisted values and
// then handed to the controller so the view layer can read it through the snapshot.
func (c *pluginSettingsController) SetForm(form *pluginSettingsFormState) {
	c.pluginForm = form
}

func (c *pluginSettingsController) PluginsStore() bool {
	return c.pluginsStore
}

func (c *pluginSettingsController) SetPluginsStore(store bool) {
	c.pluginsStore = store
}

func (c *pluginSettingsController) Operation() string {
	return c.pluginOperation
}

func (c *pluginSettingsController) SetOperation(op string) {
	c.pluginOperation = op
}

func (c *pluginSettingsController) OperationError() string {
	return c.pluginOperationError
}

func (c *pluginSettingsController) SetOperationError(msg string) {
	c.pluginOperationError = msg
}

func (c *pluginSettingsController) UninstallArmed() string {
	return c.pluginUninstallArmed
}

func (c *pluginSettingsController) SetUninstallArmed(id string) {
	c.pluginUninstallArmed = id
}

// ReloadPlugins fetches either the store or installed catalog from the shared core service.
// store selects which catalog is loaded.
// preferredID, when non-empty, selects which plugin becomes Selected after the load;
// when empty and the current selection is valid, the previously selected plugin's ID
// is retained. On success PluginsLoaded becomes true and PluginsError is cleared; on
// failure PluginsLoaded becomes false and PluginsError records the message.
func (c *pluginSettingsController) ReloadPlugins(ctx context.Context, service contract.PluginCatalogSettingsServices, sessionID string, store bool, preferredID string) error {
	if !c.deps.OnUI("start loading plugin catalog", func() {
		c.pluginsLoading = true
		c.pluginsError = ""
		c.deps.Invalidate()
	}) {
		return nil
	}

	plugins, err := loadPluginSettingsPlugins(ctx, service, sessionID, store)
	if err != nil {
		c.finishPluginLoadError(err)
		return err
	}

	c.deps.OnUI("apply plugin catalog", func() {
		if preferredID == "" && c.pluginSelected >= 0 && c.pluginSelected < len(c.plugins) {
			preferredID = c.plugins[c.pluginSelected].ID
		}
		c.plugins = plugins
		c.cachePlugins(store, plugins)
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
		c.deps.Invalidate()
	})
	return nil
}

// PreloadPlugins fills one catalog cache without changing the visible plugin page.
func (c *pluginSettingsController) PreloadPlugins(ctx context.Context, service contract.PluginCatalogSettingsServices, sessionID string, store bool) error {
	plugins, err := loadPluginSettingsPlugins(ctx, service, sessionID, store)
	if err != nil {
		return err
	}
	c.deps.OnUI("cache plugin catalog", func() {
		c.cachePlugins(store, plugins)
	})
	return nil
}

// loadPluginSettingsPlugins fetches and sorts one plugin catalog for active and background loads.
func loadPluginSettingsPlugins(ctx context.Context, service contract.PluginCatalogSettingsServices, sessionID string, store bool) ([]pluginSettingsPlugin, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	catalog := contract.PluginCatalogInstalled
	if store {
		catalog = contract.PluginCatalogStore
	}
	loaded, err := service.Plugins(timeoutCtx, sessionID, catalog)
	if err != nil {
		return nil, err
	}
	plugins, err := pluginSettingsPluginsFromContract(loaded)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(plugins, func(i, j int) bool {
		if !store && plugins[i].IsSystem != plugins[j].IsSystem {
			return plugins[i].IsSystem
		}
		return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
	})
	return plugins, nil
}

// pluginSettingsPluginsFromContract adapts canonical plugin metadata to launcher-owned form models.
func pluginSettingsPluginsFromContract(items []contract.PluginCatalogItem) ([]pluginSettingsPlugin, error) {
	plugins := make([]pluginSettingsPlugin, len(items))
	for index, item := range items {
		definitionPayload, err := json.Marshal(item.SettingDefinitions)
		if err != nil {
			return nil, fmt.Errorf("encode plugin setting definitions: %w", err)
		}
		var definitions []formDefinition
		if err := json.Unmarshal(definitionPayload, &definitions); err != nil {
			return nil, fmt.Errorf("decode plugin setting definitions: %w", err)
		}

		commands := make([]pluginCommand, len(item.Commands))
		for commandIndex, command := range item.Commands {
			commands[commandIndex] = pluginCommand{Command: command.Command, Description: string(command.Description)}
		}
		features := make([]pluginFeature, len(item.Features))
		for featureIndex, feature := range item.Features {
			params := make(map[string]any, len(feature.Params))
			for key, value := range feature.Params {
				params[key] = value
			}
			features[featureIndex] = pluginFeature{Name: feature.Name, Params: params}
		}
		glances := make([]pluginGlance, len(item.Glances))
		for glanceIndex, glance := range item.Glances {
			glances[glanceIndex] = pluginGlance{
				ID: glance.Id, Name: string(glance.Name), Description: string(glance.Description), Icon: glance.Icon, RefreshIntervalMs: glance.RefreshIntervalMs,
			}
		}
		plugins[index] = pluginSettingsPlugin{
			ID: item.ID, Name: item.Name, Description: item.Description, Author: item.Author, Website: item.Website, Version: item.Version,
			Runtime: item.Runtime, Entry: item.Entry, PluginDirectory: item.PluginDirectory,
			Icon:           woxImage{ImageType: item.Icon.ImageType, ImageData: item.Icon.ImageData},
			ScreenshotURLs: append([]string(nil), item.ScreenshotURLs...), TriggerKeywords: append([]string(nil), item.TriggerKeywords...),
			Commands: commands, SupportedOS: append([]string(nil), item.SupportedOS...), Features: features, Glances: glances,
			IsSystem: item.IsSystem, IsDev: item.IsDev, IsInstalled: item.IsInstalled, IsDisable: item.IsDisable, IsUpgradable: item.IsUpgradable,
			SettingDefinitions: definitions,
			Setting: pluginSettingsData{
				Disabled: item.Setting.Disabled, TriggerKeywords: append([]string(nil), item.Setting.TriggerKeywords...), Settings: cloneStringMap(item.Setting.Settings),
			},
		}
	}
	return plugins, nil
}

// finishPluginLoadError is the shared error path when the App needs to mark a
// reload failed after the controller's ReloadPlugins already ran (e.g. when
// the App's post-reload form rebuild fails). Kept here so all plugin load
// error state stays in the controller.
func (c *pluginSettingsController) finishPluginLoadError(err error) {
	c.deps.OnUI("apply plugin catalog error", func() {
		c.pluginsLoading = false
		c.pluginsLoaded = false
		c.pluginsError = err.Error()
		c.deps.Invalidate()
	})
}

// Snapshot returns a copy of the Plugin tab state for the view layer. Form is
// deep-copied via snapshotPluginSettingsFormLocked so the snapshot is safe to
// read outside the lock; all other fields are value types or copied slices.
func (c *pluginSettingsController) Snapshot() pluginSettingsSnapshot {
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
