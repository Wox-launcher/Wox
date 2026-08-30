package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type pluginSettingsPlugin struct {
	ID                 string             `json:"Id"`
	Name               string             `json:"Name"`
	NameEn             string             `json:"NameEn"`
	Description        string             `json:"Description"`
	DescriptionEn      string             `json:"DescriptionEn"`
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
// Keyword matching follows the action panel: fuzzy + optional pinyin on localized text, and English
// names/descriptions always stay searchable.
func filterPlugins(plugins []pluginSettingsPlugin, query string, filters pluginFilterState, store bool, usePinYin bool) []filteredPlugin {
	query = strings.TrimSpace(query)
	filtered := make([]filteredPlugin, 0, len(plugins))
	for index, plugin := range plugins {
		if pluginMatchesQuery(plugin, query, usePinYin) && pluginMatchesFilters(plugin, filters, store) {
			filtered = append(filtered, filteredPlugin{index: index, plugin: plugin})
		}
	}
	return filtered
}

// pluginMatchesQuery keeps catalog search aligned with action-panel matching.
func pluginMatchesQuery(plugin pluginSettingsPlugin, query string, usePinYin bool) bool {
	if query == "" {
		return true
	}
	for _, text := range pluginSearchTexts(plugin) {
		if text != "" && util.IsStringMatch(text, query, usePinYin) {
			return true
		}
	}
	return false
}

// pluginSearchTexts includes localized fields plus English aliases used by the action panel.
func pluginSearchTexts(plugin pluginSettingsPlugin) []string {
	texts := []string{plugin.Name, plugin.NameEn, plugin.ID, plugin.Author, plugin.Description, plugin.DescriptionEn, plugin.Runtime}
	return append(texts, plugin.TriggerKeywords...)
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

// reloadPlugins fetches either the store or installed catalog through the controller.
// The controller owns loading/error state; the App keeps the post-load side effects that
// touch cross-domain state (settingSearchPlugins mirror, AI model loading, form rebuild
// via setPluginSelectionLocked) in one UI-thread transaction.
func (a *App) reloadPlugins(store bool, preferredID string) error {
	if err := a.pluginSettings.ReloadPlugins(context.Background(), a.services, a.sessionID, store, preferredID); err != nil {
		return err
	}

	requestModels := false
	requestProviders := false
	if err := a.runOnUI("apply loaded plugin catalog", func() {
		plugins := a.pluginSettings.Plugins()
		if !store {
			a.settingsSearch.SetPlugins(plugins)
			a.settingsSearch.SetLoading(false)
			a.settingsSearch.SetLoaded(true)
			a.settingsSearch.SetError("")
		}
		selected := a.pluginSettings.Selected()
		if selected >= 0 && selected < len(plugins) {
			a.setPluginSelectionLocked(selected)
		}
		form := a.pluginSettings.Form()
		requestProviders = form != nil && hasFormDefinitionType(form.definitions, "selectAIModel")
		requestModels = requestProviders && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
		if requestModels {
			a.aiSettings.SetModelsLoading(true)
		}
		a.invalidateSettingsWindow()
	}); err != nil {
		return err
	}
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for plugin settings", a.loadAIModels)
	}
	if requestProviders {
		util.Go(a.lifecycleCtx, "load AI provider icons for plugin settings", a.loadAIProviderCatalog)
	}
	return nil
}

func pluginSettingsPathIsStore(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/plugins/store" || path == "plugins.store"
}

// switchPluginList swaps the shared list between installed and store data without duplicating its UI state.
func (a *App) switchPluginList(store bool) {
	form := a.pluginSettings.Form()
	if a.pluginSettings.Operation() != "" || a.pluginSettings.PluginsLoading() || (a.pluginSettings.PluginsStore() == store && a.pluginSettings.PluginsLoaded()) {
		return
	}
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	plugins, loaded := a.pluginSettings.CachedPlugins(store)
	if !loaded && !store && a.settingsSearch.Loaded() {
		plugins = a.settingsSearch.Plugins()
		a.pluginSettings.cachePlugins(false, plugins)
		loaded = true
	}
	a.pluginSettings.SetPluginsStore(store)
	a.pluginSettings.SetPlugins(plugins)
	a.pluginSettings.SetPluginsLoaded(loaded)
	a.pluginSettings.SetPluginsLoading(!loaded)
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
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	if loaded {
		if len(plugins) > 0 {
			a.setPluginSelectionLocked(0)
		}
		a.invalidateSettingsWindow()
		return
	}
	util.Go(a.lifecycleCtx, "switch plugin list", func() {
		if err := a.reloadPlugins(store, ""); err != nil {
			log.Printf("switch plugin list: %v", err)
		}
	})
}

// runPluginOperation uses the same core install operation for fresh installs and upgrades.
func (a *App) runPluginOperation(kind string) {
	plugins := a.pluginSettings.Plugins()
	selected := a.pluginSettings.Selected()
	form := a.pluginSettings.Form()
	if a.pluginSettings.Operation() != "" || selected < 0 || selected >= len(plugins) {
		return
	}
	plugin := plugins[selected]
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	switch kind {
	case "install":
		if plugin.IsInstalled {
			return
		}
	case "upgrade":
		if !plugin.IsInstalled || !plugin.IsUpgradable {
			return
		}
	case "uninstall":
		if !plugin.IsInstalled || plugin.IsSystem {
			return
		}
		if a.pluginSettings.UninstallArmed() != plugin.ID {
			a.pluginSettings.SetUninstallArmed(plugin.ID)
			a.invalidateSettingsWindow()
			return
		}
	case "enable":
		if !plugin.IsInstalled || !plugin.IsDisable {
			return
		}
	case "disable":
		if !plugin.IsInstalled || plugin.IsDisable {
			return
		}
	default:
		return
	}
	a.pluginSettings.SetUninstallArmed("")
	a.pluginSettings.SetOperationError("")
	a.pluginSettings.SetOperation(kind + ":" + plugin.ID)
	store := a.pluginSettings.PluginsStore()
	a.invalidateSettingsWindow()

	util.Go(a.lifecycleCtx, kind+" plugin", func() {
		operation := contract.PluginOperation(kind)
		if kind == "upgrade" {
			operation = contract.PluginOperationInstall
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := a.services.OperatePlugin(ctx, a.sessionID, plugin.ID, operation)
		cancel()
		if err == nil {
			err = a.reloadPlugins(store, plugin.ID)
		}
		if err == nil {
			_ = a.runOnUI("invalidate related plugin catalog", func() {
				a.pluginSettings.invalidateCachedPlugins(!store)
				if store {
					a.settingsSearch.SetLoaded(false)
				}
			})
			util.Go(a.lifecycleCtx, "refresh related plugin catalog", func() {
				if preloadErr := a.pluginSettings.PreloadPlugins(a.lifecycleCtx, a.services, a.sessionID, !store); preloadErr != nil {
					log.Printf("refresh related plugin catalog: %v", preloadErr)
					return
				}
				if store {
					_ = a.runOnUI("refresh installed plugin search cache", func() {
						plugins, _ := a.pluginSettings.CachedPlugins(false)
						a.settingsSearch.SetPlugins(plugins)
						a.settingsSearch.SetLoaded(true)
						a.settingsSearch.SetError("")
					})
				}
			})
		}
		_ = a.runOnUI("apply plugin operation", func() {
			a.pluginSettings.SetOperation("")
			if err != nil {
				a.pluginSettings.SetOperationError(err.Error())
			} else {
				a.pluginSettings.SetOperationError("")
			}
			a.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("%s plugin %s: %v", kind, plugin.ID, err)
		}
	})
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
		a.pluginSettings.SetOperationError(err.Error())
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
	util.Go(a.lifecycleCtx, "open selected plugin directory", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err := a.services.OpenPath(ctx, a.sessionID, directory)
		cancel()
		if err != nil {
			_ = a.runOnUI("apply plugin directory error", func() {
				a.pluginSettings.SetOperationError(err.Error())
				a.invalidateSettingsWindow()
			})
		}
	})
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
// The controller's Form/Selected are swapped together on the UI thread so cross-domain readers
// (model_manager, form_table) observing the same form pointer stay in sync.
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
	definitions := pluginSettingsFormDefinitions(plugin)
	values := make(map[string]string, len(plugin.Setting.Settings)+2)
	triggerKeywords := plugin.Setting.TriggerKeywords
	if len(triggerKeywords) == 0 {
		triggerKeywords = plugin.TriggerKeywords
	}
	values["TriggerKeywords"] = encodePluginTriggerKeywordRows(triggerKeywords)
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

// pluginSettingsFormDefinitions separates plugin metadata controls from the
// manifest-defined Settings tab while retaining one shared save transaction.
func pluginSettingsFormDefinitions(plugin pluginSettingsPlugin) []formDefinition {
	// Plugin lifecycle actions belong to the detail header, while trigger keywords have
	// their own tab. Keeping only the keyword editor ahead of manifest definitions
	// preserves its save flow without duplicating either control in the Settings tab.
	definitions := []formDefinition{pluginTriggerKeywordDefinition()}
	definitions = append(definitions, plugin.SettingDefinitions...)
	allSettingsAreTables := len(plugin.SettingDefinitions) > 0
	for _, definition := range plugin.SettingDefinitions {
		if definition.Type != "table" {
			allSettingsAreTables = false
			break
		}
	}
	if allSettingsAreTables {
		for index := 1; index < len(definitions); index++ {
			definitions[index].Value.InlineTable = true
		}
	}
	return definitions
}

// pluginTriggerKeywordDefinition maps the built-in keyword editor onto the shared table flow.
func pluginTriggerKeywordDefinition() formDefinition {
	return formDefinition{Type: "table", Value: formDefinitionValue{
		Key: "TriggerKeywords", InlineTable: true, MaxHeight: 300, MinimumRowCount: 1, MinimumRowMessage: "i18n:ui_plugin_trigger_keyword_keep_one",
		Columns:       []formTableColumn{{Key: "keyword", Label: "i18n:ui_plugin_trigger_keyword_column", Tooltip: "i18n:ui_plugin_trigger_keyword_tooltip", Type: "text", TextMaxLines: 1, Validators: []formValidator{{Type: "not_empty"}}}},
		SortColumnKey: "keyword",
	}}
}

// encodePluginTriggerKeywordRows adapts core's string list to the portable table value.
func encodePluginTriggerKeywordRows(keywords []string) string {
	rows := make([]map[string]string, 0, len(keywords))
	for _, keyword := range keywords {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			rows = append(rows, map[string]string{"keyword": keyword})
		}
	}
	encoded, _ := json.Marshal(rows)
	return string(encoded)
}

// decodePluginTriggerKeywordRows adapts table rows back to core's normalized string list.
func decodePluginTriggerKeywordRows(value string) ([]string, error) {
	rows, err := decodeFormTableRows(value)
	if err != nil {
		return nil, err
	}
	keywords := make([]string, 0, len(rows))
	for _, row := range rows {
		keyword := strings.TrimSpace(fmt.Sprint(row["keyword"]))
		if keyword == "" {
			return nil, fmt.Errorf("trigger keyword must not be empty")
		}
		keywords = append(keywords, keyword)
	}
	return keywords, nil
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
	plugins := a.pluginSettings.Plugins()
	current := a.pluginSettings.Selected()
	if index < 0 || index >= len(plugins) || index == current {
		return
	}
	form := a.pluginSettings.Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	a.setPluginSelectionLocked(index)
	form = a.pluginSettings.Form()
	requestProviders := form != nil && hasFormDefinitionType(form.definitions, "selectAIModel")
	requestModels := requestProviders && !a.aiSettings.ModelsLoaded() && !a.aiSettings.ModelsLoading()
	if requestModels {
		a.aiSettings.SetModelsLoading(true)
	}
	a.updateSettingsTextInput(false)
	if requestModels {
		util.Go(a.lifecycleCtx, "load AI models for selected plugin", a.loadAIModels)
	}
	if requestProviders {
		util.Go(a.lifecycleCtx, "load AI provider icons for selected plugin", a.loadAIProviderCatalog)
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
	a.invalidateSettingsWindow()
}

// setPluginSearchValue applies accessibility value changes and resets the filtered viewport.
func (a *App) setPluginSearchValue(value string) error {
	editor := a.pluginSettings.SearchEditor()
	if editor == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(value))
	} else {
		editor.SetText(value, false)
	}
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearPluginSearch() {
	editor := a.pluginSettings.SearchEditor()
	if editor == nil {
		a.pluginSettings.SetSearchEditor(woxui.NewTextEditor(""))
	} else {
		editor.SetText("", false)
	}
	a.invalidateSettingsWindow()
}

// togglePluginFilterPanel shows or hides the catalog's anchored advanced filters.
func (a *App) togglePluginFilterPanel() {
	a.pluginSettings.SetFilterOpen(!a.pluginSettings.FilterOpen())
	a.pluginSettings.SetSearchFocused(false)
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) closePluginFilterPanel() {
	a.pluginSettings.SetFilterOpen(false)
	a.invalidateSettingsWindow()
}

// togglePluginFilter updates one filter while keeping the current detail selected whenever possible.
func (a *App) togglePluginFilter(id string) {
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
	filtered := filterPlugins(plugins, query, filters, store, a.usePinYin())
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
	a.invalidateSettingsWindow()
}

// refreshPluginCatalog preserves the search and selection while reloading the current catalog.
func (a *App) refreshPluginCatalog() {
	if a.pluginSettings.PluginsLoading() || a.pluginSettings.Operation() != "" {
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
	util.Go(a.lifecycleCtx, "refresh plugin catalog", func() {
		if err := a.reloadPlugins(store, preferredID); err != nil {
			log.Printf("refresh plugin catalog: %v", err)
		}
	})
}

func (a *App) blurPluginSearch() {
	a.pluginSettings.SetSearchFocused(false)
	host := a.settingsHost
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
	filtered := filterPlugins(plugins, query, filters, store, a.usePinYin())
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
	active := a.settingsOpen && a.settingTab == "plugins" && a.pluginSettings.SearchFocused() && a.pluginSettings.SearchEditor() != nil
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
	return a.settingsOpen && a.settingTab == "plugins" && a.pluginSettings.SearchFocused() && a.pluginSettings.SearchEditor() != nil
}

// selectPluginDetailTab changes detail content without discarding staged plugin settings.
func (a *App) selectPluginDetailTab(tab string) {
	switch tab {
	case "settings", "description", "keywords", "commands", "privacy":
	default:
		return
	}
	a.stopHotkeyRecording()
	a.pluginSettings.SetDetailTab(tab)
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	a.pluginSettings.SetSearchFocused(false)
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

// onPluginSettingsKey routes keys either to list navigation or the active shared field editor.
func (a *App) onPluginSettingsKey(event woxui.KeyEvent) bool {
	if a.onPluginSearchKey(event) {
		return true
	}
	if !event.Down || event.Composing {
		return false
	}
	if a.settingTab != "plugins" {
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
		} else if fieldType == "fileIndexService" {
			a.runFocusedPluginServiceAction(focused)
		} else if fieldType == "table" {
			a.openPluginFormTable(focused)
		} else if fieldType == "dictationModel" || fieldType == "ocrModel" {
			anchor := woxui.Rect{}
			if host := a.settingsHost; host != nil {
				anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", focused)))
			}
			a.openPluginModelManager(focused, anchor)
		} else if fieldType == "dictationHotkey" {
			a.recordPluginFormHotkey(focused)
		} else if fieldType == "select" || fieldType == "selectAIModel" {
			a.openFocusedPluginFormChoice(focused)
		} else if fieldType == "checkbox" {
			a.changePluginFormChoice(focused, 1)
		}
	default:
		a.editPluginFormKey(event)
	}
	return true
}

func (a *App) runFocusedPluginServiceAction(index int) {
	form := a.pluginSettings.Form()
	if form == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	for _, action := range form.definitions[index].Value.Actions {
		if action.Primary && action.Enabled {
			a.runPluginServiceAction(action.ID)
			return
		}
	}
}

// runPluginServiceAction executes one service lifecycle operation and reloads its dynamic state.
func (a *App) runPluginServiceAction(actionID string) {
	form := a.pluginSettings.Form()
	if form == nil || form.saving || actionID == "" {
		return
	}
	pluginID := form.pluginID
	form.saving = true
	form.status = ""
	form.statusError = false
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "run plugin service action", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := a.services.ExecutePluginSettingAction(ctx, a.sessionID, pluginID, actionID)
		cancel()
		if err == nil {
			err = a.reloadPlugins(false, pluginID)
		}
		_ = a.runOnUI("finish plugin service action", func() {
			if current := a.pluginSettings.Form(); current != nil && current.pluginID == pluginID {
				current.saving = false
				if err != nil {
					current.status = err.Error()
					current.statusError = true
				}
			}
			a.invalidateSettingsWindow()
		})
	})
}

// recordPluginFormHotkey reuses core's dictation-aware recorder while keeping the value staged with other plugin changes.
func (a *App) recordPluginFormHotkey(index int) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) || state.definitions[index].Type != "dictationHotkey" {
		return
	}
	target := &state.formFieldsState
	a.startHotkeyRecording("plugin-settings", target, index, "", dictationHotkeyRecordingKinds)
}

