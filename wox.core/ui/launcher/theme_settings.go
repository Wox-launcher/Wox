package launcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"wox/setting"
	"wox/ui/contract"
	"wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type themeSettingsTheme struct {
	ID             string   `json:"ThemeId"`
	Name           string   `json:"ThemeName"`
	Author         string   `json:"ThemeAuthor"`
	URL            string   `json:"ThemeUrl"`
	Version        string   `json:"Version"`
	Description    string   `json:"Description"`
	IsSystem       bool     `json:"IsSystem"`
	IsInstalled    bool     `json:"IsInstalled"`
	IsUpgradable   bool     `json:"IsUpgradable"`
	IsAuto         bool     `json:"IsAutoAppearance"`
	DarkThemeID    string   `json:"DarkThemeId"`
	LightThemeID   string   `json:"LightThemeId"`
	ScreenshotURLs []string `json:"ScreenshotUrls"`
	HasSwatch      bool
	ImageTheme     bool
	previewTheme   themeData
}

// buildThemeCatalog converts core theme metadata into the pure catalog view.
func (a *App) buildThemeCatalog(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	return launcherview.ThemeSettingsView(a.themeCatalogProps(snapshot, width, height, imageScale))
}

// themeCatalogProps maps the current theme snapshot into the catalog view model.
func (a *App) themeCatalogProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.ThemeSettingsProps {
	themeSnap := snapshot.theme
	filtered := filterThemes(themeSnap.Themes, themeSnap.ThemeSearch.Text)
	items := make([]launcherview.ThemeCatalogItem, 0, len(filtered))
	for _, entry := range filtered {
		items = append(items, themeCatalogItem(entry.theme, entry.index, snapshot))
	}
	var detail *launcherview.ThemeCatalogItem
	if themeSnap.ThemeSelected >= 0 && themeSnap.ThemeSelected < len(themeSnap.Themes) {
		item := a.presentThemeCatalogItem(themeSnap.Themes[themeSnap.ThemeSelected], themeSnap.ThemeSelected, snapshot, width)
		detail = &item
	}
	iconTint := snapshot.palette.Text
	searchActionTint := snapshot.palette.Text
	selectedIconTint := snapshot.palette.SelectionText
	installedTint := woxui.Color{R: 56, G: 176, B: 92, A: 255}
	previewTexts := make([]string, 5)
	previewSubtitles := make([]string, 5)
	for index := range previewTexts {
		previewTexts[index] = a.translate(fmt.Sprintf("i18n:ui_theme_preview_text_%d", index+1))
		previewSubtitles[index] = strings.ReplaceAll(a.translate("i18n:ui_theme_preview_subtitle"), "{index}", fmt.Sprintf("%d", index+1))
	}
	props := launcherview.ThemeSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette, Mode: themeSnap.ThemesMode,
		Error: themeSnap.ThemesError, Operation: themeSnap.ThemeOperation, UninstallArmed: themeSnap.ThemeUninstallArmed, Items: items, Detail: detail,
		Search: themeSnap.ThemeSearch, SearchFocused: themeSnap.ThemeSearchFocused, SearchPlaceholder: fmt.Sprintf(a.translate("i18n:ui_setting_theme_search_placeholder"), len(items)),
		LocateLabel: a.translate("i18n:ui_setting_theme_locate_current"),
		EmptyLabel:  a.translate("i18n:ui_setting_theme_empty_data"), WebsiteLabel: a.translate("i18n:ui_setting_theme_website"), InstallLabel: a.translate("i18n:ui_setting_theme_install"),
		ApplyLabel: a.translate("i18n:ui_setting_theme_apply"), AppliedLabel: a.translate("i18n:ui_setting_theme_applied"), UninstallLabel: a.translate("i18n:ui_setting_theme_uninstall"), UpdateLabel: a.translate("i18n:ui_update"),
		PreviewLabel: a.translate("i18n:ui_setting_theme_preview"), DescriptionLabel: a.translate("i18n:ui_setting_theme_description"), SystemLabel: a.translate("i18n:ui_setting_theme_system_tag"),
		ImageLabel: a.translate("i18n:ui_setting_theme_image_tag"), ImageMemoryLabel: a.translate("i18n:ui_setting_theme_image_memory"),
		PreviewTitle: a.translate("i18n:ui_theme_preview_title"), PreviewTexts: previewTexts,
		PreviewSubtitles: previewSubtitles, PreviewOpenLabel: a.translate("i18n:ui_theme_preview_open"),
		PreviewMoreLabel: a.translate("i18n:toolbar_more_actions"), ActiveDetailTab: themeSnap.ThemeDetailTab, Window: a.settingsNativeWindow(),
		LocateIcon: a.imageForTint(settingControlIconSource("locate"), &searchActionTint, physicalImageSize(18, imageScale)),
		Wallpaper:  themeSnap.ThemeWallpaperImage, WallpaperBlurred: themeSnap.ThemeWallpaperBlurred,
		ExternalIcon: a.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(13, imageScale)), InstalledIcon: a.imageForTint(settingControlIconSource("check-circle"), &installedTint, physicalImageSize(20, imageScale)),
		InstalledSelectedIcon: a.imageForTint(settingControlIconSource("check-circle"), &selectedIconTint, physicalImageSize(20, imageScale)),
		OnSelect:              a.selectTheme,
		OnSearchKey:           a.onThemeSearchKey, OnSearchFocusChange: a.setThemeSearchFocused,
		OnSearchChanged: func(value string) { _ = a.setThemeSearchValue(value) }, OnSetSearchValue: a.setThemeSearchValue,
		OnClear:         func() { _ = a.setThemeSearchValue("") },
		OnLocateCurrent: a.locateCurrentTheme, OnTooltip: a.setSettingChoiceTooltip, OnSelectDetailTab: a.selectThemeDetailTab,
		OnOpenWebsite: a.openSelectedThemeWebsite, OnOperation: a.runThemeOperation,
		CreateAutoLabel: a.translate("i18n:ui_setting_theme_new_auto"), CreateAutoIcon: a.imageForTint(settingControlIconSource("add"), &iconTint, physicalImageSize(16, imageScale)),
		EditAutoLabel:   a.translate("i18n:ui_setting_theme_edit"),
		ModifyAutoLabel: a.translate("i18n:ui_setting_theme_auto_modify"), AutoNamePlaceholder: a.translate("i18n:ui_setting_theme_auto_name"),
		SaveAutoLabel: a.translate("i18n:ui_save"), CancelAutoLabel: a.translate("i18n:ui_cancel"),
		PickLightTitle: a.translate("i18n:ui_setting_theme_auto_pick_light"), PickDarkTitle: a.translate("i18n:ui_setting_theme_auto_pick_dark"),
		AutoPickerThemes: themeAutoPickerItems(snapshot.theme.Themes, snapshot),
		OnCreateAuto:     a.openNewAutoThemeEditor, OnEditAuto: a.openSelectedAutoThemeEditor,
		OnCancelAuto: a.closeAutoThemeEditor, OnSaveAuto: a.saveAutoThemeEditor,
		OnAutoNameChanged: func(value string) { _ = a.setAutoThemeName(value) }, OnSetAutoName: a.setAutoThemeName, OnAutoNameFocusChange: a.setAutoThemeNameFocused,
		OnAutoEditSlot: a.openAutoThemePicker, OnAutoPickTheme: a.pickAutoThemeEndpoint, OnCancelAutoPicker: a.closeAutoThemePicker,
	}
	if editor := themeSnap.AutoEditor; editor != nil {
		props.AutoEditor = &launcherview.AutoThemeEditorState{
			Active: editor.Active, Overwrite: editor.Overwrite, Name: editor.Name, NameFocused: editor.NameFocused,
			LightThemeID: editor.LightThemeID, DarkThemeID: editor.DarkThemeID, HoverSlot: editor.HoverSlot, PickerSlot: editor.PickerSlot,
			Error: editor.Error, Saving: editor.Saving,
		}
	}
	if themeSnap.ThemesLoading && len(themeSnap.Themes) == 0 {
		props.Message = a.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
	} else if themeSnap.ThemesError != "" && len(themeSnap.Themes) == 0 {
		props.Message = themeSnap.ThemesError
		props.MessageError = true
	} else {
		a.applyThemeCatalogEmptyState(&props, themeSnap, filtered, searchActionTint, imageScale)
	}
	return props
}

