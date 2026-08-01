package launcher

import (
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/fuzzymatch"
)

const settingsSearchHighlightDuration = 1500 * time.Millisecond

type settingsSearchResultKind uint8

const (
	settingsSearchSection settingsSearchResultKind = iota
	settingsSearchSetting
	settingsSearchPlugin
	settingsSearchPluginSetting
)

type settingsSearchResult struct {
	kind        settingsSearchResultKind
	title       string
	subtitle    string
	icon        woxImage
	tab         string
	settingKey  string
	pluginID    string
	searchTexts []string
	score       int64
}

var builtInSettingSearchAliases = map[string][]string{
	"MainHotkey":                {"shortcut", "main hotkey"},
	"UsePinYin":                 {"pinyin"},
	"LangCode":                  {"language"},
	"ShowPosition":              {"position"},
	"ShowTray":                  {"tray"},
	"AppWidth":                  {"width"},
	"AppFontFamily":             {"font"},
	"EnableGlance":              {"glance"},
	"AIProviders":               {"ai provider", "api key", "model"},
	"AIMCPServers":              {"mcp", "tool", "server"},
	"AISkills":                  {"skill", "repo", "path"},
	"HttpProxyEnabled":          {"proxy"},
	"HttpProxyUrl":              {"proxy url"},
	"EnableAnonymousUsageStats": {"telemetry", "analytics"},
}

// loadSettingsSearchPlugins keeps the search index independent from the installed/store plugin page state.
// The controller owns the loading/loaded/error/plugins state and the load guard; the App still drives
// the call so it can invalidate the settings window at the right lifecycle points.
func (a *App) loadSettingsSearchPlugins() {
	a.invalidateSettingsWindow()
	if err := a.settingsSearch.ReloadPlugins(a.lifecycleCtx, a.services, a.sessionID); err != nil {
		return
	}
	_ = a.runOnUI("cache installed plugin catalog", func() {
		a.pluginSettings.cachePlugins(false, a.settingsSearch.Plugins())
	})
}

// settingsSearchResults builds one ranked index across built-in controls, sections, plugins, and plugin settings.
func (a *App) settingsSearchResults(snapshot settingsSnapshot) []settingsSearchResult {
	query := strings.TrimSpace(snapshot.search.Query.Text)
	if query == "" {
		return nil
	}

	candidates := make([]settingsSearchResult, 0, 96)
	for _, tab := range settingTabs(snapshot.isDev) {
		tabLabel := a.settingsSearchTabLabel(tab, snapshot.isDev)
		candidates = append(candidates, settingsSearchResult{
			kind: settingsSearchSection, title: tabLabel, subtitle: "Settings section", tab: tab.id,
			searchTexts: []string{tab.id, tabLabel},
		})
		tabSnapshot := snapshot
		tabSnapshot.tab = tab.id
		for _, item := range settingItemsForSnapshot(tabSnapshot) {
			item = a.localizedSettingItem(item)
			texts := []string{item.key, item.title, tabLabel}
			texts = append(texts, builtInSettingSearchAliases[item.key]...)
			candidates = append(candidates, settingsSearchResult{
				kind: settingsSearchSetting, title: item.title, subtitle: tabLabel, tab: tab.id, settingKey: item.key, searchTexts: normalizeSettingsSearchTexts(texts),
			})
		}
	}
	candidates = append(candidates, a.settingsFormSearchCandidates(snapshot.hotkey.Form, "general", "General")...)
	candidates = append(candidates, a.settingsFormSearchCandidates(snapshot.ai.Form, "ai", "AI")...)

	plugins := snapshot.search.Plugins
	if len(plugins) == 0 && !snapshot.plugins.PluginsStore {
		plugins = snapshot.plugins.Plugins
	}
	for _, plugin := range plugins {
		pluginTitle := strings.TrimSpace(plugin.Name)
		if pluginTitle == "" {
			pluginTitle = plugin.ID
		}
		candidates = append(candidates, settingsSearchResult{
			kind: settingsSearchPlugin, title: pluginTitle, subtitle: firstNonEmpty(plugin.Description, plugin.ID), icon: plugin.Icon, tab: "plugins", pluginID: plugin.ID,
			searchTexts: normalizeSettingsSearchTexts(append([]string{plugin.ID, pluginTitle, plugin.Author, plugin.Runtime}, plugin.TriggerKeywords...)),
		})
		for _, definition := range plugin.SettingDefinitions {
			key := strings.TrimSpace(definition.Value.Key)
			title := a.formDefinitionSearchTitle(definition)
			if key == "" || title == "" {
				continue
			}
			texts := []string{key, title}
			for _, alias := range definition.SearchAliases {
				texts = append(texts, a.translate(alias))
			}
			candidates = append(candidates, settingsSearchResult{
				kind: settingsSearchPluginSetting, title: title, subtitle: pluginTitle, icon: plugin.Icon, tab: "plugins", pluginID: plugin.ID, settingKey: key,
				searchTexts: normalizeSettingsSearchTexts(texts),
			})
		}
	}

	results := make([]settingsSearchResult, 0, 8)
	for _, candidate := range candidates {
		candidate.score = bestSettingsSearchScore(candidate.searchTexts, query, snapshot.general.Data.UsePinYin)
		if candidate.score > 0 {
			results = append(results, candidate)
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		if results[i].kind != results[j].kind {
			return results[i].kind < results[j].kind
		}
		return strings.ToLower(results[i].title) < strings.ToLower(results[j].title)
	})
	if len(results) > 8 {
		results = results[:8]
	}
	return results
}

