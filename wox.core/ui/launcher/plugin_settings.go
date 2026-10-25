package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wox/plugin"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/shell"
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
	Tools              []pluginTool       `json:"Tools"`
	SupportedOS        []string           `json:"SupportedOS"`
	Features           []pluginFeature    `json:"Features"`
	Glances            []pluginGlance     `json:"Glances"`
	IsSystem           bool               `json:"IsSystem"`
	IsDev              bool               `json:"IsDev"`
	IsInstalled        bool               `json:"IsInstalled"`
	IsDisable          bool               `json:"IsDisable"`
	IsUpgradable       bool               `json:"IsUpgradable"`
	Store              string             `json:"Store"`
	SettingDefinitions []formDefinition   `json:"SettingDefinitions"`
	Setting            pluginSettingsData `json:"Setting"`
	HasNativeSettings  bool               `json:"HasNativeSettings"`
}

type pluginCommand struct {
	Command     string `json:"Command"`
	Description string `json:"Description"`
}

type pluginTool struct {
	Name        string
	Description string
}

type pluginFeature struct {
	Name   string         `json:"Name"`
	Params map[string]any `json:"Params"`
}

type filteredPlugin struct {
	index  int
	plugin pluginSettingsPlugin
}

const (
	pluginFilterAll = "all"

	pluginFilterEnabled  = "enabled"
	pluginFilterDisabled = "disabled"

	pluginFilterUpgradable    = "upgradable"
	pluginFilterNotUpgradable = "not-upgradable"

	pluginFilterSystem     = "system"
	pluginFilterThirdParty = "third-party"

	pluginFilterInstalled   = "installed"
	pluginFilterUninstalled = "uninstalled"

	pluginFilterRuntimeNodeJS = "nodejs"
	pluginFilterRuntimePython = "python"
	pluginFilterRuntimeScript = "script"
)

// pluginFilterState stores exclusive catalog dropdowns. Empty or "all" means no restriction.
type pluginFilterState struct {
	enabledStatus string
	upgradeStatus string
	pluginType    string
	runtime       string
	installStatus string
}

func pluginFilterIsAll(value string) bool {
	return value == "" || value == pluginFilterAll
}

