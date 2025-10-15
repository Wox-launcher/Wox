package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/ai"
	"wox/common"
	"wox/i18n"
	"wox/setting"

	"wox/util"
	"wox/util/notifier"
	"wox/util/selection"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/jinzhu/copier"
	"github.com/samber/lo"
	"github.com/wissance/stringFormatter"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type debounceTimer struct {
	timer  *time.Timer
	onStop func()
}

type Manager struct {
	instances          []*Instance
	ui                 common.UI
	resultCache        *util.HashMap[string, *QueryResultCache]
	debounceQueryTimer *util.HashMap[string, *debounceTimer]
	aiProviders        *util.HashMap[common.ProviderName, ai.Provider]

	activeBrowserUrl string //active browser url before wox is activated

	// Script plugin monitoring
	scriptPluginWatcher *fsnotify.Watcher
	scriptReloadTimers  *util.HashMap[string, *time.Timer]
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			resultCache:        util.NewHashMap[string, *QueryResultCache](),
			debounceQueryTimer: util.NewHashMap[string, *debounceTimer](),
			aiProviders:        util.NewHashMap[common.ProviderName, ai.Provider](),
			scriptReloadTimers: util.NewHashMap[string, *time.Timer](),
		}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Start(ctx context.Context, ui common.UI) error {
	m.ui = ui

	loadErr := m.loadPlugins(ctx)
	if loadErr != nil {
		return fmt.Errorf("failed to load plugins: %w", loadErr)
	}

	// Start script plugin monitoring
	util.Go(ctx, "start script plugin monitoring", func() {
		m.startScriptPluginMonitoring(util.NewTraceContext())
	})

	util.Go(ctx, "start store manager", func() {
		GetStoreManager().Start(util.NewTraceContext())
	})

	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	// Stop script plugin monitoring
	if m.scriptPluginWatcher != nil {
		m.scriptPluginWatcher.Close()
	}

	for _, host := range AllHosts {
		host.Stop(ctx)
	}
}

func (m *Manager) SetActiveBrowserUrl(url string) {
	m.activeBrowserUrl = url
}

func (m *Manager) loadPlugins(ctx context.Context) error {
	logger.Info(ctx, "start loading plugins")

	// load system plugin first
	m.loadSystemPlugins(ctx)

	logger.Debug(ctx, "start loading user plugin metadata")
	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to read plugin directory (%s), continue without user plugins: %s", basePluginDirectory, readErr.Error()))
		pluginDirectories = []os.DirEntry{}
	}

	var metaDataList []MetadataWithDirectory
	for _, entry := range pluginDirectories {
		if entry.Name() == ".DS_Store" {
			continue
		}
		if !entry.IsDir() {
			continue
		}

		pluginDirectory := path.Join(basePluginDirectory, entry.Name())
		metadata, metadataErr := m.ParseMetadata(ctx, pluginDirectory)
		if metadataErr != nil {
			logger.Error(ctx, metadataErr.Error())
			continue
		}

		//check if metadata already exist, only add newer version
		existMetadata, exist := lo.Find(metaDataList, func(item MetadataWithDirectory) bool {
			return item.Metadata.Id == metadata.Id
		})
		if exist {
			existVersion, existVersionErr := semver.NewVersion(existMetadata.Metadata.Version)
			currentVersion, currentVersionErr := semver.NewVersion(metadata.Version)
			if existVersionErr == nil && currentVersionErr == nil {
				if existVersion.GreaterThan(currentVersion) || existVersion.Equal(currentVersion) {
					logger.Info(ctx, fmt.Sprintf("skip parse %s(%s) metadata, because it's already parsed(%s)", metadata.Name, metadata.Version, existMetadata.Metadata.Version))
					continue
				} else {
					// remove older version
					logger.Info(ctx, fmt.Sprintf("remove older metadata version %s(%s)", existMetadata.Metadata.Name, existMetadata.Metadata.Version))
					var newMetaDataList []MetadataWithDirectory
					for _, item := range metaDataList {
						if item.Metadata.Id != existMetadata.Metadata.Id {
							newMetaDataList = append(newMetaDataList, item)
						}
					}
					metaDataList = newMetaDataList
				}
			}
		}
		metaDataList = append(metaDataList, MetadataWithDirectory{Metadata: metadata, Directory: pluginDirectory})
	}

	// Load script plugins
	scriptMetaDataList, err := m.loadScriptPlugins(ctx)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load script plugins: %s", err.Error()))
	} else {
		metaDataList = append(metaDataList, scriptMetaDataList...)
	}

	logger.Info(ctx, fmt.Sprintf("start loading user plugins, found %d user plugins", len(metaDataList)))

	for _, host := range AllHosts {
		util.Go(ctx, fmt.Sprintf("[%s] start host", host.GetRuntime(ctx)), func() {
			newCtx := util.NewTraceContext()
			hostErr := host.Start(newCtx)
			if hostErr != nil {
				logger.Error(newCtx, fmt.Errorf("[%s HOST] %w", host.GetRuntime(newCtx), hostErr).Error())
				return
			}

			for _, metadata := range metaDataList {
				if !strings.EqualFold(metadata.Metadata.Runtime, string(host.GetRuntime(newCtx))) {
					continue
				}

				loadErr := m.loadHostPlugin(newCtx, host, metadata)
				if loadErr != nil {
					logger.Error(newCtx, fmt.Errorf("[%s HOST] %w", host.GetRuntime(newCtx), loadErr).Error())
					continue
				}
			}
		})
	}

	return nil
}

// loadScriptPlugins loads script plugins from the user script plugins directory
func (m *Manager) loadScriptPlugins(ctx context.Context) ([]MetadataWithDirectory, error) {
	logger.Debug(ctx, "start loading script plugin metadata")

	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
	scriptFiles, readErr := os.ReadDir(userScriptPluginDirectory)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read user script plugin directory: %w", readErr)
	}

	var metaDataList []MetadataWithDirectory
	for _, entry := range scriptFiles {
		if entry.Name() == ".DS_Store" || entry.Name() == "README.md" {
			continue
		}
		if entry.IsDir() {
			continue
		}

		scriptPath := path.Join(userScriptPluginDirectory, entry.Name())
		metadata, metadataErr := m.ParseScriptMetadata(ctx, scriptPath)
		if metadataErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse script plugin metadata for %s: %s", entry.Name(), metadataErr.Error()))
			continue
		}

		// Create a virtual directory for the script plugin
		virtualDirectory := path.Join(userScriptPluginDirectory, metadata.Id)
		metaDataList = append(metaDataList, MetadataWithDirectory{
			Metadata:  metadata,
			Directory: virtualDirectory,
		})
	}

	logger.Debug(ctx, fmt.Sprintf("found %d script plugins", len(metaDataList)))
	return metaDataList, nil
}

