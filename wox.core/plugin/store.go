package plugin

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
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
			Url:  "https://raw.githubusercontent.com/Wox-launcher/Wox/master/store-plugin.json",
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

	for i := range storePluginManifests {
		if IsSupportedRuntime(string(storePluginManifests[i].Runtime)) {
			storePluginManifests[i].Runtime = ConvertToRuntime(string(storePluginManifests[i].Runtime))
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

		// only uninstall for non-script plugins; script plugins will be hot-swapped with rollback
		if manifest.Runtime != PLUGIN_RUNTIME_SCRIPT {
			uninstallErr := s.Uninstall(ctx, installedPlugin)
			if uninstallErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error()))
				return fmt.Errorf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error())
			}
		}
	}

	// handle script plugins differently
	if manifest.Runtime == PLUGIN_RUNTIME_SCRIPT {
		return s.installScriptPlugin(ctx, manifest)
	} else {
		return s.installNormalPlugin(ctx, manifest)
	}
}

func (s *Store) installNormalPlugin(ctx context.Context, manifest StorePluginManifest) error {
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

func (s *Store) installScriptPlugin(ctx context.Context, manifest StorePluginManifest) error {
	logger.Info(ctx, fmt.Sprintf("detected script plugin, use script install flow: %s", manifest.Name))

	userScriptDir := util.GetLocation().GetUserScriptPluginsDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(userScriptDir); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to ensure user script plugin directory %s: %s", userScriptDir, err.Error()))
		return fmt.Errorf("failed to ensure user script plugin directory %s: %s", userScriptDir, err.Error())
	}

	// 1) find existing script by plugin id
	existingScriptPath := ""
	entries, readErr := os.ReadDir(userScriptDir)
	if readErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to read user script plugin directory: %s", readErr.Error()))
		return fmt.Errorf("failed to read user script plugin directory: %s", readErr.Error())
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == ".DS_Store" || name == "README.md" {
			continue
		}
		scriptPath := path.Join(userScriptDir, name)
		metadata, metaErr := GetPluginManager().ParseScriptMetadata(ctx, scriptPath)
		if metaErr != nil {
			continue
		}
		if strings.EqualFold(metadata.Id, manifest.Id) {
			existingScriptPath = scriptPath
			break
		}
	}

	// 2) move existing script to a temp directory for rollback
	backupDir := ""
	backupPath := ""
	hasBackup := false
	if existingScriptPath != "" {
		var mkErr error
		backupDir, mkErr = os.MkdirTemp("", "wox_script_backup_*")
		if mkErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to create temp directory for backup: %s", mkErr.Error()))
			return fmt.Errorf("failed to create temp directory for backup: %s", mkErr.Error())
		}
		backupPath = path.Join(backupDir, path.Base(existingScriptPath))

		// try rename; if cross-device, fallback to copy+remove
		if err := os.Rename(existingScriptPath, backupPath); err != nil {
			// fallback to copy
			srcF, cErr := os.Open(existingScriptPath)
			if cErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to open existing script for backup: %s", cErr.Error()))
				_ = os.RemoveAll(backupDir)
				return fmt.Errorf("failed to open existing script for backup: %s", cErr.Error())
			}
			defer srcF.Close()

			dstF, cErr := os.Create(backupPath)
			if cErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to create backup file: %s", cErr.Error()))
				_ = os.RemoveAll(backupDir)
				return fmt.Errorf("failed to create backup file: %s", cErr.Error())
			}
			_, cErr = io.Copy(dstF, srcF)
			closeErr := dstF.Close()
			if cErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to copy to backup file: %s", cErr.Error()))
				_ = os.Remove(backupPath)
				_ = os.RemoveAll(backupDir)
				return fmt.Errorf("failed to copy to backup file: %s", cErr.Error())
			}
			if closeErr != nil {
				logger.Warn(ctx, fmt.Sprintf("failed to close backup file: %s", closeErr.Error()))
			}
			if info, statErr := os.Stat(existingScriptPath); statErr == nil {
				_ = os.Chmod(backupPath, info.Mode())
			}
			// remove original after copying to complete the move
			if cErr = os.Remove(existingScriptPath); cErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to remove original script after backup: %s", cErr.Error()))
				_ = os.Remove(backupPath)
				_ = os.RemoveAll(backupDir)
				return fmt.Errorf("failed to remove original script after backup: %s", cErr.Error())
			}
		}
		hasBackup = true
	}

	// 3) derive new file name from url, fallback
	fileName := ""
	if u, err := url.Parse(manifest.DownloadUrl); err == nil {
		base := path.Base(u.Path)
		if base != "" && base != "." && base != "/" {
			fileName = base
		}
	}
	if fileName == "" {
		if existingScriptPath != "" {
			fileName = path.Base(existingScriptPath)
		} else {
			fileName = strings.ReplaceAll(strings.ToLower(fmt.Sprintf("%s_%s", manifest.Id, manifest.Name)), " ", "-") + ".sh"
		}
	}
	newScriptPath := path.Join(userScriptDir, fileName)

	// 4) download new script
	logger.Info(ctx, fmt.Sprintf("start to download script plugin: %s", manifest.DownloadUrl))
	if err := util.HttpDownload(ctx, manifest.DownloadUrl, newScriptPath); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to download script plugin %s(%s): %s", manifest.Name, manifest.Version, err.Error()))
		// rollback
		if hasBackup {
			_ = os.Remove(newScriptPath)
			_ = os.Rename(backupPath, existingScriptPath)
			_ = os.RemoveAll(backupDir)
		}
		return fmt.Errorf("failed to download script plugin %s(%s): %s", manifest.Name, manifest.Version, err.Error())
	}
	_ = os.Chmod(newScriptPath, 0755)

	// 5) parse metadata from the new script
	metadata, metaErr := GetPluginManager().ParseScriptMetadata(ctx, newScriptPath)
	if metaErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to parse script plugin metadata: %s", metaErr.Error()))
		// rollback
		if hasBackup {
			_ = os.Remove(newScriptPath)
			_ = os.Rename(backupPath, existingScriptPath)
			_ = os.RemoveAll(backupDir)
		}
		return fmt.Errorf("failed to parse script plugin metadata: %s", metaErr.Error())
	}
	if manifest.Id != "" && metadata.Id != "" && !strings.EqualFold(manifest.Id, metadata.Id) {
		logger.Warn(ctx, fmt.Sprintf("script metadata id(%s) not equal to store manifest id(%s), proceed with metadata id", metadata.Id, manifest.Id))
	}

	// 6) load (reload) the script plugin
	virtualDirectory := path.Join(userScriptDir, metadata.Id)
	loadErr := GetPluginManager().ReloadPlugin(ctx, MetadataWithDirectory{Metadata: metadata, Directory: virtualDirectory})
	if loadErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load script plugin %s(%s): %s", metadata.Name, metadata.Version, loadErr.Error()))
		// rollback
		if hasBackup {
			_ = os.Remove(newScriptPath)
			_ = os.Rename(backupPath, existingScriptPath)
			_ = os.RemoveAll(backupDir)
		}
		return fmt.Errorf("failed to load script plugin %s(%s): %s", metadata.Name, metadata.Version, loadErr.Error())
	}

	// 7) success - cleanup backup
	if hasBackup {
		_ = os.RemoveAll(backupDir)
	}

	logger.Info(ctx, fmt.Sprintf("script plugin %s(%s) installed", metadata.Name, metadata.Version))
	return nil
}