// buildAutoThemeEditorOverlay hosts the Auto editor on the settings window overlay.
func (a *App) buildAutoThemeEditorOverlay(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	props := a.themeCatalogProps(snapshot, width, height, imageScale)
	props.OverlayWidth = width
	props.OverlayHeight = height
	return launcherview.ThemeAutoEditorOverlay(props)
}

func (a *App) applyThemeCatalogEmptyState(props *launcherview.ThemeSettingsProps, themeSnap themeSettingsSnapshot, filtered []filteredTheme, iconTint woxui.Color, imageScale float32) {
	if len(filtered) > 0 {
		return
	}
	emptyIconTint := iconTint
	emptyIconTint.A = 160
	props.EmptyIcon = a.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale))
	if len(themeSnap.Themes) > 0 || strings.TrimSpace(themeSnap.ThemeSearch.Text) != "" {
		props.EmptyTitle = a.translate("i18n:ui_no_matches")
		props.EmptyDescription = a.translate("i18n:ui_setting_catalog_search_empty_subtitle")
		return
	}
	props.EmptyTitle = a.translate("i18n:ui_setting_theme_empty_data")
	props.EmptyDescription = a.translate("i18n:ui_setting_theme_empty_subtitle")
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
	previewTheme := paletteForTheme(theme.previewTheme).componentTheme()
	lightTheme := previewTheme
	darkTheme := previewTheme
	if theme.IsAuto {
		lightTheme = themeVariantPreview(snapshot.theme.Themes, theme.LightThemeID, true)
		darkTheme = themeVariantPreview(snapshot.theme.Themes, theme.DarkThemeID, false)
	}
	return launcherview.ThemeCatalogItem{
		SourceIndex: sourceIndex, ID: theme.ID, Name: theme.Name, Author: theme.Author, URL: theme.URL, Version: theme.Version, Description: theme.Description,
		IsSystem: theme.IsSystem, IsInstalled: theme.IsInstalled, IsUpgradable: theme.IsUpgradable, IsAuto: theme.IsAuto,
		HasSwatch: theme.HasSwatch, ImageTheme: theme.ImageTheme,
		Active: theme.ID == snapshot.general.Data.ThemeID, Selected: sourceIndex == snapshot.theme.ThemeSelected,
		PreviewTheme: previewTheme, LightPreviewTheme: lightTheme, DarkPreviewTheme: darkTheme,
	}
}