func (m *Manager) ReloadPlugin(ctx context.Context, metadata MetadataWithDirectory) error {
	logger.Info(ctx, fmt.Sprintf("start reloading dev plugin: %s", metadata.Metadata.Name))

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.ToLower(string(item.GetRuntime(ctx))) == strings.ToLower(metadata.Metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Metadata.Runtime)
	}

	pluginInstance, pluginInstanceExist := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == metadata.Metadata.Id
	})
	if pluginInstanceExist {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is loaded, unload first", metadata.Metadata.Name))
		m.UnloadPlugin(ctx, pluginInstance)
	} else {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is not loaded, skip unload", metadata.Metadata.Name))
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, metadata)
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) loadHostPlugin(ctx context.Context, host Host, metadata MetadataWithDirectory) error {
	loadStartTimestamp := util.GetSystemTimestamp()
	plugin, loadErr := host.LoadPlugin(ctx, metadata.Metadata, metadata.Directory)
	if loadErr != nil {
		logger.Error(ctx, fmt.Errorf("[%s HOST] failed to load plugin: %w", host.GetRuntime(ctx), loadErr).Error())
		return loadErr
	}
	loadFinishTimestamp := util.GetSystemTimestamp()

	instance := &Instance{
		Metadata:              metadata.Metadata,
		PluginDirectory:       metadata.Directory,
		Plugin:                plugin,
		Host:                  host,
		LoadStartTimestamp:    loadStartTimestamp,
		LoadFinishedTimestamp: loadFinishTimestamp,
		IsDevPlugin:           metadata.IsDev,
		DevPluginDirectory:    metadata.DevPluginDirectory,
	}
	instance.API = NewAPI(instance)
	pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Metadata.Id, metadata.Metadata.SettingDefinitions.ToMap())
	if settingErr != nil {
		instance.API.Log(ctx, LogLevelError, fmt.Errorf("[SYS] failed to load plugin[%s] setting: %w", metadata.Metadata.Name, settingErr).Error())
		return settingErr
	}
	instance.Setting = pluginSetting

	m.instances = append(m.instances, instance)

	if pluginSetting.Disabled.Get() {
		logger.Info(ctx, fmt.Errorf("[%s HOST] plugin is disabled by user, skip init: %s", host.GetRuntime(ctx), metadata.Metadata.Name).Error())
		instance.API.Log(ctx, LogLevelWarning, fmt.Sprintf("[SYS] plugin is disabled by user, skip init: %s", metadata.Metadata.Name))
		return nil
	}

	util.Go(ctx, fmt.Sprintf("[%s] init plugin", metadata.Metadata.Name), func() {
		m.initPlugin(ctx, instance)
	})

	return nil
}

func (m *Manager) LoadPlugin(ctx context.Context, pluginDirectory string) error {
	metadata, parseErr := m.ParseMetadata(ctx, pluginDirectory)
	if parseErr != nil {
		return parseErr
	}

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.ToLower(string(item.GetRuntime(ctx))) == strings.ToLower(metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}

	// Ensure host is started before loading the plugin
	if !pluginHost.IsStarted(ctx) {
		if err := pluginHost.Start(ctx); err != nil {
			return fmt.Errorf("failed to start host for runtime %s: %w", metadata.Runtime, err)
		}
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, MetadataWithDirectory{Metadata: metadata, Directory: pluginDirectory})
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
	for _, callback := range pluginInstance.UnloadCallbacks {
		callback()
	}
	pluginInstance.Host.UnloadPlugin(ctx, pluginInstance.Metadata)

	var newInstances []*Instance
	for _, instance := range m.instances {
		if instance.Metadata.Id != pluginInstance.Metadata.Id {
			newInstances = append(newInstances, instance)
		}
	}
	m.instances = newInstances
}

func (m *Manager) loadSystemPlugins(ctx context.Context) {
	start := util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("start loading system plugins, found %d system plugins", len(AllSystemPlugin)))

	for _, plugin := range AllSystemPlugin {
		util.Go(ctx, fmt.Sprintf("load system plugin <%s>", plugin.GetMetadata().Name), func() {
			metadata := plugin.GetMetadata()
			instance := &Instance{
				Metadata:              metadata,
				Plugin:                plugin,
				Host:                  nil,
				IsSystemPlugin:        true,
				LoadStartTimestamp:    util.GetSystemTimestamp(),
				LoadFinishedTimestamp: util.GetSystemTimestamp(),
			}
			instance.API = NewAPI(instance)

			startTimestamp := util.GetSystemTimestamp()
			pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
			if settingErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to load system plugin[%s] setting, use default plugin setting. err: %s", metadata.Name, settingErr.Error()))
				return
			}

			instance.Setting = pluginSetting
			if util.GetSystemTimestamp()-startTimestamp > 100 {
				logger.Warn(ctx, fmt.Sprintf("load system plugin[%s] setting too slow, cost %d ms", metadata.Name, util.GetSystemTimestamp()-startTimestamp))
			}

			m.instances = append(m.instances, instance)

			m.initPlugin(util.NewTraceContext(), instance)
		})
	}

	logger.Debug(ctx, fmt.Sprintf("finish loading system plugins, cost %d ms", util.GetSystemTimestamp()-start))
}

func (m *Manager) initPlugin(ctx context.Context, instance *Instance) {
	logger.Info(ctx, fmt.Sprintf("start init plugin: %s", instance.Metadata.Name))
	instance.InitStartTimestamp = util.GetSystemTimestamp()
	instance.Plugin.Init(ctx, InitParams{
		API:             instance.API,
		PluginDirectory: instance.PluginDirectory,
	})
	instance.InitFinishedTimestamp = util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("init plugin %s finished, cost %d ms", instance.Metadata.Name, instance.InitFinishedTimestamp-instance.InitStartTimestamp))
}

func (m *Manager) ParseMetadata(ctx context.Context, pluginDirectory string) (Metadata, error) {
	configPath := path.Join(pluginDirectory, "plugin.json")
	if _, statErr := os.Stat(configPath); statErr != nil {
		return Metadata{}, fmt.Errorf("missing plugin.json file in %s", configPath)
	}

	metadataJson, err := os.ReadFile(configPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read plugin.json file: %w", err)
	}

	var metadata Metadata
	unmarshalErr := json.Unmarshal(metadataJson, &metadata)
	if unmarshalErr != nil {
		return Metadata{}, fmt.Errorf("failed to unmarshal plugin.json file (%s): %w", pluginDirectory, unmarshalErr)
	}

	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing trigger keywords in plugin.json file (%s)", pluginDirectory)
	}
	if !IsSupportedRuntime(metadata.Runtime) {
		return Metadata{}, fmt.Errorf("unsupported runtime in plugin.json file (%s), runtime=%s", pluginDirectory, metadata.Runtime)
	}
	if !IsSupportedOSAny(metadata.SupportedOS) {
		return Metadata{}, fmt.Errorf("unsupported os in plugin.json file (%s), os=%s", pluginDirectory, metadata.SupportedOS)
	}

	return metadata, nil
}

// ParseScriptMetadata parses metadata from script plugin file comments
func (m *Manager) ParseScriptMetadata(ctx context.Context, scriptPath string) (Metadata, error) {
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read script file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	metadata := Metadata{
		Runtime: string(PLUGIN_RUNTIME_SCRIPT),
		Entry:   filepath.Base(scriptPath),
		Version: "1.0.0", // Default version
	}

	// Parse metadata from comments
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Stop parsing when we reach non-comment lines (except shebang)
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "#!/") {
			if line != "" {
				break
			}
			continue
		}

		// Remove comment markers
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimPrefix(line, "#!/usr/bin/env")
		line = strings.TrimPrefix(line, "#!/bin/")
		line = strings.TrimSpace(line)

		// Parse @wox.xxx metadata
		if strings.HasPrefix(line, "@wox.") {
			parts := strings.SplitN(line, " ", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimPrefix(parts[0], "@wox.")
			value := strings.TrimSpace(parts[1])

			switch key {
			case "id":
				metadata.Id = value
			case "name":
				metadata.Name = value
			case "author":
				metadata.Author = value
			case "version":
				metadata.Version = value
			case "description":
				metadata.Description = value
			case "icon":
				metadata.Icon = value
			case "keywords":
				// Split keywords by comma or space
				keywords := strings.FieldsFunc(value, func(c rune) bool {
					return c == ',' || c == ' '
				})
				for i, keyword := range keywords {
					keywords[i] = strings.TrimSpace(keyword)
				}
				metadata.TriggerKeywords = keywords
			case "minWoxVersion":
				metadata.MinWoxVersion = value
			}
		}
	}

	// Validate required fields
	if metadata.Id == "" {
		return Metadata{}, fmt.Errorf("missing required field: @wox.id")
	}
	if metadata.Name == "" {
		return Metadata{}, fmt.Errorf("missing required field: @wox.name")
	}
	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing required field: @wox.keywords")
	}

	// Set default values
	if metadata.Author == "" {
		metadata.Author = "Unknown"
	}
	if metadata.Description == "" {
		metadata.Description = "A script plugin"
	}
	if metadata.Icon == "" {
		metadata.Icon = "emoji:📝"
	}
	if metadata.MinWoxVersion == "" {
		metadata.MinWoxVersion = "2.0.0"
	}

	// Set supported OS to all platforms by default
	metadata.SupportedOS = []string{"Windows", "Linux", "Macos"}

	return metadata, nil
}

