package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"wox/common"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// themeSettingsSnapshot is the immutable Theme tab state consumed by the view layer.
type themeSettingsSnapshot struct {
	Themes                []themeSettingsTheme
	ThemesMode            string
	ThemesLoading         bool
	ThemesLoaded          bool
	ThemesError           string
	ThemeSelected         int
	ThemeSearch           woxui.TextEditingState
	ThemeSearchFocused    bool
	ThemeDetailTab        string
	ThemeOperation        string
	ThemeUninstallArmed   string
	ThemeWallpaperPath    string
	ThemeWallpaperImage   *woxui.Image
	ThemeWallpaperBlurred *woxui.Image
	ThemeWallpaperLoading bool
	ThemeEditor           *themeEditorPreviewSnapshot
	AutoEditor            *autoThemeEditorSnapshot
}

// themeSettingsController owns the Theme settings page and the theme editor: catalog
// list, selection, search, install/apply operations, auto-theme editing, wallpaper
// loading, the live editor draft with its color dialog and AI assistant, and their view
// props. Applying a theme to every window stays with App through applyTheme; the other
// collaborators are injected by bindThemeSettings.
type themeSettingsController struct {
	deps CommonDeps

	services     themeSettingsServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	language     *languageCatalog
	settings     *settingsWindowController
	// onboarding shares the demo wallpaper with the first-run appearance page.
	onboarding *onboardingController

	generalSettings func() *generalSettingsController
	pluginSettings  func() *pluginSettingsController
	settingsSearch  func() *settingsSearchController

	reloadTheme             func() error
	applyTheme              func(theme themeData)
	reloadSettings          func() error
	imageForTint            func(woxImage, *woxui.Color, int) *woxui.Image
	imageForSurface         func(woxImage, int, woxui.Color) *woxui.Image
	updateSettingsText      func(bool)
	buildFormField          func(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette woxcomponent.ControlTheme, index int, definition formDefinition, width, height float32) woxwidget.Widget
	openPreviewImage        func(image woxImage)
	setSettingChoiceTooltip func(inside bool, text string, anchor woxui.Rect)
	setSettingsHoverTooltip func(inside bool, text string, anchor woxui.Rect, side string)

	// The editor's AI assistant renders its conversation with the AI Chat message views.
	chatDensityScale   func() float32
	chatRoundProps     func(item chatRenderItem, theme woxcomponent.Theme, toggle func()) previewview.ChatMessageProps
	prepareChatMessage func(props previewview.ChatMessageProps, window *woxui.Window, width, imageScale float32) previewview.ChatMessageProps

	themes                []themeSettingsTheme
	themesMode            string
	themesLoading         bool
	themesLoaded          bool
	themesError           string
	themeSelected         int
	themeSearchEditor     *woxui.TextEditor
	themeSearchFocused    bool
	themeDetailTab        string
	themeOperation        string
	themeUninstallArmed   string
	themeWallpaperPath    string
	themeWallpaperImage   *woxui.Image
	themeWallpaperBlurred *woxui.Image
	themeWallpaperLoading bool
	themeWallpaperLoadID  uint64
	themeEditor           *themeEditorPreviewState
	autoEditor            *autoThemeEditorState
}

// themeSettingsServices is the Core surface used by the Theme page and theme editor.
type themeSettingsServices interface {
	contract.ThemeCatalogSettingsServices
	contract.ThemeCurrentSettingsServices
	contract.ThemeOperationSettingsServices
	AIModels(ctx context.Context, sessionID string) ([]contract.AIModel, error)
}

func newThemeSettingsController(deps CommonDeps) *themeSettingsController {
	return &themeSettingsController{deps: deps}
}

// bindThemeSettings wires the Theme page to Core, the Settings window, the first-run
// guide and the App-wide theme application.
func (a *App) bindThemeSettings() {
	c := a.themeSettings
	if c == nil {
		return
	}
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.language = &a.language
	c.settings = &a.settingsWindow
	c.onboarding = &a.onboarding

	c.generalSettings = func() *generalSettingsController { return a.generalSettings }
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.settingsSearch = func() *settingsSearchController { return a.settingsSearch }

	c.reloadTheme = a.appearance.reloadTheme
	c.applyTheme = a.applyTheme
	c.reloadSettings = a.reloadSettings
	c.imageForTint = a.imageForTint
	c.imageForSurface = a.imageForSurface
	c.updateSettingsText = a.updateSettingsTextInput
	c.buildFormField = a.buildFormField
	c.openPreviewImage = a.openPreviewImageOverlay
	c.setSettingChoiceTooltip = a.setSettingChoiceTooltip
	c.setSettingsHoverTooltip = a.setSettingsHoverTooltip

	c.chatDensityScale = a.chatDensityScale
	c.chatRoundProps = a.chatRoundProps
	c.prepareChatMessage = a.prepareChatMessage
}

