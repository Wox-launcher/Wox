package launcher

import (
	"sort"
	"strings"
	"time"
	"unicode"

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
	mode        string
	navID       string
	settingKey  string
	pluginID    string
	searchTexts []string
	score       int64
}

var builtInSettingSearchAliases = map[string][]string{
	"UseSoftwareRendering":      {"CPU", "GPU", "WARP", "software rendering"},
	"MainHotkey":                {"shortcut", "main hotkey"},
	"ActionPanelHotkey":         {"action hotkey", "more actions", "action panel"},
	"UsePinYin":                 {"pinyin"},
	"LangCode":                  {"language"},
	"ShowPosition":              {"position", "screen", "monitor", "display"},
	"ShowTray":                  {"tray"},
	"AppWidth":                  {"width"},
	"AppFontFamily":             {"font"},
	"EnableGlance":              {"glance"},
	"AIProviders":               {"ai provider", "api key", "model"},
	"AIBuiltinTools":            {"builtin", "tool", "enable", "disable"},
	"AIDisabledBuiltinTools":    {"builtin", "tool", "enable", "disable"},
	"AIMCPServers":              {"mcp", "tool", "server"},
	"AISkills":                  {"skill", "repo", "path"},
	"HttpProxyEnabled":          {"proxy"},
	"HttpProxyUrl":              {"proxy url"},
	"EnableAnonymousUsageStats": {"telemetry", "analytics"},
}

// loadSettingsSearchPlugins keeps the search index independent from the installed/store plugin page state.
// The controller owns the loading/loaded/error/plugins state and the load guard; the App still drives
// the call so it can invalidate the settings window at the right lifecycle points.
func (c *settingsSearchController) loadSettingsSearchPlugins() {
	c.settings.invalidateSettingsWindow()
	if err := c.ReloadPlugins(c.lifecycleCtx, c.services, c.sessionID); err != nil {
		return
	}
	_ = c.runOnUI("cache installed plugin catalog", func() {
		c.pluginSettings().cachePlugins(false, c.Plugins())
	})
}