// startScriptPluginMonitoring starts monitoring the user script plugins directory for changes
func (m *Manager) startScriptPluginMonitoring(ctx context.Context) {
	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()

	// Create file system watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to create script plugin watcher: %s", err.Error()))
		return
	}

	m.scriptPluginWatcher = watcher

	// Add the script plugins directory to the watcher
	err = watcher.Add(userScriptPluginDirectory)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to watch script plugin directory: %s", err.Error()))
		watcher.Close()
		return
	}

	logger.Info(ctx, fmt.Sprintf("Started monitoring script plugins directory: %s", userScriptPluginDirectory))

	// Start watching for events
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				logger.Info(ctx, "Script plugin watcher closed")
				return
			}
			m.handleScriptPluginEvent(ctx, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				logger.Info(ctx, "Script plugin watcher error channel closed")
				return
			}
			logger.Error(ctx, fmt.Sprintf("Script plugin watcher error: %s", err.Error()))
		case <-ctx.Done():
			logger.Info(ctx, "Script plugin monitoring stopped due to context cancellation")
			watcher.Close()
			return
		}
	}
}

// handleScriptPluginEvent handles file system events for script plugins
func (m *Manager) handleScriptPluginEvent(ctx context.Context, event fsnotify.Event) {
	// Skip non-script files and temporary files
	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, "~") || strings.HasSuffix(fileName, ".tmp") {
		return
	}

	// Skip directories
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		return
	}

	logger.Info(ctx, fmt.Sprintf("Script plugin file event: %s (%s)", event.Name, event.Op))

	switch event.Op {
	case fsnotify.Create, fsnotify.Write:
		// File created or modified - reload the plugin
		m.debounceScriptPluginReload(ctx, event.Name, "file changed")
	case fsnotify.Remove, fsnotify.Rename:
		// File deleted or renamed - unload the plugin
		m.unloadScriptPluginByPath(ctx, event.Name)
	case fsnotify.Chmod:
		// File permissions changed - ignore for now
		logger.Debug(ctx, fmt.Sprintf("Script plugin file permissions changed: %s", event.Name))
	}
}

// debounceScriptPluginReload debounces script plugin reload to avoid multiple reloads in a short time
func (m *Manager) debounceScriptPluginReload(ctx context.Context, scriptPath, reason string) {
	fileName := filepath.Base(scriptPath)

	// Cancel existing timer if any
	if timer, exists := m.scriptReloadTimers.Load(fileName); exists {
		timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(2*time.Second, func() {
		m.reloadScriptPlugin(util.NewTraceContext(), scriptPath, reason)
		m.scriptReloadTimers.Delete(fileName)
	})

	m.scriptReloadTimers.Store(fileName, timer)
}

// reloadScriptPlugin reloads a script plugin
func (m *Manager) reloadScriptPlugin(ctx context.Context, scriptPath, reason string) {
	logger.Info(ctx, fmt.Sprintf("Reloading script plugin: %s, reason: %s", scriptPath, reason))

	// Check if file still exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		logger.Warn(ctx, fmt.Sprintf("Script plugin file no longer exists: %s", scriptPath))
		return
	}

	// Parse metadata from the script file
	metadata, err := m.ParseScriptMetadata(ctx, scriptPath)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to parse script plugin metadata: %s", err.Error()))
		return
	}

	// Find and unload existing plugin instance if any
	existingInstance, exists := lo.Find(m.instances, func(instance *Instance) bool {
		return instance.Metadata.Id == metadata.Id
	})
	if exists {
		logger.Info(ctx, fmt.Sprintf("Unloading existing script plugin instance: %s", metadata.Name))
		m.UnloadPlugin(ctx, existingInstance)
	}

	// Create metadata with directory for loading
	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
	virtualDirectory := path.Join(userScriptPluginDirectory, metadata.Id)
	metadataWithDirectory := MetadataWithDirectory{
		Metadata:  metadata,
		Directory: virtualDirectory,
	}

	// Find script plugin host
	scriptHost, hostExists := lo.Find(AllHosts, func(host Host) bool {
		return host.GetRuntime(ctx) == PLUGIN_RUNTIME_SCRIPT
	})
	if !hostExists {
		logger.Error(ctx, "Script plugin host not found")
		return
	}

	// Load the plugin
	err = m.loadHostPlugin(ctx, scriptHost, metadataWithDirectory)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to reload script plugin: %s", err.Error()))
		return
	}

	logger.Info(ctx, fmt.Sprintf("Successfully reloaded script plugin: %s", metadata.Name))
}

// unloadScriptPluginByPath unloads a script plugin by its file path
func (m *Manager) unloadScriptPluginByPath(ctx context.Context, scriptPath string) {
	fileName := filepath.Base(scriptPath)
	logger.Info(ctx, fmt.Sprintf("Unloading script plugin: %s", fileName))

	// Find plugin instance by script file name
	var pluginToUnload *Instance
	for _, instance := range m.instances {
		if instance.Metadata.Runtime == string(PLUGIN_RUNTIME_SCRIPT) && instance.Metadata.Entry == fileName {
			pluginToUnload = instance
			break
		}
	}

	if pluginToUnload != nil {
		logger.Info(ctx, fmt.Sprintf("Found script plugin to unload: %s", pluginToUnload.Metadata.Name))
		m.UnloadPlugin(ctx, pluginToUnload)
	} else {
		logger.Debug(ctx, fmt.Sprintf("No script plugin found for file: %s", fileName))
	}
}

func (m *Manager) GetPluginInstances() []*Instance {
	return m.instances
}

func (m *Manager) canOperateQuery(ctx context.Context, pluginInstance *Instance, query Query) bool {
	if pluginInstance.Setting.Disabled.Get() {
		return false
	}

	if query.Type == QueryTypeSelection {
		isPluginSupportSelection := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQuerySelection)
		return isPluginSupportSelection
	}

	var validGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), "*") && query.TriggerKeyword == ""
	var validNonGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword)
	if !validGlobalQuery && !validNonGlobalQuery {
		return false
	}

	return true
}