func (filters pluginFilterState) applied(store bool) bool {
	if store {
		return !pluginFilterIsAll(filters.installStatus) || !pluginFilterIsAll(filters.pluginType) || !pluginFilterIsAll(filters.runtime)
	}
	return !pluginFilterIsAll(filters.enabledStatus) || !pluginFilterIsAll(filters.upgradeStatus) || !pluginFilterIsAll(filters.pluginType) || !pluginFilterIsAll(filters.runtime)
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

const (
	pluginSectionEnabled  = pluginFilterEnabled
	pluginSectionDisabled = pluginFilterDisabled
)

type installedPluginSection struct {
	ID      string
	Plugins []filteredPlugin
}

// groupInstalledPlugins splits the visible installed catalog into Enabled, then Disabled.
// Empty groups are omitted so search and filters can leave a single section.
func groupInstalledPlugins(filtered []filteredPlugin) []installedPluginSection {
	enabled := make([]filteredPlugin, 0, len(filtered))
	disabled := make([]filteredPlugin, 0, len(filtered))
	for _, entry := range filtered {
		if entry.plugin.IsDisable {
			disabled = append(disabled, entry)
			continue
		}
		enabled = append(enabled, entry)
	}
	sections := make([]installedPluginSection, 0, 2)
	if len(enabled) > 0 {
		sortInstalledPluginsByName(enabled)
		sections = append(sections, installedPluginSection{ID: pluginSectionEnabled, Plugins: enabled})
	}
	if len(disabled) > 0 {
		sortInstalledPluginsByName(disabled)
		sections = append(sections, installedPluginSection{ID: pluginSectionDisabled, Plugins: disabled})
	}
	return sections
}

// pluginStoreSectionID maps a missing catalog id onto the official store.
func pluginStoreSectionID(store string) string {
	store = strings.ToLower(strings.TrimSpace(store))
	if store == "" {
		return plugin.OfficialPluginStoreID
	}
	return store
}

// pluginStoreSectionLess keeps the official store above every other catalog.
func pluginStoreSectionLess(left, right string) bool {
	if left == plugin.OfficialPluginStoreID {
		return right != plugin.OfficialPluginStoreID
	}
	if right == plugin.OfficialPluginStoreID {
		return false
	}
	return left < right
}

// pluginCatalogLess orders installed plugins with system entries first, and store plugins by catalog then name.
func pluginCatalogLess(left, right pluginSettingsPlugin, store bool) bool {
	if store {
		leftStore := pluginStoreSectionID(left.Store)
		rightStore := pluginStoreSectionID(right.Store)
		if leftStore != rightStore {
			return pluginStoreSectionLess(leftStore, rightStore)
		}
	} else if left.IsSystem != right.IsSystem {
		return left.IsSystem
	}
	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

type storePluginSection struct {
	ID      string
	Plugins []filteredPlugin
}

// groupStorePlugins splits the visible store catalog by publishing store.
// The official store stays first. Empty groups are omitted.
func groupStorePlugins(filtered []filteredPlugin) []storePluginSection {
	order := make([]string, 0)
	grouped := map[string][]filteredPlugin{}
	for _, entry := range filtered {
		id := pluginStoreSectionID(entry.plugin.Store)
		if _, found := grouped[id]; !found {
			order = append(order, id)
		}
		grouped[id] = append(grouped[id], entry)
	}
	sort.SliceStable(order, func(i, j int) bool {
		return pluginStoreSectionLess(order[i], order[j])
	})
	sections := make([]storePluginSection, 0, len(order))
	for _, id := range order {
		plugins := grouped[id]
		sortInstalledPluginsByName(plugins)
		sections = append(sections, storePluginSection{ID: id, Plugins: plugins})
	}
	return sections
}

// sortInstalledPluginsByName keeps each section alphabetical, including system plugins.
func sortInstalledPluginsByName(plugins []filteredPlugin) {
	sort.SliceStable(plugins, func(i, j int) bool {
		return strings.ToLower(plugins[i].plugin.Name) < strings.ToLower(plugins[j].plugin.Name)
	})
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

// pluginMatchesFilters applies one exclusive value per dropdown, keeping store and installed catalogs separate.
func pluginMatchesFilters(plugin pluginSettingsPlugin, filters pluginFilterState, store bool) bool {
	if store {
		switch filters.installStatus {
		case pluginFilterInstalled:
			if !plugin.IsInstalled {
				return false
			}
		case pluginFilterUninstalled:
			if plugin.IsInstalled {
				return false
			}
		}
	} else {
		switch filters.enabledStatus {
		case pluginFilterEnabled:
			if plugin.IsDisable {
				return false
			}
		case pluginFilterDisabled:
			if !plugin.IsDisable {
				return false
			}
		}
		switch filters.upgradeStatus {
		case pluginFilterUpgradable:
			if !plugin.IsUpgradable {
				return false
			}
		case pluginFilterNotUpgradable:
			if plugin.IsUpgradable {
				return false
			}
		}
	}
	switch filters.pluginType {
	case pluginFilterSystem:
		if !plugin.IsSystem {
			return false
		}
	case pluginFilterThirdParty:
		if plugin.IsSystem {
			return false
		}
	}
	return pluginMatchesRuntimeFilter(plugin, filters.runtime)
}

// pluginMatchesRuntimeFilter treats every Wox script plugin as one runtime, in both catalogs.
func pluginMatchesRuntimeFilter(plugin pluginSettingsPlugin, runtime string) bool {
	if pluginFilterIsAll(runtime) {
		return true
	}
	switch runtime {
	case pluginFilterRuntimeNodeJS:
		return strings.EqualFold(plugin.Runtime, "nodejs")
	case pluginFilterRuntimePython:
		return strings.EqualFold(plugin.Runtime, "python")
	case pluginFilterRuntimeScript:
		return strings.EqualFold(plugin.Runtime, "script")
	default:
		return strings.EqualFold(plugin.Runtime, runtime)
	}
}

// thirdPartyRuntimeFilterValues lists host runtimes that are not the built-in Node.js, Python, script, or Go hosts.
// Compatibility hosts stay out of this list. Imported plugins still appear in the installed catalog.
func thirdPartyRuntimeFilterValues() []string {
	seen := map[string]struct{}{}
	values := make([]string, 0)
	for _, host := range plugin.AllHosts {
		if host == nil || !plugin.RuntimeSettingsVisible(host) {
			continue
		}
		runtime := strings.TrimSpace(string(host.GetRuntime(context.Background())))
		key := strings.ToLower(runtime)
		if key == "" || isBuiltinRuntimeFilter(key) {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, runtime)
	}
	sort.Slice(values, func(i, j int) bool {
		return strings.ToLower(values[i]) < strings.ToLower(values[j])
	})
	return values
}

// isBuiltinRuntimeFilter reports runtimes that already have their own filter rows.
func isBuiltinRuntimeFilter(runtime string) bool {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case pluginFilterRuntimeNodeJS, pluginFilterRuntimePython, pluginFilterRuntimeScript, "go":
		return true
	default:
		return false
	}
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
	saving      bool // Only editable setting values participate in this save transaction.
	status      string
	statusError bool
	fieldErrors map[string]string
	revision    uint64
}

type pluginSettingsFormSnapshot struct {
	formFieldsSnapshot
	pluginID    string
	initial     map[string]string
	saving      bool
	status      string
	statusError bool
	fieldErrors map[string]string
	dirty       bool
}

// reloadPlugins fetches either the store or installed catalog, then applies the side
// effects on other Settings state (settings search mirror, AI model loading, form
// rebuild via setPluginSelectionLocked) in one UI-thread transaction.
func (c *pluginSettingsController) reloadPlugins(store bool, preferredID string) error {
	if err := c.ReloadPlugins(context.Background(), c.services, c.sessionID, store, preferredID); err != nil {
		return err
	}

	requestModels := false
	requestProviders := false
	if err := c.runOnUI("apply loaded plugin catalog", func() {
		plugins := c.Plugins()
		if !store {
			c.settingsSearch().SetPlugins(plugins)
			c.settingsSearch().SetLoading(false)
			c.settingsSearch().SetLoaded(true)
			c.settingsSearch().SetError("")
		}
		selected := c.Selected()
		if selected >= 0 && selected < len(plugins) {
			c.setPluginSelectionLocked(selected)
		}
		requestModels, requestProviders = c.queuePluginFormAIModelsLocked()
		c.settings.invalidateSettingsWindow()
	}); err != nil {
		return err
	}
	c.startPluginFormAIModelLoads(requestModels, requestProviders)
	return nil
}

func pluginSettingsPathIsStore(path string) bool {
	path = strings.TrimSpace(path)
	return path == "/plugins/store" || path == "plugins.store"
}

// switchPluginList swaps the shared list between installed and store data without duplicating its UI state.
func (c *pluginSettingsController) switchPluginList(store bool) {
	form := c.Form()
	if c.Operation() != "" || c.PluginsLoading() || (c.PluginsStore() == store && c.PluginsLoaded()) {
		return
	}
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.submitPluginSettings()
		}
	}
	plugins, loaded := c.CachedPlugins(store)
	if !loaded && !store && c.settingsSearch().Loaded() {
		plugins = c.settingsSearch().Plugins()
		c.cachePlugins(false, plugins)
		loaded = true
	}
	c.SetPluginsStore(store)
	c.SetPlugins(plugins)
	c.SetPluginsLoaded(loaded)
	c.SetPluginsLoading(!loaded)
	c.SetPluginsError("")
	c.SetSelected(-1)
	c.SetForm(nil)
	c.SetUninstallArmed("")
	c.SetOperationError("")
	c.SetSearchEditor(woxui.NewTextEditor(""))
	c.SetSearchFocused(true)
	c.settingsSearch().SetFocused(false)
	c.settingsSearch().SetPanel(false)
	c.SetFilters(pluginFilterState{})
	c.SetFilterOpen(false)
	if store {
		c.SetDetailTab("description")
	} else {
		c.SetDetailTab("settings")
	}
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	if loaded {
		if len(plugins) > 0 {
			c.setPluginSelectionLocked(0)
		}
		c.settings.invalidateSettingsWindow()
		return
	}
	util.Go(c.lifecycleCtx, "switch plugin list", func() {
		if err := c.reloadPlugins(store, ""); err != nil {
			log.Printf("switch plugin list: %v", err)
		}
	})
}

