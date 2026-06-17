package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"
	"wox/cloudsync"
	"wox/common"
	"wox/i18n"
	"wox/util"
	"wox/util/trash"

	"github.com/samber/lo"
	"github.com/tidwall/pretty"
)

type storeManifest struct {
	Name string
	Url  string
}

var storeInstance *Store
var storeOnce sync.Once

type Store struct {
	themes []common.Theme
}

func GetStoreManager() *Store {
	storeOnce.Do(func() {
		storeInstance = &Store{}
	})
	return storeInstance
}

func (s *Store) getStoreManifests(ctx context.Context) []storeManifest {
	return []storeManifest{
		{
			Name: "Wox Official Theme Store",
			Url:  "https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-theme.json",
		},
	}
}

func (s *Store) Start(ctx context.Context) {
	s.themes = s.GetStoreThemes(ctx)

	util.Go(ctx, "load theme plugins", func() {
		for range time.NewTicker(time.Minute * 10).C {
			pluginManifests := s.GetStoreThemes(util.NewTraceContext())
			if len(pluginManifests) > 0 {
				s.themes = pluginManifests
			}
		}
	})
}

func (s *Store) GetStoreThemes(ctx context.Context) []common.Theme {
	var storeThemeManifests []common.Theme

	for _, store := range s.getStoreManifests(ctx) {
		themeManifest, manifestErr := s.GetStoreTheme(ctx, store)
		if manifestErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get theme manifest from %s store: %s", store.Name, manifestErr.Error()))
			continue
		}

		for _, manifest := range themeManifest {
			_, found := lo.Find(storeThemeManifests, func(m common.Theme) bool {
				return manifest.ThemeId == m.ThemeId
			})
			if found {
				//skip duplicated theme
				continue
			}

			storeThemeManifests = append(storeThemeManifests, manifest)
		}
	}

	logger.Info(ctx, fmt.Sprintf("found %d themes from stores", len(storeThemeManifests)))
	return storeThemeManifests
}

func (s *Store) GetStoreTheme(ctx context.Context, store storeManifest) ([]common.Theme, error) {
	logger.Info(ctx, fmt.Sprintf("start to get theme manifest from %s(%s)", store.Name, store.Url))

	response, getErr := util.HttpGet(ctx, store.Url)
	if getErr != nil {
		return nil, getErr
	}

	var storeThemeManifests []common.Theme
	unmarshalErr := json.Unmarshal(response, &storeThemeManifests)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return storeThemeManifests, nil
}

func (s *Store) Install(ctx context.Context, theme common.Theme) error {
	return s.install(ctx, theme, true, true)
}

// InstallLocal installs a theme from cloud sync without selecting it as the
// active theme; ThemeId sync owns the active-theme choice.
func (s *Store) InstallLocal(ctx context.Context, theme common.Theme) error {
	return s.install(ctx, theme, false, false)
}

// install shares persistence for user installs and cloud restores while
// controlling whether the theme should be applied and synced.
func (s *Store) install(ctx context.Context, theme common.Theme, syncInstall bool, applyTheme bool) error {
	logger.Info(ctx, fmt.Sprintf("start to install theme %s(%s)", theme.ThemeId, theme.ThemeAuthor))

	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
	theme.IsInstalled = true
	theme.IsSystem = false

	themeJson, err := json.Marshal(theme)
	if err != nil {
		return err
	}

	writeErr := os.WriteFile(themePath, pretty.Pretty(themeJson), os.ModePerm)
	if writeErr != nil {
		return writeErr
	}

	if applyTheme {
		GetUIManager().AddTheme(ctx, theme)
	} else {
		GetUIManager().themes.Store(theme.ThemeId, theme)
	}
	if syncInstall {
		s.logInstalledThemeUpsert(ctx, theme)
	}

	return nil
}

func (s *Store) Uninstall(ctx context.Context, theme common.Theme) error {
	return s.uninstall(ctx, theme, true)
}

// UninstallLocal removes a theme from cloud sync without writing a new delete
// oplog.
func (s *Store) UninstallLocal(ctx context.Context, theme common.Theme) error {
	return s.uninstall(ctx, theme, false)
}

// uninstall shares user and cloud removal paths while controlling whether the
// removal is synced.
func (s *Store) uninstall(ctx context.Context, theme common.Theme, syncInstall bool) error {
	logger.Info(ctx, fmt.Sprintf("uninstalling theme: %s", theme.ThemeName))

	if GetUIManager().IsSystemTheme(theme.ThemeId) {
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_theme_uninstall_system_forbidden"))
	}

	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))

	if util.IsFileExists(themePath) {
		removeErr := trash.MoveToTrash(themePath)
		if removeErr != nil {
			return removeErr
		}
	}

	GetUIManager().RemoveTheme(ctx, theme)
	if syncInstall {
		s.logInstalledThemeDelete(ctx, theme.ThemeId)
	}

	return nil
}

func (s *Store) GetThemes() []common.Theme {
	return s.themes
}

// QueueInstalledThemesForSync seeds user themes into the oplog during first-time
// cloud sync bootstrap.
func (s *Store) QueueInstalledThemesForSync(ctx context.Context) {
	GetUIManager().themes.Range(func(key string, theme common.Theme) bool {
		if theme.IsSystem {
			return true
		}
		s.logInstalledThemeUpsert(ctx, theme)
		return true
	})
}

// logInstalledThemeUpsert records full theme JSON so custom themes can restore
// without relying on the remote theme store.
func (s *Store) logInstalledThemeUpsert(ctx context.Context, theme common.Theme) {
	themeJSON, err := json.Marshal(theme)
	if err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to encode installed theme sync value for %s: %s", theme.ThemeId, err.Error()))
		return
	}
	value := cloudsync.InstalledThemeValue{
		ID:      theme.ThemeId,
		Version: theme.Version,
		Source:  cloudsync.InstallSyncSourceUser,
		Theme:   themeJSON,
	}
	if err := cloudsync.LogInstalledThemeUpsert(ctx, value); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to log installed theme sync value for %s: %s", theme.ThemeId, err.Error()))
	}
}

// logInstalledThemeDelete records successful user-triggered theme removals.
func (s *Store) logInstalledThemeDelete(ctx context.Context, themeID string) {
	if err := cloudsync.LogInstalledThemeDelete(ctx, themeID); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to log installed theme delete for %s: %s", themeID, err.Error()))
	}
}