func (m *Manager) queryForPlugin(ctx context.Context, pluginInstance *Instance, query Query) (results []QueryResult) {
	defer util.GoRecover(ctx, fmt.Sprintf("<%s> query panic", pluginInstance.Metadata.Name), func(err error) {
		// if plugin query panic, return error result
		failedResult := m.GetResultForFailedQuery(ctx, pluginInstance.Metadata, query, err)
		results = []QueryResult{
			m.PolishResult(ctx, pluginInstance, query, failedResult),
		}
	})

	logger.Info(ctx, fmt.Sprintf("<%s> start query: %s", pluginInstance.Metadata.Name, query.RawQuery))
	start := util.GetSystemTimestamp()

	// set query env base on plugin's feature
	currentEnv := query.Env
	newEnv := QueryEnv{}
	if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQueryEnv) {
		queryEnvParams, err := pluginInstance.Metadata.GetFeatureParamsForQueryEnv()
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("<%s> invalid query env config: %s", pluginInstance.Metadata.Name, err))
		} else {
			if queryEnvParams.RequireActiveWindowName {
				newEnv.ActiveWindowTitle = currentEnv.ActiveWindowTitle
			}
			if queryEnvParams.RequireActiveWindowPid {
				newEnv.ActiveWindowPid = currentEnv.ActiveWindowPid
			}
			if queryEnvParams.RequireActiveWindowIcon {
				newEnv.ActiveWindowIcon = currentEnv.ActiveWindowIcon
			}
			if queryEnvParams.RequireActiveBrowserUrl {
				newEnv.ActiveBrowserUrl = currentEnv.ActiveBrowserUrl
			}
		}
	}
	query.Env = newEnv

	results = pluginInstance.Plugin.Query(ctx, query)
	logger.Debug(ctx, fmt.Sprintf("<%s> finish query, result count: %d, cost: %dms", pluginInstance.Metadata.Name, len(results), util.GetSystemTimestamp()-start))

	for i := range results {
		if results[i].Group == "" {
			defaultActions := m.getDefaultActions(ctx, pluginInstance, query, results[i].Title, results[i].SubTitle)
			results[i].Actions = append(results[i].Actions, defaultActions...)
		}
		results[i] = m.PolishResult(ctx, pluginInstance, query, results[i])
	}

	if query.Type == QueryTypeSelection && query.Search != "" {
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		results = lo.Filter(results, func(item QueryResult, _ int) bool {
			match, _ := util.IsStringMatchScore(item.Title, query.Search, woxSetting.UsePinYin.Get())
			return match
		})
	}

	return results
}

func (m *Manager) GetResultForFailedQuery(ctx context.Context, pluginMetadata Metadata, query Query, err error) QueryResult {
	overlayIcon := common.NewWoxImageEmoji("🚫")
	pluginIcon := common.ParseWoxImageOrDefault(pluginMetadata.Icon, overlayIcon)
	icon := pluginIcon.OverlayFullPercentage(overlayIcon, 0.6)

	return QueryResult{
		Title:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_query_failed"), pluginMetadata.Name),
		SubTitle: util.EllipsisEnd(err.Error(), 20),
		Icon:     icon,
		Preview: WoxPreview{
			PreviewType: WoxPreviewTypeText,
			PreviewData: err.Error(),
		},
	}
}

func (m *Manager) getDefaultActions(ctx context.Context, pluginInstance *Instance, query Query, title, subTitle string) (defaultActions []QueryResultAction) {
	if setting.GetSettingManager().IsFavoriteResult(ctx, pluginInstance.Metadata.Id, title, subTitle) {
		defaultActions = append(defaultActions, QueryResultAction{
			Name:           "i18n:plugin_manager_remove_from_favorite",
			Icon:           RemoveFromFavIcon,
			IsSystemAction: true,
			Action: func(ctx context.Context, actionContext ActionContext) {
				setting.GetSettingManager().RemoveFavoriteResult(ctx, pluginInstance.Metadata.Id, title, subTitle)
			},
		})
	} else {
		defaultActions = append(defaultActions, QueryResultAction{
			Name:           "i18n:plugin_manager_add_to_favorite",
			Icon:           AddToFavIcon,
			IsSystemAction: true,
			Action: func(ctx context.Context, actionContext ActionContext) {
				setting.GetSettingManager().AddFavoriteResult(ctx, pluginInstance.Metadata.Id, title, subTitle)
			},
		})
	}

	return defaultActions
}

func (m *Manager) PolishResult(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult) QueryResult {
	// set default id
	if result.Id == "" {
		result.Id = uuid.NewString()
	}
	for actionIndex := range result.Actions {
		if result.Actions[actionIndex].Id == "" {
			result.Actions[actionIndex].Id = uuid.NewString()
		}
		if result.Actions[actionIndex].Icon.IsEmpty() {
			// set default action icon if not present
			result.Actions[actionIndex].Icon = DefaultActionIcon
		}
	}

	originalIcon := result.Icon

	// convert icon
	result.Icon = common.ConvertIcon(ctx, result.Icon, pluginInstance.PluginDirectory)
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeImage {
			result.Tails[i].Image = common.ConvertIcon(ctx, result.Tails[i].Image, pluginInstance.PluginDirectory)
		}
	}

	// add default preview for selection query if no preview is set
	if query.Type == QueryTypeSelection && result.Preview.PreviewType == "" {
		if query.Selection.Type == selection.SelectionTypeText {
			result.Preview = WoxPreview{
				PreviewType: WoxPreviewTypeText,
				PreviewData: query.Selection.Text,
			}
		}
		if query.Selection.Type == selection.SelectionTypeFile {
			result.Preview = WoxPreview{
				PreviewType: WoxPreviewTypeMarkdown,
				PreviewData: m.formatFileListPreview(ctx, query.Selection.FilePaths),
			}
		}
	}

	// translate title
	result.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	// translate subtitle
	result.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	// translate preview properties
	var previewProperties = make(map[string]string)
	for key, value := range result.Preview.PreviewProperties {
		translatedKey := m.translatePlugin(ctx, pluginInstance, key)
		previewProperties[translatedKey] = value
	}
	result.Preview.PreviewProperties = previewProperties
	// translate action names
	for actionIndex := range result.Actions {
		result.Actions[actionIndex].Name = m.translatePlugin(ctx, pluginInstance, result.Actions[actionIndex].Name)
	}
	// translate preview data if preview type is text
	if result.Preview.PreviewType == WoxPreviewTypeText || result.Preview.PreviewType == WoxPreviewTypeMarkdown {
		result.Preview.PreviewData = m.translatePlugin(ctx, pluginInstance, result.Preview.PreviewData)
	}

	// set first action as default if no default action is set
	defaultActionCount := lo.CountBy(result.Actions, func(item QueryResultAction) bool {
		return item.IsDefault
	})
	if defaultActionCount == 0 && len(result.Actions) > 0 {
		result.Actions[0].IsDefault = true
		result.Actions[0].Hotkey = "Enter"
	}

	//move default action to first one of the actions
	sort.Slice(result.Actions, func(i, j int) bool {
		return result.Actions[i].IsDefault
	})

	var resultCache = &QueryResultCache{
		ResultId:       result.Id,
		ResultTitle:    result.Title,
		ResultSubTitle: result.SubTitle,
		ContextData:    result.ContextData,
		Icon:           originalIcon,
		PluginInstance: pluginInstance,
		Query:          query,
		Actions:        util.NewHashMap[string, func(ctx context.Context, actionContext ActionContext)](),
	}

	// store actions for ui invoke later
	for actionIndex := range result.Actions {
		var action = result.Actions[actionIndex]

		// if default action's hotkey is empty, set it as Enter
		if action.IsDefault && action.Hotkey == "" {
			result.Actions[actionIndex].Hotkey = "Enter"
		}

		// normalize hotkey for platform specific modifiers
		result.Actions[actionIndex].Hotkey = normalizeHotkeyForPlatform(result.Actions[actionIndex].Hotkey)

		if action.Action != nil {
			resultCache.Actions.Store(action.Id, action.Action)
		}
	}

	// if query is input and trigger keyword is global, disable preview and group
	if query.IsGlobalQuery() {
		result.Preview = WoxPreview{}
		result.Group = ""
		result.GroupScore = 0
	}

	// store preview for ui invoke later
	// because preview may contain some heavy data (E.g. image or large text),
	// we will store preview in cache and only send preview to ui when user select the result
	var maximumPreviewSize = 1024
	if !result.Preview.IsEmpty() && result.Preview.PreviewType != WoxPreviewTypeRemote && len(result.Preview.PreviewData) > maximumPreviewSize {
		resultCache.Preview = result.Preview
		result.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?id=%s", result.Id),
		}
	}

	if result.RefreshInterval > 0 && result.OnRefresh != nil {
		newInterval := int(math.Floor(float64(result.RefreshInterval)/100) * 100)
		if result.RefreshInterval != newInterval {
			logger.Info(ctx, fmt.Sprintf("[%s] result(%s) refresh interval %d is not divisible by 100, use %d instead", pluginInstance.Metadata.Name, result.Id, result.RefreshInterval, newInterval))
			result.RefreshInterval = newInterval
		}
		resultCache.Refresh = result.OnRefresh
	}

	ignoreAutoScore := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureIgnoreAutoScore)
	if !ignoreAutoScore {
		score := m.calculateResultScore(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle, query.RawQuery)
		if score > 0 {
			logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) add score: %d", pluginInstance.Metadata.Name, result.Title, score))
			result.Score += score
		}
	}
	// check if result is favorite result
	// favorite result will not be affected by ignoreAutoScore setting, so we add score here
	if setting.GetSettingManager().IsFavoriteResult(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle) {
		favScore := int64(100000)
		logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) is favorite result, add score: %d", pluginInstance.Metadata.Name, result.Title, favScore))
		result.Score += favScore
	}

	m.resultCache.Store(result.Id, resultCache)

	return result
}