// presentThemeCatalogItem adds store icon and screenshot images. Installed themes keep the live swatch.
func (a *App) presentThemeCatalogItem(theme themeSettingsTheme, sourceIndex int, snapshot settingsSnapshot, width float32) launcherview.ThemeCatalogItem {
	item := themeCatalogItem(theme, sourceIndex, snapshot)
	if snapshot.theme.ThemesMode != "store" || len(theme.ScreenshotURLs) == 0 {
		return item
	}
	source := woxImage{ImageType: "url", ImageData: theme.ScreenshotURLs[0]}
	requestSize := int(min(float32(2048), max(float32(512), width*2)))
	item.Screenshot = a.imageForSurface(source, requestSize, settingsPalette().Background)
	item.ScreenshotLoading = item.Screenshot == nil
	item.OnScreenshot = func() { a.openPreviewImageOverlay(source) }
	return item
}

// themeVariantPreview resolves an AUTO endpoint and keeps Flutter's readable fallback when it is unavailable.
func themeVariantPreview(themes []themeSettingsTheme, id string, light bool) component.Theme {
	for _, theme := range themes {
		if theme.ID == id {
			return paletteForTheme(theme.previewTheme).componentTheme()
		}
	}
	values := map[string]string{
		"AppBackgroundColor": "#2B2B2B", "QueryBoxBackgroundColor": "#3D3D3D", "QueryBoxFontColor": "#FFFFFF",
		"ResultItemTitleColor": "#FFFFFF", "ResultItemSubTitleColor": "#AAAAAA", "ResultItemActiveBackgroundColor": "#4A4A4A",
		"ResultItemActiveTitleColor": "#FFFFFF", "ResultItemActiveSubTitleColor": "#AAAAAA", "ToolbarBackgroundColor": "#2B2B2B", "ToolbarFontColor": "#FFFFFF",
	}
	if light {
		values = map[string]string{
			"AppBackgroundColor": "#F5F5F5", "QueryBoxBackgroundColor": "#E8E8E8", "QueryBoxFontColor": "#000000",
			"ResultItemTitleColor": "#000000", "ResultItemSubTitleColor": "#666666", "ResultItemActiveBackgroundColor": "#D8D8D8",
			"ResultItemActiveTitleColor": "#000000", "ResultItemActiveSubTitleColor": "#666666", "ToolbarBackgroundColor": "#F5F5F5", "ToolbarFontColor": "#000000",
		}
	}
	return themeEditorPalette(values).componentTheme()
}

