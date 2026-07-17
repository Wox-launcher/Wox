package launcher

import (
	"context"
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/fuzzymatch"
)

const settingsSearchResultRowHeight = float32(54)

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
func (a *App) loadSettingsSearchPlugins() {
	a.mu.Lock()
	if a.settingSearchLoading || a.settingSearchLoaded {
		a.mu.Unlock()
		return
	}
	a.settingSearchLoading = true
	a.settingSearchError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var plugins []pluginSettingsPlugin
	err := a.client.Post(ctx, "/plugin/installed", map[string]any{}, &plugins)
	if err == nil {
		sort.SliceStable(plugins, func(i, j int) bool {
			if plugins[i].IsSystem != plugins[j].IsSystem {
				return plugins[i].IsSystem
			}
			return strings.ToLower(plugins[i].Name) < strings.ToLower(plugins[j].Name)
		})
	}

	a.mu.Lock()
	a.settingSearchLoading = false
	a.settingSearchLoaded = err == nil
	if err != nil {
		a.settingSearchError = err.Error()
	} else {
		a.settingSearchPlugins = plugins
		a.settingSearchError = ""
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// settingsSearchResults builds one ranked index across built-in controls, sections, plugins, and plugin settings.
func (a *App) settingsSearchResults(snapshot settingsSnapshot) []settingsSearchResult {
	query := strings.TrimSpace(snapshot.searchQuery.Text)
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
	candidates = append(candidates, a.settingsFormSearchCandidates(snapshot.hotkeyForm, "general", "General")...)
	candidates = append(candidates, a.settingsFormSearchCandidates(snapshot.aiForm, "ai", "AI")...)

	plugins := snapshot.searchPlugins
	if len(plugins) == 0 && !snapshot.pluginsStore {
		plugins = snapshot.plugins
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
		candidate.score = bestSettingsSearchScore(candidate.searchTexts, query, snapshot.data.UsePinYin)
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
	if a.settingSearchEditor == nil {
		a.settingSearchEditor = woxui.NewTextEditor("")
	}
	if selectAll {
		a.settingSearchEditor.SelectAll()
	}
	a.settingSearchFocused = true
	a.pluginSearchFocused = false
	a.themeSearchFocused = false
	a.settingSearchPanel = strings.TrimSpace(a.settingSearchEditor.State().Text) != ""
	host := a.settingsHost
	a.mu.Unlock()
	if host != nil {
		host.RequestFocus(woxwidget.Key("settings-search-field"))
	}
	a.invalidateSettingsWindow()
}

func (a *App) setSettingsSearchCaret(offset int) {
	a.mu.Lock()
	if a.settingSearchEditor == nil {
		a.settingSearchEditor = woxui.NewTextEditor("")
	}
	a.settingSearchEditor.SetCaret(offset)
	a.settingSearchFocused = true
	a.pluginSearchFocused = false
	a.themeSearchFocused = false
	a.settingSearchPanel = strings.TrimSpace(a.settingSearchEditor.State().Text) != ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setSettingsSearchFocused keeps controller routing aligned with the retained text-field focus.
func (a *App) setSettingsSearchFocused(focused bool) {
	a.mu.Lock()
	if a.settingSearchEditor == nil {
		a.settingSearchEditor = woxui.NewTextEditor("")
	}
	a.settingSearchFocused = focused
	if focused {
		a.pluginSearchFocused = false
		a.themeSearchFocused = false
		a.settingSearchPanel = strings.TrimSpace(a.settingSearchEditor.State().Text) != ""
	} else {
		a.settingSearchPanel = false
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setSettingsSearchValue applies accessibility value changes through the same search state.
func (a *App) setSettingsSearchValue(value string) error {
	a.mu.Lock()
	if a.settingSearchEditor == nil {
		a.settingSearchEditor = woxui.NewTextEditor(value)
	} else {
		a.settingSearchEditor.SetText(value, false)
	}
	a.settingSearchPanel = strings.TrimSpace(value) != ""
	a.settingSearchSelected = 0
	a.settingSearchScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) clearSettingsSearch() {
	a.mu.Lock()
	if a.settingSearchEditor == nil {
		a.settingSearchEditor = woxui.NewTextEditor("")
	} else {
		a.settingSearchEditor.SetText("", false)
	}
	a.settingSearchPanel = false
	a.settingSearchSelected = 0
	a.settingSearchScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) blurSettingsSearch() {
	a.mu.Lock()
	if !a.settingSearchFocused {
		a.mu.Unlock()
		return
	}
	a.settingSearchFocused = false
	a.settingSearchPanel = false
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
	focused := a.settingSearchFocused && a.settingSearchEditor != nil
	panel := a.settingSearchPanel
	query := ""
	if a.settingSearchEditor != nil {
		query = strings.TrimSpace(a.settingSearchEditor.State().Text)
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

	a.mu.Lock()
	handled, changed := a.settingSearchEditor.HandleKey(event)
	if changed {
		a.settingSearchPanel = strings.TrimSpace(a.settingSearchEditor.State().Text) != ""
		a.settingSearchSelected = 0
		a.settingSearchScroll = 0
	}
	a.mu.Unlock()
	if handled {
		a.invalidateSettingsWindow()
	}
	return handled
}

func (a *App) onSettingsSearchTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	if !a.settingSearchFocused || a.settingSearchEditor == nil {
		a.mu.Unlock()
		return false
	}
	changed := a.settingSearchEditor.HandleTextInput(event)
	if changed {
		a.settingSearchPanel = strings.TrimSpace(a.settingSearchEditor.State().Text) != ""
		a.settingSearchSelected = 0
		a.settingSearchScroll = 0
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return true
}

func (a *App) moveSettingsSearchSelection(delta int) {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	a.mu.Lock()
	if len(results) == 0 {
		a.settingSearchSelected = 0
	} else {
		a.settingSearchSelected = min(max(0, a.settingSearchSelected+delta), len(results)-1)
		a.ensureSettingsSearchSelectionVisibleLocked(len(results))
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
	a.settingSearchSelected = index
	a.ensureSettingsSearchSelectionVisibleLocked(len(results))
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) setSettingsSearchViewport(height float32, resultCount int) {
	a.mu.Lock()
	a.settingSearchViewport = max(float32(1), height)
	a.ensureSettingsSearchSelectionVisibleLocked(resultCount)
	a.mu.Unlock()
}

func (a *App) scrollSettingsSearch(delta float32, resultCount int) {
	a.mu.Lock()
	maximum := max(float32(0), float32(resultCount)*settingsSearchResultRowHeight-a.settingSearchViewport)
	a.settingSearchScroll = min(max(float32(0), a.settingSearchScroll+delta), maximum)
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) ensureSettingsSearchSelectionVisibleLocked(resultCount int) {
	if resultCount <= 0 {
		a.settingSearchScroll = 0
		return
	}
	a.settingSearchSelected = min(max(0, a.settingSearchSelected), resultCount-1)
	viewport := max(float32(1), a.settingSearchViewport)
	top := float32(a.settingSearchSelected) * settingsSearchResultRowHeight
	bottom := top + settingsSearchResultRowHeight
	if top < a.settingSearchScroll {
		a.settingSearchScroll = top
	} else if bottom > a.settingSearchScroll+viewport {
		a.settingSearchScroll = bottom - viewport
	}
	maximum := max(float32(0), float32(resultCount)*settingsSearchResultRowHeight-viewport)
	a.settingSearchScroll = min(max(float32(0), a.settingSearchScroll), maximum)
}

func (a *App) activateSelectedSettingsSearchResult() {
	snapshot := a.settingsSnapshot()
	results := a.settingsSearchResults(snapshot)
	if len(results) == 0 {
		return
	}
	index := min(max(0, snapshot.searchSelected), len(results)-1)
	a.activateSettingsSearchResult(results[index])
}

// activateSettingsSearchResult closes the palette and routes to the result's owning settings surface.
func (a *App) activateSettingsSearchResult(result settingsSearchResult) {
	a.mu.Lock()
	a.settingSearchPanel = false
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
			a.ensureSettingRowVisibleLocked(len(items))
			return
		}
	}
	var fields *formFieldsState
	if tab == "general" {
		fields = a.hotkeySettingsForm
	} else if tab == "ai" {
		fields = a.aiSettingsForm
	}
	if fields == nil {
		return
	}
	for index, definition := range fields.definitions {
		if definition.Value.Key == settingKey {
			fields.focused = index
			a.settingsHotkeyFocus = tab == "general"
			ensureFormFieldsFocusVisibleLocked(fields, index)
			return
		}
	}
}

// activateSettingsPluginSearchResult preserves the plugin page's dirty-state guard while loading an installed destination.
func (a *App) activateSettingsPluginSearchResult(result settingsSearchResult) {
	a.mu.RLock()
	installedReady := !a.pluginsStore && a.pluginsLoaded && !a.pluginsLoading
	plugins := append([]pluginSettingsPlugin(nil), a.plugins...)
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
	if a.pluginForm != nil {
		syncFormFieldsEditorLocked(&a.pluginForm.formFieldsState)
		if pluginFormDirty(a.pluginForm.definitions, a.pluginForm.values, a.pluginForm.initial) {
			a.pluginForm.status = "Save the current plugin changes before opening a search result."
			a.pluginForm.statusError = true
			a.mu.Unlock()
			a.selectSettingTab("plugins")
			a.invalidateSettingsWindow()
			return
		}
	}
	a.pluginsStore = false
	a.pluginsLoaded = false
	a.pluginsLoading = true
	a.pluginsError = ""
	a.pluginSelected = -1
	a.pluginForm = nil
	a.mu.Unlock()
	a.selectSettingTab("plugins")
	go func() {
		if err := a.reloadPlugins(false, result.pluginID); err == nil {
			a.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
		}
	}()
}

func (a *App) focusPluginSettingsSearchTarget(pluginID, settingKey string) {
	if settingKey == "" {
		a.invalidateSettingsWindow()
		return
	}
	a.mu.Lock()
	if a.pluginForm != nil && a.pluginForm.pluginID == pluginID {
		for index, definition := range a.pluginForm.definitions {
			if definition.Value.Key == settingKey {
				a.pluginForm.focused = index
				ensureFormFieldsFocusVisibleLocked(&a.pluginForm.formFieldsState, index)
				break
			}
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}
