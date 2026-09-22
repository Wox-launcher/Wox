package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
	"wox/cloudsync"
	"wox/common"
	"wox/i18n"
	"wox/updater"
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
	mu        sync.RWMutex
	manifests []common.StoreThemeManifest
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
	s.RefreshThemeManifests(ctx)

	util.Go(ctx, "load store themes", func() {
		for range time.NewTicker(time.Minute * 10).C {
			s.RefreshThemeManifests(util.NewTraceContext())
		}
	})
}

func (s *Store) GetStoreThemes(ctx context.Context) ([]common.StoreThemeManifest, error) {
	var storeThemeManifests []common.StoreThemeManifest

	for _, store := range s.getStoreManifests(ctx) {
		themeManifest, manifestErr := s.GetStoreTheme(ctx, store)
		if manifestErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to get theme manifest from %s store: %s", store.Name, manifestErr.Error()))
			return nil, manifestErr
		}

		for _, manifest := range themeManifest {
			_, found := lo.Find(storeThemeManifests, func(m common.StoreThemeManifest) bool {
				return manifest.Id == m.Id
			})
			if found {
				continue
			}

			storeThemeManifests = append(storeThemeManifests, manifest)
		}
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("found %d themes from stores", len(storeThemeManifests)))
	return storeThemeManifests, nil
}

// RefreshThemeManifests replaces the catalog only after a successful fetch.
func (s *Store) RefreshThemeManifests(ctx context.Context) error {
	manifests, err := s.GetStoreThemes(ctx)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.manifests = manifests
	s.mu.Unlock()
	return nil
}

func (s *Store) GetStoreTheme(ctx context.Context, store storeManifest) ([]common.StoreThemeManifest, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("start to get theme manifest from %s(%s)", store.Name, store.Url))

	response, getErr := util.HttpGet(ctx, store.Url)
	if getErr != nil {
		return nil, getErr
	}

	return parseStoreThemes(ctx, response)
}

// parseStoreThemes keeps valid catalog entries when one row is incomplete.
func parseStoreThemes(ctx context.Context, data []byte) ([]common.StoreThemeManifest, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	manifests := make([]common.StoreThemeManifest, 0, len(entries))
	for _, entry := range entries {
		var manifest common.StoreThemeManifest
		if err := json.Unmarshal(entry, &manifest); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("skip invalid store theme: %s", err))
			continue
		}
		if err := manifest.Validate(); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("skip invalid store theme: %s", err))
			continue
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func (s *Store) Install(ctx context.Context, theme common.Theme) error {
	return s.install(ctx, theme, true, true)
}

// PersistInstalled writes a user theme without selecting it as the active theme.
func (s *Store) PersistInstalled(ctx context.Context, theme common.Theme) error {
	return s.install(ctx, theme, true, false)
}

// InstallLocal installs a theme from cloud sync without selecting it as the
// active theme; ThemeId sync owns the active-theme choice.
func (s *Store) InstallLocal(ctx context.Context, theme common.Theme) error {
	return s.install(ctx, theme, false, false)
}