// reloadThemes fetches one catalog while retaining the full resolved palette for local preview.
// Delegates state management to themeSettings; passes the active ThemeID as fallback selection.
func (a *App) reloadThemes(mode, preferredID string) error {
	fallbackID := a.currentThemeID()
	return a.themeSettings.ReloadThemes(context.Background(), a.services, a.sessionID, mode, preferredID, fallbackID)
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
	if a.themeSettings.ThemeOperation() != "" || a.themeSettings.ThemesLoading() || a.themeSettings.ThemesMode() == mode {
		return
	}
	themeEditor := a.themeSettings.ThemeEditor()
	if a.themeSettings.ThemesMode() == "editor" && themeEditorDirtyLocked(themeEditor) {
		themeEditor.error = "Save the current theme changes before switching views."
		a.invalidateSettingsWindow()
		return
	}
	a.themeSettings.SetThemesMode(mode)
	a.themeSettings.SetThemesError("")
	a.themeSettings.SetThemeUninstallArmed("")
	a.themeSettings.SetThemeSearchFocused(false)
	if mode != "editor" {
		a.themeSettings.SetThemes(nil)
		a.themeSettings.SetThemesLoaded(false)
		a.themeSettings.SetThemesLoading(true)
		a.themeSettings.SetThemeSelected(-1)
		a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(""))
		a.themeSettings.SetThemeSearchFocused(false)
		a.themeSettings.SetThemeDetailTab("preview")
		a.themeSettings.SetAutoEditor(nil)
	}
	loadEditor := mode == "editor" && (themeEditor == nil || !strings.HasPrefix(themeEditor.key, "settings-theme|"))
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()

	if loadEditor {
		util.Go(a.lifecycleCtx, "load settings theme editor", func() {
			if err := a.loadSettingsThemeEditor(); err != nil {
				_ = a.runOnUI("apply settings theme editor error", func() {
					a.themeSettings.SetThemesError(err.Error())
					a.invalidateSettingsWindow()
				})
			}
		})
		return
	}
	if mode != "editor" {
		util.Go(a.lifecycleCtx, "load theme catalog", func() {
			if err := a.reloadThemes(mode, ""); err != nil {
				log.Printf("load %s themes: %v", mode, err)
			}
		})
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
	themes := a.themeSettings.Themes()
	selected := a.themeSettings.ThemeSelected()
	if a.themeSettings.ThemeOperation() != "" || selected < 0 || selected >= len(themes) {
		return
	}
	theme := themes[selected]
	switch kind {
	case "install":
		if theme.IsInstalled {
			return
		}
	case "upgrade":
		if !theme.IsInstalled || !theme.IsUpgradable {
			return
		}
	case "apply":
		if !theme.IsInstalled || a.generalSettings.Data().ThemeID == theme.ID {
			return
		}
	case "uninstall":
		if !theme.IsInstalled || theme.IsSystem {
			return
		}
		if a.themeSettings.ThemeUninstallArmed() != theme.ID {
			a.themeSettings.SetThemeUninstallArmed(theme.ID)
			a.invalidateSettingsWindow()
			return
		}
	default:
		return
	}
	a.themeSettings.SetThemeUninstallArmed("")
	a.themeSettings.SetThemesError("")
	a.themeSettings.SetThemeOperation(kind + ":" + theme.ID)
	mode := a.themeSettings.ThemesMode()
	a.invalidateSettingsWindow()

	util.Go(a.lifecycleCtx, kind+" theme", func() {
		operation := contract.ThemeOperation(kind)
		if kind == "upgrade" {
			operation = contract.ThemeOperationInstall
		}
		timeout := 30 * time.Second
		if kind == "install" || kind == "upgrade" {
			timeout = 2 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := a.services.OperateTheme(ctx, a.sessionID, theme.ID, operation)
		cancel()
		if err == nil && kind == "apply" {
			err = a.reloadTheme()
			if err == nil {
				a.generalSettings.Update(func(d *settingsData) { d.ThemeID = theme.ID })
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
			// After uninstall the removed id is gone and core may have switched to Glass.
			// Keep the catalog on the theme that is actually applied.
			selectID := theme.ID
			if kind == "uninstall" {
				selectID = a.currentThemeID()
			}
			err = a.reloadThemes(mode, selectID)
		}
		_ = a.runOnUI("apply theme operation result", func() {
			a.themeSettings.SetThemeOperation("")
			if err != nil {
				a.themeSettings.SetThemesError(err.Error())
			} else {
				a.themeSettings.SetThemesError("")
			}
			a.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("%s theme %s: %v", kind, theme.ID, err)
		}
	})
}

func (a *App) currentThemeID() string {
	return a.generalSettings.Data().ThemeID
}

func (a *App) openSelectedThemeWebsite() {
	themes := a.themeSettings.Themes()
	selected := a.themeSettings.ThemeSelected()
	if selected < 0 || selected >= len(themes) {
		return
	}
	target := strings.TrimSpace(themes[selected].URL)
	if target == "" {
		return
	}
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		a.themeSettings.SetThemesError(err.Error())
		a.invalidateSettingsWindow()
	}
}

func (a *App) selectTheme(index int) {
	themes := a.themeSettings.Themes()
	if a.themeSettings.ThemeOperation() != "" || index < 0 || index >= len(themes) {
		return
	}
	a.themeSettings.SetThemeSelected(index)
	a.themeSettings.SetThemeUninstallArmed("")
	a.themeSettings.SetThemesError("")
	a.themeSettings.SetAutoEditor(nil)
	a.invalidateSettingsWindow()
}

func (a *App) themeSearchQueryLocked() string {
	editor := a.themeSettings.ThemeSearchEditor()
	if editor == nil {
		return ""
	}
	return editor.State().Text
}

func (a *App) setThemeSearchFocused(focused bool) {
	if a.themeSettings.ThemeSearchEditor() == nil {
		a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(""))
	}
	a.themeSettings.SetThemeSearchFocused(focused)
	if focused {
		a.settingsSearch.SetFocused(false)
		a.settingsSearch.SetPanel(false)
		a.pluginSettings.SetSearchFocused(false)
	}
	a.invalidateSettingsWindow()
}

func (a *App) setThemeSearchValue(value string) error {
	editor := a.themeSettings.ThemeSearchEditor()
	if editor == nil {
		a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(value))
	} else {
		editor.SetText(value, false)
	}
	a.invalidateSettingsWindow()
	return nil
}