func (m *Manager) formatFileListPreview(ctx context.Context, filePaths []string) string {
	totalFiles := len(filePaths)
	if totalFiles == 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "selection_no_files_selected")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_selected_files_count"), totalFiles))
	sb.WriteString("\n\n")

	maxDisplayFiles := 10
	for i, filePath := range filePaths {
		if i < maxDisplayFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", filePath))
		} else {
			remainingFiles := totalFiles - maxDisplayFiles
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_remaining_files_not_shown"), remainingFiles))
			break
		}
	}

	return sb.String()
}

func (m *Manager) calculateResultScore(ctx context.Context, pluginId, title, subTitle string, currentQuery string) int64 {
	var score int64 = 0

	resultHash := setting.NewResultHash(pluginId, title, subTitle)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	actionResults, ok := woxSetting.ActionedResults.Get().Load(resultHash)
	if !ok {
		return score
	}

	// actioned score are based on actioned counts, the more actioned, the more score
	// also, action timestamp will be considered, the more recent actioned, the more score weight. If action is in recent 7 days, it will be considered as recent actioned and add score weight
	// we will use fibonacci sequence to calculate score, the more recent actioned, the more score: 5, 8, 13, 21, 34, 55, 89
	// that means, actions in day one, we will add weight 89, day two, we will add weight 55, day three, we will add weight 34, and so on
	// E.g. if actioned 3 times in day one, 2 times in day two, 1 time in day three, the score will be: 89*3 + 55*2 + 34*1 = 450

	for _, actionResult := range actionResults {
		var weight int64 = 2

		hours := (util.GetSystemTimestamp() - actionResult.Timestamp) / 1000 / 60 / 60
		if hours < 24*7 {
			fibonacciIndex := int(math.Ceil(float64(hours) / 24))
			if fibonacciIndex > 7 {
				fibonacciIndex = 7
			}
			if fibonacciIndex < 1 {
				fibonacciIndex = 1
			}
			fibonacci := []int64{5, 8, 13, 21, 34, 55, 89}
			score += fibonacci[7-fibonacciIndex]
		}

		// If the current query is within the historical selected actions, it indicates a stronger connection and increases the score.
		if currentQuery != "" && actionResult.Query == currentQuery {
			score += 20
		}

		score += weight
	}

	return score
}

func (m *Manager) polishRefreshableResult(ctx context.Context, resultCache *QueryResultCache, result RefreshableResult) RefreshableResult {
	pluginInstance := resultCache.PluginInstance

	for actionIndex := range result.Actions {
		if result.Actions[actionIndex].Id == "" {
			result.Actions[actionIndex].Id = uuid.NewString()
		}
		if result.Actions[actionIndex].Icon.IsEmpty() {
			// set default action icon if not present
			result.Actions[actionIndex].Icon = DefaultActionIcon
		}
	}

	// set first action as default if no default action is set
	defaultActionCount := lo.CountBy(result.Actions, func(item QueryResultAction) bool {
		return item.IsDefault
	})
	if defaultActionCount == 0 && len(result.Actions) > 0 {
		result.Actions[0].IsDefault = true
		result.Actions[0].Hotkey = "Enter"
	}

	//move default action to first one of the actions
	sort.Slice(result.Actions, func(i, j int) bool {
		return result.Actions[i].IsDefault
	})

	// convert icon
	result.Icon = common.ConvertIcon(ctx, result.Icon, pluginInstance.PluginDirectory)
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeImage {
			result.Tails[i].Image = common.ConvertIcon(ctx, result.Tails[i].Image, pluginInstance.PluginDirectory)
		}
	}

	// translate title
	result.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	// translate subtitle
	result.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	// translate tail text
	for i := range result.Tails {
		if result.Tails[i].Type == QueryResultTailTypeText {
			result.Tails[i].Text = m.translatePlugin(ctx, pluginInstance, result.Tails[i].Text)
		}
	}
	// translate preview properties
	var previewProperties = make(map[string]string)
	for key, value := range result.Preview.PreviewProperties {
		translatedKey := m.translatePlugin(ctx, pluginInstance, key)
		previewProperties[translatedKey] = value
	}
	result.Preview.PreviewProperties = previewProperties
	// translate action names
	for actionIndex := range result.Actions {
		result.Actions[actionIndex].Name = m.translatePlugin(ctx, pluginInstance, result.Actions[actionIndex].Name)
	}

	// update result cache
	resultCache.ResultTitle = result.Title
	resultCache.ResultSubTitle = result.SubTitle
	resultCache.ContextData = result.ContextData
	resultCache.Actions = util.NewHashMap[string, func(ctx context.Context, actionContext ActionContext)]()
	for _, newAction := range result.Actions {
		if newAction.Action != nil {
			resultCache.Actions.Store(newAction.Id, newAction.Action)
		}
	}

	// convert non-remote preview to remote preview
	// because preview may contain some heavy data (E.g. image or large text),
	// we will store preview in cache and only send preview to ui when user select the result
	if !result.Preview.IsEmpty() && result.Preview.PreviewType != WoxPreviewTypeRemote {
		resultCache.Preview = result.Preview
		result.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?id=%s", resultCache.ResultId),
		}
	}

	return result
}

