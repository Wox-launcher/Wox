package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const themeSettingsListRowHeight = launcherview.ThemeListRowHeight

type themeSettingsTheme struct {
	ID            string `json:"ThemeId"`
	Name          string `json:"ThemeName"`
	Author        string `json:"ThemeAuthor"`
	URL           string `json:"ThemeUrl"`
	Version       string `json:"Version"`
	Description   string `json:"Description"`
	IsSystem      bool   `json:"IsSystem"`
	IsInstalled   bool   `json:"IsInstalled"`
	IsUpgradable  bool   `json:"IsUpgradable"`
	IsAuto        bool   `json:"IsAutoAppearance"`
	previewValues map[string]string
}

// buildThemeCatalog converts core theme metadata into the pure catalog view.
func (a *App) buildThemeCatalog(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	filtered := filterThemes(snapshot.themes, snapshot.themeSearch.Text)
	items := make([]launcherview.ThemeCatalogItem, 0, len(filtered))
	for _, entry := range filtered {
		items = append(items, themeCatalogItem(entry.theme, entry.index, snapshot))
	}
	var detail *launcherview.ThemeCatalogItem
	if snapshot.themeSelected >= 0 && snapshot.themeSelected < len(snapshot.themes) {
		item := themeCatalogItem(snapshot.themes[snapshot.themeSelected], snapshot.themeSelected, snapshot)
		detail = &item
	}
	iconTint := snapshot.palette.resultTitle
	selectedIconTint := snapshot.palette.selectedTitle
	installedTint := woxui.Color{R: 56, G: 176, B: 92, A: 255}
	previewTexts := make([]string, 5)
	previewSubtitles := make([]string, 5)
	for index := range previewTexts {
		previewTexts[index] = a.translate(fmt.Sprintf("i18n:ui_theme_preview_text_%d", index+1))
		previewSubtitles[index] = strings.ReplaceAll(a.translate("i18n:ui_theme_preview_subtitle"), "{index}", fmt.Sprintf("%d", index+1))
	}
	props := launcherview.ThemeSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Mode: snapshot.themesMode,
		Error: snapshot.themesError, Scroll: snapshot.themeListScroll, Operation: snapshot.themeOperation, UninstallArmed: snapshot.themeUninstallArmed, Items: items, Detail: detail,
		Search: snapshot.themeSearch, SearchFocused: snapshot.themeSearchFocused, SearchPlaceholder: fmt.Sprintf(a.translate("i18n:ui_setting_theme_search_placeholder"), len(items)),
		EmptyLabel: a.translate("i18n:ui_setting_theme_empty_data"), WebsiteLabel: a.translate("i18n:ui_setting_theme_website"), InstallLabel: a.translate("i18n:ui_setting_theme_install"),
		ApplyLabel: a.translate("i18n:ui_setting_theme_apply"), UninstallLabel: a.translate("i18n:ui_setting_theme_uninstall"), UpdateLabel: a.translate("i18n:ui_update"),
		PreviewLabel: a.translate("i18n:ui_setting_theme_preview"), DescriptionLabel: a.translate("i18n:ui_setting_theme_description"), SystemLabel: a.translate("i18n:ui_setting_theme_system_tag"),
		AutoAppearanceHint: a.translate("i18n:ui_setting_theme_auto_appearance_hint"), PreviewTitle: a.translate("i18n:ui_theme_preview_title"), PreviewTexts: previewTexts,
		PreviewSubtitles: previewSubtitles, PreviewOpenLabel: a.translate("i18n:ui_theme_preview_open"), ActiveDetailTab: snapshot.themeDetailTab, Window: a.settingsNativeWindow(),
		SearchIcon: a.imageForTint(settingControlIconSource("search"), &iconTint, 20), LocateIcon: a.imageForTint(settingControlIconSource("locate"), &iconTint, 18),
		ExternalIcon: a.imageForTint(settingControlIconSource("external"), &iconTint, 13), InstalledIcon: a.imageForTint(settingControlIconSource("check-circle"), &installedTint, 20),
		InstalledSelectedIcon: a.imageForTint(settingControlIconSource("check-circle"), &selectedIconTint, 20),
		OnSelect:              a.selectTheme, OnScroll: a.scrollThemeList, OnSetViewport: a.setThemeListViewport,
		OnSearchCaret: a.focusThemeSearch, OnSearchKey: a.onThemeSearchKey, OnSearchTextInput: a.onThemeSearchTextInput, OnSearchFocusChange: a.setThemeSearchFocused,
		OnSetSearchValue: a.setThemeSearchValue, OnLocateCurrent: a.locateCurrentTheme, OnSelectDetailTab: a.selectThemeDetailTab,
		OnOpenWebsite: a.openSelectedThemeWebsite, OnOperation: a.runThemeOperation,
	}
	if snapshot.themesLoading && len(snapshot.themes) == 0 {
		props.Message = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
	} else if snapshot.themesError != "" && len(snapshot.themes) == 0 {
		props.Message = snapshot.themesError
		props.MessageError = true
	}
	return launcherview.ThemeSettingsView(props)
}

