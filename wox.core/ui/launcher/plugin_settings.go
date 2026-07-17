package launcher

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const pluginSettingsListRowHeight = float32(62)

type pluginSettingsPlugin struct {
	ID                 string             `json:"Id"`
	Name               string             `json:"Name"`
	Description        string             `json:"Description"`
	Author             string             `json:"Author"`
	Website            string             `json:"Website"`
	Version            string             `json:"Version"`
	Runtime            string             `json:"Runtime"`
	Entry              string             `json:"Entry"`
	PluginDirectory    string             `json:"PluginDirectory"`
	Icon               woxImage           `json:"Icon"`
	ScreenshotURLs     []string           `json:"ScreenshotUrls"`
	TriggerKeywords    []string           `json:"TriggerKeywords"`
	Commands           []pluginCommand    `json:"Commands"`
	SupportedOS        []string           `json:"SupportedOS"`
	Features           []pluginFeature    `json:"Features"`
	Glances            []pluginGlance     `json:"Glances"`
	IsSystem           bool               `json:"IsSystem"`
	IsDev              bool               `json:"IsDev"`
	IsInstalled        bool               `json:"IsInstalled"`
	IsDisable          bool               `json:"IsDisable"`
	IsUpgradable       bool               `json:"IsUpgradable"`
	SettingDefinitions []formDefinition   `json:"SettingDefinitions"`
	Setting            pluginSettingsData `json:"Setting"`
}

type pluginCommand struct {
	Command     string `json:"Command"`
	Description string `json:"Description"`
}

type pluginFeature struct {
	Name   string         `json:"Name"`
	Params map[string]any `json:"Params"`
}

type filteredPlugin struct {
	index  int
	plugin pluginSettingsPlugin
}

type pluginFilterState struct {
	enabledOnly             bool
	disabledOnly            bool
	upgradableOnly          bool
	uninstalledOnly         bool
	thirdPartyOnly          bool
	runtimeNodeJSOnly       bool
	runtimePythonOnly       bool
	runtimeScriptOnly       bool
	runtimeScriptNodeJSOnly bool
	runtimeScriptPythonOnly bool
}

func (filters pluginFilterState) applied(store bool) bool {
	if store {
		return filters.uninstalledOnly || filters.thirdPartyOnly || filters.runtimeNodeJSOnly || filters.runtimePythonOnly || filters.runtimeScriptOnly
	}
	return filters.enabledOnly || filters.disabledOnly || filters.upgradableOnly || filters.thirdPartyOnly || filters.runtimeNodeJSOnly || filters.runtimePythonOnly || filters.runtimeScriptNodeJSOnly || filters.runtimeScriptPythonOnly
}

// filterPlugins applies the same keyword and advanced-filter contract as the retired Flutter catalog.
func filterPlugins(plugins []pluginSettingsPlugin, query string, filters pluginFilterState, store bool) []filteredPlugin {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]filteredPlugin, 0, len(plugins))
	for index, plugin := range plugins {
		searchText := strings.ToLower(strings.Join(append([]string{plugin.Name, plugin.ID, plugin.Author, plugin.Description, plugin.Runtime}, plugin.TriggerKeywords...), " "))
		if (query == "" || strings.Contains(searchText, query)) && pluginMatchesFilters(plugin, filters, store) {
			filtered = append(filtered, filteredPlugin{index: index, plugin: plugin})
		}
	}
	return filtered
}

// pluginMatchesFilters keeps store and installed-only predicates from leaking into each other.
func pluginMatchesFilters(plugin pluginSettingsPlugin, filters pluginFilterState, store bool) bool {
	if store {
		if filters.uninstalledOnly && plugin.IsInstalled {
			return false
		}
	} else {
		if filters.enabledOnly && plugin.IsDisable {
			return false
		}
		if filters.disabledOnly && !plugin.IsDisable {
			return false
		}
		if filters.upgradableOnly && !plugin.IsUpgradable {
			return false
		}
	}
	if filters.thirdPartyOnly && plugin.IsSystem {
		return false
	}

	runtimeNodeJS := filters.runtimeNodeJSOnly && strings.EqualFold(plugin.Runtime, "nodejs")
	runtimePython := filters.runtimePythonOnly && strings.EqualFold(plugin.Runtime, "python")
	runtimeScript := store && filters.runtimeScriptOnly && strings.EqualFold(plugin.Runtime, "script")
	runtimeScriptNodeJS := !store && filters.runtimeScriptNodeJSOnly && strings.EqualFold(plugin.Runtime, "script") && strings.HasSuffix(strings.ToLower(plugin.Entry), ".js")
	runtimeScriptPython := !store && filters.runtimeScriptPythonOnly && strings.EqualFold(plugin.Runtime, "script") && strings.HasSuffix(strings.ToLower(plugin.Entry), ".py")
	runtimeFilterApplied := filters.runtimeNodeJSOnly || filters.runtimePythonOnly
	if store {
		runtimeFilterApplied = runtimeFilterApplied || filters.runtimeScriptOnly
	} else {
		runtimeFilterApplied = runtimeFilterApplied || filters.runtimeScriptNodeJSOnly || filters.runtimeScriptPythonOnly
	}
	return !runtimeFilterApplied || runtimeNodeJS || runtimePython || runtimeScript || runtimeScriptNodeJS || runtimeScriptPython
}