func (m *Manager) Query(ctx context.Context, query Query) (results chan []QueryResultUI, done chan bool) {
	results = make(chan []QueryResultUI, 10)
	done = make(chan bool)

	// clear old result cache
	m.resultCache.Clear()

	counter := &atomic.Int32{}
	counter.Store(int32(len(m.instances)))

	for _, pluginInstance := range m.instances {
		if !m.canOperateQuery(ctx, pluginInstance, query) {
			counter.Add(-1)
			if counter.Load() == 0 {
				done <- true
			}
			continue
		}

		if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDebounce) {
			debounceParams, err := pluginInstance.Metadata.GetFeatureParamsForDebounce()
			if err == nil {
				logger.Debug(ctx, fmt.Sprintf("[%s] debounce query, will execute in %d ms", pluginInstance.Metadata.Name, debounceParams.IntervalMs))
				if v, ok := m.debounceQueryTimer.Load(pluginInstance.Metadata.Id); ok {
					if v.timer.Stop() {
						v.onStop()
					}
				}

				timer := time.AfterFunc(time.Duration(debounceParams.IntervalMs)*time.Millisecond, func() {
					m.queryParallel(ctx, pluginInstance, query, results, done, counter)
				})
				onStop := func() {
					logger.Debug(ctx, fmt.Sprintf("[%s] previous debounced query cancelled", pluginInstance.Metadata.Name))
					counter.Add(-1)
					if counter.Load() == 0 {
						done <- true
					}
				}
				m.debounceQueryTimer.Store(pluginInstance.Metadata.Id, &debounceTimer{
					timer:  timer,
					onStop: onStop,
				})
				continue
			} else {
				logger.Error(ctx, fmt.Sprintf("[%s] %s, query directlly", pluginInstance.Metadata.Name, err))
			}
		}

		m.queryParallel(ctx, pluginInstance, query, results, done, counter)
	}

	return
}

func (m *Manager) QuerySilent(ctx context.Context, query Query) bool {
	var startTimestamp = util.GetSystemTimestamp()
	var results []QueryResultUI
	resultChan, doneChan := m.Query(ctx, query)
	for {
		select {
		case r := <-resultChan:
			results = append(results, r...)
		case <-doneChan:
			logger.Info(ctx, fmt.Sprintf("silent query done, total results: %d, cost %d ms", len(results), util.GetSystemTimestamp()-startTimestamp))

			// execute default action if only one result
			if len(results) == 1 {
				result := results[0]
				for _, action := range result.Actions {
					if action.IsDefault {
						m.ExecuteAction(ctx, result.Id, action.Id)
						return true
					}
				}
			} else {
				notifier.Notify(fmt.Sprintf("Silent query failed, there shouldbe only one result, but got %d", len(results)))
			}

			return false
		case <-time.After(time.Minute):
			logger.Error(ctx, "silent query timeout")
			return false
		}
	}
}

func (m *Manager) QueryFallback(ctx context.Context, query Query, queryPlugin *Instance) (results []QueryResultUI) {
	var queryResults []QueryResult
	if query.IsGlobalQuery() {
		for _, pluginInstance := range m.instances {
			if v, ok := pluginInstance.Plugin.(FallbackSearcher); ok {
				fallbackResults := v.QueryFallback(ctx, query)
				for _, fallbackResult := range fallbackResults {
					polishedFallbackResult := m.PolishResult(ctx, pluginInstance, query, fallbackResult)
					queryResults = append(queryResults, polishedFallbackResult)
				}
				continue
			}
		}
	} else {
		if query.Command != "" {
			return results
		}

		// search query commands
		commands := lo.Filter(queryPlugin.GetQueryCommands(), func(item MetadataCommand, index int) bool {
			return strings.Contains(item.Command, query.Search) || query.Search == ""
		})
		queryResults = lo.Map(commands, func(item MetadataCommand, index int) QueryResult {
			return QueryResult{
				Title:    item.Command,
				SubTitle: item.Description,
				Icon:     common.ParseWoxImageOrDefault(queryPlugin.Metadata.Icon, common.NewWoxImageEmoji("🔍")),
				Actions: []QueryResultAction{
					{
						Name:                   "Execute",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext ActionContext) {
							m.ui.ChangeQuery(ctx, common.PlainQuery{
								QueryType: QueryTypeInput,
								QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, item.Command),
							})
						},
					},
				},
			}
		})
		for i := range queryResults {
			queryResults[i] = m.PolishResult(ctx, queryPlugin, query, queryResults[i])
		}
	}

	queryResultsUI := lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
		return item.ToUI()
	})
	results = append(results, queryResultsUI...)
	return results
}

func (m *Manager) queryParallel(ctx context.Context, pluginInstance *Instance, query Query, results chan []QueryResultUI, done chan bool, counter *atomic.Int32) {
	util.Go(ctx, fmt.Sprintf("[%s] parallel query", pluginInstance.Metadata.Name), func() {
		queryResults := m.queryForPlugin(ctx, pluginInstance, query)
		results <- lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
			return item.ToUI()
		})
		counter.Add(-1)
		if counter.Load() == 0 {
			done <- true
		}
	}, func() {
		counter.Add(-1)
		if counter.Load() == 0 {
			done <- true
		}
	})
}

func (m *Manager) translatePlugin(ctx context.Context, pluginInstance *Instance, key string) string {
	if !strings.HasPrefix(key, "i18n:") {
		return key
	}

	if pluginInstance.IsSystemPlugin {
		return i18n.GetI18nManager().TranslateWox(ctx, key)
	} else {
		return i18n.GetI18nManager().TranslatePlugin(ctx, key, pluginInstance.PluginDirectory)
	}
}

func (m *Manager) GetUI() common.UI {
	return m.ui
}

func (m *Manager) NewQuery(ctx context.Context, plainQuery common.PlainQuery) (Query, *Instance, error) {
	if plainQuery.QueryType == QueryTypeInput {
		newQuery := plainQuery.QueryText
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		if len(woxSetting.QueryShortcuts.Get()) > 0 {
			originQuery := plainQuery.QueryText
			expandedQuery := m.expandQueryShortcut(ctx, plainQuery.QueryText, woxSetting.QueryShortcuts.Get())
			if originQuery != expandedQuery {
				logger.Info(ctx, fmt.Sprintf("expand query shortcut: %s -> %s", originQuery, expandedQuery))
				newQuery = expandedQuery
			}
		}
		query, instance := newQueryInputWithPlugins(newQuery, GetPluginManager().GetPluginInstances())
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveWindowIcon = m.GetUI().GetActiveWindowIcon()
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
		return query, instance, nil
	}

	if plainQuery.QueryType == QueryTypeSelection {
		query := Query{
			Type:      QueryTypeSelection,
			RawQuery:  plainQuery.QueryText,
			Search:    plainQuery.QueryText,
			Selection: plainQuery.QuerySelection,
		}
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveWindowIcon = m.GetUI().GetActiveWindowIcon()
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
		return query, nil, nil
	}

	return Query{}, nil, errors.New("invalid query type")
}

func (m *Manager) getActiveBrowserUrl(ctx context.Context) string {
	activeWindowName := m.GetUI().GetActiveWindowName()
	isGoogleChrome := strings.ToLower(activeWindowName) == "google chrome"
	if !isGoogleChrome {
		return ""
	}

	return m.activeBrowserUrl
}

func (m *Manager) expandQueryShortcut(ctx context.Context, query string, queryShorts []setting.QueryShortcut) (newQuery string) {
	newQuery = query

	//sort query shorts by shortcut length, we will expand the longest shortcut first
	slices.SortFunc(queryShorts, func(i, j setting.QueryShortcut) int {
		return len(j.Shortcut) - len(i.Shortcut)
	})

	for _, shortcut := range queryShorts {
		if strings.HasPrefix(query, shortcut.Shortcut) {
			if !shortcut.HasPlaceholder() {
				newQuery = strings.Replace(query, shortcut.Shortcut, shortcut.Query, 1)
				break
			} else {
				queryWithoutShortcut := strings.Replace(query, shortcut.Shortcut, "", 1)
				queryWithoutShortcut = strings.TrimLeft(queryWithoutShortcut, " ")
				parameters := strings.Split(queryWithoutShortcut, " ")
				placeholderCount := shortcut.PlaceholderCount()
				var paramsCount = 0

				var params []any
				var nonPrams string
				for _, param := range parameters {
					if paramsCount < placeholderCount {
						paramsCount++
						params = append(params, param)
					} else {
						nonPrams += " " + param
					}
				}
				newQuery = stringFormatter.Format(shortcut.Query, params...) + nonPrams
				break
			}
		}
	}

	return newQuery
}