type filteredTheme struct {
	index int
	theme themeSettingsTheme
}

func filterThemes(themes []themeSettingsTheme, query string) []filteredTheme {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]filteredTheme, 0, len(themes))
	for index, theme := range themes {
		if query == "" || strings.Contains(strings.ToLower(theme.Name), query) {
			filtered = append(filtered, filteredTheme{index: index, theme: theme})
		}
	}
	return filtered
}

// themeCatalogItem resolves controller state into one immutable view item.
func themeCatalogItem(theme themeSettingsTheme, sourceIndex int, snapshot settingsSnapshot) launcherview.ThemeCatalogItem {
	return launcherview.ThemeCatalogItem{
		SourceIndex: sourceIndex, ID: theme.ID, Name: theme.Name, Author: theme.Author, URL: theme.URL, Version: theme.Version, Description: theme.Description,
		IsSystem: theme.IsSystem, IsInstalled: theme.IsInstalled, IsUpgradable: theme.IsUpgradable, IsAuto: theme.IsAuto,
		Active: theme.ID == snapshot.data.ThemeID, Selected: sourceIndex == snapshot.themeSelected, PreviewTheme: themeEditorPalette(theme.previewValues).componentTheme(),
	}
}

// reloadThemes fetches one catalog while retaining the full resolved palette for local preview.
func (a *App) reloadThemes(mode, preferredID string) error {
	if mode != "store" && mode != "installed" {
		return fmt.Errorf("unsupported theme catalog %q", mode)
	}
	a.mu.Lock()
	a.themesLoading = true
	a.themesError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var payloads []json.RawMessage
	if err := a.client.Post(ctx, "/theme/"+mode, map[string]any{}, &payloads); err != nil {
		return a.finishThemeLoadError(err)
	}

	themes := make([]themeSettingsTheme, 0, len(payloads))
	for _, payload := range payloads {
		var theme themeSettingsTheme
		if err := json.Unmarshal(payload, &theme); err != nil {
			return a.finishThemeLoadError(fmt.Errorf("decode theme catalog: %w", err))
		}
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return a.finishThemeLoadError(fmt.Errorf("decode theme values: %w", err))
		}
		_, theme.previewValues = themeEditorForm(raw)
		themes = append(themes, theme)
	}
	sort.SliceStable(themes, func(i, j int) bool {
		if mode == "installed" && themes[i].IsSystem != themes[j].IsSystem {
			return themes[i].IsSystem
		}
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})

	a.mu.Lock()
	if preferredID == "" && a.themesMode == mode && a.themeSelected >= 0 && a.themeSelected < len(a.themes) {
		preferredID = a.themes[a.themeSelected].ID
	}
	if preferredID == "" && mode == "installed" {
		preferredID = a.settings.ThemeID
	}
	a.themes = themes
	a.themesMode = mode
	a.themesLoading = false
	a.themesLoaded = true
	a.themesError = ""
	if a.themeSearchEditor == nil {
		a.themeSearchEditor = woxui.NewTextEditor("")
	}
	if a.themeDetailTab == "" {
		a.themeDetailTab = "preview"
	}
	selected := 0
	for index, theme := range themes {
		if theme.ID == preferredID {
			selected = index
			break
		}
	}
	if len(themes) == 0 {
		a.themeSelected = -1
	} else {
		a.themeSelected = selected
	}
	a.ensureThemeSelectionVisibleLocked()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