// install shares persistence for user installs and cloud restores while
// controlling whether the theme should be applied and synced.
func (s *Store) install(ctx context.Context, theme common.Theme, syncInstall bool, applyTheme bool) error {
	if err := theme.EnsureWoxVersionSupported(updater.CURRENT_VERSION); err != nil {
		return err
	}
	if GetUIManager().IsSystemTheme(theme.ThemeId) {
		return fmt.Errorf("cannot overwrite system theme")
	}
	if err := theme.ValidateAssets(); err != nil {
		return err
	}
	logger.Info(ctx, fmt.Sprintf("start to install theme %s(%s)", theme.ThemeId, theme.ThemeAuthor))

	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
	GetUIManager().IgnoreThemeWatch(themePath)
	GetUIManager().IgnoreThemeWatch(filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId, "theme.json"))
	theme.IsInstalled = true
	theme.IsSystem = false

	themeJson, err := json.Marshal(theme)
	if err != nil {
		return err
	}

	var writeErr error
	if len(theme.AssetFiles) > 0 {
		writeErr = persistThemePackage(util.GetLocation().GetThemeDirectory(), theme)
		if writeErr == nil {
			if err := os.Remove(themePath); err != nil && !os.IsNotExist(err) {
				return err
			}
			themePath = filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId, "theme.json")
		}
	} else {
		packagePath := filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId, "theme.json")
		if util.IsFileExists(packagePath) {
			themePath = packagePath
		}
		writeErr = os.WriteFile(themePath, pretty.Pretty(themeJson), os.ModePerm)
	}
	if writeErr != nil {
		return writeErr
	}

	GetUIManager().rememberUserThemeFile(themePath, theme.ThemeId)
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

	if !common.ValidThemeAssetPath(theme.ThemeId) || filepath.Base(theme.ThemeId) != theme.ThemeId {
		return fmt.Errorf("invalid theme id")
	}
	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))
	GetUIManager().IgnoreThemeWatch(themePath)
	GetUIManager().IgnoreThemeWatch(filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId, "theme.json"))
	packagePath := filepath.Join(util.GetLocation().GetThemeDirectory(), theme.ThemeId)
	if util.IsFileExists(filepath.Join(packagePath, "theme.json")) {
		if err := trash.MoveToTrash(packagePath); err != nil {
			return err
		}
	}
	if util.IsFileExists(themePath) {
		removeErr := trash.MoveToTrash(themePath)
		if removeErr != nil {
			return removeErr
		}
	}

	GetUIManager().RemoveTheme(ctx, theme)
	if syncInstall {
		if _, ok := s.FindThemeManifest(ctx, theme.ThemeId); ok {
			s.logInstalledThemeDelete(ctx, theme.ThemeId)
		}
	}

	return nil
}

func (s *Store) GetThemeManifests() []common.StoreThemeManifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manifests
}

// QueueInstalledThemesForSync seeds installed store themes into the oplog.
// Themes created on this device are omitted.
func (s *Store) QueueInstalledThemesForSync(ctx context.Context) {
	GetUIManager().themes.Range(func(key string, theme common.Theme) bool {
		if theme.IsSystem {
			return true
		}
		s.logInstalledThemeUpsert(ctx, theme)
		return true
	})
}

// FindThemeManifest classifies local themes using the catalog, loading it if empty.
func (s *Store) FindThemeManifest(ctx context.Context, themeID string) (common.StoreThemeManifest, bool) {
	if manifest, ok := s.cachedThemeManifest(themeID); ok {
		return manifest, true
	}
	// A loaded catalog that does not contain the ID is a local theme. Don't
	// refetch the store for every local save.
	if len(s.GetThemeManifests()) > 0 {
		return common.StoreThemeManifest{}, false
	}
	s.RefreshThemeManifests(ctx)
	return s.cachedThemeManifest(themeID)
}

// ResolveThemeManifest refreshes cache misses during sync and preserves fetch errors.
// A stale catalog must not turn a remote installation into a successful no-op.
func (s *Store) ResolveThemeManifest(ctx context.Context, themeID string) (common.StoreThemeManifest, bool, error) {
	if manifest, ok := s.cachedThemeManifest(themeID); ok {
		return manifest, true, nil
	}
	if err := s.RefreshThemeManifests(ctx); err != nil {
		return common.StoreThemeManifest{}, false, err
	}
	manifest, ok := s.cachedThemeManifest(themeID)
	return manifest, ok, nil
}

func (s *Store) cachedThemeManifest(themeID string) (common.StoreThemeManifest, bool) {
	for _, manifest := range s.GetThemeManifests() {
		if manifest.Id == themeID {
			return manifest, true
		}
	}
	return common.StoreThemeManifest{}, false
}

// logInstalledThemeUpsert records a store theme ID. Local themes are not synced.
func (s *Store) logInstalledThemeUpsert(ctx context.Context, theme common.Theme) {
	if _, ok := s.FindThemeManifest(ctx, theme.ThemeId); !ok {
		return
	}
	if err := cloudsync.LogInstalledThemeUpsert(ctx, cloudsync.InstalledThemeValue{ID: theme.ThemeId}); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to log installed theme sync value for %s: %s", theme.ThemeId, err.Error()))
	}
}

// logInstalledThemeDelete records successful user-triggered theme removals.
func (s *Store) logInstalledThemeDelete(ctx context.Context, themeID string) {
	if err := cloudsync.LogInstalledThemeDelete(ctx, themeID); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to log installed theme delete for %s: %s", themeID, err.Error()))
	}
}