func (m *Manager) ExecuteAction(ctx context.Context, resultId string, actionId string) error {
	resultCache, found := m.resultCache.Load(resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (execute action): %s", resultId)
	}
	action, exist := resultCache.Actions.Load(actionId)
	if !exist {
		return fmt.Errorf("action not found for result id: %s, action id: %s", resultId, actionId)
	}

	action(ctx, ActionContext{
		ContextData: resultCache.ContextData,
	})

	util.Go(ctx, fmt.Sprintf("[%s] post execute action", resultCache.PluginInstance.Metadata.Name), func() {
		m.postExecuteAction(ctx, resultCache)
	})

	return nil
}

func (m *Manager) postExecuteAction(ctx context.Context, resultCache *QueryResultCache) {
	// Add actioned result for statistics
	setting.GetSettingManager().AddActionedResult(ctx, resultCache.PluginInstance.Metadata.Id, resultCache.ResultTitle, resultCache.ResultSubTitle, resultCache.Query.RawQuery)

	// Add to MRU if plugin supports it
	if resultCache.PluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
		mruItem := setting.MRUItem{
			PluginID:    resultCache.PluginInstance.Metadata.Id,
			Title:       resultCache.ResultTitle,
			SubTitle:    resultCache.ResultSubTitle,
			Icon:        resultCache.Icon,
			ContextData: resultCache.ContextData,
		}
		if err := setting.GetSettingManager().AddMRUItem(ctx, mruItem); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to add MRU item: %s", err.Error()))
		}
	}

	// Add to query history only if query is not empty (skip empty queries like MRU)
	if resultCache.Query.RawQuery != "" {
		plainQuery := common.PlainQuery{
			QueryType: resultCache.Query.Type,
			QueryText: resultCache.Query.RawQuery,
		}
		setting.GetSettingManager().AddQueryHistory(ctx, plainQuery)
	}
}

func (m *Manager) ExecuteRefresh(ctx context.Context, refreshableResultWithId RefreshableResultWithResultId) (RefreshableResultWithResultId, error) {
	var refreshableResult RefreshableResult
	copyErr := copier.Copy(&refreshableResult, &refreshableResultWithId)
	if copyErr != nil {
		return RefreshableResultWithResultId{}, fmt.Errorf("failed to copy refreshable result: %w", copyErr)
	}

	// maybe user has changed the query, which may flush the result cache
	resultCache, found := m.resultCache.Load(refreshableResultWithId.ResultId)
	if !found {
		return refreshableResultWithId, fmt.Errorf("result cache not found for result id (execute refresh): %s", refreshableResultWithId.ResultId)
	}

	//restore actions in cache
	refreshableResult.Actions = []QueryResultAction{}
	for _, action := range refreshableResultWithId.Actions {
		// get actual action from cache
		actionFunc, exist := resultCache.Actions.Load(action.Id)
		if !exist {
			continue
		}
		refreshableResult.Actions = append(refreshableResult.Actions, QueryResultAction{
			Id:                     action.Id,
			Name:                   action.Name,
			Icon:                   action.Icon,
			IsDefault:              action.IsDefault,
			PreventHideAfterAction: action.PreventHideAfterAction,
			Hotkey:                 action.Hotkey,
			Action:                 actionFunc,
			IsSystemAction:         action.IsSystemAction,
		})
	}

	newResult := resultCache.Refresh(ctx, refreshableResult)

	// add default actions if there is no system action
	if lo.CountBy(newResult.Actions, func(action QueryResultAction) bool {
		return action.IsSystemAction
	}) == 0 {
		defaultActions := m.getDefaultActions(ctx, resultCache.PluginInstance, resultCache.Query, newResult.Title, newResult.SubTitle)
		newResult.Actions = append(newResult.Actions, defaultActions...)
	}

	newResult = m.polishRefreshableResult(ctx, resultCache, newResult)
	return RefreshableResultWithResultId{
		ResultId:        refreshableResultWithId.ResultId,
		Title:           newResult.Title,
		SubTitle:        newResult.SubTitle,
		Icon:            newResult.Icon,
		Tails:           newResult.Tails,
		Preview:         newResult.Preview,
		ContextData:     newResult.ContextData,
		RefreshInterval: newResult.RefreshInterval,
		Actions: lo.Map(newResult.Actions, func(action QueryResultAction, index int) QueryResultActionUI {
			return QueryResultActionUI{
				Id:                     action.Id,
				Name:                   action.Name,
				Icon:                   action.Icon,
				IsDefault:              action.IsDefault,
				PreventHideAfterAction: action.PreventHideAfterAction,
				Hotkey:                 action.Hotkey,
				IsSystemAction:         action.IsSystemAction,
			}
		}),
	}, nil
}

func (m *Manager) GetResultPreview(ctx context.Context, resultId string) (WoxPreview, error) {
	resultCache, found := m.resultCache.Load(resultId)
	if !found {
		return WoxPreview{}, fmt.Errorf("result cache not found for result id (get preview): %s", resultId)
	}

	preview := m.polishPreview(ctx, resultCache.Preview)

	// if preview text is too long, ellipsis it, otherwise UI maybe freeze when render
	if preview.PreviewType == WoxPreviewTypeText {
		preview.PreviewData = util.EllipsisMiddle(preview.PreviewData, 2000)
		// translate preview data if preview type is text
		preview.PreviewData = m.translatePlugin(ctx, resultCache.PluginInstance, preview.PreviewData)
	}

	return preview, nil
}

func (m *Manager) polishPreview(ctx context.Context, preview WoxPreview) WoxPreview {
	if preview.PreviewType == WoxPreviewTypeImage {
		woxImage, err := common.ParseWoxImage(preview.PreviewData)
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse wox image for preview: %s", err.Error()))
			return preview
		}

		if woxImage.ImageType == common.WoxImageTypeAbsolutePath {
			newWoxImage := common.ConvertLocalImageToUrl(ctx, woxImage)
			return WoxPreview{
				PreviewType:       WoxPreviewTypeImage,
				PreviewData:       newWoxImage.String(),
				PreviewProperties: preview.PreviewProperties,
			}
		}
	}

	return preview
}

func (m *Manager) ReplaceQueryVariable(ctx context.Context, query string) string {
	if strings.Contains(query, QueryVariableSelectedText) {
		selected, selectedErr := selection.GetSelected(ctx)
		if selectedErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get selected text: %s", selectedErr.Error()))
		} else {
			if selected.Type == selection.SelectionTypeText {
				query = strings.ReplaceAll(query, QueryVariableSelectedText, selected.Text)
			} else {
				logger.Error(ctx, fmt.Sprintf("selected data is not text, type: %s", selected.Type))
			}
		}
	}

	if strings.Contains(query, QueryVariableActiveBrowserUrl) {
		activeBrowserUrl := m.activeBrowserUrl
		query = strings.ReplaceAll(query, QueryVariableActiveBrowserUrl, activeBrowserUrl)
	}

	return query
}

func (m *Manager) IsHostStarted(ctx context.Context, runtime Runtime) bool {
	if runtime == PLUGIN_RUNTIME_GO {
		return true
	}

	for _, host := range AllHosts {
		if host.GetRuntime(ctx) == runtime {
			return host.IsStarted(ctx)
		}
	}

	return false
}