// settingsSearchResults builds one ranked index across built-in controls, sections, plugins, and plugin settings.
func (c *settingsSearchController) settingsSearchResults(snapshot settingsSnapshot) []settingsSearchResult {
	query := strings.TrimSpace(snapshot.search.Query.Text)
	if query == "" {
		return nil
	}

	candidates := make([]settingsSearchResult, 0, 96)
	navSpecs := settingNavSpecs(snapshot.isDev)
	for _, spec := range navSpecs {
		if spec.parent || spec.tab == "" {
			continue
		}
		title := c.settingNavLabel(spec)
		subtitle := c.settingsSearchNavSubtitle(spec, navSpecs)
		texts := []string{spec.id, spec.fallback, title, subtitle, compactSettingsSearchText(spec.fallback), compactSettingsSearchText(title)}
		candidates = append(candidates, settingsSearchResult{
			kind: settingsSearchSection, title: title, subtitle: subtitle, tab: spec.tab, mode: spec.mode, navID: spec.id,
			searchTexts: normalizeSettingsSearchTexts(texts),
		})
	}
	for _, tab := range settingTabs(snapshot.isDev) {
		tabLabel := c.settingsSearchTabLabel(tab, snapshot.isDev)
		tabSnapshot := snapshot
		tabSnapshot.tab = tab.id
		for _, item := range settingItemsForSnapshot(tabSnapshot) {
			item = c.localizedSettingItem(item)
			texts := []string{item.key, item.title, tabLabel}
			texts = append(texts, builtInSettingSearchAliases[item.key]...)
			candidates = append(candidates, settingsSearchResult{
				kind: settingsSearchSetting, title: item.title, subtitle: tabLabel, tab: tab.id, settingKey: item.key, searchTexts: normalizeSettingsSearchTexts(texts),
			})
		}
	}
	candidates = append(candidates, c.settingsFormSearchCandidates(snapshot.hotkey.Form, "hotkey", "Hotkey")...)
	candidates = append(candidates, c.settingsFormSearchCandidates(snapshot.general.Form, "general", "General")...)
	candidates = append(candidates, c.settingsFormSearchCandidates(snapshot.ai.Form, "ai", "AI")...)

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
			searchTexts: normalizeSettingsSearchTexts(append([]string{plugin.ID, pluginTitle, plugin.NameEn, plugin.Author, plugin.Description, plugin.DescriptionEn, plugin.Runtime}, plugin.TriggerKeywords...)),
		})
		for _, definition := range plugin.SettingDefinitions {
			key := strings.TrimSpace(definition.Value.Key)
			title := c.formDefinitionSearchTitle(definition)
			if key == "" || title == "" {
				continue
			}
			texts := []string{key, title}
			for _, alias := range definition.SearchAliases {
				texts = append(texts, c.language.translate(alias))
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
func (c *settingsSearchController) settingsSearchTabLabel(tab settingTab, isDev bool) string {
	for _, spec := range settingNavSpecs(isDev) {
		if spec.tab == tab.id {
			return c.settingNavLabel(spec)
		}
	}
	return tab.label
}

// settingsSearchNavSubtitle uses the parent group for nested pages so Theme Editor ranks under Themes.
func (c *settingsSearchController) settingsSearchNavSubtitle(spec settingNavSpec, specs []settingNavSpec) string {
	if spec.depth == 0 {
		return "Settings section"
	}
	parentID, _, found := strings.Cut(spec.id, ".")
	if !found {
		return "Settings section"
	}
	for _, candidate := range specs {
		if candidate.id == parentID {
			return c.settingNavLabel(candidate)
		}
	}
	return "Settings section"
}

// settingsSearchNavItem reconstructs the sidebar destination stored on a section result.
func settingsSearchNavItem(result settingsSearchResult) settingNavSpec {
	return settingNavSpec{id: result.navID, tab: result.tab, mode: result.mode}
}

// compactSettingsSearchText lets "themeeditor" match labels such as Theme Editor.
func compactSettingsSearchText(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func (c *settingsSearchController) settingsFormSearchCandidates(form *formFieldsSnapshot, tab, subtitle string) []settingsSearchResult {
	if form == nil {
		return nil
	}
	results := make([]settingsSearchResult, 0, len(form.definitions))
	for _, definition := range form.definitions {
		key := strings.TrimSpace(definition.Value.Key)
		title := c.formDefinitionSearchTitle(definition)
		if key == "" || title == "" {
			continue
		}
		texts := []string{key, title, subtitle}
		texts = append(texts, builtInSettingSearchAliases[key]...)
		for _, alias := range definition.SearchAliases {
			texts = append(texts, c.language.translate(alias))
		}
		results = append(results, settingsSearchResult{
			kind: settingsSearchSetting, title: title, subtitle: subtitle, tab: tab, settingKey: key, searchTexts: normalizeSettingsSearchTexts(texts),
		})
	}
	return results
}

func (c *settingsSearchController) formDefinitionSearchTitle(definition formDefinition) string {
	title := definition.Value.Title
	if title == "" {
		title = definition.Value.Label
	}
	return strings.TrimSpace(c.language.translate(title))
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

func (c *settingsSearchController) focusSettingsSearch(selectAll bool) {
	if c.Editor() == nil {
		c.SetEditor(woxwidget.NewTextEditingController(""))
	}
	if selectAll && c.Editor() != nil {
		editor := c.Editor()
		editor.SetText(editor.Text(), true)
	}
	c.SetFocused(true)
	c.pluginSettings().SetSearchFocused(false)
	c.themeSettings().SetThemeSearchFocused(false)
	queryText := ""
	if editor := c.Editor(); editor != nil {
		queryText = strings.TrimSpace(editor.State().Text)
	}
	c.SetPanel(queryText != "")
	host := c.settings.host
	if host != nil {
		host.RequestFocus(woxwidget.Key("settings-search-field"))
	}
	c.settings.invalidateSettingsWindow()
}

// setSettingsSearchFocused keeps controller routing aligned with the retained text-field focus.
func (c *settingsSearchController) setSettingsSearchFocused(focused bool) {
	if c.Editor() == nil {
		c.SetEditor(woxwidget.NewTextEditingController(""))
	}
	c.SetFocused(focused)
	if focused {
		c.pluginSettings().SetSearchFocused(false)
		c.themeSettings().SetThemeSearchFocused(false)
		queryText := ""
		if editor := c.Editor(); editor != nil {
			queryText = strings.TrimSpace(editor.State().Text)
		}
		c.SetPanel(queryText != "")
	} else {
		c.SetPanel(false)
	}
	c.settings.invalidateSettingsWindow()
}

// setSettingsSearchValue applies accessibility value changes through the same search state.
func (c *settingsSearchController) setSettingsSearchValue(value string) error {
	if c.Editor() == nil {
		c.SetEditor(woxwidget.NewTextEditingController(value))
	} else {
		editor := c.Editor()
		if editor.Text() != value {
			editor.SetText(value, false)
		}
	}
	c.SetPanel(strings.TrimSpace(value) != "")
	c.SetSelected(0)
	c.settings.invalidateSettingsWindow()
	return nil
}

func (c *settingsSearchController) clearSettingsSearch() {
	if c.Editor() == nil {
		c.SetEditor(woxwidget.NewTextEditingController(""))
	} else {
		editor := c.Editor()
		editor.SetText("", false)
	}
	c.SetPanel(false)
	c.SetSelected(0)
	c.settings.invalidateSettingsWindow()
}

func (c *settingsSearchController) blurSettingsSearch() {
	if !c.Focused() {
		return
	}
	c.SetFocused(false)
	c.SetPanel(false)
	host := c.settings.host
	if host != nil {
		host.ClearFocus()
	}
	c.settings.invalidateSettingsWindow()
}

// onSettingsSearchKey gives the floating result palette first ownership of search navigation keys.
func (c *settingsSearchController) onSettingsSearchKey(event woxui.KeyEvent) bool {
	if event.Down && !event.Composing && event.Key == woxui.Key("f") && event.Modifiers.HasPrimary() {
		c.focusSettingsSearch(true)
		return true
	}
	// Key releases must not repeat palette navigation, and composing keys belong to native text input.
	if !event.Down || event.Composing {
		return false
	}
	focused := c.Focused() && c.Editor() != nil
	panel := c.Panel()
	query := ""
	if editor := c.Editor(); editor != nil {
		query = strings.TrimSpace(editor.State().Text)
	}
	if !focused {
		return false
	}
	if panel && query != "" {
		switch event.Key {
		case woxui.KeyArrowDown:
			c.moveSettingsSearchSelection(1)
			return true
		case woxui.KeyArrowUp:
			c.moveSettingsSearchSelection(-1)
			return true
		case woxui.KeyEnter:
			c.activateSelectedSettingsSearchResult()
			return true
		case woxui.KeyEscape:
			c.clearSettingsSearch()
			return true
		}
	}
	if event.Key == woxui.KeyEscape && query == "" {
		c.blurSettingsSearch()
		return false
	}
	return false
}

func (c *settingsSearchController) onSettingsSearchTextInput(_ woxui.TextInputEvent) bool {
	return c.Focused() && c.Editor() != nil
}

func (c *settingsSearchController) moveSettingsSearchSelection(delta int) {
	snapshot := c.settingsSnapshot()
	results := c.settingsSearchResults(snapshot)
	if len(results) == 0 {
		c.SetSelected(0)
	} else {
		c.SetSelected(min(max(0, c.Selected()+delta), len(results)-1))
	}
	c.settings.invalidateSettingsWindow()
}

func (c *settingsSearchController) selectSettingsSearchResult(index int) {
	snapshot := c.settingsSnapshot()
	results := c.settingsSearchResults(snapshot)
	if index < 0 || index >= len(results) {
		return
	}
	c.SetSelected(index)
	c.settings.invalidateSettingsWindow()
}

func (c *settingsSearchController) activateSelectedSettingsSearchResult() {
	snapshot := c.settingsSnapshot()
	results := c.settingsSearchResults(snapshot)
	if len(results) == 0 {
		return
	}
	index := min(max(0, snapshot.search.Selected), len(results)-1)
	c.activateSettingsSearchResult(results[index])
}

// activateSettingsSearchResult closes the palette and routes to the result's owning settings surface.
func (c *settingsSearchController) activateSettingsSearchResult(result settingsSearchResult) {
	c.SetPanel(false)
	if result.kind == settingsSearchPlugin || result.kind == settingsSearchPluginSetting {
		c.activateSettingsPluginSearchResult(result)
		return
	}
	if result.kind == settingsSearchSection {
		c.selectSettingsNavItem(settingsSearchNavItem(result))
		c.settings.invalidateSettingsWindow()
		return
	}

	c.selectSettingTab(result.tab)
	if result.settingKey != "" {
		c.focusBuiltInSettingsSearchTarget(result.tab, result.settingKey)
		c.settings.flashLocked(settingsSearchHighlightTarget(result))
	}
	c.settings.invalidateSettingsWindow()
}

func (c *settingsSearchController) focusBuiltInSettingsSearchTarget(tab, settingKey string) {
	snapshot := c.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	for index, item := range items {
		if item.key == settingKey {
			c.settings.row = index
			if tab == "general" {
				c.generalSettings().SetFormFocused(false)
			}
			return
		}
	}
	var fields *formFieldsState
	if tab == "hotkey" {
		fields = c.hotkeySettings().Form()
	} else if tab == "ai" {
		fields = c.aiSettings().Form()
	} else if tab == "general" {
		fields = c.generalSettings().Form()
	}
	if fields == nil {
		return
	}
	for index, definition := range fields.definitions {
		if definition.Value.Key == settingKey {
			fields.focused = index
			c.hotkeySettings().SetFocused(tab == "hotkey")
			c.generalSettings().SetFormFocused(tab == "general")
			return
		}
	}
}

// activateSettingsPluginSearchResult preserves the plugin page's dirty-state guard while loading an installed destination.
func (c *settingsSearchController) activateSettingsPluginSearchResult(result settingsSearchResult) {
	installedReady := !c.pluginSettings().PluginsStore() && c.pluginSettings().PluginsLoaded() && !c.pluginSettings().PluginsLoading()
	plugins := append([]pluginSettingsPlugin(nil), c.pluginSettings().Plugins()...)
	if installedReady {
		c.selectSettingTab("plugins")
		for index, plugin := range plugins {
			if plugin.ID == result.pluginID {
				c.pluginSettings().selectPlugin(index)
				c.settings.flashLocked(settingsSearchHighlightTarget(result))
				c.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
				return
			}
		}
	}

	form := c.pluginSettings().Form()
	if form != nil {
		syncFormFieldsEditorLocked(&form.formFieldsState)
		if pluginFormDirty(form.definitions, form.values, form.initial) {
			c.pluginSettings().submitPluginSettings()
		}
	}
	c.pluginSettings().SetPluginsStore(false)
	c.pluginSettings().SetPluginsLoaded(false)
	c.pluginSettings().SetPluginsLoading(true)
	c.pluginSettings().SetPluginsError("")
	c.pluginSettings().SetSelected(-1)
	c.pluginSettings().SetForm(nil)
	c.selectSettingTab("plugins")
	util.Go(c.lifecycleCtx, "open plugin setting search result", func() {
		if err := c.pluginSettings().reloadPlugins(false, result.pluginID); err == nil {
			if dispatchErr := c.runOnUI("focus plugin setting search target", func() {
				c.settings.flashLocked(settingsSearchHighlightTarget(result))
				c.focusPluginSettingsSearchTarget(result.pluginID, result.settingKey)
			}); dispatchErr != nil {
				return
			}
		}
	})
}

func (c *settingsSearchController) focusPluginSettingsSearchTarget(pluginID, settingKey string) {
	if settingKey == "" {
		c.settings.invalidateSettingsWindow()
		return
	}
	form := c.pluginSettings().Form()
	if form != nil && form.pluginID == pluginID {
		c.pluginSettings().SetDetailTab("settings")
		for index, definition := range form.definitions {
			if definition.Value.Key == settingKey {
				form.focused = index
				break
			}
		}
	}
	c.settings.invalidateSettingsWindow()
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