// themeEditorNativeWindow is the Settings window, which hosts the theme editor.
func (c *themeSettingsController) themeEditorNativeWindow() *woxui.Window {
	return c.settings.settingsNativeWindow()
}

func (c *themeSettingsController) invalidateThemeEditorWindow() {
	if window := c.themeEditorNativeWindow(); window != nil {
		_ = window.Invalidate()
	}
}

func (c *themeSettingsController) updateThemeEditorTextInput(enabled bool) {
	c.updateSettingsText(enabled)
}

func (c *themeSettingsController) restoreThemeEditorTextInput() {
	c.updateSettingsText(false)
}

func (c *themeSettingsController) Themes() []themeSettingsTheme {
	return append([]themeSettingsTheme(nil), c.themes...)
}

func (c *themeSettingsController) SetThemes(themes []themeSettingsTheme) {
	c.themes = append([]themeSettingsTheme(nil), themes...)
}

func (c *themeSettingsController) ThemesMode() string {
	return c.themesMode
}

func (c *themeSettingsController) SetThemesMode(mode string) {
	c.themesMode = mode
}

func (c *themeSettingsController) ThemesLoading() bool {
	return c.themesLoading
}

func (c *themeSettingsController) SetThemesLoading(loading bool) {
	c.themesLoading = loading
}

func (c *themeSettingsController) ThemesLoaded() bool {
	return c.themesLoaded
}

func (c *themeSettingsController) SetThemesLoaded(loaded bool) {
	c.themesLoaded = loaded
}

func (c *themeSettingsController) ThemesError() string {
	return c.themesError
}

func (c *themeSettingsController) SetThemesError(msg string) {
	c.themesError = msg
}

func (c *themeSettingsController) ThemeSelected() int {
	return c.themeSelected
}

func (c *themeSettingsController) SetThemeSelected(index int) {
	c.themeSelected = index
}

func (c *themeSettingsController) ThemeSearchEditor() *woxui.TextEditor {
	return c.themeSearchEditor
}

func (c *themeSettingsController) SetThemeSearchEditor(editor *woxui.TextEditor) {
	c.themeSearchEditor = editor
}

func (c *themeSettingsController) ThemeSearchFocused() bool {
	return c.themeSearchFocused
}

func (c *themeSettingsController) SetThemeSearchFocused(focused bool) {
	c.themeSearchFocused = focused
}

func (c *themeSettingsController) ThemeDetailTab() string {
	return c.themeDetailTab
}

func (c *themeSettingsController) SetThemeDetailTab(tab string) {
	c.themeDetailTab = tab
}

func (c *themeSettingsController) ThemeOperation() string {
	return c.themeOperation
}

func (c *themeSettingsController) SetThemeOperation(op string) {
	c.themeOperation = op
}

func (c *themeSettingsController) ThemeUninstallArmed() string {
	return c.themeUninstallArmed
}

func (c *themeSettingsController) SetThemeUninstallArmed(id string) {
	c.themeUninstallArmed = id
}

func (c *themeSettingsController) ThemeWallpaperPath() string {
	return c.themeWallpaperPath
}

func (c *themeSettingsController) SetThemeWallpaperPath(path string) {
	c.themeWallpaperPath = path
}

func (c *themeSettingsController) ThemeWallpaperImage() *woxui.Image {
	return c.themeWallpaperImage
}

func (c *themeSettingsController) SetThemeWallpaperImage(img *woxui.Image) {
	c.themeWallpaperImage = img
}

func (c *themeSettingsController) ThemeWallpaperBlurred() *woxui.Image {
	return c.themeWallpaperBlurred
}

func (c *themeSettingsController) SetThemeWallpaperBlurred(img *woxui.Image) {
	c.themeWallpaperBlurred = img
}

func (c *themeSettingsController) ThemeWallpaperLoading() bool {
	return c.themeWallpaperLoading
}