// settingsSearchTabLabel uses the same localized navigation label shown by the destination page.
func (a *App) settingsSearchTabLabel(tab settingTab, isDev bool) string {
	for _, spec := range settingNavSpecs(isDev) {
		if spec.tab == tab.id {
			return a.settingNavLabel(spec)
		}
	}
	return tab.label
}

func (a *App) settingsFormSearchCandidates(form *formFieldsSnapshot, tab, subtitle string) []settingsSearchResult {
	if form == nil {
		return nil
	}
	results := make([]settingsSearchResult, 0, len(form.definitions))
	for _, definition := range form.definitions {
		key := strings.TrimSpace(definition.Value.Key)
		title := a.formDefinitionSearchTitle(definition)
		if key == "" || title == "" {
			continue
		}
		texts := []string{key, title, subtitle}
		texts = append(texts, builtInSettingSearchAliases[key]...)
		for _, alias := range definition.SearchAliases {
			texts = append(texts, a.translate(alias))
		}
		results = append(results, settingsSearchResult{
			kind: settingsSearchSetting, title: title, subtitle: subtitle, tab: tab, settingKey: key, searchTexts: normalizeSettingsSearchTexts(texts),
		})
	}
	return results
}

func (a *App) formDefinitionSearchTitle(definition formDefinition) string {
	title := definition.Value.Title
	if title == "" {
		title = definition.Value.Label
	}
	return strings.TrimSpace(a.translate(title))
}

func normalizeSettingsSearchTexts(texts []string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(texts))
	for _, text := range texts {
		text = strings.TrimSpace(text)
		key := strings.ToLower(text)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, text)
	}
	return normalized
}

func bestSettingsSearchScore(texts []string, query string, usePinYin bool) int64 {
	best := int64(0)
	for _, text := range texts {
		match := fuzzymatch.FuzzyMatch(text, query, usePinYin)
		if match.IsMatch && match.Score > best {
			best = match.Score
		}
	}
	return best
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (a *App) focusSettingsSearch(selectAll bool) {
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(""))
	}
	if selectAll && a.settingsSearch.Editor() != nil {
		editor := a.settingsSearch.Editor()
		editor.SelectAll()
	}
	a.settingsSearch.SetFocused(true)
	a.pluginSettings.SetSearchFocused(false)
	a.themeSettings.SetThemeSearchFocused(false)
	queryText := ""
	if editor := a.settingsSearch.Editor(); editor != nil {
		queryText = strings.TrimSpace(editor.State().Text)
	}
	a.settingsSearch.SetPanel(queryText != "")
	host := a.settingsHost
	if host != nil {
		host.RequestFocus(woxwidget.Key("settings-search-field"))
	}
	a.invalidateSettingsWindow()
}

