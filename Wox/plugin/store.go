package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"wox/util"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
)

type storeManifest struct {
	Name string
	Url  string
}

type StorePluginManifest struct {
	Id             string
	Name           string
	Author         string
	Version        string
	MinWoxVersion  string
	Runtime        Runtime
	Description    string
	IconUrl        string
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	DateCreated    string
	DateUpdated    string
}

var storeInstance *Store
var storeOnce sync.Once

type Store struct {
	pluginManifests []StorePluginManifest
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
			Name: "Wox Official Plugin Store",
			Url:  "https://raw.githubusercontent.com/Wox-launcher/Wox/v2/store-plugin.json",
		},
	}
}

// get plugin manifests from plugin stores, and update in the background every 10 minutes
func (s *Store) Start(ctx context.Context) {
	s.pluginManifests = s.GetStorePluginManifests(ctx)

	util.Go(ctx, "load store plugins immediately", func() {
		pluginManifests := s.GetStorePluginManifests(util.NewTraceContext())
		if len(pluginManifests) > 0 {
			s.pluginManifests = pluginManifests
		}
	})

	util.Go(ctx, "load store plugins", func() {
		for range time.NewTicker(time.Minute * 10).C {
			pluginManifests := s.GetStorePluginManifests(util.NewTraceContext())
			if len(pluginManifests) > 0 {
				s.pluginManifests = pluginManifests
			}
		}
	})
}

func (s *Store) GetStorePluginManifests(ctx context.Context) []StorePluginManifest {
	var storePluginManifests []StorePluginManifest

	for _, store := range s.getStoreManifests(ctx) {
		pluginManifest, manifestErr := s.GetStorePluginManifest(ctx, store)
		if manifestErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get plugin manifest from %s store: %s", store.Name, manifestErr.Error()))
			continue
		}

		for _, manifest := range pluginManifest {
			existingManifest, found := lo.Find(storePluginManifests, func(manifest StorePluginManifest) bool {
				return manifest.Id == manifest.Id
			})
			if found {
				existingVersion, existingErr := semver.NewVersion(existingManifest.Version)
				currentVersion, currentErr := semver.NewVersion(manifest.Version)
				if existingErr != nil && currentErr != nil {
					if existingVersion.GreaterThan(currentVersion) {
						logger.Info(ctx, fmt.Sprintf("skip %s(%s) from %s store, because it's already installed(%s)", manifest.Name, manifest.Version, store.Name, existingManifest.Version))
						continue
					}
				}
			}

			storePluginManifests = append(storePluginManifests, manifest)
		}
	}

	logger.Info(ctx, fmt.Sprintf("found %d plugins from stores", len(storePluginManifests)))
	return storePluginManifests
}

func (s *Store) GetStorePluginManifest(ctx context.Context, store storeManifest) ([]StorePluginManifest, error) {
	logger.Info(ctx, fmt.Sprintf("start to get plugin manifest from %s(%s)", store.Name, store.Url))

	response, getErr := util.HttpGet(ctx, store.Url)
	if getErr != nil {
		return nil, getErr
	}

	var storePluginManifests []StorePluginManifest
	unmarshalErr := json.Unmarshal(response, &storePluginManifests)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	var finalStorePluginManifests []StorePluginManifest
	for i := range storePluginManifests {
		if IsSupportedRuntime(string(storePluginManifests[i].Runtime)) {
			storePluginManifests[i].Runtime = Runtime(strings.ToUpper(string(storePluginManifests[i].Runtime)))
			finalStorePluginManifests = append(finalStorePluginManifests, storePluginManifests[i])
		}
	}

	return storePluginManifests, nil
}

func (s *Store) GetStorePluginManifestById(ctx context.Context, id string) (StorePluginManifest, error) {
	manifest, found := lo.Find(s.pluginManifests, func(manifest StorePluginManifest) bool {
		return manifest.Id == id
	})
	if found {
		return manifest, nil
	}

	return StorePluginManifest{}, fmt.Errorf("plugin %s not found", id)
}

func (s *Store) Search(ctx context.Context, keyword string) []StorePluginManifest {
	return lo.Filter(s.pluginManifests, func(manifest StorePluginManifest, _ int) bool {
		if keyword == "" {
			return true
		}

		return util.IsStringMatch(manifest.Name, keyword, false)
	})
}