func (c *themeSettingsController) SetThemeWallpaperLoading(loading bool) {
	c.themeWallpaperLoading = loading
}

func (c *themeSettingsController) ThemeWallpaperLoadID() uint64 {
	return c.themeWallpaperLoadID
}

func (c *themeSettingsController) SetThemeWallpaperLoadID(id uint64) {
	c.themeWallpaperLoadID = id
}

func (c *themeSettingsController) ThemeEditor() *themeEditorPreviewState {
	return c.themeEditor
}

func (c *themeSettingsController) SetThemeEditor(editor *themeEditorPreviewState) {
	c.themeEditor = editor
}

func (c *themeSettingsController) AutoEditor() *autoThemeEditorState {
	return c.autoEditor
}

func (c *themeSettingsController) SetAutoEditor(editor *autoThemeEditorState) {
	c.autoEditor = editor
}

// ReloadThemes fetches one catalog while retaining the full resolved palette for local preview.
// preferredID, when non-empty, selects which theme becomes ThemeSelected after the load.
func (c *themeSettingsController) ReloadThemes(ctx context.Context, service contract.ThemeCatalogSettingsServices, sessionID string, mode, preferredID, fallbackID string) error {
	if mode != "store" && mode != "installed" {
		return fmt.Errorf("unsupported theme catalog %q", mode)
	}
	if !c.deps.OnUI("start loading theme catalog", func() {
		c.themesLoading = true
		c.themesError = ""
		c.deps.Invalidate()
	}) {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	items, err := service.Themes(timeoutCtx, sessionID, contract.ThemeCatalog(mode))
	if err != nil {
		return c.finishThemeLoadError(err)
	}

	themes := make([]themeSettingsTheme, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Manifest.Id) != "" {
			manifest := item.Manifest
			entry := themeSettingsTheme{
				ID: manifest.Id, Name: manifest.GetName(ctx), Author: manifest.Author, URL: manifest.Website,
				Version: manifest.Version, Description: manifest.GetDescription(ctx),
				IsInstalled: item.Theme.IsInstalled, IsUpgradable: item.IsUpgradable, ImageTheme: manifest.ImageTheme,
				ScreenshotURLs: append([]string(nil), manifest.ScreenshotUrls...),
			}
			if manifest.IconColors.HasColors() {
				entry.HasSwatch = true
				entry.previewTheme = storeThemeSwatchPreview(manifest.IconColors)
			}
			themes = append(themes, entry)
			continue
		}
		source := item.Theme
		// Installed documents do not carry the store ImageTheme flag. Packaged surface images are the same memory cost.
		sources, sourceErr := source.ThemeAssetSources()
		themes = append(themes, themeSettingsTheme{
			ID: source.ThemeId, Name: source.GetName(ctx), Author: source.ThemeAuthor, URL: source.ThemeUrl, Version: source.Version, Description: source.GetDescription(ctx),
			IsSystem: source.IsSystem, IsInstalled: source.IsInstalled, IsUpgradable: item.IsUpgradable, IsAuto: source.IsAutoAppearance,
			ImageTheme:  sourceErr == nil && len(sources) > 0,
			DarkThemeID: source.DarkThemeId, LightThemeID: source.LightThemeId,
			previewTheme: fromCoreTheme(source),
		})
	}
	sort.SliceStable(themes, func(i, j int) bool {
		if mode == "installed" && themes[i].IsSystem != themes[j].IsSystem {
			return themes[i].IsSystem
		}
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})

	c.deps.OnUI("apply theme catalog", func() {
		if preferredID == "" && c.themesMode == mode && c.themeSelected >= 0 && c.themeSelected < len(c.themes) {
			preferredID = c.themes[c.themeSelected].ID
		}
		if preferredID == "" && mode == "installed" {
			preferredID = fallbackID
		}
		c.themes = themes
		c.themesMode = mode
		c.themesLoading = false
		c.themesLoaded = true
		c.themesError = ""
		if c.themeSearchEditor == nil {
			c.themeSearchEditor = woxui.NewTextEditor("")
		}
		if c.themeDetailTab == "" {
			c.themeDetailTab = "preview"
		}
		c.themeSelected = themeCatalogSelection(themes, preferredID, fallbackID)
		c.deps.Invalidate()
	})
	return nil
}

