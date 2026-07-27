package launcher

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

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
// The controller owns loading/error state; the App keeps the post-load side effects that
// touch cross-domain state (settingSearchPlugins mirror, AI model loading, form rebuild
// via setPluginSelectionLocked) since those need a.mu coordination the controller cannot do.
func (a *App) reloadPlugins(store bool, preferredID string) error {
	a.mu.Lock()
	a.pluginSettings.SetPluginsLoading(true)
	a.pluginSettings.SetPluginsError("")
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
		a.pluginSettings.SetPluginsLoading(false)
		a.pluginSettings.SetPluginsLoaded(false)
		a.pluginSettings.SetPluginsError(err.Error())
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
	if preferredID == "" {
		selected := a.pluginSettings.Selected()
		current := a.pluginSettings.Plugins()
		if selected >= 0 && selected < len(current) {
			preferredID = current[selected].ID
		}
	}
	a.pluginSettings.SetPlugins(plugins)
	a.pluginSettings.SetPluginsLoading(false)
	a.pluginSettings.SetPluginsLoaded(true)
	a.pluginSettings.SetPluginsError("")
	if !store {
		a.settingsSearch.SetPlugins(plugins)
		a.settingsSearch.SetLoading(false)
		a.settingsSearch.SetLoaded(true)
		a.settingsSearch.SetError("")
	}
	a.pluginSettings.SetOperationError("")
	if a.pluginSettings.SearchEditor() == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
	}
	if a.pluginSettings.DetailTab() == "" {
		a.pluginSettings.SetDetailTab("settings")
	}
	selected := 0
	for index, plugin := range plugins {
		if plugin.ID == preferredID {
			selected = index
			break
		}
	}
	if len(plugins) == 0 {
		a.pluginSettings.SetSelected(-1)
		a.pluginSettings.SetForm(nil)
	} else {
		a.setPluginSelectionLocked(selected)
	}
	form := a.pluginSettings.Form()
	requestModels := form != nil && hasFormDefinitionType(form.definitions, "selectAIModel") && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
	if requestModels {
		a.aiSettings.SetModelsLoading(true)
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
	form := a.pluginSettings.Form()
	if a.pluginSettings.Operation() != "" || a.pluginSettings.PluginsLoading() || (a.pluginSettings.PluginsStore() == store && a.pluginSettings.PluginsLoaded()) {
		a.mu.Unlock()
		return
	}
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			form.status = "Save the current plugin changes before switching lists."
			form.statusError = true
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	a.pluginSettings.SetPluginsStore(store)
	a.pluginSettings.SetPlugins(nil)
	a.pluginSettings.SetPluginsLoaded(false)
	a.pluginSettings.SetPluginsLoading(true)
	a.pluginSettings.SetPluginsError("")
	a.pluginSettings.SetSelected(-1)
	a.pluginSettings.SetForm(nil)
	a.pluginSettings.SetUninstallArmed("")
	a.pluginSettings.SetOperationError("")
	a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
	a.pluginSettings.SetSearchFocused(true)
	a.settingsSearch.SetFocused(false)
	a.settingsSearch.SetPanel(false)
	a.pluginSettings.SetFilterOpen(false)
	if store {
		a.pluginSettings.SetDetailTab("description")
	} else {
		a.pluginSettings.SetDetailTab("settings")
	}
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
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	form := a.pluginSettings.Form()
	if a.pluginSettings.Operation() != "" || selected < 0 || selected >= len(plugins) {
		a.mu.Unlock()
		return
	}
	plugin := plugins[selected]
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			form.status = "Save the current plugin changes before managing this plugin."
			form.statusError = true
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
		if a.pluginSettings.UninstallArmed() != plugin.ID {
			a.pluginSettings.SetUninstallArmed(plugin.ID)
			a.settingNote = "Press Confirm uninstall to remove " + plugin.Name + "."
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	default:
		a.mu.Unlock()
		return
	}
	a.pluginSettings.SetUninstallArmed("")
	a.pluginSettings.SetOperationError("")
	a.pluginSettings.SetOperation(kind + ":" + plugin.ID)
	store := a.pluginSettings.PluginsStore()
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
		a.pluginSettings.SetOperation("")
		if err != nil {
			a.pluginSettings.SetOperationError(err.Error())
		} else {
			a.pluginSettings.SetOperationError("")
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
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	target := strings.TrimSpace(plugins[selected].Website)
	if target == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		a.mu.Lock()
		a.pluginSettings.SetOperationError(err.Error())
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}

// openSelectedPluginDirectory delegates reveal behavior to core's cross-platform shell adapter.
func (a *App) openSelectedPluginDirectory() {
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	directory := strings.TrimSpace(plugins[selected].PluginDirectory)
	if directory == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.client.Post(ctx, "/open", map[string]string{"path": directory}, nil)
		cancel()
		if err != nil {
			a.mu.Lock()
			a.pluginSettings.SetOperationError(err.Error())
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
}

// runSelectedPluginPrimaryOperation gives keyboard users the same install or upgrade action as the detail button.
func (a *App) runSelectedPluginPrimaryOperation() {
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	plugin := plugins[selected]
	if !plugin.IsInstalled {
		a.runPluginOperation("install")
	} else if plugin.IsUpgradable {
		a.runPluginOperation("upgrade")
	}
}

// setPluginSelectionLocked replaces the editor state with one plugin's current persisted values.
// Caller must hold a.mu; the controller's Form/Selected are swapped atomically under a.mu so
// cross-domain readers (model_manager, form_table) observing the same form pointer stay in sync.
func (a *App) setPluginSelectionLocked(index int) {
	plugins := a.pluginSettings.Plugins()
	if index < 0 || index >= len(plugins) {
		return
	}
	a.aiSettings.SetModelManager(nil)
	a.pluginSettings.SetDetailTab("settings")
	plugin := plugins[index]
	if a.pluginSettings.PluginsStore() {
		a.pluginSettings.SetDetailTab("description")
		a.pluginSettings.SetSelected(index)
		a.pluginSettings.SetForm(nil)
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
	if models := a.aiSettings.Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&fields, models)
	}
	initial := make(map[string]string, len(fields.values))
	for key, value := range fields.values {
		initial[key] = value
	}
	a.pluginSettings.SetSelected(index)
	a.pluginSettings.SetForm(&pluginSettingsFormState{formFieldsState: fields, pluginID: plugin.ID, initial: initial})
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
	plugins := a.pluginSettings.Plugins()
	current := a.pluginSettings.Selected()
	if index < 0 || index >= len(plugins) || index == current {
		a.mu.Unlock()
		return
	}
	form := a.pluginSettings.Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			form.status = "Save the current plugin changes before selecting another plugin."
			form.statusError = true
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	}
	a.setPluginSelectionLocked(index)
	form = a.pluginSettings.Form()
	requestModels := form != nil && hasFormDefinitionType(form.definitions, "selectAIModel") && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
	if requestModels {
		a.aiSettings.SetModelsLoading(true)
	}
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	if requestModels {
		go a.loadAIModels()
	}
	a.invalidateSettingsWindow()
}