// setSettingsSearchFocused keeps controller routing aligned with the retained text-field focus.
func (a *App) setSettingsSearchFocused(focused bool) {
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(""))
	}
	a.settingsSearch.SetFocused(focused)
	if focused {
		a.pluginSettings.SetSearchFocused(false)
		a.themeSettings.SetThemeSearchFocused(false)
		queryText := ""
		if editor := a.settingsSearch.Editor(); editor != nil {
			queryText = strings.TrimSpace(editor.State().Text)
		}
		a.settingsSearch.SetPanel(queryText != "")
	} else {
		a.settingsSearch.SetPanel(false)
	}
	a.invalidateSettingsWindow()
}

// setSettingsSearchValue applies accessibility value changes through the same search state.
func (a *App) setSettingsSearchValue(value string) error {
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(value))
	} else {
		editor := a.settingsSearch.Editor()
		editor.SetText(value, false)
	}
	a.settingsSearch.SetPanel(strings.TrimSpace(value) != "")
	a.settingsSearch.SetSelected(0)
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearSettingsSearch() {
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(""))
	} else {
		editor := a.settingsSearch.Editor()
		editor.SetText("", false)
	}
	a.settingsSearch.SetPanel(false)
	a.settingsSearch.SetSelected(0)
	a.invalidateSettingsWindow()
}

func (a *App) blurSettingsSearch() {
	if !a.settingsSearch.Focused() {
		return
	}
	a.settingsSearch.SetFocused(false)
	a.settingsSearch.SetPanel(false)
	host := a.settingsHost
	if host != nil {
		host.ClearFocus()
	}
	a.invalidateSettingsWindow()
}

// onSettingsSearchKey gives the floating result palette first ownership of search navigation keys.
func (a *App) onSettingsSearchKey(event woxui.KeyEvent) bool {
	if event.Down && !event.Composing && event.Key == woxui.Key("f") && event.Modifiers.HasPrimary() {
		a.focusSettingsSearch(true)
		return true
	}
	// Key releases must not repeat palette navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	focused := a.settingsSearch.Focused() && a.settingsSearch.Editor() != nil
	panel := a.settingsSearch.Panel()
	query := ""
	if editor := a.settingsSearch.Editor(); editor != nil {
		query = strings.TrimSpace(editor.State().Text)
	}
	if !focused {
		return false
	}
	if panel && query != "" {
		switch event.Key {
		case woxui.KeyArrowDown:
			a.moveSettingsSearchSelection(1)
			return true
		case woxui.KeyArrowUp:
			a.moveSettingsSearchSelection(-1)
			return true
		case woxui.KeyEnter:
			a.activateSelectedSettingsSearchResult()
			return true
		case woxui.KeyEscape:
			a.clearSettingsSearch()
			return true
		}
	}
	if event.Key == woxui.KeyEscape && query == "" {
		a.blurSettingsSearch()
		return false
	}
	return false
}

func (a *App) onSettingsSearchTextInput(_ woxui.TextInputEvent) bool {
	return a.settingsSearch.Focused() && a.settingsSearch.Editor() != nil
}

func (a *App) moveSettingsSearchSelection(delta int) {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	if len(results) == 0 {
		a.settingsSearch.SetSelected(0)
	} else {
		a.settingsSearch.SetSelected(min(max(0, a.settingsSearch.Selected()+delta), len(results)-1))
	}
	a.invalidateSettingsWindow()
}

func (a *App) selectSettingsSearchResult(index int) {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	if index < 0 || index >= len(results) {
		return
	}
	a.settingsSearch.SetSelected(index)
	a.invalidateSettingsWindow()
}

func (a *App) activateSelectedSettingsSearchResult() {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	if len(results) == 0 {
		return
	}
	index := min(max(0, snapshot.search.Selected), len(results)-1)
	a.activateSettingsSearchResult(results[index])
}

// activateSettingsSearchResult closes the palette and routes to the result's owning settings surface.
func (a *App) activateSettingsSearchResult(result settingsSearchResult) {
	a.settingsSearch.SetPanel(false)
	if result.kind == settingsSearchPlugin || result.kind == settingsSearchPluginSetting {
		a.activateSettingsPluginSearchResult(result)
		return
	}

	a.selectSettingTab(result.tab)
	if result.settingKey != "" {
		a.focusBuiltInSettingsSearchTarget(result.tab, result.settingKey)
		a.startSettingsSearchHighlight(settingsSearchHighlightTarget(result))
	}
	a.invalidateSettingsWindow()
}

