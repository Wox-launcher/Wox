package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
	"github.com/tidwall/pretty"
	"os"
	"path"
	"sync"
	"time"
	"wox/util"
)

type storeManifest struct {
	Name string
	Url  string
}

var storeInstance *Store
var storeOnce sync.Once

type Store struct {
	themes []Theme
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
			Url:  "https://raw.githubusercontent.com/Wox-launcher/Wox/v2/theme-store.json",
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

func (s *Store) GetStoreThemes(ctx context.Context) []Theme {
	var storeThemeManifests []Theme

	for _, store := range s.getStoreManifests(ctx) {
		themeManifest, manifestErr := s.GetStoreTheme(ctx, store)
		if manifestErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get theme manifest from %s store: %s", store.Name, manifestErr.Error()))
			continue
		}

		for _, manifest := range themeManifest {
			_, found := lo.Find(storeThemeManifests, func(manifest Theme) bool {
				return manifest.ThemeId == manifest.ThemeId
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

func (s *Store) GetStoreTheme(ctx context.Context, store storeManifest) ([]Theme, error) {
	logger.Info(ctx, fmt.Sprintf("start to get theme manifest from %s(%s)", store.Name, store.Url))

	response, getErr := util.HttpGet(ctx, store.Url)
	if getErr != nil {
		return nil, getErr
	}

	var storeThemeManifests []Theme
	unmarshalErr := json.Unmarshal(response, &storeThemeManifests)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	return storeThemeManifests, nil
}

func (s *Store) Install(ctx context.Context, theme Theme) error {
	logger.Info(ctx, fmt.Sprintf("start to install theme %s(%s)", theme.ThemeId, theme.ThemeAuthor))

	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))

	themeJson, err := json.Marshal(theme)
	if err != nil {
		return err
	}

	writeErr := os.WriteFile(themePath, pretty.Pretty(themeJson), os.ModePerm)
	if writeErr != nil {
		return writeErr
	}

	GetUIManager().AddTheme(ctx, theme)

	return nil
}

func (s *Store) Uninstall(ctx context.Context, theme Theme) error {
	if GetUIManager().IsSystemTheme(theme.ThemeId) {
		return fmt.Errorf("can't uninstall system theme")
	}

	themePath := path.Join(util.GetLocation().GetThemeDirectory(), fmt.Sprintf("%s.json", theme.ThemeId))

	removeErr := os.Remove(themePath)
	if removeErr != nil {
		return removeErr
	}

	GetUIManager().RemoveTheme(ctx, theme)

	return nil
}

func (s *Store) GetThemes() []Theme {
	return s.themes
}
