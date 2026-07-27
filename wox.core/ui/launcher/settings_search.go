package launcher

import (
	"sort"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/fuzzymatch"
)

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
	a.mu.RLock()
	loading := a.settingsSearch.Loading()
	loaded := a.settingsSearch.Loaded()
	a.mu.RUnlock()
	if loading || loaded {
		return
	}
	a.invalidateSettingsWindow()
	_ = a.settingsSearch.ReloadPlugins(a.lifecycleCtx, a.services, a.sessionID)
}

// settingsSearchResults builds one ranked index across built-in controls, sections, plugins, and plugin settings.
func (a *App) settingsSearchResults(snapshot settingsSnapshot) []settingsSearchResult {
	query := strings.TrimSpace(snapshot.search.Query.Text)
	if query == "" {
		return nil
	}

	candidates := make([]settingsSearchResult, 0, 96)
	for _, tab := range settingTabs(snapshot.isDev) {
		candidates = append(candidates, settingsSearchResult{
			kind: settingsSearchSection, title: tab.label, subtitle: "Settings section", tab: tab.id,
			searchTexts: []string{tab.id, tab.label},
		})
		tabSnapshot := snapshot
		tabSnapshot.tab = tab.id
		for _, item := range settingItemsForSnapshot(tabSnapshot) {
			texts := []string{item.key, item.title, tab.label}
			texts = append(texts, builtInSettingSearchAliases[item.key]...)
			candidates = append(candidates, settingsSearchResult{
				kind: settingsSearchSetting, title: item.title, subtitle: tab.label, tab: tab.id, settingKey: item.key, searchTexts: normalizeSettingsSearchTexts(texts),
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
			kind: settingsSearchPlugin, title: pluginTitle, subtitle: firstNonEmpty(plugin.Description, plugin.ID), tab: "plugins", pluginID: plugin.ID,
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
				kind: settingsSearchPluginSetting, title: title, subtitle: pluginTitle, tab: "plugins", pluginID: plugin.ID, settingKey: key,
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
	a.mu.Lock()
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
	a.mu.Unlock()
	if host != nil {
		host.RequestFocus(woxwidget.Key("settings-search-field"))
	}
	a.invalidateSettingsWindow()
}

// setSettingsSearchFocused keeps controller routing aligned with the retained text-field focus.
func (a *App) setSettingsSearchFocused(focused bool) {
	a.mu.Lock()
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
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setSettingsSearchValue applies accessibility value changes through the same search state.
func (a *App) setSettingsSearchValue(value string) error {
	a.mu.Lock()
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(value))
	} else {
		editor := a.settingsSearch.Editor()
		editor.SetText(value, false)
	}
	a.settingsSearch.SetPanel(strings.TrimSpace(value) != "")
	a.settingsSearch.SetSelected(0)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearSettingsSearch() {
	a.mu.Lock()
	if a.settingsSearch.Editor() == nil {
		a.settingsSearch.SetEditor(woxui.NewTextEditor(""))
	} else {
		editor := a.settingsSearch.Editor()
		editor.SetText("", false)
	}
	a.settingsSearch.SetPanel(false)
	a.settingsSearch.SetSelected(0)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) blurSettingsSearch() {
	a.mu.Lock()
	if !a.settingsSearch.Focused() {
		a.mu.Unlock()
		return
	}
	a.settingsSearch.SetFocused(false)
	a.settingsSearch.SetPanel(false)
	host := a.settingsHost
	a.mu.Unlock()
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
	a.mu.RLock()
	focused := a.settingsSearch.Focused() && a.settingsSearch.Editor() != nil
	panel := a.settingsSearch.Panel()
	query := ""
	if editor := a.settingsSearch.Editor(); editor != nil {
		query = strings.TrimSpace(editor.State().Text)
	}
	a.mu.RUnlock()
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
	if event.Key == woxui.KeyTab {
		a.blurSettingsSearch()
		a.moveSettingTab(1)
		return true
	}
	if event.Key == woxui.KeyEscape && query == "" {
		a.blurSettingsSearch()
		return false
	}
	return false
}

func (a *App) onSettingsSearchTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	active := a.settingsSearch.Focused() && a.settingsSearch.Editor() != nil
	a.mu.RUnlock()
	return active
}

func (a *App) moveSettingsSearchSelection(delta int) {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	a.mu.Lock()
	if len(results) == 0 {
		a.settingsSearch.SetSelected(0)
	} else {
		a.settingsSearch.SetSelected(min(max(0, a.settingsSearch.Selected()+delta), len(results)-1))
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) selectSettingsSearchResult(index int) {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	if index < 0 || index >= len(results) {
		return
	}
	a.mu.Lock()
	a.settingsSearch.SetSelected(index)
	a.mu.Unlock()
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
	a.mu.Lock()
	a.settingsSearch.SetPanel(false)
	a.mu.Unlock()
	if result.kind == settingsSearchPlugin || result.kind == settingsSearchPluginSetting {
		a.activateSettingsPluginSearchResult(result)
		return
	}

	a.selectSettingTab(result.tab)
	if result.settingKey != "" {
		a.focusBuiltInSettingsSearchTarget(result.tab, result.settingKey)
	}
	a.invalidateSettingsWindow()
}

func (a *App) focusBuiltInSettingsSearchTarget(tab, settingKey string) {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	a.mu.Lock()
	defer a.mu.Unlock()
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
	a.mu.RLock()
	installedReady := !a.pluginSettings.PluginsStore() && a.pluginSettings.PluginsLoaded() && !a.pluginSettings.PluginsLoading()
	plugins := append([]pluginSettingsPlugin(nil), a.pluginSettings.Plugins()...)
	a.mu.RUnlock()
	if installedReady {
		a.selectSettingTab("plugins")
		for index, plugin := range plugins {
			if plugin.ID == result.pluginID {
				a.selectPlugin(index)
				a.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
				return
			}
		}
	}

	a.mu.Lock()
	form := a.pluginSettings.Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			form.status = "Save the current plugin changes before opening a search result."
			form.statusError = true
			a.mu.Unlock()
			a.selectSettingTab("plugins")
			a.invalidateSettingsWindow()
			return
		}
	}
	a.pluginSettings.SetPluginsStore(false)
	a.pluginSettings.SetPluginsLoaded(false)
	a.pluginSettings.SetPluginsLoading(true)
	a.pluginSettings.SetPluginsError("")
	a.pluginSettings.SetSelected(-1)
	a.pluginSettings.SetForm(nil)
	a.mu.Unlock()
	a.selectSettingTab("plugins")
	util.Go(a.lifecycleCtx, "open plugin setting search result", func() {
		if err := a.reloadPlugins(false, result.pluginID); err == nil {
			a.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
		}
	})
}

func (a *App) focusPluginSettingsSearchTarget(pluginID, settingKey string) {
	if settingKey == "" {
		a.invalidateSettingsWindow()
		return
	}
	a.mu.Lock()
	form := a.pluginSettings.Form()
	if form != nil && form.pluginID == pluginID {
		for index, definition := range form.definitions {
			if definition.Value.Key == settingKey {
				form.focused = index
				break
			}
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}
