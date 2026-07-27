package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
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
// the controller's getters/setters while still coordinating cross-domain state (focus
// routing, shared setting note) under a.mu before delegating.
type themeSettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

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
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]themeSettingsTheme(nil), c.themes...)
}

func (c *themeSettingsController) SetThemes(themes []themeSettingsTheme) {
	c.mu.Lock()
	c.themes = append([]themeSettingsTheme(nil), themes...)
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemesMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themesMode
}

func (c *themeSettingsController) SetThemesMode(mode string) {
	c.mu.Lock()
	c.themesMode = mode
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemesLoading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themesLoading
}

func (c *themeSettingsController) SetThemesLoading(loading bool) {
	c.mu.Lock()
	c.themesLoading = loading
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemesLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themesLoaded
}

func (c *themeSettingsController) SetThemesLoaded(loaded bool) {
	c.mu.Lock()
	c.themesLoaded = loaded
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemesError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themesError
}

func (c *themeSettingsController) SetThemesError(msg string) {
	c.mu.Lock()
	c.themesError = msg
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeSelected() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeSelected
}

func (c *themeSettingsController) SetThemeSelected(index int) {
	c.mu.Lock()
	c.themeSelected = index
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeSearchEditor() *woxui.TextEditor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeSearchEditor
}

func (c *themeSettingsController) SetThemeSearchEditor(editor *woxui.TextEditor) {
	c.mu.Lock()
	c.themeSearchEditor = editor
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeSearchFocused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeSearchFocused
}

func (c *themeSettingsController) SetThemeSearchFocused(focused bool) {
	c.mu.Lock()
	c.themeSearchFocused = focused
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeDetailTab() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeDetailTab
}

func (c *themeSettingsController) SetThemeDetailTab(tab string) {
	c.mu.Lock()
	c.themeDetailTab = tab
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeOperation() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeOperation
}

func (c *themeSettingsController) SetThemeOperation(op string) {
	c.mu.Lock()
	c.themeOperation = op
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeUninstallArmed() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeUninstallArmed
}

func (c *themeSettingsController) SetThemeUninstallArmed(id string) {
	c.mu.Lock()
	c.themeUninstallArmed = id
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeWallpaperPath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeWallpaperPath
}

func (c *themeSettingsController) SetThemeWallpaperPath(path string) {
	c.mu.Lock()
	c.themeWallpaperPath = path
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeWallpaperImage() *woxui.Image {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeWallpaperImage
}

func (c *themeSettingsController) SetThemeWallpaperImage(img *woxui.Image) {
	c.mu.Lock()
	c.themeWallpaperImage = img
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeWallpaperBlurred() *woxui.Image {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeWallpaperBlurred
}

func (c *themeSettingsController) SetThemeWallpaperBlurred(img *woxui.Image) {
	c.mu.Lock()
	c.themeWallpaperBlurred = img
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeWallpaperLoading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeWallpaperLoading
}

func (c *themeSettingsController) SetThemeWallpaperLoading(loading bool) {
	c.mu.Lock()
	c.themeWallpaperLoading = loading
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeWallpaperLoadID() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeWallpaperLoadID
}

func (c *themeSettingsController) SetThemeWallpaperLoadID(id uint64) {
	c.mu.Lock()
	c.themeWallpaperLoadID = id
	c.mu.Unlock()
}

func (c *themeSettingsController) ThemeEditor() *themeEditorPreviewState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.themeEditor
}

func (c *themeSettingsController) SetThemeEditor(editor *themeEditorPreviewState) {
	c.mu.Lock()
	c.themeEditor = editor
	c.mu.Unlock()
}

// ReloadThemes fetches one catalog while retaining the full resolved palette for local preview.
// preferredID, when non-empty, selects which theme becomes ThemeSelected after the load.
func (c *themeSettingsController) ReloadThemes(ctx context.Context, service contract.ThemeCatalogSettingsServices, sessionID string, mode, preferredID, fallbackID string) error {
	if mode != "store" && mode != "installed" {
		return fmt.Errorf("unsupported theme catalog %q", mode)
	}
	c.mu.Lock()
	c.themesLoading = true
	c.themesError = ""
	c.mu.Unlock()
	c.deps.Invalidate()

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
		}
		payload, err := json.Marshal(source)
		if err != nil {
			return c.finishThemeLoadError(fmt.Errorf("encode theme values: %w", err))
		}
		var raw map[string]any
		if err := json.Unmarshal(payload, &raw); err != nil {
			return c.finishThemeLoadError(fmt.Errorf("decode theme values: %w", err))
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

	c.mu.Lock()
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
	c.mu.Unlock()
	c.deps.Invalidate()
	return nil
}

// finishThemeLoadError releases the loading gate on both transport and decode failures.
func (c *themeSettingsController) finishThemeLoadError(err error) error {
	c.mu.Lock()
	c.themesLoading = false
	c.themesLoaded = false
	c.themesError = err.Error()
	c.mu.Unlock()
	c.deps.Invalidate()
	return err
}

// Snapshot returns a copy of the Theme state for the view layer.
func (c *themeSettingsController) Snapshot() themeSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
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