// runPluginOperation uses the same core install operation for fresh installs and upgrades.
func (c *pluginSettingsController) runPluginOperation(kind string) {
	plugins := c.Plugins()
	selected := c.Selected()
	form := c.Form()
	if c.Operation() != "" || selected < 0 || selected >= len(plugins) {
		return
	}
	plugin := plugins[selected]
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.submitPluginSettings()
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
		if c.UninstallArmed() != plugin.ID {
			c.SetUninstallArmed(plugin.ID)
			c.settings.invalidateSettingsWindow()
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
	c.SetUninstallArmed("")
	c.SetOperationError("")
	c.SetOperation(kind + ":" + plugin.ID)
	store := c.PluginsStore()
	c.settings.invalidateSettingsWindow()

	util.Go(c.lifecycleCtx, kind+" plugin", func() {
		operation := contract.PluginOperation(kind)
		if kind == "upgrade" {
			operation = contract.PluginOperationInstall
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := c.services.OperatePlugin(ctx, c.sessionID, plugin.ID, operation)
		cancel()
		if err == nil {
			err = c.reloadPlugins(store, plugin.ID)
		}
		if err == nil {
			_ = c.runOnUI("invalidate related plugin catalog", func() {
				c.invalidateCachedPlugins(!store)
				if store {
					c.settingsSearch().SetLoaded(false)
				}
			})
			util.Go(c.lifecycleCtx, "refresh related plugin catalog", func() {
				if preloadErr := c.PreloadPlugins(c.lifecycleCtx, c.services, c.sessionID, !store); preloadErr != nil {
					log.Printf("refresh related plugin catalog: %v", preloadErr)
					return
				}
				if store {
					_ = c.runOnUI("refresh installed plugin search cache", func() {
						plugins, _ := c.CachedPlugins(false)
						c.settingsSearch().SetPlugins(plugins)
						c.settingsSearch().SetLoaded(true)
						c.settingsSearch().SetError("")
					})
				}
			})
		}
		_ = c.runOnUI("apply plugin operation", func() {
			c.SetOperation("")
			if err != nil {
				c.SetOperationError(err.Error())
			} else {
				c.SetOperationError("")
			}
			c.settings.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("%s plugin %s: %v", kind, plugin.ID, err)
		}
	})
}

// openSelectedPluginWebsite keeps browser dispatch behind the portable Window capability.
func (c *pluginSettingsController) openSelectedPluginWebsite() {
	plugins := c.Plugins()
	selected := c.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	target := strings.TrimSpace(plugins[selected].Website)
	if target == "" {
		return
	}
	if err := c.settings.settingsNativeWindow().OpenExternalURL(target); err != nil {
		c.SetOperationError(err.Error())
		c.settings.invalidateSettingsWindow()
	}
}

// openSelectedPluginDirectory delegates reveal behavior to core's cross-platform shell adapter.
func (c *pluginSettingsController) openSelectedPluginDirectory() {
	plugins := c.Plugins()
	selected := c.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	target := plugins[selected]
	directory := strings.TrimSpace(target.PluginDirectory)
	if directory == "" {
		return
	}
	util.Go(c.lifecycleCtx, "open selected plugin directory", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var err error
		if plugin.IsSingleFilePluginDirectory(directory) && strings.TrimSpace(target.Entry) != "" {
			err = shell.OpenFileInFolder(filepath.Join(directory, target.Entry))
		} else {
			err = c.services.OpenPath(ctx, c.sessionID, directory)
		}
		cancel()
		if err != nil {
			_ = c.runOnUI("apply plugin directory error", func() {
				c.SetOperationError(err.Error())
				c.settings.invalidateSettingsWindow()
			})
		}
	})
}

// runSelectedPluginPrimaryOperation gives keyboard users the same install or upgrade action as the detail button.
func (c *pluginSettingsController) runSelectedPluginPrimaryOperation() {
	plugins := c.Plugins()
	selected := c.Selected()
	if selected < 0 || selected >= len(plugins) {
		return
	}
	plugin := plugins[selected]
	if !plugin.IsInstalled {
		c.runPluginOperation("install")
	} else if plugin.IsUpgradable {
		c.runPluginOperation("upgrade")
	}
}

// setPluginSelectionLocked replaces the editor state with one plugin's current persisted values.
// The controller's Form/Selected are swapped together on the UI thread so cross-domain readers
// (model_manager, form_table) observing the same form pointer stay in sync.
func (c *pluginSettingsController) setPluginSelectionLocked(index int) {
	plugins := c.Plugins()
	if index < 0 || index >= len(plugins) {
		return
	}
	plugin := plugins[index]
	if form := c.Form(); form != nil && form.pluginID == plugin.ID {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		refreshFormDisplayDefinitions(&form.formFieldsState, pluginSettingsFormDefinitions(plugin))
		if editor := c.formTables.settingsEditor; editor != nil && editor.target == &form.formFieldsState {
			// A post-save definition refresh rebuilds this form. The settings overlay
			// is published only while settingsTableEditor.target matches the live form
			// pointer, so replacing it here would hide an in-flight row editor.
			c.SetSelected(index)
			return
		}
		// Refresh can arrive while a save is still in flight, before initial moves
		// forward. An edit that matches the previous initial then looks clean, and
		// replacing the form would drop it. A dirty form is kept for the same reason.
		if form.saving || pluginFormDirty(form.definitions, form.values, form.initial) {
			c.SetSelected(index)
			return
		}
	}
	c.abandonModelManager()
	// Only a change of plugin resets the detail tab. Catalog reloads and post-save
	// definition refreshes re-run this for the already selected plugin, and jumping
	// back to Settings there loses the Trigger Keywords tab the user just edited in.
	previous := c.Selected()
	samePlugin := previous >= 0 && previous < len(plugins) && plugins[previous].ID == plugin.ID
	if !samePlugin {
		c.SetDetailTab("settings")
		if c.PluginsStore() {
			c.SetDetailTab("description")
		}
	}
	if c.PluginsStore() {
		c.SetSelected(index)
		c.SetForm(nil)
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
	if models := c.aiSettings().Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&fields, models)
	}
	initial := make(map[string]string, len(fields.values))
	for key, value := range fields.values {
		initial[key] = value
	}
	c.SetSelected(index)
	c.SetForm(&pluginSettingsFormState{formFieldsState: fields, pluginID: plugin.ID, initial: initial})
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

// pluginKeywordQueryText is the query a user types to activate this trigger keyword.
func pluginKeywordQueryText(keyword string) string {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || keyword == "*" {
		return ""
	}
	return keyword + " "
}

// pluginCommandQueryText is the query a user types to enter this command, including the trailing space.
func pluginCommandQueryText(triggerKeywords []string, command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if trigger := primaryPluginTriggerKeyword(triggerKeywords); trigger != "" {
		return trigger + " " + command + " "
	}
	return command + " "
}

// primaryPluginTriggerKeyword skips "*" because command queries need a typed prefix.
func primaryPluginTriggerKeyword(keywords []string) string {
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword != "" && keyword != "*" {
			return keyword
		}
	}
	return ""
}

// pluginFormTriggerKeywords prefers unsaved keyword editor values so command tests match the table.
func pluginFormTriggerKeywords(plugin pluginSettingsPlugin, values map[string]string) []string {
	if keywords, err := decodePluginTriggerKeywordRows(values["TriggerKeywords"]); err == nil && len(keywords) > 0 {
		return keywords
	}
	if len(plugin.Setting.TriggerKeywords) > 0 {
		return plugin.Setting.TriggerKeywords
	}
	return plugin.TriggerKeywords
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
		fieldErrors:        cloneFormTableFieldErrors(state.fieldErrors),
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
func (c *pluginSettingsController) selectPlugin(index int) {
	plugins := c.Plugins()
	current := c.Selected()
	if index < 0 || index >= len(plugins) || index == current {
		return
	}
	c.SetSearchFocused(false)
	form := c.Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.submitPluginSettings()
		}
	}
	c.setPluginSelectionLocked(index)
	requestModels, requestProviders := c.queuePluginFormAIModelsLocked()
	c.updateSettingsText(false)
	c.startPluginFormAIModelLoads(requestModels, requestProviders)
	c.settings.invalidateSettingsWindow()
}