// activatePluginForm transfers keyboard and IME ownership from the plugin list to its first field.
func (a *App) activatePluginForm() {
	state := a.pluginSettings.Form()
	if state == nil || len(state.definitions) == 0 {
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
		return
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	textInput := state.editor != nil
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
}

// deactivatePluginForm returns keyboard ownership to the settings page while preserving edits.
func (a *App) deactivatePluginForm() {
	a.stopHotkeyRecording()
	if form := a.pluginSettings.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) focusPluginFormField(index int) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		return
	}
	a.stopHotkeyRecordingForDifferentField(&state.formFieldsState, index)
	previousFocused := state.focused
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.status = ""
	textInput := state.editor != nil
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
	if previousFocused != index && pluginFormDirty(state.definitions, state.values, state.initial) {
		a.submitPluginSettings()
	}
}

func (a *App) movePluginFormFocus(delta int) {
	state := a.pluginSettings.Form()
	if state == nil || len(state.definitions) == 0 {
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
	if host := a.settingsHost; host != nil {
		host.RequestFocus(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", index)))
	}
	a.stopHotkeyRecordingForDifferentField(&state.formFieldsState, index)
	textInput := state.editor != nil
	a.updateSettingsTextInput(textInput)
	a.invalidateSettingsWindow()
}

func (a *App) changePluginFormChoice(index, delta int) {
	state := a.pluginSettings.Form()
	if state == nil || !state.active {
		return
	}
	changeFormFieldsChoiceLocked(&state.formFieldsState, index, delta)
	state.status = ""
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	a.submitPluginSettings()
}

// openFocusedPluginFormChoice resolves the retained field bounds for keyboard-opened menus.
func (a *App) openFocusedPluginFormChoice(index int) {
	anchor := woxui.Rect{}
	if host := a.settingsHost; host != nil {
		key := fmt.Sprintf("plugin-settings-field-%d", index)
		if form := a.pluginSettings.Form(); form != nil && index >= 0 && index < len(form.definitions) && form.definitions[index].Type == "selectAIModel" {
			key += "-model"
		}
		anchor, _ = host.BoundsForKey(woxwidget.Key(key))
	}
	a.openPluginFormChoice(index, anchor)
}

// openPluginFormChoice presents the shared anchored dropdown for a plugin select field.
func (a *App) openPluginFormChoice(index int, anchor woxui.Rect) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	if definition.Type == "selectAIModel" {
		a.openPluginAIModelChoice(index, false, anchor)
		return
	}
	if definition.Type != "select" || len(definition.Value.Options) == 0 {
		return
	}
	if anchor.Width <= 0 || anchor.Height <= 0 {
		if host := a.settingsHost; host != nil {
			anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", index)))
		}
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	choices := make([]settingChoice, len(definition.Value.Options))
	for optionIndex, option := range definition.Value.Options {
		label := a.translate(option.Label)
		if strings.TrimSpace(label) == "" {
			label = option.Value
		}
		choices[optionIndex] = settingChoice{value: option.Value, label: label}
	}
	item := settingItem{
		key: "plugin:" + state.pluginID + ":" + definition.Value.Key, title: a.translate(definition.Value.Label),
		value: state.values[definition.Value.Key], choices: choices,
	}
	a.generalSettings.SetChoicePicker(&settingChoicePickerState{
		item: item, anchor: anchor,
		onChoose: func(choice settingChoice) { a.setPluginFormChoice(index, choice.value) },
	})
	state.status = ""
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func aiModelProviderKey(model aiModel) string {
	return model.Provider + "\x00" + model.ProviderAlias
}

// openPluginAIModelChoice opens the provider or filterable model menu for one AI model field.
func (a *App) openPluginAIModelChoice(index int, providerChoice bool, anchor woxui.Rect) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	if definition.Type != "selectAIModel" {
		return
	}
	models := aiModelsFromOptions(definition.Value.Options)
	if len(models) == 0 {
		return
	}
	var selected aiModel
	_ = json.Unmarshal([]byte(state.values[definition.Value.Key]), &selected)
	selectedProvider := aiModelProviderKey(selected)
	choices := make([]settingChoice, 0, len(models))
	icons := make(map[string]woxImage)
	if providerChoice {
		seen := make(map[string]bool)
		for _, model := range models {
			key := aiModelProviderKey(model)
			if seen[key] {
				continue
			}
			seen[key] = true
			label := model.Provider
			if model.ProviderAlias != "" {
				label = model.ProviderAlias
			}
			choices = append(choices, settingChoice{value: key, label: label})
		}
		for _, provider := range a.aiSettings.ProviderCatalog() {
			for _, choice := range choices {
				providerName, _, _ := strings.Cut(choice.value, "\x00")
				if provider.Name == providerName {
					icons[choice.value] = provider.Icon
				}
			}
		}
	} else {
		for _, model := range models {
			if selectedProvider != "\x00" && aiModelProviderKey(model) != selectedProvider {
				continue
			}
			encoded, err := json.Marshal(model)
			if err == nil {
				choices = append(choices, settingChoice{value: string(encoded), label: model.Name})
			}
		}
		if len(choices) == 0 && selected.Name != "" {
			choices = append(choices, settingChoice{value: state.values[definition.Value.Key], label: selected.Name})
		}
		for _, provider := range a.aiSettings.ProviderCatalog() {
			if provider.Name != selected.Provider {
				continue
			}
			for _, choice := range choices {
				icons[choice.value] = provider.Icon
			}
		}
	}
	if len(choices) == 0 {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	currentValue := state.values[definition.Value.Key]
	if providerChoice {
		currentValue = selectedProvider
	}
	a.generalSettings.SetChoicePicker(&settingChoicePickerState{
		item: settingItem{
			key: "plugin-ai-model:" + definition.Value.Key, title: a.translate(definition.Value.Label), value: currentValue,
			choices: choices, icons: icons, preserveIconColor: true, filterable: !providerChoice,
		},
		anchor: anchor,
		onChoose: func(choice settingChoice) {
			if providerChoice {
				a.setPluginAIModelProvider(index, choice.value)
			} else {
				a.setPluginFormChoice(index, choice.value)
			}
		},
	})
	state.status = ""
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) setPluginAIModelProvider(index int, providerKey string) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	for _, model := range aiModelsFromOptions(state.definitions[index].Value.Options) {
		if aiModelProviderKey(model) == providerKey {
			encoded, err := json.Marshal(model)
			if err == nil {
				a.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
}

func (a *App) setPluginAIModelName(index int, name string) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) || strings.TrimSpace(name) == "" {
		return
	}
	definition := state.definitions[index]
	var model aiModel
	if json.Unmarshal([]byte(state.values[definition.Value.Key]), &model) != nil || model.Provider == "" {
		return
	}
	model.Name = name
	encoded, err := json.Marshal(model)
	if err != nil {
		return
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.values[definition.Value.Key] = string(encoded)
	state.status = ""
	a.invalidateSettingsWindow()
	a.submitPluginSettings()
}

func (a *App) finishPluginAIModelEdit(index int, name string) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	var selected aiModel
	_ = json.Unmarshal([]byte(state.values[definition.Value.Key]), &selected)
	models := aiModelsFromOptions(definition.Value.Options)
	for _, model := range models {
		if aiModelProviderKey(model) == aiModelProviderKey(selected) && model.Name == name {
			encoded, err := json.Marshal(model)
			if err == nil {
				a.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
	for _, model := range models {
		if aiModelProviderKey(model) == aiModelProviderKey(selected) {
			encoded, err := json.Marshal(model)
			if err == nil {
				a.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
}

// setPluginFormChoice stages one exact dropdown value without depending on option ordering.
func (a *App) setPluginFormChoice(index int, value string) {
	state := a.pluginSettings.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	found := false
	for _, option := range definition.Value.Options {
		if option.Value == value {
			found = true
			break
		}
	}
	if !found {
		return
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.values[definition.Value.Key] = value
	state.status = ""
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	a.submitPluginSettings()
}

func (a *App) editPluginFormKey(event woxui.KeyEvent) {
	state := a.pluginSettings.Form()
	if state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.status = ""
		}
	}
	a.invalidateSettingsWindow()
}

// onPluginSettingsTextInput commits native IME events only while a plugin textbox owns focus.
func (a *App) onPluginSettingsTextInput(_ woxui.TextInputEvent) bool {
	state := a.pluginSettings.Form()
	active := a.settingsOpen && a.settingTab == "plugins" && state != nil && state.active
	return active
}

func (a *App) setPluginFormText(index int, value string) {
	form := a.pluginSettings.Form()
	changed := form != nil && setFormFieldsTextLocked(&form.formFieldsState, index, value)
	if !changed {
		return
	}
	form.status = ""
	a.invalidateSettingsWindow()
	// dirPath is a completed folder, like a select: persist immediately so the next
	// launcher query does not keep using the previous directory.
	if form.definitions[index].Type == "dirPath" {
		a.submitPluginSettings()
	}
}

// pickPluginFormDirectory fills a plugin dirPath setting from the native folder picker.
func (a *App) pickPluginFormDirectory(index int) {
	form := a.pluginSettings.Form()
	if form == nil || index < 0 || index >= len(form.definitions) || form.definitions[index].Type != "dirPath" {
		return
	}
	window := a.settingsNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Directory: true})
	if a.pluginSettings.Form() != form {
		return
	}
	if err != nil {
		form.status = err.Error()
		a.invalidateSettingsWindow()
		return
	}
	if path != "" {
		a.setPluginFormText(index, path)
	}
}

// preparePluginSettingSaveValues separates staged form values from the normalized payload sent to core.
func preparePluginSettingSaveValues(state *pluginSettingsFormState) (map[string]string, map[string]string, error) {
	submitted := make(map[string]string)
	for _, key := range editableFormKeys(state.definitions) {
		if state.values[key] != state.initial[key] {
			submitted[key] = state.values[key]
		}
	}
	persisted := make(map[string]string, len(submitted))
	for key, value := range submitted {
		persisted[key] = value
	}
	if value, ok := persisted["TriggerKeywords"]; ok {
		keywords, err := decodePluginTriggerKeywordRows(value)
		if err != nil {
			return nil, nil, err
		}
		persisted["TriggerKeywords"] = strings.Join(keywords, ",")
	}
	if err := rewriteDictationSaveValues(state.pluginID, state.values, state.initial, persisted); err != nil {
		return nil, nil, err
	}
	return submitted, persisted, nil
}

// validatePluginTriggerKeywordTableRow prevents installed plugins from claiming the same non-global route.
func (a *App) validatePluginTriggerKeywordTableRow(state *formTableEditorState) string {
	if state == nil || state.definition.Value.Key != "TriggerKeywords" || state.rowForm == nil {
		return ""
	}
	keyword := strings.TrimSpace(state.rowForm.values["keyword"])
	state.rowForm.values["keyword"] = keyword
	if keyword == "" || keyword == "*" {
		return ""
	}
	for index, row := range state.rows {
		if index != state.rowIndex && strings.TrimSpace(fmt.Sprint(row["keyword"])) == keyword {
			return a.translate("i18n:ui_plugin_trigger_keyword_duplicate_in_plugin")
		}
	}
	pluginForm := a.pluginSettings.Form()
	if pluginForm == nil {
		return ""
	}
	for _, plugin := range a.pluginSettings.Plugins() {
		if plugin.ID == pluginForm.pluginID {
			continue
		}
		for _, existing := range plugin.TriggerKeywords {
			if strings.TrimSpace(existing) == keyword {
				name := strings.TrimSpace(plugin.Name)
				if name == "" {
					name = plugin.ID
				}
				return fmt.Sprintf(a.translate("i18n:ui_plugin_trigger_keyword_duplicate_in_other_plugin"), name)
			}
		}
	}
	return ""
}

// submitPluginSettings serializes auto-saves while retaining edits made during an in-flight request.
func (a *App) submitPluginSettings() {
	state := a.pluginSettings.Form()
	if state == nil || state.saving || a.pluginSettings.Operation() != "" {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if validationKey := validateFormFields(state.definitions, state.values); validationKey != "" {
		pluginID := state.pluginID
		message := a.translate(validationKey)
		if form := a.pluginSettings.Form(); form != nil && form.pluginID == pluginID {
			form.status = message
			form.statusError = true
		}
		a.invalidateSettingsWindow()
		return
	}
	submittedValues, persistedValues, err := preparePluginSettingSaveValues(state)
	if err != nil {
		state.status = "Could not prepare dictation actions: " + err.Error()
		state.statusError = true
		a.invalidateSettingsWindow()
		return
	}
	if len(persistedValues) == 0 {
		return
	}
	state.saving = true
	state.status = ""
	state.statusError = false
	state.revision++
	pluginID := state.pluginID
	a.invalidateSettingsWindow()

	util.Go(a.lifecycleCtx, "save plugin settings", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		saveErr := a.services.UpdatePluginSettings(ctx, a.sessionID, pluginID, persistedValues)
		_ = a.runOnUI("apply plugin settings save", func() {
			form := a.pluginSettings.Form()
			if form != nil && form.pluginID == pluginID {
				form.saving = false
				if saveErr != nil {
					form.status = saveErr.Error()
					form.statusError = true
				} else {
					for key, value := range submittedValues {
						form.initial[key] = value
					}
					form.status = ""
					form.statusError = false
				}
			}
			if saveErr == nil {
				a.applySavedPluginSettingValues(pluginID, persistedValues)
				if form != nil && form.pluginID == pluginID && pluginFormDirty(form.definitions, form.values, form.initial) {
					a.submitPluginSettings()
				}
			}
			a.invalidateSettingsWindow()
		})
		if saveErr != nil {
			log.Printf("save plugin settings: %v", saveErr)
			return
		}
		// Dynamic settings may depend on any saved value, so always resolve them again.
		a.refreshPluginFormDefinitions(pluginID)
	})
}

// refreshPluginFormDefinitions reloads one installed plugin's setting definitions without flashing the catalog.
func (a *App) refreshPluginFormDefinitions(pluginID string) {
	plugins, err := loadPluginSettingsPlugins(a.lifecycleCtx, a.services, a.sessionID, false)
	if err != nil {
		log.Printf("refresh plugin form definitions: %v", err)
		return
	}
	_ = a.runOnUI("refresh plugin form definitions", func() {
		current := a.pluginSettings.Plugins()
		for index := range current {
			if current[index].ID != pluginID {
				continue
			}
			for _, updated := range plugins {
				if updated.ID != pluginID {
					continue
				}
				current[index].SettingDefinitions = updated.SettingDefinitions
				current[index].Setting = updated.Setting
				break
			}
			a.pluginSettings.SetPlugins(current)
			a.pluginSettings.cachePlugins(false, current)
			a.settingsSearch.SetPlugins(current)
			if form := a.pluginSettings.Form(); form != nil && form.pluginID == pluginID {
				a.setPluginSelectionLocked(a.pluginSettings.Selected())
			}
			break
		}
		a.invalidateSettingsWindow()
	})
}

// applySavedPluginSettingValues keeps the local catalog consistent until its next normal refresh.
func (a *App) applySavedPluginSettingValues(pluginID string, values map[string]string) {
	plugins := a.pluginSettings.Plugins()
	for index := range plugins {
		if plugins[index].ID != pluginID {
			continue
		}
		if plugins[index].Setting.Settings == nil {
			plugins[index].Setting.Settings = make(map[string]string)
		}
		for key, value := range values {
			if key == "TriggerKeywords" {
				keywords := splitPluginTriggerKeywords(value)
				plugins[index].Setting.TriggerKeywords = keywords
				plugins[index].TriggerKeywords = append([]string(nil), keywords...)
				continue
			}
			plugins[index].Setting.Settings[key] = value
		}
		break
	}
	a.pluginSettings.SetPlugins(plugins)
	a.pluginSettings.cachePlugins(a.pluginSettings.PluginsStore(), plugins)
	a.settingsSearch.SetPlugins(plugins)
}

// splitPluginTriggerKeywords normalizes the comma-separated core representation.
func splitPluginTriggerKeywords(value string) []string {
	parts := strings.Split(value, ",")
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		if keyword := strings.TrimSpace(part); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	return keywords
}
