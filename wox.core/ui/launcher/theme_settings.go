package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

const themeSettingsListRowHeight = float32(62)

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

// reloadThemes fetches one catalog while retaining the full resolved palette for local preview.
func (a *App) reloadThemes(mode, preferredID string) error {
	if mode != "store" && mode != "installed" {
		return fmt.Errorf("unsupported theme catalog %q", mode)
	}
	a.mu.Lock()
	a.themesLoading = true
	a.themesError = ""
	a.mu.Unlock()
	_ = a.window.Invalidate()

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
	_ = a.window.Invalidate()
	return nil
}

// finishThemeLoadError releases the loading gate on both transport and decode failures.
func (a *App) finishThemeLoadError(err error) error {
	a.mu.Lock()
	a.themesLoading = false
	a.themesLoaded = false
	a.themesError = err.Error()
	a.mu.Unlock()
	_ = a.window.Invalidate()
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
		_ = a.window.Invalidate()
		return
	}
	a.themesMode = mode
	a.themesError = ""
	a.themeUninstallArmed = ""
	if mode != "editor" {
		a.themes = nil
		a.themesLoaded = false
		a.themesLoading = true
		a.themeSelected = -1
		a.themeListScroll = 0
	}
	loadEditor := mode == "editor" && (a.themeEditor == nil || !strings.HasPrefix(a.themeEditor.key, "settings-theme|"))
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()

	if loadEditor {
		go func() {
			if err := a.loadSettingsThemeEditor(); err != nil {
				a.mu.Lock()
				a.themesError = err.Error()
				a.mu.Unlock()
				_ = a.window.Invalidate()
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
			_ = a.window.Invalidate()
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
	_ = a.window.Invalidate()

	go func() {
		path := "/theme/" + kind
		if kind == "upgrade" {
			path = "/theme/install"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.client.Post(ctx, path, map[string]string{"id": theme.ID}, nil)
		cancel()
		if err == nil && (kind == "apply" || kind == "install" || kind == "upgrade") {
			err = a.reloadTheme()
			if err == nil {
				a.mu.Lock()
				a.settings.ThemeID = theme.ID
				a.mu.Unlock()
			}
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
		_ = a.window.Invalidate()
	}()
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
	if err := a.window.OpenExternalURL(target); err != nil {
		a.mu.Lock()
		a.themesError = err.Error()
		a.mu.Unlock()
		_ = a.window.Invalidate()
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
	_ = a.window.Invalidate()
}

func (a *App) setThemeListViewport(height float32) {
	a.mu.Lock()
	a.themeListViewport = max(float32(1), height)
	a.ensureThemeSelectionVisibleLocked()
	a.mu.Unlock()
}

func (a *App) scrollThemeList(delta float32) {
	a.mu.Lock()
	a.themeListScroll += delta
	a.clampThemeListScrollLocked()
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) clampThemeListScrollLocked() {
	maxOffset := max(float32(0), float32(len(a.themes))*themeSettingsListRowHeight-max(float32(1), a.themeListViewport))
	a.themeListScroll = min(max(float32(0), a.themeListScroll), maxOffset)
}

// ensureThemeSelectionVisibleLocked follows keyboard selection without taking ownership from manual scrolling.
func (a *App) ensureThemeSelectionVisibleLocked() {
	viewport := a.themeListViewport
	if viewport <= 1 {
		viewport = 600
	}
	rowTop := float32(a.themeSelected) * themeSettingsListRowHeight
	rowBottom := rowTop + themeSettingsListRowHeight
	if rowTop < a.themeListScroll {
		a.themeListScroll = rowTop
	} else if rowBottom > a.themeListScroll+viewport {
		a.themeListScroll = rowBottom - viewport
	}
	a.clampThemeListScrollLocked()
}

// onThemeSettingsKey gives catalog selection the same basic keyboard access as plugin settings.
func (a *App) onThemeSettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.mode == viewSettings && a.settingTab == "theme" && a.themesMode != "editor"
	selected := a.themeSelected
	themeCount := len(a.themes)
	a.mu.RUnlock()
	if !active || themeCount == 0 {
		return false
	}
	switch event.Key {
	case woxui.KeyArrowUp:
		a.selectTheme((selected - 1 + themeCount) % themeCount)
	case woxui.KeyArrowDown:
		a.selectTheme((selected + 1) % themeCount)
	case woxui.KeyEnter, woxui.KeySpace:
		a.mu.RLock()
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
