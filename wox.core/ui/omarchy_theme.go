package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wox/common"
	"wox/setting"
	"wox/ui/omarchytheme"
	"wox/updater"
	"wox/util"

	"github.com/fsnotify/fsnotify"
	"github.com/samber/lo"
)

const (
	omarchyThemeReloadDelay = 200 * time.Millisecond
	omarchyThemeReadWindow  = time.Second
)

// loadOmarchyAutoTheme registers the live Omarchy theme when this machine is Omarchy.
// Other desktops skip it, so the theme never appears in their catalogs.
func (m *Manager) loadOmarchyAutoTheme(ctx context.Context) {
	if !omarchytheme.Available() {
		return
	}
	theme, err := omarchytheme.Load(updater.CURRENT_VERSION)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy auto theme uses fallback palette: %v", err))
		theme, err = omarchytheme.Fallback(updater.CURRENT_VERSION)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("omarchy auto theme fallback: %v", err))
			return
		}
	}
	m.replaceOmarchyAutoTheme(ctx, theme, false)
}

// watchOmarchyTheme follows palette changes for the life of the process.
// Omarchy publishes a switch by replacing current/theme, so the watch stays on
// the parent directory that survives that replacement.
func (m *Manager) watchOmarchyTheme(ctx context.Context) {
	current := omarchytheme.StateDir()
	if current == "" {
		return
	}
	var mu sync.Mutex
	watched := map[string]*util.DirectoryWatch{}
	var add func(string)
	add = func(dir string) {
		dir = filepath.Clean(dir)
		mu.Lock()
		_, exists := watched[dir]
		mu.Unlock()
		if exists {
			return
		}
		info, statErr := os.Stat(dir)
		if statErr != nil || !info.IsDir() {
			return
		}
		watch, watchErr := util.WatchDirectory(dir, func(event fsnotify.Event) {
			if ctx.Err() != nil {
				return
			}
			if filepath.Clean(event.Name) == current {
				add(current)
			}
			if omarchyThemeEvent(event) {
				m.scheduleOmarchyAutoReload(ctx)
			}
		}, func(err error) {
			util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy theme watch: %v", err))
		})
		if watchErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("watch omarchy theme directory %s: %v", dir, watchErr))
			return
		}
		mu.Lock()
		if _, exists := watched[dir]; exists {
			mu.Unlock()
			watch.Close()
			return
		}
		watched[dir] = watch
		mu.Unlock()
	}
	add(current)
	if _, err := os.Stat(current); err != nil {
		add(filepath.Dir(current))
	}
}

func (m *Manager) scheduleOmarchyAutoReload(ctx context.Context) {
	m.omarchyReloadMu.Lock()
	defer m.omarchyReloadMu.Unlock()
	if m.omarchyReloadTimer != nil {
		m.omarchyReloadTimer.Stop()
	}
	m.omarchyReloadTimer = time.AfterFunc(omarchyThemeReloadDelay, func() {
		if ctx.Err() != nil {
			return
		}
		m.reloadOmarchyAutoTheme(ctx)
	})
}

// reloadOmarchyAutoTheme reads the palette after a theme switch settles.
// The directory can be missing for a moment while Omarchy renames it into place.
func (m *Manager) reloadOmarchyAutoTheme(ctx context.Context) {
	deadline := time.Now().Add(omarchyThemeReadWindow)
	var theme common.Theme
	var err error
	for {
		if ctx.Err() != nil {
			return
		}
		theme, err = omarchytheme.Load(updater.CURRENT_VERSION)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			util.GetLogger().Warn(ctx, fmt.Sprintf("omarchy auto theme: %v", err))
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.replaceOmarchyAutoTheme(ctx, theme, true)
}

// replaceOmarchyAutoTheme stores a newly read palette. push applies it only when
// Omarchy Auto is the theme the user selected.
func (m *Manager) replaceOmarchyAutoTheme(ctx context.Context, theme common.Theme, push bool) {
	theme.IsSystem = true
	theme.IsInstalled = true
	previous, exists := m.themes.Load(omarchytheme.ThemeID)
	if exists && omarchyThemeColorsEqual(previous, theme) {
		return
	}
	m.themes.Store(omarchytheme.ThemeID, theme)
	if !lo.Contains(m.systemThemeIds, omarchytheme.ThemeID) {
		m.systemThemeIds = append(m.systemThemeIds, omarchytheme.ThemeID)
	}
	slug := omarchytheme.CurrentSlug()
	if slug == "" {
		slug = "fallback"
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("omarchy auto theme updated: %s", slug))
	if !push || m.ui == nil {
		return
	}
	if m.currentThemeID(ctx) != omarchytheme.ThemeID {
		return
	}
	impl, ok := m.ui.(*uiImpl)
	if !ok {
		return
	}
	impl.ChangeThemeWithoutSave(ctx, theme)
}

func omarchyThemeColorsEqual(left, right common.Theme) bool {
	return left.AppBackgroundColor == right.AppBackgroundColor &&
		left.ResultItemTitleColor == right.ResultItemTitleColor &&
		left.ResultItemSubTitleColor == right.ResultItemSubTitleColor &&
		left.ResultItemActiveTitleColor == right.ResultItemActiveTitleColor &&
		left.ResultItemActiveIndicatorColor == right.ResultItemActiveIndicatorColor &&
		left.ResultItemActiveBackgroundColor == right.ResultItemActiveBackgroundColor &&
		left.QueryBoxBackgroundColor == right.QueryBoxBackgroundColor &&
		left.ActionContainerBackgroundColor == right.ActionContainerBackgroundColor &&
		left.ToolbarBackgroundColor == right.ToolbarBackgroundColor &&
		left.OverlayBackgroundColor == right.OverlayBackgroundColor &&
		left.ResolvedColors()["AppBorderColor"] == right.ResolvedColors()["AppBorderColor"] &&
		left.ResolvedColors()["PreviewBackgroundColor"] == right.ResolvedColors()["PreviewBackgroundColor"] &&
		left.ResolvedColors()["ResultItemHoverBackgroundColor"] == right.ResolvedColors()["ResultItemHoverBackgroundColor"] &&
		sameOptionalInt(left.AppBorderWidth, right.AppBorderWidth) &&
		sameOptionalInt(left.AppBorderRadius, right.AppBorderRadius)
}

// sameOptionalInt compares authored geometry. An explicit zero differs from an omitted value.
func sameOptionalInt(left, right *int) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func omarchyThemeEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0
}

// omarchyAutoThemeForDisplay hides the theme from catalogs on machines that are not Omarchy.
func omarchyAutoThemeForDisplay(themeID string) bool {
	return showOmarchyAutoTheme(themeID, omarchytheme.Available())
}

// showOmarchyAutoTheme is true for every theme except Omarchy Auto on a machine that is not Omarchy.
func showOmarchyAutoTheme(themeID string, omarchyAvailable bool) bool {
	return themeID != omarchytheme.ThemeID || omarchyAvailable
}

// defaultThemeWhenOmarchyAutoUnavailable keeps a synced Omarchy Auto choice from
// leaving the launcher without a palette. The saved theme id is left unchanged.
func (m *Manager) defaultThemeWhenOmarchyAutoUnavailable(ctx context.Context, themeID string) (common.Theme, bool) {
	if themeID != omarchytheme.ThemeID || omarchytheme.Available() {
		return common.Theme{}, false
	}
	theme, ok := m.themes.Load(setting.DefaultThemeId)
	if !ok {
		return common.Theme{}, false
	}
	return m.resolvePlatformTheme(ctx, m.ThemeWithAssets(ctx, theme)), true
}
