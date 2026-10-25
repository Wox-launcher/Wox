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
func (c *themeSettingsController) buildThemeCatalog(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	return launcherview.ThemeSettingsView(c.themeCatalogProps(snapshot, width, height, imageScale))
}

// themeCatalogProps maps the current theme snapshot into the catalog view model.
func (c *themeSettingsController) themeCatalogProps(snapshot settingsSnapshot, width, height, imageScale float32) launcherview.ThemeSettingsProps {
	themeSnap := snapshot.theme
	filtered := filterThemes(themeSnap.Themes, themeSnap.ThemeSearch.Text)
	items := make([]launcherview.ThemeCatalogItem, 0, len(filtered))
	for _, entry := range filtered {
		items = append(items, themeCatalogItem(entry.theme, entry.index, snapshot))
	}
	var detail *launcherview.ThemeCatalogItem
	if themeSnap.ThemeSelected >= 0 && themeSnap.ThemeSelected < len(themeSnap.Themes) {
		item := c.presentThemeCatalogItem(themeSnap.Themes[themeSnap.ThemeSelected], themeSnap.ThemeSelected, snapshot, width)
		detail = &item
	}
	iconTint := snapshot.palette.Text
	searchActionTint := snapshot.palette.Text
	selectedIconTint := snapshot.palette.SelectionText
	installedTint := woxui.Color{R: 56, G: 176, B: 92, A: 255}
	previewTexts := make([]string, 5)
	previewSubtitles := make([]string, 5)
	for index := range previewTexts {
		previewTexts[index] = c.language.translate(fmt.Sprintf("i18n:ui_theme_preview_text_%d", index+1))
		previewSubtitles[index] = strings.ReplaceAll(c.language.translate("i18n:ui_theme_preview_subtitle"), "{index}", fmt.Sprintf("%d", index+1))
	}
	props := launcherview.ThemeSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette, Mode: themeSnap.ThemesMode,
		Error: themeSnap.ThemesError, Operation: themeSnap.ThemeOperation, UninstallArmed: themeSnap.ThemeUninstallArmed, Items: items, Detail: detail,
		Search: themeSnap.ThemeSearch, SearchFocused: themeSnap.ThemeSearchFocused, SearchPlaceholder: fmt.Sprintf(c.language.translate("i18n:ui_setting_theme_search_placeholder"), len(items)),
		LocateLabel: c.language.translate("i18n:ui_setting_theme_locate_current"),
		EmptyLabel:  c.language.translate("i18n:ui_setting_theme_empty_data"), WebsiteLabel: c.language.translate("i18n:ui_setting_theme_website"), InstallLabel: c.language.translate("i18n:ui_setting_theme_install"),
		ApplyLabel: c.language.translate("i18n:ui_setting_theme_apply"), AppliedLabel: c.language.translate("i18n:ui_setting_theme_applied"), UninstallLabel: c.language.translate("i18n:ui_setting_theme_uninstall"), UpdateLabel: c.language.translate("i18n:ui_update"),
		PreviewLabel: c.language.translate("i18n:ui_setting_theme_preview"), DescriptionLabel: c.language.translate("i18n:ui_setting_theme_description"), SystemLabel: c.language.translate("i18n:ui_setting_theme_system_tag"),
		ImageLabel: c.language.translate("i18n:ui_setting_theme_image_tag"), ImageMemoryLabel: c.language.translate("i18n:ui_setting_theme_image_memory"),
		PreviewTitle: c.language.translate("i18n:ui_theme_preview_title"), PreviewTexts: previewTexts,
		PreviewSubtitles: previewSubtitles, PreviewOpenLabel: c.language.translate("i18n:ui_theme_preview_open"),
		PreviewMoreLabel: c.language.translate("i18n:toolbar_more_actions"), ActiveDetailTab: themeSnap.ThemeDetailTab, Window: c.settings.settingsNativeWindow(),
		LocateIcon: c.imageForTint(settingControlIconSource("locate"), &searchActionTint, physicalImageSize(18, imageScale)),
		Wallpaper:  themeSnap.ThemeWallpaperImage, WallpaperBlurred: themeSnap.ThemeWallpaperBlurred,
		ExternalIcon: c.imageForTint(settingControlIconSource("external"), &iconTint, physicalImageSize(13, imageScale)), InstalledIcon: c.imageForTint(settingControlIconSource("check-circle"), &installedTint, physicalImageSize(20, imageScale)),
		InstalledSelectedIcon: c.imageForTint(settingControlIconSource("check-circle"), &selectedIconTint, physicalImageSize(20, imageScale)),
		OnSelect:              c.selectTheme,
		OnSearchKey:           c.onThemeSearchKey, OnSearchFocusChange: c.setThemeSearchFocused,
		OnSearchChanged: func(value string) { _ = c.setThemeSearchValue(value) }, OnSetSearchValue: c.setThemeSearchValue,
		OnClear:         func() { _ = c.setThemeSearchValue("") },
		OnLocateCurrent: c.locateCurrentTheme, OnTooltip: c.setSettingChoiceTooltip, OnSelectDetailTab: c.selectThemeDetailTab,
		OnOpenWebsite: c.openSelectedThemeWebsite, OnOperation: c.runThemeOperation,
		CreateAutoLabel: c.language.translate("i18n:ui_setting_theme_new_auto"), CreateAutoIcon: c.imageForTint(settingControlIconSource("add"), &iconTint, physicalImageSize(16, imageScale)),
		EditAutoLabel:   c.language.translate("i18n:ui_setting_theme_edit"),
		ModifyAutoLabel: c.language.translate("i18n:ui_setting_theme_auto_modify"), AutoNamePlaceholder: c.language.translate("i18n:ui_setting_theme_auto_name"),
		SaveAutoLabel: c.language.translate("i18n:ui_save"), CancelAutoLabel: c.language.translate("i18n:ui_cancel"),
		PickLightTitle: c.language.translate("i18n:ui_setting_theme_auto_pick_light"), PickDarkTitle: c.language.translate("i18n:ui_setting_theme_auto_pick_dark"),
		AutoPickerThemes: themeAutoPickerItems(snapshot.theme.Themes, snapshot),
		OnCreateAuto:     c.openNewAutoThemeEditor, OnEditAuto: c.openSelectedAutoThemeEditor,
		OnCancelAuto: c.closeAutoThemeEditor, OnSaveAuto: c.saveAutoThemeEditor,
		OnAutoNameChanged: func(value string) { _ = c.setAutoThemeName(value) }, OnSetAutoName: c.setAutoThemeName, OnAutoNameFocusChange: c.setAutoThemeNameFocused,
		OnAutoEditSlot: c.openAutoThemePicker, OnAutoPickTheme: c.pickAutoThemeEndpoint, OnCancelAutoPicker: c.closeAutoThemePicker,
	}
	if editor := themeSnap.AutoEditor; editor != nil {
		props.AutoEditor = &launcherview.AutoThemeEditorState{
			Active: editor.Active, Overwrite: editor.Overwrite, Name: editor.Name, NameFocused: editor.NameFocused,
			LightThemeID: editor.LightThemeID, DarkThemeID: editor.DarkThemeID, HoverSlot: editor.HoverSlot, PickerSlot: editor.PickerSlot,
			Error: editor.Error, Saving: editor.Saving,
		}
	}
	if themeSnap.ThemesLoading && len(themeSnap.Themes) == 0 {
		props.Message = c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_loading")
	} else if themeSnap.ThemesError != "" && len(themeSnap.Themes) == 0 {
		props.Message = themeSnap.ThemesError
		props.MessageError = true
	} else {
		c.applyThemeCatalogEmptyState(&props, themeSnap, filtered, searchActionTint, imageScale)
	}
	return props
}

