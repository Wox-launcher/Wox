package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
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
}

// themeSettingsController owns the Theme tab state: catalog list, selection, search,
// install/apply operation, wallpaper loading, and the live editor draft. All 17 fields
// that used to live on App are now held here; App methods became thin wrappers that call
// the controller's getters/setters while coordinating cross-domain state on the UI thread.
type themeSettingsController struct {
	deps CommonDeps

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
}

func newThemeSettingsController(deps CommonDeps) *themeSettingsController {
	return &themeSettingsController{deps: deps}
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
		source := item.Theme
		theme := themeSettingsTheme{
			ID: source.ThemeId, Name: source.ThemeName, Author: source.ThemeAuthor, URL: source.ThemeUrl, Version: source.Version, Description: source.Description,
			IsSystem: source.IsSystem, IsInstalled: source.IsInstalled, IsUpgradable: item.IsUpgradable, IsAuto: source.IsAutoAppearance,
			DarkThemeID: source.DarkThemeId, LightThemeID: source.LightThemeId,
			previewTheme: fromCoreTheme(source),
		}
		themes = append(themes, theme)
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
		selected := 0
		for index, theme := range themes {
			if theme.ID == preferredID {
				selected = index
				break
			}
		}
		if len(themes) == 0 {
			c.themeSelected = -1
		} else {
			c.themeSelected = selected
		}
		c.deps.Invalidate()
	})
	return nil
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
	}
}