// queuePluginFormAIModelsLocked applies any cached catalog to selectAIModel fields
// and reports whether a fetch is still needed. Callers start the async load after
// finishing other UI-thread work so the helper stays free of goroutine launches.
func (c *pluginSettingsController) queuePluginFormAIModelsLocked() (requestModels, requestProviders bool) {
	form := c.Form()
	if form == nil || !hasFormDefinitionType(form.definitions, "selectAIModel") {
		return false, false
	}
	if models := c.aiSettings().Models(); len(models) > 0 {
		applyAIModelOptionsLocked(&form.formFieldsState, models)
	}
	if !c.aiSettings().ModelsLoaded() && !c.aiSettings().ModelsLoading() {
		c.aiSettings().SetModelsLoading(true)
		requestModels = true
	}
	return requestModels, true
}

// startPluginFormAIModelLoads fetches the shared model catalog and provider icons
// after a plugin form that contains selectAIModel has been built or rebuilt.
func (c *pluginSettingsController) startPluginFormAIModelLoads(requestModels, requestProviders bool) {
	if requestModels {
		util.Go(c.lifecycleCtx, "load AI models for plugin settings", c.loadAIModels)
	}
	if requestProviders {
		util.Go(c.lifecycleCtx, "load AI provider icons for plugin settings", c.loadAIProviderCatalog)
	}
}

func (c *pluginSettingsController) movePluginSelection(delta int) {
	count := len(c.Plugins())
	selected := c.Selected()
	if count == 0 {
		return
	}
	selected = (selected + delta + count) % count
	c.selectPlugin(selected)
}

// setPluginSearchFocused keeps plugin input routing aligned with retained focus changes.
func (c *pluginSettingsController) setPluginSearchFocused(focused bool) {
	if c.SearchEditor() == nil {
		c.SetSearchEditor(woxui.NewTextEditor(""))
	}
	c.SetSearchFocused(focused)
	if focused {
		c.settingsSearch().SetFocused(false)
		c.settingsSearch().SetPanel(false)
		c.themeSettings().SetThemeSearchFocused(false)
		if form := c.Form(); form != nil {
			syncFormFieldsEditorLocked(&form.formFieldsState)
			form.active = false
		}
	}
	c.settings.invalidateSettingsWindow()
}

// setPluginSearchValue applies accessibility value changes and resets the filtered viewport.
func (c *pluginSettingsController) setPluginSearchValue(value string) error {
	editor := c.SearchEditor()
	if editor == nil {
		c.SetSearchEditor(woxui.NewTextEditor(value))
	} else {
		editor.SetText(value, false)
	}
	c.settings.invalidateSettingsWindow()
	return nil
}

func (c *pluginSettingsController) clearPluginSearch() {
	editor := c.SearchEditor()
	if editor == nil {
		c.SetSearchEditor(woxui.NewTextEditor(""))
	} else {
		editor.SetText("", false)
	}
	c.settings.invalidateSettingsWindow()
}

// togglePluginFilterPanel shows or hides the catalog's anchored advanced filters.
func (c *pluginSettingsController) togglePluginFilterPanel() {
	if c.FilterOpen() {
		c.closePluginFilterPanel()
		return
	}
	c.SetFilterOpen(true)
	c.SetSearchFocused(false)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *pluginSettingsController) closePluginFilterPanel() {
	c.SetFilterOpen(false)
	if picker := c.generalSettings().ChoicePicker(); picker != nil && strings.HasPrefix(picker.item.key, "plugin-filter-") {
		c.closeSettingChoicePicker()
		return
	}
	c.settings.invalidateSettingsWindow()
}