func (a *App) movePluginSelection(delta int) {
	count := len(a.pluginSettings.Plugins())
	selected := a.pluginSettings.Selected()
	if count == 0 {
		return
	}
	selected = (selected + delta + count) % count
	a.selectPlugin(selected)
}

// setPluginSearchFocused keeps plugin input routing aligned with retained focus changes.
func (a *App) setPluginSearchFocused(focused bool) {
	a.mu.Lock()
	if a.pluginSettings.SearchEditor() == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
	}
	a.pluginSettings.SetSearchFocused(focused)
	if focused {
		a.settingsSearch.SetFocused(false)
		a.settingsSearch.SetPanel(false)
		a.themeSettings.SetThemeSearchFocused(false)
		if form := a.pluginSettings.Form(); form != nil {
			syncFormFieldsEditorLocked(&form.formFieldsState)
			form.active = false
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setPluginSearchValue applies accessibility value changes and resets the filtered viewport.
func (a *App) setPluginSearchValue(value string) error {
	a.mu.Lock()
	editor := a.pluginSettings.SearchEditor()
	if editor == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(value))
	} else {
		editor.SetText(value, false)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearPluginSearch() {
	a.mu.Lock()
	editor := a.pluginSettings.SearchEditor()
	if editor == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
	} else {
		editor.SetText("", false)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// togglePluginFilterPanel shows or hides the catalog's anchored advanced filters.
func (a *App) togglePluginFilterPanel() {
	a.mu.Lock()
	a.pluginSettings.SetFilterOpen(!a.pluginSettings.FilterOpen())
	a.pluginSettings.SetSearchFocused(false)
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) closePluginFilterPanel() {
	a.mu.Lock()
	a.pluginSettings.SetFilterOpen(false)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// togglePluginFilter updates one filter while keeping the current detail selected whenever possible.
func (a *App) togglePluginFilter(id string) {
	a.mu.Lock()
	filters := a.pluginSettings.Filters()
	switch id {
	case "enabled":
		filters.enabledOnly = !filters.enabledOnly
	case "disabled":
		filters.disabledOnly = !filters.disabledOnly
	case "upgradable":
		filters.upgradableOnly = !filters.upgradableOnly
	case "uninstalled":
		filters.uninstalledOnly = !filters.uninstalledOnly
	case "third-party":
		filters.thirdPartyOnly = !filters.thirdPartyOnly
	case "runtime-nodejs":
		filters.runtimeNodeJSOnly = !filters.runtimeNodeJSOnly
	case "runtime-python":
		filters.runtimePythonOnly = !filters.runtimePythonOnly
	case "runtime-script":
		filters.runtimeScriptOnly = !filters.runtimeScriptOnly
	case "runtime-script-nodejs":
		filters.runtimeScriptNodeJSOnly = !filters.runtimeScriptNodeJSOnly
	case "runtime-script-python":
		filters.runtimeScriptPythonOnly = !filters.runtimeScriptPythonOnly
	default:
		a.mu.Unlock()
		return
	}
	a.pluginSettings.SetFilters(filters)
	query := ""
	if editor := a.pluginSettings.SearchEditor(); editor != nil {
		query = editor.State().Text
	}
	plugins := a.pluginSettings.Plugins()
	store := a.pluginSettings.PluginsStore()
	selected := a.pluginSettings.Selected()
	filtered := filterPlugins(plugins, query, filters, store)
	selectedVisible := false
	for _, entry := range filtered {
		if entry.index == selected {
			selectedVisible = true
			break
		}
	}
	if !selectedVisible && len(filtered) > 0 {
		a.setPluginSelectionLocked(filtered[0].index)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// refreshPluginCatalog preserves the search and selection while reloading the current catalog.
func (a *App) refreshPluginCatalog() {
	a.mu.Lock()
	if a.pluginSettings.PluginsLoading() || a.pluginSettings.Operation() != "" {
		a.mu.Unlock()
		return
	}
	store := a.pluginSettings.PluginsStore()
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	preferredID := ""
	if selected >= 0 && selected < len(plugins) {
		preferredID = plugins[selected].ID
	}
	a.pluginSettings.SetFilterOpen(false)
	a.mu.Unlock()
	go func() {
		if err := a.reloadPlugins(store, preferredID); err != nil {
			log.Printf("refresh plugin catalog: %v", err)
		}
	}()
}

func (a *App) blurPluginSearch() {
	a.mu.Lock()
	a.pluginSettings.SetSearchFocused(false)
	host := a.settingsHost
	a.mu.Unlock()
	if host != nil {
		host.ClearFocus()
	}
	a.invalidateSettingsWindow()
}

func (a *App) moveFilteredPluginSelection(delta int) {
	query := ""
	if editor := a.pluginSettings.SearchEditor(); editor != nil {
		query = editor.State().Text
	}
	plugins := append([]pluginSettingsPlugin(nil), a.pluginSettings.Plugins()...)
	selected := a.pluginSettings.Selected()
	filters := a.pluginSettings.Filters()
	store := a.pluginSettings.PluginsStore()
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
	active := a.settingsOpen && a.settingTab == "plugins" && a.pluginSettings.SearchFocused() && a.pluginSettings.SearchEditor() != nil
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
		return false
	}
	return true
}

func (a *App) onPluginSearchTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	active := a.settingsOpen && a.settingTab == "plugins" && a.pluginSettings.SearchFocused() && a.pluginSettings.SearchEditor() != nil
	a.mu.RUnlock()
	return active
}

// selectPluginDetailTab changes detail content without discarding staged plugin settings.
func (a *App) selectPluginDetailTab(tab string) {
	switch tab {
	case "settings", "description", "keywords", "commands", "privacy":
	default:
		return
	}
	a.mu.Lock()
	a.pluginSettings.SetDetailTab(tab)
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
	}
	a.pluginSettings.SetSearchFocused(false)
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
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
	state := a.pluginSettings.Form()
	store := a.pluginSettings.PluginsStore()
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
	textEditable := fieldType == "textbox" || fieldType == "password" || fieldType == "dirPath"
	if textEditable {
		switch event.Key {
		case woxui.KeyTab:
			delta := 1
			if event.Modifiers&woxui.KeyModifierShift != 0 {
				delta = -1
			}
			a.movePluginFormFocus(delta)
			return true
		case woxui.KeyArrowDown:
			if !multiline {
				a.movePluginFormFocus(1)
				return true
			}
		case woxui.KeyArrowUp:
			if !multiline {
				a.movePluginFormFocus(-1)
				return true
			}
		case woxui.KeyEnter:
			return !multiline
		}
		return false
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
	state := a.pluginSettings.Form()
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
	state := a.pluginSettings.Form()
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
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
	}
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) focusPluginFormField(index int) {
	a.mu.Lock()
	state := a.pluginSettings.Form()
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
	state := a.pluginSettings.Form()
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
	state := a.pluginSettings.Form()
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
	state := a.pluginSettings.Form()
	if state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
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
func (a *App) onPluginSettingsTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	state := a.pluginSettings.Form()
	active := a.settingsOpen && a.settingTab == "plugins" && state != nil && state.active
	a.mu.RUnlock()
	return active
}

func (a *App) setPluginFormText(index int, value string) {
	a.mu.Lock()
	form := a.pluginSettings.Form()
	changed := form != nil && setFormFieldsTextLocked(&form.formFieldsState, index, value)
	if changed {
		form.status = ""
	}
	a.mu.Unlock()
	if changed {
		a.invalidateSettingsWindow()
	}
}

// submitPluginSettings saves only changed keys, then reloads dynamic definitions from core.
func (a *App) submitPluginSettings() {
	a.mu.Lock()
	state := a.pluginSettings.Form()
	if state == nil || state.saving || a.pluginSettings.Operation() != "" {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationKey := validateFormFields(state.definitions, state.values); validationKey != "" {
		pluginID := state.pluginID
		a.mu.Unlock()
		message := a.translate(validationKey)
		a.mu.Lock()
		if form := a.pluginSettings.Form(); form != nil && form.pluginID == pluginID {
			form.status = message
			form.statusError = true
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
	store := a.pluginSettings.PluginsStore()
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
		form := a.pluginSettings.Form()
		if form != nil && form.pluginID == pluginID {
			if form.revision == revision || saveErr == nil {
				form.saving = false
			}
			if saveErr != nil {
				form.status = saveErr.Error()
				form.statusError = true
			} else {
				form.status = "Saved"
				form.statusError = false
			}
		}
		a.mu.Unlock()
		if saveErr != nil {
			log.Printf("save plugin settings: %v", saveErr)
		}
		a.invalidateSettingsWindow()
	}()
}