func (m *Manager) IsTriggerKeywordAIChat(ctx context.Context, triggerKeyword string) bool {
	aiChatPluginInstance := m.GetAIChatPluginInstance(ctx)
	if aiChatPluginInstance == nil {
		return false
	}

	return lo.Contains(aiChatPluginInstance.GetTriggerKeywords(), triggerKeyword)
}

func (m *Manager) GetAIChatPluginInstance(ctx context.Context) *Instance {
	aiChatPlugin := m.GetPluginInstances()
	aiChatPluginInstance, exist := lo.Find(aiChatPlugin, func(item *Instance) bool {
		return item.Metadata.Id == "a9cfd85a-6e53-415c-9d44-68777aa6323d"
	})
	if exist {
		return aiChatPluginInstance
	}

	return nil
}

func (m *Manager) GetAIChatPluginChater(ctx context.Context) common.AIChater {
	aiChatPluginInstance := m.GetAIChatPluginInstance(ctx)
	if aiChatPluginInstance == nil {
		return nil
	}

	chater, ok := aiChatPluginInstance.Plugin.(common.AIChater)
	if ok {
		return chater
	}

	return nil
}

func (m *Manager) GetAIProvider(ctx context.Context, provider common.ProviderName) (ai.Provider, error) {
	if v, exist := m.aiProviders.Load(provider); exist {
		return v, nil
	}

	//check if provider has setting
	aiProviderSettings := setting.GetSettingManager().GetWoxSetting(ctx).AIProviders.Get()
	providerSetting, providerSettingExist := lo.Find(aiProviderSettings, func(item setting.AIProvider) bool {
		return item.Name == provider
	})
	if !providerSettingExist {
		return nil, fmt.Errorf("ai provider setting not found: %s", provider)
	}

	newProvider, newProviderErr := ai.NewProvider(ctx, providerSetting)
	if newProviderErr != nil {
		return nil, newProviderErr
	}
	m.aiProviders.Store(provider, newProvider)
	return newProvider, nil
}

func (m *Manager) ExecutePluginDeeplink(ctx context.Context, pluginId string, arguments map[string]string) {
	pluginInstance, exist := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == pluginId
	})
	if !exist {
		logger.Error(ctx, fmt.Sprintf("plugin not found: %s", pluginId))
		return
	}

	for _, callback := range pluginInstance.DeepLinkCallbacks {
		util.Go(ctx, fmt.Sprintf("[%s] execute deeplink callback", pluginInstance.Metadata.Name), func() {
			callback(arguments)
		})
	}
}

func (m *Manager) QueryMRU(ctx context.Context) []QueryResultUI {
	mruItems, err := setting.GetSettingManager().GetMRUItems(ctx, 10)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to get MRU items: %s", err.Error()))
		return []QueryResultUI{}
	}

	// Deduplicate restored MRU results by (pluginId, title, subTitle, contextData)
	seen := make(map[string]bool)

	var results []QueryResultUI
	for _, item := range mruItems {
		util.GetLogger().Debug(ctx, fmt.Sprintf("start to restore mru item: %s", item.Title))
		pluginInstance := m.getPluginInstance(item.PluginID)
		if pluginInstance == nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("plugin not found, skip restore mru item: %s", item.Title))
			continue
		}
		if !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
			util.GetLogger().Debug(ctx, fmt.Sprintf("plugin does not support mru, skip restore mru item: %s", item.Title))
			continue
		}

		if restored := m.restoreFromMRU(ctx, pluginInstance, item); restored != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("mru item restored: %s", item.Title))

			// Build a stable dedupe key using restored values, which are language-independent for Go plugins
			key := fmt.Sprintf("%s|%s|%s|%s", item.PluginID, restored.Title, restored.SubTitle, restored.ContextData)
			if seen[key] {
				util.GetLogger().Debug(ctx, fmt.Sprintf("duplicate mru item, skip restore mru item: %s", item.Title))
				continue
			}
			seen[key] = true

			// Add "Remove from MRU" action to each MRU result
			removeMRUAction := QueryResultAction{
				Id:     uuid.NewString(),
				Name:   i18n.GetI18nManager().TranslateWox(ctx, "mru_remove_action"),
				Icon:   common.NewWoxImageEmoji("🗑️"),
				Hotkey: "ctrl+d",
				Action: func(ctx context.Context, actionContext ActionContext) {
					err := setting.GetSettingManager().RemoveMRUItem(ctx, item.PluginID, item.Title, item.SubTitle)
					if err != nil {
						util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove MRU item: %s", err.Error()))
					} else {
						util.GetLogger().Info(ctx, fmt.Sprintf("removed MRU item: %s - %s", item.Title, item.SubTitle))
					}
				},
			}

			// Add the remove action to the result
			restored.Actions = append(restored.Actions, removeMRUAction)

			polishedResult := m.PolishResult(ctx, pluginInstance, Query{}, *restored)
			results = append(results, polishedResult.ToUI())
		}
	}

	return results
}

// getPluginInstance finds a plugin instance by ID
func (m *Manager) getPluginInstance(pluginID string) *Instance {
	pluginInstance, found := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == pluginID
	})
	if found {
		return pluginInstance
	}
	return nil
}

// restoreFromMRU attempts to restore a QueryResult from MRU data
func (m *Manager) restoreFromMRU(ctx context.Context, pluginInstance *Instance, item setting.MRUItem) *QueryResult {
	// For Go plugins, call MRU restore callbacks directly
	if len(pluginInstance.MRURestoreCallbacks) > 0 {
		mruData := MRUData{
			PluginID:    item.PluginID,
			Title:       item.Title,
			SubTitle:    item.SubTitle,
			Icon:        item.Icon,
			ContextData: item.ContextData,
			LastUsed:    item.LastUsed,
			UseCount:    item.UseCount,
		}

		// Call the first (and typically only) MRU restore callback
		if restored, err := pluginInstance.MRURestoreCallbacks[0](mruData); err == nil {
			return restored
		} else {
			util.GetLogger().Debug(ctx, fmt.Sprintf("MRU restore failed for plugin %s: %s", pluginInstance.Metadata.Name, err.Error()))
		}
	}

	// For external plugins (Python/Node.js), MRU support will be implemented later
	// Currently only Go plugins support MRU functionality
	if pluginInstance.Host != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("External plugin MRU restore not yet implemented for plugin %s", pluginInstance.Metadata.Name))
	}

	return nil
}

// normalizeHotkeyForPlatform converts hotkey strings to platform-specific format
// This function provides better cross-platform hotkey support by handling various
// modifier key aliases and converting them to the appropriate platform format
func normalizeHotkeyForPlatform(hotkey string) string {
	if hotkey == "" {
		return hotkey
	}

	// Convert to lowercase for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(hotkey))

	// Define platform-specific modifier mappings
	var modifierMappings map[string]string

	if util.IsMacOS() {
		// On macOS: convert Windows/Linux style to macOS style
		modifierMappings = map[string]string{
			"win":     "cmd",
			"windows": "cmd",
			"ctrl":    "control", // Keep ctrl as control on macOS
			"control": "control", // Keep explicit control as-is
			"alt":     "option",
		}
	} else {
		// On Windows/Linux: convert macOS style to Windows/Linux style
		modifierMappings = map[string]string{
			"cmd":     "win", // Map cmd to win key on Windows/Linux
			"command": "win",
			"option":  "alt",
			"control": "ctrl", // Keep control as ctrl
			"ctrl":    "ctrl", // Keep ctrl as-is
		}
	}

	// Split hotkey into parts and process each modifier
	parts := strings.Split(normalized, "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if replacement, exists := modifierMappings[part]; exists {
			parts[i] = replacement
		} else {
			parts[i] = part
		}
	}

	return strings.Join(parts, "+")
}