// openPluginFilterChoice opens the shared settings dropdown for one catalog filter field.
func (c *pluginSettingsController) openPluginFilterChoice(id string, anchor woxui.Rect) {
	item := c.pluginFilterChoiceItem(id)
	if len(item.choices) == 0 {
		return
	}
	if anchor.Width <= 0 || anchor.Height <= 0 {
		if host := c.settings.host; host != nil {
			anchor, _ = host.BoundsForKey(woxwidget.Key("setting-choice-anchor-plugin-filter-" + id))
		}
	}
	c.generalSettings().EndEdit()
	c.generalSettings().SetChoicePicker(&settingChoicePickerState{
		item: item, anchor: anchor,
		onChoose: func(choice settingChoice) { c.setPluginFilter(id, choice.value) },
	})
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

// setPluginFilter writes one exclusive dropdown value and keeps the current detail selected when it still matches.
func (c *pluginSettingsController) setPluginFilter(id, value string) {
	if value == pluginFilterAll {
		value = ""
	}
	filters := c.Filters()
	switch id {
	case "enabled":
		filters.enabledStatus = value
	case "upgrade":
		filters.upgradeStatus = value
	case "type":
		filters.pluginType = value
	case "runtime":
		filters.runtime = value
	case "install":
		filters.installStatus = value
	default:
		return
	}
	c.applyPluginFilters(filters)
}

// resetPluginFilters restores every catalog dropdown to All.
func (c *pluginSettingsController) resetPluginFilters() {
	c.applyPluginFilters(pluginFilterState{})
}

// filteredPluginCatalog applies search and dropdowns to the visible catalog.
func (c *pluginSettingsController) filteredPluginCatalog() []filteredPlugin {
	query := ""
	if editor := c.SearchEditor(); editor != nil {
		query = editor.State().Text
	}
	store := c.PluginsStore()
	return filterPlugins(c.Plugins(), query, c.Filters(), store, c.usePinYin())
}

// applyPluginFilters commits catalog filters and keeps the current plugin selected when it still matches.
func (c *pluginSettingsController) applyPluginFilters(filters pluginFilterState) {
	c.SetFilters(filters)
	selected := c.Selected()
	filtered := c.filteredPluginCatalog()
	selectedVisible := false
	for _, entry := range filtered {
		if entry.index == selected {
			selectedVisible = true
			break
		}
	}
	if !selectedVisible && len(filtered) > 0 {
		c.setPluginSelectionLocked(filtered[0].index)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *pluginSettingsController) blurPluginSearch() {
	c.SetSearchFocused(false)
	host := c.settings.host
	if host != nil {
		host.ClearFocus()
	}
	c.settings.invalidateSettingsWindow()
}

func (c *pluginSettingsController) moveFilteredPluginSelection(delta int) {
	selected := c.Selected()
	filtered := c.filteredPluginCatalog()
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
	c.selectPlugin(filtered[position].index)
}

func (c *pluginSettingsController) onPluginSearchKey(event woxui.KeyEvent) bool {
	// Key releases must not repeat list navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	active := c.settings.open && c.settings.tab == "plugins" && c.SearchFocused() && c.SearchEditor() != nil
	if !active {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.blurPluginSearch()
	case woxui.KeyArrowUp:
		c.moveFilteredPluginSelection(-1)
	case woxui.KeyArrowDown:
		c.moveFilteredPluginSelection(1)
	case woxui.KeyEnter, woxui.KeyTab:
		c.blurPluginSearch()
	default:
		return false
	}
	return true
}

func (c *pluginSettingsController) onPluginSearchTextInput(_ woxui.TextInputEvent) bool {
	return c.settings.open && c.settings.tab == "plugins" && c.SearchFocused() && c.SearchEditor() != nil
}

// selectPluginDetailTab changes detail content without discarding staged plugin settings.
func (c *pluginSettingsController) selectPluginDetailTab(tab string) {
	switch tab {
	case "settings", "description", "keywords", "commands", "privacy":
	default:
		return
	}
	c.stopHotkeyRecording()
	c.SetDetailTab(tab)
	if form := c.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		form.active = false
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.submitPluginSettings()
		}
	}
	c.SetSearchFocused(false)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

// onPluginSettingsKey routes keys either to list navigation or the active shared field editor.
func (c *pluginSettingsController) onPluginSettingsKey(event woxui.KeyEvent) bool {
	if event.Down && !event.Composing && event.Key == woxui.KeyEscape && c.FilterOpen() {
		c.closePluginFilterPanel()
		return true
	}
	if c.onPluginSearchKey(event) {
		return true
	}
	if !event.Down || event.Composing {
		return false
	}
	if c.settings.tab != "plugins" {
		return false
	}
	state := c.Form()
	store := c.PluginsStore()
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
			c.movePluginSelection(-1)
			return true
		case woxui.KeyArrowDown:
			c.movePluginSelection(1)
			return true
		case woxui.KeyEnter:
			if store && state == nil {
				c.runSelectedPluginPrimaryOperation()
			} else {
				c.activatePluginForm()
			}
			return true
		case woxui.KeySpace:
			if store {
				c.runSelectedPluginPrimaryOperation()
			}
			return true
		case woxui.KeyTab:
			c.activatePluginForm()
			return true
		default:
			return false
		}
	}
	if event.Key == woxui.KeyEscape {
		c.deactivatePluginForm()
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
			c.movePluginFormFocus(delta)
			return true
		case woxui.KeyArrowDown:
			if !multiline {
				c.movePluginFormFocus(1)
				return true
			}
		case woxui.KeyArrowUp:
			if !multiline {
				c.movePluginFormFocus(-1)
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
			c.editPluginFormKey(event)
			break
		}
		delta := 1
		if event.Key == woxui.KeyTab && event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		c.movePluginFormFocus(delta)
	case woxui.KeyArrowUp:
		if multiline {
			c.editPluginFormKey(event)
		} else {
			c.movePluginFormFocus(-1)
		}
	case woxui.KeyArrowLeft:
		if fieldType == "select" || fieldType == "selectAIModel" {
			c.changePluginFormChoice(focused, -1)
		} else {
			c.editPluginFormKey(event)
		}
	case woxui.KeyArrowRight:
		if fieldType == "select" || fieldType == "selectAIModel" {
			c.changePluginFormChoice(focused, 1)
		} else {
			c.editPluginFormKey(event)
		}
	case woxui.KeySpace, woxui.KeyEnter:
		if event.Key == woxui.KeyEnter && multiline {
			c.editPluginFormKey(event)
		} else if fieldType == "fileIndexService" {
			c.runFocusedPluginServiceAction(focused)
		} else if fieldType == "table" {
			c.openFormTable(focused)
		} else if fieldType == "dictationModel" || fieldType == "ocrModel" {
			anchor := woxui.Rect{}
			if host := c.settings.host; host != nil {
				anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", focused)))
			}
			c.openPluginModelManager(focused, anchor)
		} else if fieldType == "dictationHotkey" {
			c.recordPluginFormHotkey(focused)
		} else if fieldType == "select" || fieldType == "selectAIModel" {
			c.openFocusedPluginFormChoice(focused)
		} else if fieldType == "checkbox" {
			c.changePluginFormChoice(focused, 1)
		}
	default:
		c.editPluginFormKey(event)
	}
	return true
}

func (c *pluginSettingsController) runFocusedPluginServiceAction(index int) {
	form := c.Form()
	if form == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	for _, action := range form.definitions[index].Value.Actions {
		if action.Primary && action.Enabled {
			c.runPluginServiceAction(action.ID)
			return
		}
	}
}

// openNativePluginSettings shows the plugin host's own settings window.
// The button stays disabled until that window closes and the plugin reloads.
func (c *pluginSettingsController) openNativePluginSettings(pluginID string) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" || c.services == nil || c.nativeSettingsPluginID != "" {
		return
	}
	c.nativeSettingsPluginID = pluginID
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "open native plugin settings", func() {
		err := c.services.OpenNativePluginSettings(context.Background(), c.sessionID, pluginID)
		if err != nil {
			util.GetLogger().Error(c.lifecycleCtx, fmt.Sprintf("open native plugin settings: %v", err))
		}
		var reloadErr error
		if err == nil {
			reloadErr = c.reloadPlugins(false, pluginID)
		}
		_ = c.runOnUI("finish native plugin settings", func() {
			if c.nativeSettingsPluginID == pluginID {
				c.nativeSettingsPluginID = ""
			}
			if current := c.Form(); current != nil && current.pluginID == pluginID {
				if err != nil {
					current.status = err.Error()
					current.statusError = true
				} else if reloadErr != nil {
					current.status = reloadErr.Error()
					current.statusError = true
				}
			}
			c.settings.invalidateSettingsWindow()
		})
	})
}