func (a *App) onThemeSearchKey(event woxui.KeyEvent) bool {
	active := a.settingsOpen && a.settingTab == "theme" && a.themeSettings.ThemesMode() != "editor" && a.themeSettings.ThemeSearchFocused() && a.themeSettings.ThemeSearchEditor() != nil
	if !active {
		return false
	}
	if event.Key == woxui.KeyEnter {
		return true
	}
	return false
}

func (a *App) onThemeSearchTextInput(_ woxui.TextInputEvent) bool {
	active := a.settingsOpen && a.settingTab == "theme" && a.themeSettings.ThemesMode() != "editor" && a.themeSettings.ThemeSearchFocused() && a.themeSettings.ThemeSearchEditor() != nil
	return active
}

func (a *App) locateCurrentTheme() {
	if a.themeSettings.ThemesMode() != "installed" || a.generalSettings.Data().ThemeID == "" {
		return
	}
	editor := a.themeSettings.ThemeSearchEditor()
	if editor == nil {
		a.themeSettings.SetThemeSearchEditor(woxui.NewTextEditor(""))
	} else {
		editor.SetText("", false)
	}
	themes := a.themeSettings.Themes()
	for index, theme := range themes {
		if theme.ID == a.generalSettings.Data().ThemeID {
			a.themeSettings.SetThemeSelected(index)
			break
		}
	}
	a.invalidateSettingsWindow()
}