func (s *Store) ParsePluginManifestFromLocal(ctx context.Context, filePath string) (Metadata, error) {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to open wox plugin file: %s", err.Error())
	}
	defer reader.Close()

	var pluginMetadata Metadata
	for _, file := range reader.File {
		if file.Name != "plugin.json" {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return Metadata{}, fmt.Errorf("failed to read plugin.json: %s", err.Error())
		}
		defer rc.Close()

		bytes, err := io.ReadAll(rc)
		if err != nil {
			return Metadata{}, fmt.Errorf("failed to read plugin.json content: %s", err.Error())
		}

		err = json.Unmarshal(bytes, &pluginMetadata)
		if err != nil {
			return Metadata{}, fmt.Errorf("failed to parse plugin.json: %s", err.Error())
		}

		break
	}

	if pluginMetadata.Id == "" {
		return Metadata{}, fmt.Errorf("plugin.json not found or invalid")
	}

	return pluginMetadata, nil
}

func (s *Store) InstallFromLocal(ctx context.Context, filePath string) error {
	pluginMetadata, err := s.ParsePluginManifestFromLocal(ctx, filePath)
	if err != nil {
		return err
	}

	// check if plugin's runtime is started
	if !GetPluginManager().IsHostStarted(ctx, ConvertToRuntime(pluginMetadata.Runtime)) {
		logger.Error(ctx, fmt.Sprintf("%s runtime is not started, please start first", pluginMetadata.Runtime))
		return fmt.Errorf("%s runtime is not started, please start first", pluginMetadata.Runtime)
	}

	// check if installed newer version
	installedPlugin, exist := lo.Find(GetPluginManager().GetPluginInstances(), func(item *Instance) bool {
		return item.Metadata.Id == pluginMetadata.Id
	})
	if exist {
		logger.Info(ctx, fmt.Sprintf("found this plugin has installed %s(%s)", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version))
		installedVersion, installedErr := semver.NewVersion(installedPlugin.Metadata.Version)
		currentVersion, currentErr := semver.NewVersion(pluginMetadata.Version)
		if installedErr == nil && currentErr == nil {
			if installedVersion.GreaterThan(currentVersion) {
				logger.Info(ctx, fmt.Sprintf("skip %s(%s) from %s store, because it's already installed(%s)", pluginMetadata.Name, pluginMetadata.Version, pluginMetadata.Name, installedPlugin.Metadata.Version))
				return fmt.Errorf("skip %s(%s) from %s store, because it's already installed(%s)", pluginMetadata.Name, pluginMetadata.Version, pluginMetadata.Name, installedPlugin.Metadata.Version)
			}
		}

		uninstallErr := s.Uninstall(ctx, installedPlugin)
		if uninstallErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error()))
			return fmt.Errorf("failed to uninstall plugin %s(%s): %s", installedPlugin.Metadata.Name, installedPlugin.Metadata.Version, uninstallErr.Error())
		}
	}

	pluginDirectory := path.Join(util.GetLocation().GetPluginDirectory(), fmt.Sprintf("%s_%s@%s", pluginMetadata.Id, pluginMetadata.Name, pluginMetadata.Version))
	directoryErr := util.GetLocation().EnsureDirectoryExist(pluginDirectory)
	if directoryErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to create plugin directory %s: %s", pluginDirectory, directoryErr.Error()))
		return fmt.Errorf("failed to create plugin directory %s: %s", pluginDirectory, directoryErr.Error())
	}

	//unzip plugin
	logger.Info(ctx, fmt.Sprintf("start to unzip plugin %s(%s)", pluginMetadata.Name, pluginMetadata.Version))
	unzipErr := util.Unzip(filePath, pluginDirectory)
	if unzipErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to unzip plugin %s(%s): %s", pluginMetadata.Name, pluginMetadata.Version, unzipErr.Error()))
		return fmt.Errorf("failed to unzip plugin %s(%s): %s", pluginMetadata.Name, pluginMetadata.Version, unzipErr.Error())
	}

	//load plugin
	logger.Info(ctx, fmt.Sprintf("start to load plugin %s(%s)", pluginMetadata.Name, pluginMetadata.Version))
	loadErr := GetPluginManager().LoadPlugin(ctx, pluginDirectory)
	if loadErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load plugin %s(%s): %s", pluginMetadata.Name, pluginMetadata.Version, loadErr.Error()))

		// remove plugin directory
		// removeErr := os.RemoveAll(pluginDirectory)
		// if removeErr != nil {
		// 	logger.Error(ctx, fmt.Sprintf("failed to remove plugin directory %s: %s", pluginDirectory, removeErr.Error()))
		// }

		return fmt.Errorf("failed to load plugin %s(%s): %s", pluginMetadata.Name, pluginMetadata.Version, loadErr.Error())
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
		// uninstall for non-dev plugins
		if strings.EqualFold(plugin.Metadata.Runtime, string(PLUGIN_RUNTIME_SCRIPT)) {
			// script plugin: delete the actual script file under user scripts directory
			scriptPath := path.Join(util.GetLocation().GetUserScriptPluginsDirectory(), plugin.Metadata.Entry)
			if util.IsFileExists(scriptPath) {
				if removeErr := os.Remove(scriptPath); removeErr != nil {
					logger.Error(ctx, fmt.Sprintf("failed to remove script file %s: %s", scriptPath, removeErr.Error()))
					return removeErr
				}
			}
		} else {
			removeErr := os.RemoveAll(plugin.PluginDirectory)
			if removeErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to remove plugin directory %s: %s", plugin.PluginDirectory, removeErr.Error()))
				return removeErr
			}
		}
	}

	GetPluginManager().UnloadPlugin(ctx, plugin)

	return nil
}