// buildAutoThemeEditorOverlay hosts the Auto editor on the settings window overlay.
func (c *themeSettingsController) buildAutoThemeEditorOverlay(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	props := c.themeCatalogProps(snapshot, width, height, imageScale)
	props.OverlayWidth = width
	props.OverlayHeight = height
	return launcherview.ThemeAutoEditorOverlay(props)
}

func (c *themeSettingsController) applyThemeCatalogEmptyState(props *launcherview.ThemeSettingsProps, themeSnap themeSettingsSnapshot, filtered []filteredTheme, iconTint woxui.Color, imageScale float32) {
	if len(filtered) > 0 {
		return
	}
	emptyIconTint := iconTint
	emptyIconTint.A = 160
	props.EmptyIcon = c.imageForTint(settingControlIconSource("search"), &emptyIconTint, physicalImageSize(24, imageScale))
	if len(themeSnap.Themes) > 0 || strings.TrimSpace(themeSnap.ThemeSearch.Text) != "" {
		props.EmptyTitle = c.language.translate("i18n:ui_no_matches")
		props.EmptyDescription = c.language.translate("i18n:ui_setting_catalog_search_empty_subtitle")
		return
	}
	props.EmptyTitle = c.language.translate("i18n:ui_setting_theme_empty_data")
	props.EmptyDescription = c.language.translate("i18n:ui_setting_theme_empty_subtitle")
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
func (c *themeSettingsController) presentThemeCatalogItem(theme themeSettingsTheme, sourceIndex int, snapshot settingsSnapshot, width float32) launcherview.ThemeCatalogItem {
	item := themeCatalogItem(theme, sourceIndex, snapshot)
	if snapshot.theme.ThemesMode != "store" || len(theme.ScreenshotURLs) == 0 {
		return item
	}
	source := woxImage{ImageType: "url", ImageData: theme.ScreenshotURLs[0]}
	requestSize := int(min(float32(2048), max(float32(512), width*2)))
	item.Screenshot = c.imageForSurface(source, requestSize, settingsPalette().Background)
	item.ScreenshotLoading = item.Screenshot == nil
	item.OnScreenshot = func() { c.openPreviewImage(source) }
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
func (c *themeSettingsController) reloadThemes(mode, preferredID string) error {
	fallbackID := c.currentThemeID()
	return c.ReloadThemes(context.Background(), c.services, c.sessionID, mode, preferredID, fallbackID)
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
func (c *themeSettingsController) switchThemeSettingsMode(mode string) {
	if mode != "installed" && mode != "store" && mode != "editor" {
		return
	}
	if c.ThemeOperation() != "" || c.ThemesLoading() || c.ThemesMode() == mode {
		return
	}
	themeEditor := c.ThemeEditor()
	if c.ThemesMode() == "editor" && themeEditorDirtyLocked(themeEditor) {
		themeEditor.error = "Save the current theme changes before switching views."
		c.settings.invalidateSettingsWindow()
		return
	}
	c.SetThemesMode(mode)
	c.SetThemesError("")
	c.SetThemeUninstallArmed("")
	c.SetThemeSearchFocused(false)
	if mode != "editor" {
		c.SetThemes(nil)
		c.SetThemesLoaded(false)
		c.SetThemesLoading(true)
		c.SetThemeSelected(-1)
		c.SetThemeSearchEditor(woxui.NewTextEditor(""))
		c.SetThemeSearchFocused(false)
		c.SetThemeDetailTab("preview")
		c.SetAutoEditor(nil)
	}
	loadEditor := mode == "editor" && (themeEditor == nil || !strings.HasPrefix(themeEditor.key, "settings-theme|"))
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()

	if loadEditor {
		util.Go(c.lifecycleCtx, "load settings theme editor", func() {
			if err := c.loadSettingsThemeEditor(); err != nil {
				_ = c.runOnUI("apply settings theme editor error", func() {
					c.SetThemesError(err.Error())
					c.settings.invalidateSettingsWindow()
				})
			}
		})
		return
	}
	if mode != "editor" {
		util.Go(c.lifecycleCtx, "load theme catalog", func() {
			if err := c.reloadThemes(mode, ""); err != nil {
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
func (c *themeSettingsController) runThemeOperation(kind string) {
	themes := c.Themes()
	selected := c.ThemeSelected()
	if c.ThemeOperation() != "" || selected < 0 || selected >= len(themes) {
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
		if !theme.IsInstalled || c.generalSettings().Data().ThemeID == theme.ID {
			return
		}
	case "uninstall":
		if !theme.IsInstalled || theme.IsSystem {
			return
		}
		if c.ThemeUninstallArmed() != theme.ID {
			c.SetThemeUninstallArmed(theme.ID)
			c.settings.invalidateSettingsWindow()
			return
		}
	default:
		return
	}
	c.SetThemeUninstallArmed("")
	c.SetThemesError("")
	c.SetThemeOperation(kind + ":" + theme.ID)
	mode := c.ThemesMode()
	c.settings.invalidateSettingsWindow()

	util.Go(c.lifecycleCtx, kind+" theme", func() {
		operation := contract.ThemeOperation(kind)
		if kind == "upgrade" {
			operation = contract.ThemeOperationInstall
		}
		timeout := 30 * time.Second
		if kind == "install" || kind == "upgrade" {
			timeout = 2 * time.Minute
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := c.services.OperateTheme(ctx, c.sessionID, theme.ID, operation)
		cancel()
		if err == nil && kind == "apply" {
			err = c.reloadTheme()
			if err == nil {
				c.generalSettings().Update(func(d *settingsData) { d.ThemeID = theme.ID })
			}
		}
		if err == nil && kind == "upgrade" && theme.ID == c.currentThemeID() {
			err = c.reloadTheme()
		}
		if err == nil && kind == "uninstall" {
			if reloadErr := c.reloadTheme(); reloadErr != nil {
				err = reloadErr
			} else if reloadErr := c.reloadSettings(); reloadErr != nil {
				err = reloadErr
			}
		}
		if err == nil {
			// After uninstall the removed id is gone and core may have switched to Glass.
			// Keep the catalog on the theme that is actually applied.
			selectID := theme.ID
			if kind == "uninstall" {
				selectID = c.currentThemeID()
			}
			err = c.reloadThemes(mode, selectID)
		}
		_ = c.runOnUI("apply theme operation result", func() {
			c.SetThemeOperation("")
			if err != nil {
				c.SetThemesError(err.Error())
			} else {
				c.SetThemesError("")
			}
			c.settings.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("%s theme %s: %v", kind, theme.ID, err)
		}
	})
}

func (c *themeSettingsController) currentThemeID() string {
	return c.generalSettings().Data().ThemeID
}

func (c *themeSettingsController) openSelectedThemeWebsite() {
	themes := c.Themes()
	selected := c.ThemeSelected()
	if selected < 0 || selected >= len(themes) {
		return
	}
	target := strings.TrimSpace(themes[selected].URL)
	if target == "" {
		return
	}
	if err := c.settings.settingsNativeWindow().OpenExternalURL(target); err != nil {
		c.SetThemesError(err.Error())
		c.settings.invalidateSettingsWindow()
	}
}

func (c *themeSettingsController) selectTheme(index int) {
	themes := c.Themes()
	if c.ThemeOperation() != "" || index < 0 || index >= len(themes) {
		return
	}
	c.SetThemeSelected(index)
	c.SetThemeUninstallArmed("")
	c.SetThemesError("")
	c.SetAutoEditor(nil)
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) themeSearchQueryLocked() string {
	editor := c.ThemeSearchEditor()
	if editor == nil {
		return ""
	}
	return editor.State().Text
}

func (c *themeSettingsController) setThemeSearchFocused(focused bool) {
	if c.ThemeSearchEditor() == nil {
		c.SetThemeSearchEditor(woxui.NewTextEditor(""))
	}
	c.SetThemeSearchFocused(focused)
	if focused {
		c.settingsSearch().SetFocused(false)
		c.settingsSearch().SetPanel(false)
		c.pluginSettings().SetSearchFocused(false)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) setThemeSearchValue(value string) error {
	editor := c.ThemeSearchEditor()
	if editor == nil {
		c.SetThemeSearchEditor(woxui.NewTextEditor(value))
	} else {
		editor.SetText(value, false)
	}
	c.settings.invalidateSettingsWindow()
	return nil
}

func (c *themeSettingsController) onThemeSearchKey(event woxui.KeyEvent) bool {
	active := c.settings.open && c.settings.tab == "theme" && c.ThemesMode() != "editor" && c.ThemeSearchFocused() && c.ThemeSearchEditor() != nil
	if !active {
		return false
	}
	if event.Key == woxui.KeyEnter {
		return true
	}
	return false
}

func (c *themeSettingsController) onThemeSearchTextInput(_ woxui.TextInputEvent) bool {
	active := c.settings.open && c.settings.tab == "theme" && c.ThemesMode() != "editor" && c.ThemeSearchFocused() && c.ThemeSearchEditor() != nil
	return active
}

func (c *themeSettingsController) locateCurrentTheme() {
	if c.ThemesMode() != "installed" || c.generalSettings().Data().ThemeID == "" {
		return
	}
	editor := c.ThemeSearchEditor()
	if editor == nil {
		c.SetThemeSearchEditor(woxui.NewTextEditor(""))
	} else {
		editor.SetText("", false)
	}
	themes := c.Themes()
	for index, theme := range themes {
		if theme.ID == c.generalSettings().Data().ThemeID {
			c.SetThemeSelected(index)
			break
		}
	}
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) selectThemeDetailTab(tab string) {
	if tab != "preview" && tab != "description" {
		return
	}
	c.SetThemeDetailTab(tab)
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) moveFilteredThemeSelection(delta int) {
	themes := c.Themes()
	filtered := filterThemes(themes, c.themeSearchQueryLocked())
	selected := c.ThemeSelected()
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
	c.selectTheme(filtered[position].index)
}

// onThemeSettingsKey gives catalog selection the same basic keyboard access as plugin settings.
func (c *themeSettingsController) onThemeSettingsKey(event woxui.KeyEvent) bool {
	if editor := c.AutoEditor(); editor != nil && editor.active {
		return false
	}
	active := c.settings.open && c.settings.tab == "theme" && c.ThemesMode() != "editor" && !c.ThemeSearchFocused()
	themes := c.Themes()
	filtered := filterThemes(themes, c.themeSearchQueryLocked())
	if !active || len(filtered) == 0 {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		c.moveFilteredThemeSelection(-1)
	case woxui.KeyArrowDown:
		c.moveFilteredThemeSelection(1)
	case woxui.KeyEnter, woxui.KeySpace:
		selected := c.ThemeSelected()
		themes := c.Themes()
		if selected < 0 || selected >= len(themes) {
			return false
		}
		theme := themes[selected]
		current := c.currentThemeID()
		if !theme.IsInstalled {
			c.runThemeOperation("install")
		} else if theme.IsUpgradable {
			c.runThemeOperation("upgrade")
		} else if theme.ID != current {
			c.runThemeOperation("apply")
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

func (c *themeSettingsController) openNewAutoThemeEditor() {
	c.openAutoThemeEditor(autoThemeEditorState{
		active: true, nameEditor: woxui.NewTextEditor(c.language.translate("i18n:ui_setting_theme_auto_default_name")),
		nameFocused: true, lightThemeID: setting.DefaultLightThemeId, darkThemeID: setting.DefaultDarkThemeId,
	})
}

func (c *themeSettingsController) openSelectedAutoThemeEditor() {
	theme, ok := c.selectedInstalledTheme()
	if !ok || !theme.IsAuto || theme.IsSystem {
		return
	}
	c.openAutoThemeEditor(autoThemeEditorState{
		active: true, overwrite: true, sourceID: theme.ID, nameEditor: woxui.NewTextEditor(theme.Name),
		nameFocused: true, lightThemeID: theme.LightThemeID, darkThemeID: theme.DarkThemeID,
	})
}

func (c *themeSettingsController) openAutoThemeEditor(state autoThemeEditorState) {
	if c.ThemesMode() != "installed" || c.ThemeOperation() != "" {
		return
	}
	copied := state
	c.SetAutoEditor(&copied)
	c.SetThemeUninstallArmed("")
	c.SetThemeSearchFocused(false)
	c.SetThemesError("")
	c.updateSettingsText(true)
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) closeAutoThemeEditor() {
	c.SetAutoEditor(nil)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) setAutoThemeName(value string) error {
	editor := c.AutoEditor()
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
		c.settings.invalidateSettingsWindow()
	}
	return nil
}

func (c *themeSettingsController) setAutoThemeNameFocused(focused bool) {
	editor := c.AutoEditor()
	if editor == nil || !editor.active {
		return
	}
	editor.nameFocused = focused
	if focused {
		c.SetThemeSearchFocused(false)
		c.updateSettingsText(true)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) setAutoThemeHoverSlot(slot string) {
	editor := c.AutoEditor()
	if editor == nil || !editor.active || editor.hoverSlot == slot {
		return
	}
	editor.hoverSlot = slot
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) openAutoThemePicker(slot string) {
	if slot != "light" && slot != "dark" {
		return
	}
	editor := c.AutoEditor()
	if editor == nil || !editor.active {
		theme, ok := c.selectedInstalledTheme()
		if !ok || !theme.IsAuto || theme.IsSystem {
			return
		}
		c.openAutoThemeEditor(autoThemeEditorState{
			active: true, overwrite: true, sourceID: theme.ID, nameEditor: woxui.NewTextEditor(theme.Name),
			nameFocused: true, lightThemeID: theme.LightThemeID, darkThemeID: theme.DarkThemeID,
		})
		editor = c.AutoEditor()
	}
	if editor == nil || !editor.active || editor.saving {
		return
	}
	editor.pickerSlot = slot
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) closeAutoThemePicker() {
	editor := c.AutoEditor()
	if editor == nil || !editor.active {
		return
	}
	editor.pickerSlot = ""
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) pickAutoThemeEndpoint(themeID string) {
	editor := c.AutoEditor()
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
	c.settings.invalidateSettingsWindow()
}

func (c *themeSettingsController) saveAutoThemeEditor() {
	editor := c.AutoEditor()
	if editor == nil || !editor.active || editor.saving {
		return
	}
	name := ""
	if editor.nameEditor != nil {
		name = strings.TrimSpace(editor.nameEditor.State().Text)
	}
	if name == "" {
		editor.error = c.language.translate("i18n:ui_theme_editor_name_required")
		c.settings.invalidateSettingsWindow()
		return
	}
	editor.saving = true
	editor.error = ""
	overwrite := editor.overwrite
	sourceID := editor.sourceID
	lightID := editor.lightThemeID
	darkID := editor.darkThemeID
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "save auto theme", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		saved, err := c.services.SaveAutoTheme(ctx, c.sessionID, name, lightID, darkID, sourceID, overwrite)
		cancel()
		if err == nil {
			err = c.reloadThemes("installed", saved.ThemeId)
		}
		if err == nil && saved.ThemeId == c.currentThemeID() {
			err = c.reloadTheme()
		}
		_ = c.runOnUI("apply saved auto theme", func() {
			current := c.AutoEditor()
			if current == nil {
				c.settings.invalidateSettingsWindow()
				return
			}
			current.saving = false
			if err != nil {
				current.error = err.Error()
			} else {
				c.SetAutoEditor(nil)
				c.updateSettingsText(false)
			}
			c.settings.invalidateSettingsWindow()
		})
		if err != nil {
			log.Printf("save auto theme: %v", err)
		}
	})
}

func (c *themeSettingsController) selectedInstalledTheme() (themeSettingsTheme, bool) {
	themes := c.Themes()
	selected := c.ThemeSelected()
	if selected < 0 || selected >= len(themes) {
		return themeSettingsTheme{}, false
	}
	return themes[selected], true
}