// themeCatalogSelection keeps the list on the requested theme, then the applied
// theme, instead of the first row when an uninstalled id is gone.
// storeThemeSwatchPreview keeps only the colors the catalog icon paints.
func storeThemeSwatchPreview(colors common.StoreThemeIconColors) themeData {
	query := colors.Query
	if strings.EqualFold(strings.TrimSpace(query), "transparent") {
		query = "#00000000"
	}
	preview := themeData{
		AppBackgroundColor:              colors.Background,
		QueryBoxBackgroundColor:         query,
		ResultItemActiveBackgroundColor: colors.Selected,
		AppBorderColor:                  colors.Outline,
	}
	if colors.OutlineWidth > 0 && strings.TrimSpace(colors.Outline) != "" {
		width := colors.OutlineWidth
		preview.AppBorderWidth = &width
	}
	return preview
}

func themeCatalogSelection(themes []themeSettingsTheme, preferredID, fallbackID string) int {
	if len(themes) == 0 {
		return -1
	}
	if index := indexOfThemeID(themes, preferredID); index >= 0 {
		return index
	}
	if index := indexOfThemeID(themes, fallbackID); index >= 0 {
		return index
	}
	return 0
}

func indexOfThemeID(themes []themeSettingsTheme, id string) int {
	if id == "" {
		return -1
	}
	for index, theme := range themes {
		if theme.ID == id {
			return index
		}
	}
	return -1
}

// finishThemeLoadError releases the loading gate on both transport and decode failures.
func (c *themeSettingsController) finishThemeLoadError(err error) error {
	c.deps.OnUI("apply theme catalog error", func() {
		c.themesLoading = false
		c.themesLoaded = false
		c.themesError = err.Error()
		c.deps.Invalidate()
	})
	return err
}

// Snapshot returns a copy of the Theme state for the view layer.
func (c *themeSettingsController) Snapshot() themeSettingsSnapshot {
	var themeSearch woxui.TextEditingState
	if c.themeSearchEditor != nil {
		themeSearch = c.themeSearchEditor.State()
	}
	return themeSettingsSnapshot{
		Themes:                append([]themeSettingsTheme(nil), c.themes...),
		ThemesMode:            c.themesMode,
		ThemesLoading:         c.themesLoading,
		ThemesLoaded:          c.themesLoaded,
		ThemesError:           c.themesError,
		ThemeSelected:         c.themeSelected,
		ThemeSearch:           themeSearch,
		ThemeSearchFocused:    c.themeSearchFocused,
		ThemeDetailTab:        c.themeDetailTab,
		ThemeOperation:        c.themeOperation,
		ThemeUninstallArmed:   c.themeUninstallArmed,
		ThemeWallpaperPath:    c.themeWallpaperPath,
		ThemeWallpaperImage:   c.themeWallpaperImage,
		ThemeWallpaperBlurred: c.themeWallpaperBlurred,
		ThemeWallpaperLoading: c.themeWallpaperLoading,
		ThemeEditor:           snapshotThemeEditorPreviewLocked(c.themeEditor),
		AutoEditor:            snapshotAutoThemeEditor(c.autoEditor),
	}
}

type autoThemeEditorState struct {
	active       bool
	overwrite    bool
	sourceID     string
	nameEditor   *woxui.TextEditor
	nameFocused  bool
	lightThemeID string
	darkThemeID  string
	hoverSlot    string
	pickerSlot   string
	error        string
	saving       bool
}

type autoThemeEditorSnapshot struct {
	Active       bool
	Overwrite    bool
	SourceID     string
	Name         woxui.TextEditingState
	NameFocused  bool
	LightThemeID string
	DarkThemeID  string
	HoverSlot    string
	PickerSlot   string
	Error        string
	Saving       bool
}

func snapshotAutoThemeEditor(state *autoThemeEditorState) *autoThemeEditorSnapshot {
	if state == nil || !state.active {
		return nil
	}
	var name woxui.TextEditingState
	if state.nameEditor != nil {
		name = state.nameEditor.State()
	}
	return &autoThemeEditorSnapshot{
		Active: state.active, Overwrite: state.overwrite, SourceID: state.sourceID,
		Name: name, NameFocused: state.nameFocused, LightThemeID: state.lightThemeID, DarkThemeID: state.darkThemeID,
		HoverSlot: state.hoverSlot, PickerSlot: state.pickerSlot, Error: state.error, Saving: state.saving,
	}
}