type pluginGlance struct {
	ID                string `json:"Id"`
	Name              string `json:"Name"`
	Description       string `json:"Description"`
	Icon              string `json:"Icon"`
	RefreshIntervalMs int    `json:"RefreshIntervalMs"`
}

type pluginSettingsData struct {
	Disabled        bool              `json:"Disabled"`
	TriggerKeywords []string          `json:"TriggerKeywords"`
	Settings        map[string]string `json:"Settings"`
}

type pluginSettingsFormState struct {
	formFieldsState
	pluginID    string
	initial     map[string]string
	saving      bool
	status      string
	statusError bool
	revision    uint64
}

type pluginSettingsFormSnapshot struct {
	formFieldsSnapshot
	pluginID    string
	initial     map[string]string
	saving      bool
	status      string
	statusError bool
	dirty       bool
}

// reloadPlugins fetches either store or installed entries through the same core DTO.
func (a *App) reloadPlugins(store bool, preferredID string) error {
	a.mu.Lock()
	a.pluginsLoading = true
	a.pluginsError = ""
	a.mu.Unlock()
	if a.window != nil {
		a.invalidateSettingsWindow()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var plugins []pluginSettingsPlugin
	path := "/plugin/installed"
	if store {
		path = "/plugin/store"
	}
	if err := a.client.Post(ctx, path, map[string]any{}, &plugins); err != nil {
		a.mu.Lock()
		a.pluginsLoading = false
		a.pluginsLoaded = false
		a.pluginsError = err.Error()
		a.mu.Unlock()
		if a.window != nil {
			a.invalidateSettingsWindow()
		}
		return err
	}
	sort.SliceStable(plugins, func(i, j int) bool {
		if !store && plugins[i].IsSystem != plugins[j].IsSystem {
			return plugins[i].IsSystem
		}
		return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
	})

	a.mu.Lock()
	if preferredID == "" && a.pluginSelected >= 0 && a.pluginSelected < len(a.plugins) {
		preferredID = a.plugins[a.pluginSelected].ID
	}
	a.plugins = plugins
	a.pluginsLoading = false
	a.pluginsLoaded = true
	a.pluginsError = ""
	if !store {
		a.settingSearchPlugins = append([]pluginSettingsPlugin(nil), plugins...)
		a.settingSearchLoading = false
		a.settingSearchLoaded = true
		a.settingSearchError = ""
	}
	a.pluginOperationError = ""
	if a.pluginSearchEditor == nil {
		a.pluginSearchEditor = woxui.NewTextEditor("")
	}
	if a.pluginDetailTab == "" {
		a.pluginDetailTab = "settings"
	}
	selected := 0
	for index, plugin := range plugins {
		if plugin.ID == preferredID {
			selected = index
			break
		}
	}
	if len(plugins) == 0 {
		a.pluginSelected = -1
		a.pluginForm = nil
	} else {
		a.setPluginSelectionLocked(selected)
	}
	requestModels := a.pluginForm != nil && hasFormDefinitionType(a.pluginForm.definitions, "selectAIModel") && !a.aiModelsLoaded && !a.aiModelsLoading
	if requestModels {
		a.aiModelsLoading = true
	}
	a.mu.Unlock()
	if requestModels {
		go a.loadAIModels()
	}
	if a.window != nil {
		a.invalidateSettingsWindow()
	}
	return nil
}

func pluginSettingsPathIsStore(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/plugins/store" || path == "plugins.store"
}

// switchPluginList swaps the shared list between installed and store data without duplicating its UI state.
func (a *App) switchPluginList(store bool) {
	a.mu.Lock()
	if a.pluginOperation != "" || a.pluginsLoading || (a.pluginsStore == store && a.pluginsLoaded) {
		a.mu.Unlock()
		return
	}
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		if pluginFormDirty(a.pluginForm.definitions, a.pluginForm.values, a.pluginForm.initial) {
			a.pluginForm.status = "Save the current plugin changes before switching lists."
			a.pluginForm.statusError = true
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	a.pluginsStore = store
	a.plugins = nil
	a.pluginsLoaded = false
	a.pluginsLoading = true
	a.pluginsError = ""
	a.pluginSelected = -1
	a.pluginForm = nil
	a.pluginListScroll = 0
	a.pluginUninstallArmed = ""
	a.pluginOperationError = ""
	a.pluginSearchEditor = woxui.NewTextEditor("")
	a.pluginSearchFocused = true
	a.settingSearchFocused = false
	a.settingSearchPanel = false
	a.pluginFilterOpen = false
	if store {
		a.pluginDetailTab = "description"
	} else {
		a.pluginDetailTab = "settings"
	}
	a.ensureSettingTabVisibleLocked("plugins")
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	go func() {
		if err := a.reloadPlugins(store, ""); err != nil {
			log.Printf("switch plugin list: %v", err)
		}
	}()
}

// runPluginOperation uses core's install endpoint for both fresh installs and upgrades.
func (a *App) runPluginOperation(kind string) {
	a.mu.Lock()
	if a.pluginOperation != "" || a.pluginSelected < 0 || a.pluginSelected >= len(a.plugins) {
		a.mu.Unlock()
		return
	}
	plugin := a.plugins[a.pluginSelected]
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		if pluginFormDirty(a.pluginForm.definitions, a.pluginForm.values, a.pluginForm.initial) {
			a.pluginForm.status = "Save the current plugin changes before managing this plugin."
			a.pluginForm.statusError = true
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	switch kind {
	case "install":
		if plugin.IsInstalled {
			a.mu.Unlock()
			return
		}
	case "upgrade":
		if !plugin.IsInstalled || !plugin.IsUpgradable {
			a.mu.Unlock()
			return
		}
	case "uninstall":
		if !plugin.IsInstalled || plugin.IsSystem {
			a.mu.Unlock()
			return
		}
	case "enable":
		if !plugin.IsInstalled || !plugin.IsDisable {
			a.mu.Unlock()
			return
		}
	case "disable":
		if !plugin.IsInstalled || plugin.IsDisable {
			a.mu.Unlock()
			return
		}
		if a.pluginUninstallArmed != plugin.ID {
			a.pluginUninstallArmed = plugin.ID
			a.settingNote = "Press Confirm uninstall to remove " + plugin.Name + "."
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	default:
		a.mu.Unlock()
		return
	}
	a.pluginUninstallArmed = ""
	a.pluginOperationError = ""
	a.pluginOperation = kind + ":" + plugin.ID
	store := a.pluginsStore
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		path := "/plugin/" + kind
		if kind == "upgrade" {
			path = "/plugin/install"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := a.client.Post(ctx, path, map[string]string{"id": plugin.ID}, nil)
		cancel()
		if err == nil {
			err = a.reloadPlugins(store, plugin.ID)
		}
		a.mu.Lock()
		a.pluginOperation = ""
		if err != nil {
			a.pluginOperationError = err.Error()
		} else {
			a.pluginOperationError = ""
			a.settingNote = kind + " completed for " + plugin.Name
		}
		a.mu.Unlock()
		if err != nil {
			log.Printf("%s plugin %s: %v", kind, plugin.ID, err)
		}
		a.invalidateSettingsWindow()
	}()
}

// openSelectedPluginWebsite keeps browser dispatch behind the portable Window capability.
func (a *App) openSelectedPluginWebsite() {
	a.mu.RLock()
	if a.pluginSelected < 0 || a.pluginSelected >= len(a.plugins) {
		a.mu.RUnlock()
		return
	}
	target := strings.TrimSpace(a.plugins[a.pluginSelected].Website)
	a.mu.RUnlock()
	if target == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		a.mu.Lock()
		a.pluginOperationError = err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}

// openSelectedPluginDirectory delegates reveal behavior to core's cross-platform shell adapter.
func (a *App) openSelectedPluginDirectory() {
	a.mu.RLock()
	if a.pluginSelected < 0 || a.pluginSelected >= len(a.plugins) {
		a.mu.RUnlock()
		return
	}
	directory := strings.TrimSpace(a.plugins[a.pluginSelected].PluginDirectory)
	a.mu.RUnlock()
	if directory == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.client.Post(ctx, "/open", map[string]string{"path": directory}, nil)
		cancel()
		if err != nil {
			a.mu.Lock()
			a.pluginOperationError = err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
}

// runSelectedPluginPrimaryOperation gives keyboard users the same install or upgrade action as the detail button.
func (a *App) runSelectedPluginPrimaryOperation() {
	a.mu.RLock()
	if a.pluginSelected < 0 || a.pluginSelected >= len(a.plugins) {
		a.mu.RUnlock()
		return
	}
	plugin := a.plugins[a.pluginSelected]
	a.mu.RUnlock()
	if !plugin.IsInstalled {
		a.runPluginOperation("install")
	} else if plugin.IsUpgradable {
		a.runPluginOperation("upgrade")
	}
}

// setPluginSelectionLocked replaces the editor state with one plugin's current persisted values.
func (a *App) setPluginSelectionLocked(index int) {
	if index < 0 || index >= len(a.plugins) {
		return
	}
	a.modelManager = nil
	a.pluginDetailTab = "settings"
	plugin := a.plugins[index]
	if a.pluginsStore {
		a.pluginDetailTab = "description"
		a.pluginSelected = index
		a.pluginForm = nil
		a.ensurePluginSelectionVisibleLocked()
		return
	}
	definitions := []formDefinition{
		{Type: "head", Value: formDefinitionValue{Content: "Plugin controls"}},
		{Type: "checkbox", Value: formDefinitionValue{Key: "Disabled", Label: "Disabled", Tooltip: "Prevent this plugin from answering queries"}},
		{Type: "textbox", Value: formDefinitionValue{Key: "TriggerKeywords", Label: "Trigger keywords", Tooltip: "Comma-separated keywords that invoke this plugin"}},
		{Type: "newline"},
	}
	definitions = append(definitions, plugin.SettingDefinitions...)
	values := make(map[string]string, len(plugin.Setting.Settings)+2)
	values["Disabled"] = fmt.Sprintf("%t", plugin.Setting.Disabled || plugin.IsDisable)
	values["TriggerKeywords"] = strings.Join(plugin.Setting.TriggerKeywords, ",")
	if values["TriggerKeywords"] == "" {
		values["TriggerKeywords"] = strings.Join(plugin.TriggerKeywords, ",")
	}
	for key, value := range plugin.Setting.Settings {
		values[key] = value
	}
	applyDictationFormCompatibility(plugin, values)
	fields := newFormFieldsState(definitions, values, false)
	preserveDictationCompatibilityValues(plugin.ID, fields.values, values)
	if len(a.aiModels) > 0 {
		applyAIModelOptionsLocked(&fields, a.aiModels)
	}
	initial := make(map[string]string, len(fields.values))
	for key, value := range fields.values {
		initial[key] = value
	}
	a.pluginSelected = index
	a.pluginForm = &pluginSettingsFormState{formFieldsState: fields, pluginID: plugin.ID, initial: initial}
	a.ensurePluginSelectionVisibleLocked()
}

// snapshotPluginSettingsFormLocked copies mutable maps before the render lock is released.
func snapshotPluginSettingsFormLocked(state *pluginSettingsFormState) *pluginSettingsFormSnapshot {
	if state == nil {
		return nil
	}
	initial := make(map[string]string, len(state.initial))
	for key, value := range state.initial {
		initial[key] = value
	}
	return &pluginSettingsFormSnapshot{
		formFieldsSnapshot: snapshotFormFieldsLocked(&state.formFieldsState),
		pluginID:           state.pluginID,
		initial:            initial,
		saving:             state.saving,
		status:             state.status,
		statusError:        state.statusError,
		dirty:              pluginFormDirty(state.definitions, state.values, state.initial),
	}
}

func pluginFormDirty(definitions []formDefinition, values, initial map[string]string) bool {
	for _, key := range editableFormKeys(definitions) {
		if values[key] != initial[key] {
			return true
		}
	}
	return false
}

// selectPlugin changes the detail editor without coupling selection to a platform list control.
func (a *App) selectPlugin(index int) {
	a.mu.Lock()
	if index < 0 || index >= len(a.plugins) || index == a.pluginSelected {
		a.mu.Unlock()
		return
	}
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		if pluginFormDirty(a.pluginForm.definitions, a.pluginForm.values, a.pluginForm.initial) {
			a.pluginForm.status = "Save the current plugin changes before selecting another plugin."
			a.pluginForm.statusError = true
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	a.setPluginSelectionLocked(index)
	requestModels := a.pluginForm != nil && hasFormDefinitionType(a.pluginForm.definitions, "selectAIModel") && !a.aiModelsLoaded && !a.aiModelsLoading
	if requestModels {
		a.aiModelsLoading = true
	}
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	if requestModels {
		go a.loadAIModels()
	}
	a.invalidateSettingsWindow()
}

func (a *App) movePluginSelection(delta int) {
	a.mu.RLock()
	count := len(a.plugins)
	selected := a.pluginSelected
	a.mu.RUnlock()
	if count == 0 {
		return
	}
	selected = (selected + delta + count) % count
	a.selectPlugin(selected)
}

func (a *App) ensurePluginSelectionVisibleLocked() {
	viewport := a.pluginListViewport
	if viewport <= 1 {
		viewport = 600
	}
	query := ""
	if a.pluginSearchEditor != nil {
		query = a.pluginSearchEditor.State().Text
	}
	filtered := filterPlugins(a.plugins, query, a.pluginFilters, a.pluginsStore)
	visibleIndex := -1
	for index, entry := range filtered {
		if entry.index == a.pluginSelected {
			visibleIndex = index
			break
		}
	}
	if visibleIndex < 0 {
		a.pluginListScroll = 0
		return
	}
	rowTop := float32(visibleIndex) * pluginSettingsListRowHeight
	rowBottom := rowTop + pluginSettingsListRowHeight
	if rowTop < a.pluginListScroll {
		a.pluginListScroll = rowTop
	} else if rowBottom > a.pluginListScroll+viewport {
		a.pluginListScroll = rowBottom - viewport
	}
	a.clampPluginListScrollLocked(len(filtered), viewport)
}

// setPluginListViewport records list geometry without letting ordinary redraws reclaim manual scroll ownership.
func (a *App) setPluginListViewport(height float32) {
	a.mu.Lock()
	initialize := a.pluginListViewport <= 1
	a.pluginListViewport = max(float32(1), height)
	if initialize {
		a.ensurePluginSelectionVisibleLocked()
	} else {
		query := ""
		if a.pluginSearchEditor != nil {
			query = a.pluginSearchEditor.State().Text
		}
		a.clampPluginListScrollLocked(len(filterPlugins(a.plugins, query, a.pluginFilters, a.pluginsStore)), a.pluginListViewport)
	}
	a.mu.Unlock()
}

func (a *App) scrollPluginList(delta float32) {
	a.mu.Lock()
	viewport := max(float32(1), a.pluginListViewport)
	query := ""
	if a.pluginSearchEditor != nil {
		query = a.pluginSearchEditor.State().Text
	}
	a.pluginListScroll += delta
	a.clampPluginListScrollLocked(len(filterPlugins(a.plugins, query, a.pluginFilters, a.pluginsStore)), viewport)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// clampPluginListScrollLocked keeps the current offset inside the measured filtered list extent.
func (a *App) clampPluginListScrollLocked(itemCount int, viewport float32) {
	maxOffset := max(float32(0), float32(itemCount)*pluginSettingsListRowHeight-max(float32(1), viewport))
	a.pluginListScroll = min(max(float32(0), a.pluginListScroll), maxOffset)
}

// focusPluginSearch transfers native text input from plugin forms to the catalog filter.
func (a *App) focusPluginSearch(caret int) {
	a.blurSettingsSearch()
	a.mu.Lock()
	if a.pluginSearchEditor == nil {
		a.pluginSearchEditor = woxui.NewTextEditor("")
	}
	if caret >= 0 {
		a.pluginSearchEditor.SetCaret(caret)
	}
	a.pluginSearchFocused = true
	a.themeSearchFocused = false
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		a.pluginForm.active = false
	}
	host := a.settingsHost
	a.mu.Unlock()
	if host != nil {
		host.RequestFocus(woxwidget.Key("plugin-search"))
	}
	a.invalidateSettingsWindow()
}

// setPluginSearchFocused keeps plugin input routing aligned with retained focus changes.
func (a *App) setPluginSearchFocused(focused bool) {
	a.mu.Lock()
	if a.pluginSearchEditor == nil {
		a.pluginSearchEditor = woxui.NewTextEditor("")
	}
	a.pluginSearchFocused = focused
	if focused {
		a.settingSearchFocused = false
		a.settingSearchPanel = false
		a.themeSearchFocused = false
		if a.pluginForm != nil {
			syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
			a.pluginForm.active = false
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setPluginSearchValue applies accessibility value changes and resets the filtered viewport.
func (a *App) setPluginSearchValue(value string) error {
	a.mu.Lock()
	if a.pluginSearchEditor == nil {
		a.pluginSearchEditor = woxui.NewTextEditor(value)
	} else {
		a.pluginSearchEditor.SetText(value, false)
	}
	a.pluginListScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearPluginSearch() {
	a.mu.Lock()
	if a.pluginSearchEditor == nil {
		a.pluginSearchEditor = woxui.NewTextEditor("")
	} else {
		a.pluginSearchEditor.SetText("", false)
	}
	a.pluginListScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// togglePluginFilterPanel shows or hides the catalog's anchored advanced filters.
func (a *App) togglePluginFilterPanel() {
	a.mu.Lock()
	a.pluginFilterOpen = !a.pluginFilterOpen
	a.pluginSearchFocused = false
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) closePluginFilterPanel() {
	a.mu.Lock()
	a.pluginFilterOpen = false
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// togglePluginFilter updates one filter while keeping the current detail selected whenever possible.
func (a *App) togglePluginFilter(id string) {
	a.mu.Lock()
	switch id {
	case "enabled":
		a.pluginFilters.enabledOnly = !a.pluginFilters.enabledOnly
	case "disabled":
		a.pluginFilters.disabledOnly = !a.pluginFilters.disabledOnly
	case "upgradable":
		a.pluginFilters.upgradableOnly = !a.pluginFilters.upgradableOnly
	case "uninstalled":
		a.pluginFilters.uninstalledOnly = !a.pluginFilters.uninstalledOnly
	case "third-party":
		a.pluginFilters.thirdPartyOnly = !a.pluginFilters.thirdPartyOnly
	case "runtime-nodejs":
		a.pluginFilters.runtimeNodeJSOnly = !a.pluginFilters.runtimeNodeJSOnly
	case "runtime-python":
		a.pluginFilters.runtimePythonOnly = !a.pluginFilters.runtimePythonOnly
	case "runtime-script":
		a.pluginFilters.runtimeScriptOnly = !a.pluginFilters.runtimeScriptOnly
	case "runtime-script-nodejs":
		a.pluginFilters.runtimeScriptNodeJSOnly = !a.pluginFilters.runtimeScriptNodeJSOnly
	case "runtime-script-python":
		a.pluginFilters.runtimeScriptPythonOnly = !a.pluginFilters.runtimeScriptPythonOnly
	default:
		a.mu.Unlock()
		return
	}
	query := ""
	if a.pluginSearchEditor != nil {
		query = a.pluginSearchEditor.State().Text
	}
	filtered := filterPlugins(a.plugins, query, a.pluginFilters, a.pluginsStore)
	selectedVisible := false
	for _, entry := range filtered {
		if entry.index == a.pluginSelected {
			selectedVisible = true
			break
		}
	}
	if !selectedVisible && len(filtered) > 0 {
		a.setPluginSelectionLocked(filtered[0].index)
	}
	a.pluginListScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// refreshPluginCatalog preserves the search and selection while reloading the current catalog.
func (a *App) refreshPluginCatalog() {
	a.mu.Lock()
	if a.pluginsLoading || a.pluginOperation != "" {
		a.mu.Unlock()
		return
	}
	store := a.pluginsStore
	preferredID := ""
	if a.pluginSelected >= 0 && a.pluginSelected < len(a.plugins) {
		preferredID = a.plugins[a.pluginSelected].ID
	}
	a.pluginFilterOpen = false
	a.mu.Unlock()
	go func() {
		if err := a.reloadPlugins(store, preferredID); err != nil {
			log.Printf("refresh plugin catalog: %v", err)
		}
	}()
}

func (a *App) blurPluginSearch() {
	a.mu.Lock()
	a.pluginSearchFocused = false
	host := a.settingsHost
	a.mu.Unlock()
	if host != nil {
		host.ClearFocus()
	}
	a.invalidateSettingsWindow()
}

func (a *App) moveFilteredPluginSelection(delta int) {
	a.mu.RLock()
	query := ""
	if a.pluginSearchEditor != nil {
		query = a.pluginSearchEditor.State().Text
	}
	plugins := append([]pluginSettingsPlugin(nil), a.plugins...)
	selected := a.pluginSelected
	filters := a.pluginFilters
	store := a.pluginsStore
	a.mu.RUnlock()
	filtered := filterPlugins(plugins, query, filters, store)
	if len(filtered) == 0 {
		return
	}
	position := -1
	for index, entry := range filtered {
		if entry.index == selected {
			position = index
			break
		}
	}
	if position < 0 {
		if delta < 0 {
			position = len(filtered) - 1
		} else {
			position = 0
		}
	} else {
		position = (position + delta + len(filtered)) % len(filtered)
	}
	a.selectPlugin(filtered[position].index)
}

func (a *App) onPluginSearchKey(event woxui.KeyEvent) bool {
	// Key releases must not repeat list navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	a.mu.RLock()
	active := a.settingsOpen && a.settingTab == "plugins" && a.pluginSearchFocused && a.pluginSearchEditor != nil
	a.mu.RUnlock()
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.blurPluginSearch()
	case woxui.KeyArrowUp:
		a.moveFilteredPluginSelection(-1)
	case woxui.KeyArrowDown:
		a.moveFilteredPluginSelection(1)
	case woxui.KeyEnter, woxui.KeyTab:
		a.blurPluginSearch()
	default:
		// Printable keys stay unhandled so the platform can turn them into committed or composing text.
		a.mu.Lock()
		handled := false
		if a.pluginSearchEditor != nil {
			var changed bool
			handled, changed = a.pluginSearchEditor.HandleKey(event)
			if changed {
				a.pluginListScroll = 0
			}
		}
		a.mu.Unlock()
		if handled {
			a.invalidateSettingsWindow()
		}
		return handled
	}
	return true
}

func (a *App) onPluginSearchTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	if !a.settingsOpen || a.settingTab != "plugins" || !a.pluginSearchFocused || a.pluginSearchEditor == nil {
		a.mu.Unlock()
		return false
	}
	a.pluginSearchEditor.HandleTextInput(event)
	a.pluginListScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return true
}

// selectPluginDetailTab changes detail content without discarding staged plugin settings.
func (a *App) selectPluginDetailTab(tab string) {
	switch tab {
	case "settings", "description", "keywords", "commands", "privacy":
	default:
		return
	}
	a.mu.Lock()
	a.pluginDetailTab = tab
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		a.pluginForm.active = false
	}
	a.pluginSearchFocused = false
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) setPluginFormViewport(height float32) {
	a.mu.Lock()
	if a.pluginForm != nil {
		a.pluginForm.viewportHeight = max(float32(1), height)
		a.pluginForm.scroll = min(a.pluginForm.scroll, max(float32(0), formDefinitionsContentHeight(a.pluginForm.definitions, a.pluginForm.values)-a.pluginForm.viewportHeight))
	}
	a.mu.Unlock()
}

func (a *App) scrollPluginForm(delta float32) {
	a.mu.Lock()
	if a.pluginForm == nil {
		a.mu.Unlock()
		return
	}
	maxOffset := max(float32(0), formDefinitionsContentHeight(a.pluginForm.definitions, a.pluginForm.values)-a.pluginForm.viewportHeight)
	a.pluginForm.scroll = min(max(float32(0), a.pluginForm.scroll+delta), maxOffset)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// onPluginSettingsKey routes keys either to list navigation or the active shared field editor.
func (a *App) onPluginSettingsKey(event woxui.KeyEvent) bool {
	if a.onPluginSearchKey(event) {
		return true
	}
	a.mu.RLock()
	if a.settingTab != "plugins" {
		a.mu.RUnlock()
		return false
	}
	state := a.pluginForm
	store := a.pluginsStore
	active := state != nil && state.active
	focused := -1
	fieldType := ""
	multiline := false
	if active {
		focused = state.focused
		if focused >= 0 && focused < len(state.definitions) {
			fieldType = state.definitions[focused].Type
			multiline = fieldType == "textbox" && state.definitions[focused].Value.MaxLines > 1
		}
	}
	a.mu.RUnlock()
	if event.Modifiers.HasPrimary() && (event.Key == woxui.Key("s") || event.Key == woxui.KeyEnter) {
		a.submitPluginSettings()
		return true
	}
	if !active {
		switch event.Key {
		case woxui.KeyArrowUp:
			a.movePluginSelection(-1)
			return true
		case woxui.KeyArrowDown:
			a.movePluginSelection(1)
			return true
		case woxui.KeyEnter:
			if store && state == nil {
				a.runSelectedPluginPrimaryOperation()
			} else {
				a.activatePluginForm()
			}
			return true
		case woxui.KeySpace:
			if store {
				a.runSelectedPluginPrimaryOperation()
			}
			return true
		case woxui.KeyTab:
			a.activatePluginForm()
			return true
		default:
			return false
		}
	}
	if event.Key == woxui.KeyEscape {
		a.deactivatePluginForm()
		return true
	}
	switch event.Key {
	case woxui.KeyTab, woxui.KeyArrowDown:
		if event.Key == woxui.KeyArrowDown && multiline {
			a.editPluginFormKey(event)
			break
		}
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.movePluginFormFocus(delta)
	case woxui.KeyArrowUp:
		if multiline {
			a.editPluginFormKey(event)
		} else {
			a.movePluginFormFocus(-1)
		}
	case woxui.KeyArrowLeft:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changePluginFormChoice(focused, -1)
		} else {
			a.editPluginFormKey(event)
		}
	case woxui.KeyArrowRight:
		if fieldType == "select" || fieldType == "selectAIModel" {
			a.changePluginFormChoice(focused, 1)
		} else {
			a.editPluginFormKey(event)
		}
	case woxui.KeySpace, woxui.KeyEnter:
		if event.Key == woxui.KeyEnter && multiline {
			a.editPluginFormKey(event)
		} else if fieldType == "table" {
			a.openPluginFormTable(focused)
		} else if fieldType == "dictationModel" || fieldType == "ocrModel" {
			a.openPluginModelManager(focused)
		} else if fieldType == "dictationHotkey" {
			a.recordPluginFormHotkey(focused)
		} else if fieldType == "checkbox" || fieldType == "select" || fieldType == "selectAIModel" {
			a.changePluginFormChoice(focused, 1)
		}
	default:
		a.editPluginFormKey(event)
	}
	return true
}

// recordPluginFormHotkey reuses core's dictation-aware recorder while keeping the value staged with other plugin changes.
func (a *App) recordPluginFormHotkey(index int) {
	a.mu.RLock()
	state := a.pluginForm
	if state == nil || index < 0 || index >= len(state.definitions) || state.definitions[index].Type != "dictationHotkey" {
		a.mu.RUnlock()
		return
	}
	target := &state.formFieldsState
	a.mu.RUnlock()
	a.startHotkeyRecording("plugin-settings", target, index, "", dictationHotkeyRecordingKinds)
}

// activatePluginForm transfers keyboard and IME ownership from the plugin list to its first field.
func (a *App) activatePluginForm() {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || state.saving || len(state.definitions) == 0 {
		a.mu.Unlock()
		return
	}
	index := state.focused
	if index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		for candidate, definition := range state.definitions {
			if formDefinitionFocusable(definition) {
				index = candidate
				break
			}
		}
	}
	if index < 0 || index >= len(state.definitions) {
		a.mu.Unlock()
		return
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
}

// deactivatePluginForm returns keyboard ownership to the settings page while preserving edits.
func (a *App) deactivatePluginForm() {
	a.mu.Lock()
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		a.pluginForm.active = false
	}
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) focusPluginFormField(index int) {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || state.saving || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.status = ""
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
}

func (a *App) movePluginFormFocus(delta int) {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || len(state.definitions) == 0 {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	index := state.focused
	for step := 0; step < len(state.definitions); step++ {
		index = (index + delta + len(state.definitions)) % len(state.definitions)
		if formDefinitionFocusable(state.definitions[index]) {
			setFormFieldsFocusLocked(&state.formFieldsState, index)
			break
		}
	}
	textInput := state.editor != nil
	a.mu.Unlock()
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
}

func (a *App) changePluginFormChoice(index, delta int) {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || !state.active || state.saving {
		a.mu.Unlock()
		return
	}
	changeFormFieldsChoiceLocked(&state.formFieldsState, index, delta)
	state.status = ""
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) editPluginFormKey(event woxui.KeyEvent) {
	a.mu.Lock()
	if state := a.pluginForm; state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.status = ""
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// onPluginSettingsTextInput commits native IME events only while a plugin textbox owns focus.
func (a *App) onPluginSettingsTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	state := a.pluginForm
	if !a.settingsOpen || a.settingTab != "plugins" || state == nil || !state.active {
		a.mu.Unlock()
		return false
	}
	if state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) && formDefinitionTextEditable(state.definitions[state.focused]) {
		if state.editor.HandleTextInput(event) {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.status = ""
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return true
}

func (a *App) setPluginFormCaret(index, offset int) {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || !state.active || state.focused != index || state.editor == nil {
		a.mu.Unlock()
		return
	}
	state.editor.SetCaret(offset)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// submitPluginSettings saves only changed keys, then reloads dynamic definitions from core.
func (a *App) submitPluginSettings() {
	a.mu.Lock()
	state := a.pluginForm
	if state == nil || state.saving || a.pluginOperation != "" {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationKey := validateFormFields(state.definitions, state.values); validationKey != "" {
		pluginID := state.pluginID
		a.mu.Unlock()
		message := a.translate(validationKey)
		a.mu.Lock()
		if a.pluginForm != nil && a.pluginForm.pluginID == pluginID {
			a.pluginForm.status = message
			a.pluginForm.statusError = true
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	values := make(map[string]string)
	for _, key := range editableFormKeys(state.definitions) {
		if state.values[key] != state.initial[key] {
			values[key] = state.values[key]
		}
	}
	if err := rewriteDictationSaveValues(state.pluginID, state.values, state.initial, values); err != nil {
		state.status = "Could not prepare dictation actions: " + err.Error()
		state.statusError = true
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	if len(values) == 0 {
		state.status = "No changes to save."
		state.statusError = false
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	state.saving = true
	state.status = "Saving…"
	state.statusError = false
	state.active = false
	state.revision++
	revision := state.revision
	pluginID := state.pluginID
	store := a.pluginsStore
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var saveErr error
		for _, key := range keys {
			if err := a.client.Post(ctx, "/setting/plugin/update", map[string]string{"PluginId": pluginID, "Key": key, "Value": values[key]}, nil); err != nil {
				saveErr = fmt.Errorf("save %s: %w", key, err)
				break
			}
		}
		if saveErr == nil {
			saveErr = a.reloadPlugins(store, pluginID)
		}
		a.mu.Lock()
		if a.pluginForm != nil && a.pluginForm.pluginID == pluginID {
			if a.pluginForm.revision == revision || saveErr == nil {
				a.pluginForm.saving = false
			}
			if saveErr != nil {
				a.pluginForm.status = saveErr.Error()
				a.pluginForm.statusError = true
			} else {
				a.pluginForm.status = "Saved"
				a.pluginForm.statusError = false
			}
		}
		a.mu.Unlock()
		if saveErr != nil {
			log.Printf("save plugin settings: %v", saveErr)
		}
		a.invalidateSettingsWindow()
	}()
}