func (a *App) focusBuiltInSettingsSearchTarget(tab, settingKey string) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	for index, item := range items {
		if item.key == settingKey {
			a.settingRow = index
			return
		}
	}
	var fields *formFieldsState
	if tab == "general" {
		fields = a.hotkeySettings.Form()
	} else if tab == "ai" {
		fields = a.aiSettings.Form()
	}
	if fields == nil {
		return
	}
	for index, definition := range fields.definitions {
		if definition.Value.Key == settingKey {
			fields.focused = index
			a.hotkeySettings.SetFocused(tab == "general")
			return
		}
	}
}

// activateSettingsPluginSearchResult preserves the plugin page's dirty-state guard while loading an installed destination.
func (a *App) activateSettingsPluginSearchResult(result settingsSearchResult) {
	installedReady := !a.pluginSettings.PluginsStore() && a.pluginSettings.PluginsLoaded() && !a.pluginSettings.PluginsLoading()
	plugins := append([]pluginSettingsPlugin(nil), a.pluginSettings.Plugins()...)
	if installedReady {
		a.selectSettingTab("plugins")
		for index, plugin := range plugins {
			if plugin.ID == result.pluginID {
				a.selectPlugin(index)
				a.startSettingsSearchHighlight(settingsSearchHighlightTarget(result))
				a.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
				return
			}
		}
	}

	form := a.pluginSettings.Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			a.submitPluginSettings()
		}
	}
	a.pluginSettings.SetPluginsStore(false)
	a.pluginSettings.SetPluginsLoaded(false)
	a.pluginSettings.SetPluginsLoading(true)
	a.pluginSettings.SetPluginsError("")
	a.pluginSettings.SetSelected(-1)
	a.pluginSettings.SetForm(nil)
	a.selectSettingTab("plugins")
	util.Go(a.lifecycleCtx, "open plugin setting search result", func() {
		if err := a.reloadPlugins(false, result.pluginID); err == nil {
			if dispatchErr := a.runOnUI("focus plugin setting search target", func() {
				a.startSettingsSearchHighlight(settingsSearchHighlightTarget(result))
				a.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
			}); dispatchErr != nil {
				return
			}
		}
	})
}

func (a *App) focusPluginSettingsSearchTarget(pluginID, settingKey string) {
	if settingKey == "" {
		a.invalidateSettingsWindow()
		return
	}
	form := a.pluginSettings.Form()
	if form != nil && form.pluginID == pluginID {
		a.pluginSettings.SetDetailTab("settings")
		for index, definition := range form.definitions {
			if definition.Value.Key == settingKey {
				form.focused = index
				break
			}
		}
	}
	a.invalidateSettingsWindow()
}

func settingsSearchHighlightTarget(result settingsSearchResult) string {
	switch result.kind {
	case settingsSearchPlugin:
		return "plugin:" + result.pluginID
	case settingsSearchPluginSetting:
		return "plugin-setting:" + result.pluginID + "\x00" + result.settingKey
	default:
		if result.settingKey != "" {
			return "built-in:" + result.settingKey
		}
		return ""
	}
}

// startSettingsSearchHighlight flashes the destination after search navigation and replaces any older cue.
func (a *App) startSettingsSearchHighlight(target string) {
	if target == "" {
		return
	}
	if a.settingFlashTimer != nil {
		a.settingFlashTimer.Stop()
	}
	a.settingFlash = target
	a.settingFlashTimer = time.AfterFunc(settingsSearchHighlightDuration, func() {
		_ = a.runOnUI("clear settings search highlight", func() {
			if a.settingFlash != target {
				return
			}
			a.settingFlash = ""
			a.settingFlashTimer = nil
			a.invalidateSettingsWindow()
		})
	})
	a.invalidateSettingsWindow()
}

func (a *App) clearSettingsSearchHighlight() {
	if a.settingFlashTimer != nil {
		a.settingFlashTimer.Stop()
		a.settingFlashTimer = nil
	}
	a.settingFlash = ""
}