// runPluginServiceAction executes one service lifecycle operation and reloads its dynamic state.
func (c *pluginSettingsController) runPluginServiceAction(actionID string) {
	form := c.Form()
	if form == nil || c.settingAction.actionID != "" || actionID == "" {
		return
	}
	pluginID := form.pluginID
	c.settingAction = pluginSettingActionState{pluginID: pluginID, actionID: actionID}
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "run plugin service action", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err := c.services.ExecutePluginSettingAction(ctx, c.sessionID, pluginID, actionID)
		cancel()
		if err == nil {
			err = c.reloadPlugins(false, pluginID)
		}
		_ = c.runOnUI("finish plugin service action", func() {
			c.settingAction.actionID = ""
			if err != nil {
				c.settingAction.errorText = err.Error()
			}
			c.settings.invalidateSettingsWindow()
		})
	})
}

// recordPluginFormHotkey reuses core's dictation-aware recorder while keeping the value staged with other plugin changes.
func (c *pluginSettingsController) recordPluginFormHotkey(index int) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) || state.definitions[index].Type != "dictationHotkey" {
		return
	}
	target := &state.formFieldsState
	c.startHotkeyRecording("plugin-settings", target, index, "", dictationHotkeyRecordingKinds, nil)
}

// activatePluginForm transfers keyboard and IME ownership from the plugin list to its first field.
func (c *pluginSettingsController) activatePluginForm() {
	state := c.Form()
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
	c.updateSettingsText(textInput)
	c.settings.invalidateSettingsWindow()
}

// deactivatePluginForm returns keyboard ownership to the settings page while preserving edits.
func (c *pluginSettingsController) deactivatePluginForm() {
	c.stopHotkeyRecording()
	if form := c.Form(); form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		focused := form.focused
		form.active = false
		c.validatePluginFormField(form, focused)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.submitPluginSettings()
		}
	}
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *pluginSettingsController) focusPluginFormField(index int) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) || !formDefinitionFocusable(state.definitions[index]) {
		return
	}
	c.stopHotkeyRecordingForDifferentField(&state.formFieldsState, index)
	previousFocused := state.focused
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if previousFocused != index {
		c.validatePluginFormField(state, previousFocused)
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.status = ""
	textInput := state.editor != nil
	c.updateSettingsText(textInput)
	c.settings.invalidateSettingsWindow()
	if previousFocused != index && pluginFormDirty(state.definitions, state.values, state.initial) {
		c.submitPluginSettings()
	}
}

// blurPluginFormField shows validator messages under a plugin setting after it loses focus.
func (c *pluginSettingsController) blurPluginFormField(index int) {
	form := c.Form()
	if form == nil || form.saving {
		return
	}
	syncFormFieldsEditorLocked(&form.formFieldsState)
	c.validatePluginFormField(form, index)
	c.settings.invalidateSettingsWindow()
}

// validatePluginFormField records the first failing validator for one setting under that field.
func (c *pluginSettingsController) validatePluginFormField(form *pluginSettingsFormState, index int) {
	if form == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	key := form.definitions[index].Value.Key
	if key == "" {
		return
	}
	errors := c.formTables.translateFormTableFieldErrors(validateFormFieldErrors([]formDefinition{form.definitions[index]}, form.values))
	if form.fieldErrors == nil {
		form.fieldErrors = map[string]string{}
	}
	if message := errors[key]; message != "" {
		form.fieldErrors[key] = message
	} else {
		delete(form.fieldErrors, key)
	}
	if len(form.fieldErrors) == 0 {
		form.fieldErrors = nil
	}
}

func clearPluginFormFieldError(form *pluginSettingsFormState, index int) {
	if form == nil || form.fieldErrors == nil || index < 0 || index >= len(form.definitions) {
		return
	}
	delete(form.fieldErrors, form.definitions[index].Value.Key)
	if len(form.fieldErrors) == 0 {
		form.fieldErrors = nil
	}
}