func (a *App) selectThemeDetailTab(tab string) {
	if tab != "preview" && tab != "description" {
		return
	}
	a.themeSettings.SetThemeDetailTab(tab)
	a.invalidateSettingsWindow()
}

func (a *App) moveFilteredThemeSelection(delta int) {
	themes := a.themeSettings.Themes()
	filtered := filterThemes(themes, a.themeSearchQueryLocked())
	selected := a.themeSettings.ThemeSelected()
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
	if editor := a.themeSettings.AutoEditor(); editor != nil && editor.active {
		return false
	}
	active := a.settingsOpen && a.settingTab == "theme" && a.themeSettings.ThemesMode() != "editor" && !a.themeSettings.ThemeSearchFocused()
	themes := a.themeSettings.Themes()
	filtered := filterThemes(themes, a.themeSearchQueryLocked())
	if !active || len(filtered) == 0 {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.moveFilteredThemeSelection(-1)
	case woxui.KeyArrowDown:
		a.moveFilteredThemeSelection(1)
	case woxui.KeyEnter, woxui.KeySpace:
		selected := a.themeSettings.ThemeSelected()
		themes := a.themeSettings.Themes()
		if selected < 0 || selected >= len(themes) {
			return false
		}
		theme := themes[selected]
		current := a.currentThemeID()
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

func themeAutoPickerItems(themes []themeSettingsTheme, snapshot settingsSnapshot) []launcherview.ThemeCatalogItem {
	items := make([]launcherview.ThemeCatalogItem, 0, len(themes))
	for index, theme := range themes {
		if theme.IsAuto {
			continue
		}
		items = append(items, themeCatalogItem(theme, index, snapshot))
	}
	return items
}

func (a *App) openNewAutoThemeEditor() {
	a.openAutoThemeEditor(autoThemeEditorState{
		active: true, nameEditor: woxui.NewTextEditor(a.translate("i18n:ui_setting_theme_auto_default_name")),
		nameFocused: true, lightThemeID: setting.DefaultLightThemeId, darkThemeID: setting.DefaultDarkThemeId,
	})
}

func (a *App) openSelectedAutoThemeEditor() {
	theme, ok := a.selectedInstalledTheme()
	if !ok || !theme.IsAuto || theme.IsSystem {
		return
	}
	a.openAutoThemeEditor(autoThemeEditorState{
		active: true, overwrite: true, sourceID: theme.ID, nameEditor: woxui.NewTextEditor(theme.Name),
		nameFocused: true, lightThemeID: theme.LightThemeID, darkThemeID: theme.DarkThemeID,
	})
}

func (a *App) openAutoThemeEditor(state autoThemeEditorState) {
	if a.themeSettings.ThemesMode() != "installed" || a.themeSettings.ThemeOperation() != "" {
		return
	}
	copied := state
	a.themeSettings.SetAutoEditor(&copied)
	a.themeSettings.SetThemeUninstallArmed("")
	a.themeSettings.SetThemeSearchFocused(false)
	a.themeSettings.SetThemesError("")
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

func (a *App) closeAutoThemeEditor() {
	a.themeSettings.SetAutoEditor(nil)
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) setAutoThemeName(value string) error {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active {
		return nil
	}
	if editor.nameEditor == nil {
		editor.nameEditor = woxui.NewTextEditor(value)
	} else {
		editor.nameEditor.SetText(value, false)
	}
	// Keep typing inside the field. Rebuilding Settings on every keystroke remounts
	// the dialog and makes the name jump even when the shared setting geometry is correct.
	if editor.error != "" {
		editor.error = ""
		a.invalidateSettingsWindow()
	}
	return nil
}

func (a *App) setAutoThemeNameFocused(focused bool) {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active {
		return
	}
	editor.nameFocused = focused
	if focused {
		a.themeSettings.SetThemeSearchFocused(false)
		a.updateSettingsTextInput(true)
	}
	a.invalidateSettingsWindow()
}

func (a *App) setAutoThemeHoverSlot(slot string) {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active || editor.hoverSlot == slot {
		return
	}
	editor.hoverSlot = slot
	a.invalidateSettingsWindow()
}

func (a *App) openAutoThemePicker(slot string) {
	if slot != "light" && slot != "dark" {
		return
	}
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active {
		theme, ok := a.selectedInstalledTheme()
		if !ok || !theme.IsAuto || theme.IsSystem {
			return
		}
		a.openAutoThemeEditor(autoThemeEditorState{
			active: true, overwrite: true, sourceID: theme.ID, nameEditor: woxui.NewTextEditor(theme.Name),
			nameFocused: true, lightThemeID: theme.LightThemeID, darkThemeID: theme.DarkThemeID,
		})
		editor = a.themeSettings.AutoEditor()
	}
	if editor == nil || !editor.active || editor.saving {
		return
	}
	editor.pickerSlot = slot
	a.invalidateSettingsWindow()
}

func (a *App) closeAutoThemePicker() {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active {
		return
	}
	editor.pickerSlot = ""
	a.invalidateSettingsWindow()
}

func (a *App) pickAutoThemeEndpoint(themeID string) {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active || editor.pickerSlot == "" || strings.TrimSpace(themeID) == "" {
		return
	}
	if editor.pickerSlot == "dark" {
		editor.darkThemeID = themeID
	} else {
		editor.lightThemeID = themeID
	}
	editor.pickerSlot = ""
	editor.error = ""
	a.invalidateSettingsWindow()
}

func (a *App) saveAutoThemeEditor() {
	editor := a.themeSettings.AutoEditor()
	if editor == nil || !editor.active || editor.saving {
		return
	}
	name := ""
	if editor.nameEditor != nil {
		name = strings.TrimSpace(editor.nameEditor.State().Text)
	}
	if name == "" {
		editor.error = a.translate("i18n:ui_theme_editor_name_required")
		a.invalidateSettingsWindow()
		return
	}
	editor.saving = true
	editor.error = ""
	overwrite := editor.overwrite
	sourceID := editor.sourceID
	lightID := editor.lightThemeID
	darkID := editor.darkThemeID
	a.invalidateSettingsWindow()
	util.Go(a.lifecycleCtx, "save auto theme", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		saved, err := a.services.SaveAutoTheme(ctx, a.sessionID, name, lightID, darkID, sourceID, overwrite)
		cancel()
		if err == nil {
			err = a.reloadThemes("installed", saved.ThemeId)
		}
		if err == nil && saved.ThemeId == a.currentThemeID() {
			err = a.reloadTheme()
		}
		_ = a.runOnUI("apply saved auto theme", func() {
			current := a.themeSettings.AutoEditor()
			if current == nil {
				a.invalidateSettingsWindow()
				return
			}
			current.saving = false
			if err != nil {
				current.error = err.Error()
			} else {
				a.themeSettings.SetAutoEditor(nil)
				a.updateSettingsTextInput(false)
			}
			a.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("save auto theme: %v", err)
		}
	})
}

func (a *App) selectedInstalledTheme() (themeSettingsTheme, bool) {
	themes := a.themeSettings.Themes()
	selected := a.themeSettings.ThemeSelected()
	if selected < 0 || selected >= len(themes) {
		return themeSettingsTheme{}, false
	}
	return themes[selected], true
}