func (s *Store) Install(ctx context.Context, manifest StorePluginManifest) error {
	logger.Info(ctx, fmt.Sprintf("start to install plugin %s(%s)", manifest.Name, manifest.Version))

	// check if plugin's runtime is started
	if !GetPluginManager().IsHostStarted(ctx, manifest.Runtime) {
		logger.Error(ctx, fmt.Sprintf("%s runtime is not started, please start first", manifest.Runtime))
		return fmt.Errorf("%s runtime is not started, please start first", manifest.Runtime)
	}

	// check if installed newer version
	installedPlugin, exist := lo.Find(GetPluginManager().GetPluginInstances(), func(item *Instance) bool {
		return item.Metadata.Id == manifest.Id
	})
	if exist {
		logger.Info(ctx, fmt.Sprintf("found this plugin has installed %s(%s)", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version))
		installedVersion, installedErr := semver.NewVersion(installedPlugin.Metadata.Version)
		currentVersion, currentErr := semver.NewVersion(manifest.Version)
		if installedErr == nil && currentErr == nil {
			if installedVersion.GreaterThan(currentVersion) {
				logger.Info(ctx, fmt.Sprintf("skip %s(%s) from %s store, because it's already installed(%s)", manifest.Name, manifest.Version, manifest.Name, installedPlugin.Metadata.Version))
				return fmt.Errorf("skip %s(%s) from %s store, because it's already installed(%s)", manifest.Name, manifest.Version, manifest.Name, installedPlugin.Metadata.Version)
			}
		}

		uninstallErr := s.Uninstall(ctx, installedPlugin)
		if uninstallErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error()))
			return fmt.Errorf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error())
		}
	}

	// download plugin
	logger.Info(ctx, fmt.Sprintf("start to download plugin: %s", manifest.DownloadUrl))
	pluginDirectory := path.Join(util.GetLocation().GetPluginDirectory(), fmt.Sprintf("%s_%s@%s", manifest.Id, manifest.Name, manifest.Version))
	directoryErr := util.GetLocation().EnsureDirectoryExist(pluginDirectory)
	if directoryErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to create plugin directory %s: %s", pluginDirectory, directoryErr.Error()))
		return fmt.Errorf("failed to create plugin directory %s: %s", pluginDirectory, directoryErr.Error())
	}
	pluginZipPath := path.Join(pluginDirectory, "plugin.zip")
	downloadErr := util.HttpDownload(ctx, manifest.DownloadUrl, pluginZipPath)
	if downloadErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to download plugin %s(%s): %s", manifest.Name, manifest.Version, downloadErr.Error()))
		removeErr := os.Remove(pluginZipPath)
		if removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove plugin zip %s: %s", pluginZipPath, removeErr.Error()))
		}
		return fmt.Errorf("failed to download plugin %s(%s): %s", manifest.Name, manifest.Version, downloadErr.Error())
	}

	//unzip plugin
	logger.Info(ctx, fmt.Sprintf("start to unzip plugin %s(%s)", manifest.Name, manifest.Version))
	unzipErr := util.Unzip(pluginZipPath, pluginDirectory)
	if unzipErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to unzip plugin %s(%s): %s", manifest.Name, manifest.Version, unzipErr.Error()))
		removeErr := os.Remove(pluginZipPath)
		if removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove plugin zip %s: %s", pluginZipPath, removeErr.Error()))
		}
		return fmt.Errorf("failed to unzip plugin %s(%s): %s", manifest.Name, manifest.Version, unzipErr.Error())
	}

	//load plugin
	logger.Info(ctx, fmt.Sprintf("start to load plugin %s(%s)", manifest.Name, manifest.Version))
	loadErr := GetPluginManager().LoadPlugin(ctx, pluginDirectory)
	if loadErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load plugin %s(%s): %s", manifest.Name, manifest.Version, loadErr.Error()))

		// remove plugin zip and directory
		removeErr := os.RemoveAll(pluginDirectory)
		if removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove plugin directory %s: %s", pluginDirectory, removeErr.Error()))
		}
		removeErr = os.Remove(pluginZipPath)
		if removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove plugin zip %s: %s", pluginZipPath, removeErr.Error()))
		}

		return fmt.Errorf("failed to load plugin %s(%s): %s", manifest.Name, manifest.Version, loadErr.Error())
	}

	//remove plugin zip
	removeErr := os.Remove(pluginZipPath)
	if removeErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to remove plugin zip %s: %s", pluginZipPath, removeErr.Error()))
		return fmt.Errorf("failed to remove plugin zip %s: %s", pluginZipPath, removeErr.Error())
	}

	return nil
}

func (s *Store) Uninstall(ctx context.Context, plugin *Instance) error {
	logger.Info(ctx, fmt.Sprintf("start to uninstall plugin %s(%s)", plugin.Metadata.Name, plugin.Metadata.Version))

	if plugin.IsDevPlugin {
		var wpmPlugin *Instance
		for _, instance := range GetPluginManager().GetPluginInstances() {
			if instance.Metadata.Id == "e2c5f005-6c73-43c8-bc53-ab04def265b2" {
				wpmPlugin = instance
				break
			}
		}
		if wpmPlugin != nil {
			query, _ := newQueryInputWithPlugins("wpm dev.remove "+plugin.DevPluginDirectory, GetPluginManager().GetPluginInstances())
			wpmPlugin.Plugin.Query(ctx, query)
		}
	} else {
		removeErr := os.RemoveAll(plugin.PluginDirectory)
		if removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove plugin directory %s: %s", plugin.PluginDirectory, removeErr.Error()))
			return removeErr
		}
	}

	GetPluginManager().UnloadPlugin(ctx, plugin)

	return nil
}