func (c *pluginSettingsController) movePluginFormFocus(delta int) {
	state := c.Form()
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
	if host := c.settings.host; host != nil {
		host.RequestFocus(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", index)))
	}
	c.stopHotkeyRecordingForDifferentField(&state.formFieldsState, index)
	textInput := state.editor != nil
	c.updateSettingsText(textInput)
	c.settings.invalidateSettingsWindow()
}

func (c *pluginSettingsController) changePluginFormChoice(index, delta int) {
	state := c.Form()
	if state == nil || !state.active {
		return
	}
	if index >= 0 && index < len(state.definitions) && state.definitions[index].Type == "select" && state.definitions[index].Value.IsMulti {
		// Left/right must not replace the selected set with a single option.
		c.openFocusedPluginFormChoice(index)
		return
	}
	changeFormFieldsChoiceLocked(&state.formFieldsState, index, delta)
	state.status = ""
	clearPluginFormFieldError(state, index)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	c.submitPluginSettings()
}

// openFocusedPluginFormChoice resolves the retained field bounds for keyboard-opened menus.
func (c *pluginSettingsController) openFocusedPluginFormChoice(index int) {
	anchor := woxui.Rect{}
	if host := c.settings.host; host != nil {
		key := fmt.Sprintf("plugin-settings-field-%d", index)
		if form := c.Form(); form != nil && index >= 0 && index < len(form.definitions) && form.definitions[index].Type == "selectAIModel" {
			key += "-model"
		}
		anchor, _ = host.BoundsForKey(woxwidget.Key(key))
	}
	c.openPluginFormChoice(index, anchor)
}

// openPluginFormChoice presents the shared anchored dropdown for a plugin select field.
func (c *pluginSettingsController) openPluginFormChoice(index int, anchor woxui.Rect) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	if definition.Type == "selectAIModel" {
		c.openPluginAIModelChoice(index, false, anchor)
		return
	}
	if definition.Type != "select" || len(definition.Value.Options) == 0 {
		return
	}
	if anchor.Width <= 0 || anchor.Height <= 0 {
		if host := c.settings.host; host != nil {
			anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d", index)))
		}
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	choices := make([]settingChoice, len(definition.Value.Options))
	for optionIndex, option := range definition.Value.Options {
		label := c.language.translate(option.Label)
		if strings.TrimSpace(label) == "" {
			label = option.Value
		}
		choices[optionIndex] = settingChoice{value: option.Value, label: label}
	}
	item := settingItem{
		key: "plugin:" + state.pluginID + ":" + definition.Value.Key, title: c.language.translate(definition.Value.Label),
		value: state.values[definition.Value.Key], choices: choices,
		filterable: definition.Value.Filterable,
	}
	c.generalSettings().SetChoicePicker(&settingChoicePickerState{
		item: item, anchor: anchor,
		multi:    definition.Value.IsMulti,
		onChoose: func(choice settingChoice) { c.setPluginFormChoice(index, choice.value) },
	})
	state.status = ""
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func aiModelProviderKey(model aiModel) string {
	return model.Provider + "\x00" + model.ProviderAlias
}

// openPluginAIModelChoice opens the provider or filterable model menu for one AI model field.
func (c *pluginSettingsController) openPluginAIModelChoice(index int, providerChoice bool, anchor woxui.Rect) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	definition := state.definitions[index]
	if definition.Type != "selectAIModel" {
		return
	}
	models := aiModelsFromOptions(definition.Value.Options)
	if len(models) == 0 {
		if cached := c.aiSettings().Models(); len(cached) > 0 {
			applyAIModelOptionsLocked(&state.formFieldsState, cached)
			models = aiModelsFromOptions(state.definitions[index].Value.Options)
		}
	}
	if len(models) == 0 {
		c.openPluginAIModelEmptyChoice(index, anchor)
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
		for _, provider := range c.aiSettings().ProviderCatalog() {
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
		for _, provider := range c.aiSettings().ProviderCatalog() {
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
	c.generalSettings().SetChoicePicker(&settingChoicePickerState{
		item: settingItem{
			key: "plugin-ai-model:" + definition.Value.Key, title: c.language.translate(definition.Value.Label), value: currentValue,
			choices: choices, icons: icons, preserveIconColor: true, filterable: !providerChoice,
		},
		anchor: anchor,
		onChoose: func(choice settingChoice) {
			if providerChoice {
				c.setPluginAIModelProvider(index, choice.value)
			} else {
				c.setPluginFormChoice(index, choice.value)
			}
		},
	})
	state.status = ""
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

const pluginAIModelOpenSettingsValue = "open-ai-settings"

// openPluginAIModelEmptyChoice keeps an empty picker usable by offering a jump to AI settings.
func (c *pluginSettingsController) openPluginAIModelEmptyChoice(index int, anchor woxui.Rect) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	if anchor.Width <= 0 || anchor.Height <= 0 {
		if host := c.settings.host; host != nil {
			anchor, _ = host.BoundsForKey(woxwidget.Key(fmt.Sprintf("plugin-settings-field-%d-provider", index)))
		}
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	c.generalSettings().SetChoicePicker(&settingChoicePickerState{
		item: settingItem{
			key:   "plugin-ai-model-empty:" + state.definitions[index].Value.Key,
			title: c.language.translate("i18n:ui_ai_model_selector_no_models_title"),
			choices: []settingChoice{{
				value: pluginAIModelOpenSettingsValue,
				label: c.language.translate("i18n:ui_ai_model_selector_open_ai_settings"),
			}},
		},
		anchor:   anchor,
		onChoose: func(settingChoice) { c.openPluginAISettings() },
	})
	state.status = c.language.translate("i18n:ui_ai_model_selector_no_models_desc")
	state.statusError = false
	if !c.aiSettings().ModelsLoaded() && !c.aiSettings().ModelsLoading() {
		c.aiSettings().SetModelsLoading(true)
		util.Go(c.lifecycleCtx, "load AI models for empty plugin picker", c.loadAIModels)
	}
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

// openPluginAISettings leaves the plugin form so the user can add a provider.
func (c *pluginSettingsController) openPluginAISettings() {
	c.generalSettings().SetChoicePicker(nil)
	c.selectSettingTab("ai")
}

func (c *pluginSettingsController) setPluginAIModelProvider(index int, providerKey string) {
	state := c.Form()
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	for _, model := range aiModelsFromOptions(state.definitions[index].Value.Options) {
		if aiModelProviderKey(model) == providerKey {
			encoded, err := json.Marshal(model)
			if err == nil {
				c.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
}

func (c *pluginSettingsController) setPluginAIModelName(index int, name string) {
	state := c.Form()
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
	c.settings.invalidateSettingsWindow()
	c.submitPluginSettings()
}

func (c *pluginSettingsController) finishPluginAIModelEdit(index int, name string) {
	state := c.Form()
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
				c.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
	for _, model := range models {
		if aiModelProviderKey(model) == aiModelProviderKey(selected) {
			encoded, err := json.Marshal(model)
			if err == nil {
				c.setPluginFormChoice(index, string(encoded))
			}
			return
		}
	}
}

// togglePluginMultiChoice adds or removes one option in a comma-separated setting.
// Selecting an IsSelectAll option stores only that value. Any other choice removes select-all values,
// so a stored "all" does not keep every option selected after the user picks one.
func togglePluginMultiChoice(options []formOption, current, value string) string {
	selectAll := false
	selectAllValues := map[string]struct{}{}
	for _, option := range options {
		if !option.IsSelectAll {
			continue
		}
		selectAllValues[option.Value] = struct{}{}
		if option.Value == value {
			selectAll = true
		}
	}
	selected := false
	values := make([]string, 0)
	for _, existing := range strings.Split(current, ",") {
		existing = strings.TrimSpace(existing)
		if existing == "" {
			continue
		}
		if existing == value {
			selected = true
			continue
		}
		if selectAll {
			continue
		}
		if _, drop := selectAllValues[existing]; drop {
			continue
		}
		values = append(values, existing)
	}
	if selectAll && !selected {
		return value
	}
	if !selected {
		values = append(values, value)
	}
	return strings.Join(values, ",")
}

// setPluginFormChoice stages one exact dropdown value without depending on option ordering.
func (c *pluginSettingsController) setPluginFormChoice(index int, value string) {
	state := c.Form()
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
	if definition.Type == "select" && definition.Value.IsMulti {
		value = togglePluginMultiChoice(definition.Value.Options, state.values[definition.Value.Key], value)
		if picker := c.generalSettings().ChoicePicker(); picker != nil && picker.multi {
			picker.item.value = value
		}
	}
	setFormFieldsFocusLocked(&state.formFieldsState, index)
	state.values[definition.Value.Key] = value
	state.status = ""
	clearPluginFormFieldError(state, index)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	c.submitPluginSettings()
}

func (c *pluginSettingsController) editPluginFormKey(event woxui.KeyEvent) {
	state := c.Form()
	if state != nil && state.active && state.editor != nil && state.focused >= 0 && state.focused < len(state.definitions) {
		_, changed := handleFormEditorKey(state.editor, state.definitions[state.focused], event)
		if changed {
			syncFormFieldsEditorLocked(&state.formFieldsState)
			state.status = ""
			clearPluginFormFieldError(state, state.focused)
		}
	}
	c.settings.invalidateSettingsWindow()
}

// onPluginSettingsTextInput commits native IME events only while a plugin textbox owns focus.
func (c *pluginSettingsController) onPluginSettingsTextInput(_ woxui.TextInputEvent) bool {
	state := c.Form()
	active := c.settings.open && c.settings.tab == "plugins" && state != nil && state.active
	return active
}

func (c *pluginSettingsController) setPluginFormText(index int, value string) {
	form := c.Form()
	changed := form != nil && setFormFieldsTextLocked(&form.formFieldsState, index, value)
	if !changed {
		return
	}
	form.status = ""
	clearPluginFormFieldError(form, index)
	c.settings.invalidateSettingsWindow()
	// dirPath is a completed folder, like a select: persist immediately so the next
	// launcher query does not keep using the previous directory.
	if form.definitions[index].Type == "dirPath" {
		c.submitPluginSettings()
	}
}

// pickPluginFormDirectory fills a plugin dirPath setting from the native folder picker.
func (c *pluginSettingsController) pickPluginFormDirectory(index int) {
	form := c.Form()
	if form == nil || index < 0 || index >= len(form.definitions) || form.definitions[index].Type != "dirPath" {
		return
	}
	window := c.settings.settingsNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Directory: true})
	if c.Form() != form {
		return
	}
	if err != nil {
		form.status = err.Error()
		c.settings.invalidateSettingsWindow()
		return
	}
	if path != "" {
		c.setPluginFormText(index, path)
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
func (c *pluginSettingsController) validatePluginTriggerKeywordTableRow(state *formTableEditorState) string {
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
			return c.language.translate("i18n:ui_plugin_trigger_keyword_duplicate_in_plugin")
		}
	}
	pluginForm := c.Form()
	if pluginForm == nil {
		return ""
	}
	for _, plugin := range c.Plugins() {
		if plugin.ID == pluginForm.pluginID {
			continue
		}
		for _, existing := range plugin.TriggerKeywords {
			if strings.TrimSpace(existing) == keyword {
				name := strings.TrimSpace(plugin.Name)
				if name == "" {
					name = plugin.ID
				}
				return fmt.Sprintf(c.language.translate("i18n:ui_plugin_trigger_keyword_duplicate_in_other_plugin"), name)
			}
		}
	}
	return ""
}

// submitPluginSettings serializes auto-saves while retaining edits made during an in-flight request.
func (c *pluginSettingsController) submitPluginSettings() {
	state := c.Form()
	if state == nil || state.saving || c.Operation() != "" {
		return
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	if fieldErrors := c.formTables.translateFormTableFieldErrors(validateFormFieldErrors(state.definitions, state.values)); len(fieldErrors) > 0 {
		pluginID := state.pluginID
		if form := c.Form(); form != nil && form.pluginID == pluginID {
			form.fieldErrors = fieldErrors
		}
		c.settings.invalidateSettingsWindow()
		return
	}
	submittedValues, persistedValues, err := preparePluginSettingSaveValues(state)
	if err != nil {
		state.status = "Could not prepare dictation actions: " + err.Error()
		state.statusError = true
		c.settings.invalidateSettingsWindow()
		return
	}
	if len(persistedValues) == 0 {
		return
	}
	state.saving = true
	state.status = ""
	state.statusError = false
	state.fieldErrors = nil
	state.revision++
	pluginID := state.pluginID
	submitted := state
	c.settings.invalidateSettingsWindow()

	util.Go(c.lifecycleCtx, "save plugin settings", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		saveErr := c.services.UpdatePluginSettings(ctx, c.sessionID, pluginID, persistedValues)
		var saveRevision uint64
		_ = c.runOnUI("apply plugin settings save", func() {
			resubmit := c.reconcilePluginSettingSave(submitted, pluginID, submittedValues, persistedValues, saveErr)
			saveRevision = c.settingSaveRevisions[pluginID]
			if resubmit {
				c.submitPluginSettings()
			}
			c.settings.invalidateSettingsWindow()
		})
		if saveErr != nil {
			log.Printf("save plugin settings: %v", saveErr)
			return
		}
		// Dynamic settings may depend on any saved value, so always resolve them again.
		c.refreshPluginFormDefinitions(pluginID, saveRevision)
	})
}

// reconcilePluginSettingSave applies one finished save to the form that started it.
// Switching plugins replaces that form before UpdatePluginSettings returns. The
// callback used to match only the plugin id, write the saved value into the new
// form's initial, and resubmit. A fast switch-back had rebuilt that form from the
// stale empty value, so the resubmit failed not_empty and the refresh then refused
// to replace the now-dirty form. Edits on the original form are submitted again.
// A later form is left alone when it already has edits; a clean one is rebuilt
// from the catalog so the saved value shows up. It returns true when the original
// form still has unsaved edits.
func (c *pluginSettingsController) reconcilePluginSettingSave(submitted *pluginSettingsFormState, pluginID string, submittedValues, persistedValues map[string]string, saveErr error) bool {
	form := c.Form()
	sameForm := form != nil && form == submitted
	if sameForm {
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
	if saveErr != nil {
		return false
	}
	c.settingSaveRevisions[pluginID]++
	c.applySavedPluginSettingValues(pluginID, persistedValues)
	if sameForm {
		return pluginFormDirty(form.definitions, form.values, form.initial)
	}
	if form == nil || form.pluginID != pluginID || pluginFormDirty(form.definitions, form.values, form.initial) {
		return false
	}
	plugins := c.Plugins()
	selected := c.Selected()
	if selected < 0 || selected >= len(plugins) || plugins[selected].ID != pluginID {
		return false
	}
	c.setPluginSelectionLocked(selected)
	return false
}

// refreshPluginFormDefinitions reloads one installed plugin's setting definitions without flashing the catalog.
func (c *pluginSettingsController) refreshPluginFormDefinitions(pluginID string, saveRevision uint64) {
	plugins, err := loadPluginSettingsPlugins(c.lifecycleCtx, c.services, c.sessionID, false)
	if err != nil {
		log.Printf("refresh plugin form definitions: %v", err)
		return
	}
	requestModels := false
	requestProviders := false
	_ = c.runOnUI("refresh plugin form definitions", func() {
		// Catalog reads include setting values as well as dynamic definitions. A
		// slower earlier read must not overwrite values committed by a later save.
		if saveRevision != c.settingSaveRevisions[pluginID] {
			return
		}
		current := c.Plugins()
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
			c.SetPlugins(current)
			c.cachePlugins(false, current)
			c.settingsSearch().SetPlugins(current)
			if form := c.Form(); form != nil && form.pluginID == pluginID {
				c.setPluginSelectionLocked(c.Selected())
				requestModels, requestProviders = c.queuePluginFormAIModelsLocked()
			}
			break
		}
		c.settings.invalidateSettingsWindow()
	})
	c.startPluginFormAIModelLoads(requestModels, requestProviders)
}

// applySavedPluginSettingValues keeps the local catalog consistent until its next normal refresh.
func (c *pluginSettingsController) applySavedPluginSettingValues(pluginID string, values map[string]string) {
	plugins := c.Plugins()
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
	c.SetPlugins(plugins)
	c.cachePlugins(c.PluginsStore(), plugins)
	c.settingsSearch().SetPlugins(plugins)
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