// finishThemeLoadError releases the loading gate on both transport and decode failures.
func (a *App) finishThemeLoadError(err error) error {
	a.mu.Lock()
	a.themesLoading = false
	a.themesLoaded = false
	a.themesError = err.Error()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return err
}

func themeSettingsModeForPath(path string) string {
	switch strings.TrimSpace(path) {
	case "/themes/store", "themes.store":
		return "store"
	case "/themes/edit", "/themes.edit", "themes.edit":
		return "editor"
	default:
		return "installed"
	}
}

// switchThemeSettingsMode preserves dirty editor work and loads only the newly selected surface.
func (a *App) switchThemeSettingsMode(mode string) {
	if mode != "installed" && mode != "store" && mode != "editor" {
		return
	}
	a.mu.Lock()
	if a.themeOperation != "" || a.themesLoading || a.themesMode == mode {
		a.mu.Unlock()
		return
	}
	if a.themesMode == "editor" && themeEditorDirtyLocked(a.themeEditor) {
		a.themeEditor.error = "Save the current theme changes before switching views."
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	a.themesMode = mode
	a.themesError = ""
	a.themeUninstallArmed = ""
	a.themeSearchFocused = false
	if mode != "editor" {
		a.themes = nil
		a.themesLoaded = false
		a.themesLoading = true
		a.themeSelected = -1
		a.themeListScroll = 0
		a.themeSearchEditor = woxui.NewTextEditor("")
		a.themeSearchFocused = false
		a.themeDetailTab = "preview"
	}
	a.ensureSettingTabVisibleLocked("theme")
	loadEditor := mode == "editor" && (a.themeEditor == nil || !strings.HasPrefix(a.themeEditor.key, "settings-theme|"))
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()

	if loadEditor {
		go func() {
			if err := a.loadSettingsThemeEditor(); err != nil {
				a.mu.Lock()
				a.themesError = err.Error()
				a.mu.Unlock()
				a.invalidateSettingsWindow()
			}
		}()
		return
	}
	if mode != "editor" {
		go func() {
			if err := a.reloadThemes(mode, ""); err != nil {
				log.Printf("load %s themes: %v", mode, err)
			}
		}()
	}
}

func themeEditorDirtyLocked(state *themeEditorPreviewState) bool {
	if state == nil {
		return false
	}
	syncFormFieldsEditorLocked(&state.formFieldsState)
	for key, value := range state.values {
		if value != state.initial[key] {
			return true
		}
	}
	return false
}

// runThemeOperation keeps install/apply lifecycle in core and mirrors only the active palette locally.
func (a *App) runThemeOperation(kind string) {
	a.mu.Lock()
	if a.themeOperation != "" || a.themeSelected < 0 || a.themeSelected >= len(a.themes) {
		a.mu.Unlock()
		return
	}
	theme := a.themes[a.themeSelected]
	switch kind {
	case "install":
		if theme.IsInstalled {
			a.mu.Unlock()
			return
		}
	case "upgrade":
		if !theme.IsInstalled || !theme.IsUpgradable {
			a.mu.Unlock()
			return
		}
	case "apply":
		if !theme.IsInstalled || a.settings.ThemeID == theme.ID {
			a.mu.Unlock()
			return
		}
	case "uninstall":
		if !theme.IsInstalled || theme.IsSystem {
			a.mu.Unlock()
			return
		}
		if a.themeUninstallArmed != theme.ID {
			a.themeUninstallArmed = theme.ID
			a.settingNote = "Press Confirm uninstall to remove " + theme.Name + "."
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
	default:
		a.mu.Unlock()
		return
	}
	a.themeUninstallArmed = ""
	a.themesError = ""
	a.themeOperation = kind + ":" + theme.ID
	mode := a.themesMode
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	go func() {
		path := "/theme/" + kind
		if kind == "upgrade" {
			path = "/theme/install"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.client.Post(ctx, path, map[string]string{"id": theme.ID}, nil)
		cancel()
		if err == nil && kind == "apply" {
			err = a.reloadTheme()
			if err == nil {
				a.mu.Lock()
				a.settings.ThemeID = theme.ID
				a.mu.Unlock()
			}
		}
		if err == nil && kind == "upgrade" && theme.ID == a.currentThemeID() {
			err = a.reloadTheme()
		}
		if err == nil && kind == "uninstall" {
			if reloadErr := a.reloadTheme(); reloadErr != nil {
				err = reloadErr
			} else if reloadErr := a.reloadSettings(); reloadErr != nil {
				err = reloadErr
			}
		}
		if err == nil {
			err = a.reloadThemes(mode, theme.ID)
		}
		a.mu.Lock()
		a.themeOperation = ""
		if err != nil {
			a.themesError = err.Error()
		} else {
			a.themesError = ""
			a.settingNote = kind + " completed for " + theme.Name
		}
		a.mu.Unlock()
		if err != nil {
			log.Printf("%s theme %s: %v", kind, theme.ID, err)
		}
		a.invalidateSettingsWindow()
	}()
}

func (a *App) currentThemeID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.settings.ThemeID
}

func (a *App) openSelectedThemeWebsite() {
	a.mu.RLock()
	if a.themeSelected < 0 || a.themeSelected >= len(a.themes) {
		a.mu.RUnlock()
		return
	}
	target := strings.TrimSpace(a.themes[a.themeSelected].URL)
	a.mu.RUnlock()
	if target == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		a.mu.Lock()
		a.themesError = err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}

func (a *App) selectTheme(index int) {
	a.mu.Lock()
	if a.themeOperation != "" || index < 0 || index >= len(a.themes) {
		a.mu.Unlock()
		return
	}
	a.themeSelected = index
	a.themeUninstallArmed = ""
	a.themesError = ""
	a.ensureThemeSelectionVisibleLocked()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) setThemeListViewport(height float32) {
	a.mu.Lock()
	a.themeListViewport = max(float32(1), height)
	a.clampThemeListScrollLocked()
	a.mu.Unlock()
}

func (a *App) scrollThemeList(delta float32) {
	a.mu.Lock()
	a.themeListScroll += delta
	a.clampThemeListScrollLocked()
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) clampThemeListScrollLocked() {
	filtered := filterThemes(a.themes, a.themeSearchQueryLocked())
	maxOffset := max(float32(0), float32(len(filtered))*themeSettingsListRowHeight-max(float32(1), a.themeListViewport))
	a.themeListScroll = min(max(float32(0), a.themeListScroll), maxOffset)
}

// ensureThemeSelectionVisibleLocked follows keyboard selection without taking ownership from manual scrolling.
func (a *App) ensureThemeSelectionVisibleLocked() {
	viewport := a.themeListViewport
	if viewport <= 1 {
		viewport = 600
	}
	filtered := filterThemes(a.themes, a.themeSearchQueryLocked())
	position := -1
	for index, entry := range filtered {
		if entry.index == a.themeSelected {
			position = index
			break
		}
	}
	if position < 0 {
		a.clampThemeListScrollLocked()
		return
	}
	rowTop := float32(position) * themeSettingsListRowHeight
	rowBottom := rowTop + themeSettingsListRowHeight
	if rowTop < a.themeListScroll {
		a.themeListScroll = rowTop
	} else if rowBottom > a.themeListScroll+viewport {
		a.themeListScroll = rowBottom - viewport
	}
	a.clampThemeListScrollLocked()
}

func (a *App) themeSearchQueryLocked() string {
	if a.themeSearchEditor == nil {
		return ""
	}
	return a.themeSearchEditor.State().Text
}

func (a *App) focusThemeSearch(caret int) {
	a.mu.Lock()
	if a.themeSearchEditor == nil {
		a.themeSearchEditor = woxui.NewTextEditor("")
	}
	if caret >= 0 {
		a.themeSearchEditor.SetCaret(caret)
	}
	a.themeSearchFocused = true
	a.settingSearchFocused = false
	a.settingSearchPanel = false
	a.pluginSearchFocused = false
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) setThemeSearchFocused(focused bool) {
	a.mu.Lock()
	if a.themeSearchEditor == nil {
		a.themeSearchEditor = woxui.NewTextEditor("")
	}
	a.themeSearchFocused = focused
	if focused {
		a.settingSearchFocused = false
		a.settingSearchPanel = false
		a.pluginSearchFocused = false
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) setThemeSearchValue(value string) error {
	a.mu.Lock()
	if a.themeSearchEditor == nil {
		a.themeSearchEditor = woxui.NewTextEditor(value)
	} else {
		a.themeSearchEditor.SetText(value, false)
	}
	a.themeListScroll = 0
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) onThemeSearchKey(event woxui.KeyEvent) bool {
	a.mu.Lock()
	if !a.settingsOpen || a.settingTab != "theme" || a.themesMode == "editor" || !a.themeSearchFocused || a.themeSearchEditor == nil {
		a.mu.Unlock()
		return false
	}
	if event.Key == woxui.KeyEnter {
		a.mu.Unlock()
		return true
	}
	handled, changed := a.themeSearchEditor.HandleKey(event)
	if changed {
		a.themeListScroll = 0
	}
	a.mu.Unlock()
	if handled || changed {
		a.invalidateSettingsWindow()
	}
	return handled
}

func (a *App) onThemeSearchTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	if !a.settingsOpen || a.settingTab != "theme" || a.themesMode == "editor" || !a.themeSearchFocused || a.themeSearchEditor == nil {
		a.mu.Unlock()
		return false
	}
	changed := a.themeSearchEditor.HandleTextInput(event)
	if changed {
		a.themeListScroll = 0
	}
	a.mu.Unlock()
	if changed {
		a.invalidateSettingsWindow()
	}
	return true
}

func (a *App) locateCurrentTheme() {
	a.mu.Lock()
	if a.themesMode != "installed" || a.settings.ThemeID == "" {
		a.mu.Unlock()
		return
	}
	if a.themeSearchEditor == nil {
		a.themeSearchEditor = woxui.NewTextEditor("")
	} else {
		a.themeSearchEditor.SetText("", false)
	}
	for index, theme := range a.themes {
		if theme.ID == a.settings.ThemeID {
			a.themeSelected = index
			a.themeListScroll = 0
			a.ensureThemeSelectionVisibleLocked()
			break
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) selectThemeDetailTab(tab string) {
	if tab != "preview" && tab != "description" {
		return
	}
	a.mu.Lock()
	a.themeDetailTab = tab
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) moveFilteredThemeSelection(delta int) {
	a.mu.RLock()
	filtered := filterThemes(a.themes, a.themeSearchQueryLocked())
	selected := a.themeSelected
	a.mu.RUnlock()
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
		position = 0
	} else {
		position = (position + delta + len(filtered)) % len(filtered)
	}
	a.selectTheme(filtered[position].index)
}

// onThemeSettingsKey gives catalog selection the same basic keyboard access as plugin settings.
func (a *App) onThemeSettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.settingsOpen && a.settingTab == "theme" && a.themesMode != "editor" && !a.themeSearchFocused
	filtered := filterThemes(a.themes, a.themeSearchQueryLocked())
	a.mu.RUnlock()
	if !active || len(filtered) == 0 {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveFilteredThemeSelection(-1)
	case woxui.KeyArrowDown:
		a.moveFilteredThemeSelection(1)
	case woxui.KeyEnter, woxui.KeySpace:
		a.mu.RLock()
		if a.themeSelected < 0 || a.themeSelected >= len(a.themes) {
			a.mu.RUnlock()
			return false
		}
		theme := a.themes[a.themeSelected]
		current := a.settings.ThemeID
		a.mu.RUnlock()
		if !theme.IsInstalled {
			a.runThemeOperation("install")
		} else if theme.IsUpgradable {
			a.runThemeOperation("upgrade")
		} else if theme.ID != current {
			a.runThemeOperation("apply")
		}
	default:
		return false
	}
	return true
}
