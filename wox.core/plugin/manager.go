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
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"wox/ai"
	"wox/analytics"
	"wox/common"
	"wox/i18n"
	"wox/setting"

	"wox/util"
	"wox/util/notifier"
	"wox/util/selection"
	"wox/util/window"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/wissance/stringFormatter"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

const (
	// ContextData key/value for favorite tail
	favoriteTailContextDataKey   = "system:favorite"
	favoriteTailContextDataValue = "true"
	previewDataMaxSize           = 1024
)

func serializeContextData(contextData map[string]string) string {
	if len(contextData) == 0 {
		return ""
	}
	data, err := json.Marshal(contextData)
	if err != nil {
		return ""
	}
	return string(data)
}

type debounceTimer struct {
	timer  *time.Timer
	onStop func()
}

type QueryResultSet struct {
	Query   Query
	Results *util.HashMap[string, *QueryResultCache]
}

func newQueryResultSet(query Query) *QueryResultSet {
	set := &QueryResultSet{
		Query:   query,
		Results: util.NewHashMap[string, *QueryResultCache](),
	}
	return set
}

type Manager struct {
	instances       []*Instance
	systemPluginsWg sync.WaitGroup // waits for all system plugins to finish loading
	ui              common.UI

	// ui session based query result cache. [SessionId] => QueryResultSet
	// one ui session can only have one active query at a time
	sessionQueryResultCache *util.HashMap[string, *QueryResultSet]

	debounceQueryTimer *util.HashMap[string, *debounceTimer]
	aiProviders        *util.HashMap[string, ai.Provider]

	activeBrowserUrl string //active browser url before wox is activated

	// Script plugin monitoring
	scriptPluginWatcher *fsnotify.Watcher
	scriptReloadTimers  *util.HashMap[string, *time.Timer]
}

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			sessionQueryResultCache: util.NewHashMap[string, *QueryResultSet](),
			debounceQueryTimer:      util.NewHashMap[string, *debounceTimer](),
			aiProviders:             util.NewHashMap[string, ai.Provider](),
			scriptReloadTimers:      util.NewHashMap[string, *time.Timer](),
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

	var metaDataList []Metadata
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
		existMetadata, exist := lo.Find(metaDataList, func(item Metadata) bool {
			return item.Id == metadata.Id
		})
		if exist {
			existVersion, existVersionErr := semver.NewVersion(existMetadata.Version)
			currentVersion, currentVersionErr := semver.NewVersion(metadata.Version)
			if existVersionErr == nil && currentVersionErr == nil {
				if existVersion.GreaterThan(currentVersion) || existVersion.Equal(currentVersion) {
					logger.Info(ctx, fmt.Sprintf("skip parse %s(%s) metadata, because it's already parsed(%s)", metadata.GetName(ctx), metadata.Version, existMetadata.Version))
					continue
				} else {
					// remove older version
					logger.Info(ctx, fmt.Sprintf("remove older metadata version %s(%s)", existMetadata.GetName(ctx), existMetadata.Version))
					var newMetaDataList []Metadata
					for _, item := range metaDataList {
						if item.Id != existMetadata.Id {
							newMetaDataList = append(newMetaDataList, item)
						}
					}
					metaDataList = newMetaDataList
				}
			}
		}
		metaDataList = append(metaDataList, metadata)
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
				if !strings.EqualFold(metadata.Runtime, string(host.GetRuntime(newCtx))) {
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
func (m *Manager) loadScriptPlugins(ctx context.Context) ([]Metadata, error) {
	logger.Debug(ctx, "start loading script plugin metadata")

	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()
	scriptFiles, readErr := os.ReadDir(userScriptPluginDirectory)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read user script plugin directory: %w", readErr)
	}

	var metaDataList []Metadata
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

		metaDataList = append(metaDataList, metadata)
	}

	logger.Debug(ctx, fmt.Sprintf("found %d script plugins", len(metaDataList)))
	return metaDataList, nil
}

func (m *Manager) ReloadPlugin(ctx context.Context, metadata Metadata) error {
	logger.Info(ctx, fmt.Sprintf("start reloading dev plugin: %s", metadata.GetName(ctx)))

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}

	pluginInstance, pluginInstanceExist := lo.Find(m.instances, func(item *Instance) bool {
		return item.Metadata.Id == metadata.Id
	})
	if pluginInstanceExist {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is loaded, unload first", metadata.GetName(ctx)))
		m.UnloadPlugin(ctx, pluginInstance)
	} else {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is not loaded, skip unload", metadata.GetName(ctx)))
	}

	loadErr := m.loadHostPlugin(ctx, pluginHost, metadata)
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) loadHostPlugin(ctx context.Context, host Host, metadata Metadata) error {
	loadStartTimestamp := util.GetSystemTimestamp()
	plugin, loadErr := host.LoadPlugin(ctx, metadata, metadata.Directory)
	if loadErr != nil {
		logger.Error(ctx, fmt.Errorf("[%s HOST] failed to load plugin: %w", host.GetRuntime(ctx), loadErr).Error())
		return loadErr
	}
	loadFinishTimestamp := util.GetSystemTimestamp()

	instance := &Instance{
		Metadata:              metadata,
		PluginDirectory:       metadata.Directory,
		Plugin:                plugin,
		Host:                  host,
		LoadStartTimestamp:    loadStartTimestamp,
		LoadFinishedTimestamp: loadFinishTimestamp,
		IsDevPlugin:           metadata.IsDev,
		DevPluginDirectory:    metadata.DevPluginDirectory,
	}
	instance.API = NewAPI(instance)
	pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
	if settingErr != nil {
		instance.API.Log(ctx, LogLevelError, fmt.Errorf("[SYS] failed to load plugin[%s] setting: %w", metadata.GetName(ctx), settingErr).Error())
		return settingErr
	}
	instance.Setting = pluginSetting

	m.instances = append(m.instances, instance)

	if pluginSetting.Disabled.Get() {
		logger.Info(ctx, fmt.Errorf("[%s HOST] plugin is disabled by user, skip init: %s", host.GetRuntime(ctx), metadata.GetName(ctx)).Error())
		instance.API.Log(ctx, LogLevelWarning, fmt.Sprintf("[SYS] plugin is disabled by user, skip init: %s", metadata.GetName(ctx)))
		return nil
	}

	util.Go(ctx, fmt.Sprintf("[%s] init plugin", metadata.GetName(ctx)), func() {
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
		return strings.EqualFold(string(item.GetRuntime(ctx)), metadata.Runtime)
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

	loadErr := m.loadHostPlugin(ctx, pluginHost, metadata)
	if loadErr != nil {
		return loadErr
	}

	return nil
}

func (m *Manager) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
	for _, callback := range pluginInstance.UnloadCallbacks {
		callback(ctx)
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

	// Add all plugins to wait group before starting goroutines
	m.systemPluginsWg.Add(len(AllSystemPlugin))

	for _, p := range AllSystemPlugin {
		plugin := p
		metadata := plugin.GetMetadata()
		pluginName := metadata.GetName(ctx)

		util.Go(ctx, fmt.Sprintf("load system plugin <%s>", pluginName), func() {
			defer m.systemPluginsWg.Done()

			metadata := plugin.GetMetadata()
			metadata.LoadSystemI18nFromDirectory(ctx)
			instance := &Instance{
				Metadata:              metadata,
				Plugin:                plugin,
				Host:                  nil,
				IsSystemPlugin:        true,
				PluginDirectory:       metadata.Directory,
				LoadStartTimestamp:    util.GetSystemTimestamp(),
				LoadFinishedTimestamp: util.GetSystemTimestamp(),
			}
			instance.API = NewAPI(instance)

			startTimestamp := util.GetSystemTimestamp()
			pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
			if settingErr != nil {
				logger.Error(ctx, fmt.Sprintf("failed to load system plugin[%s] setting, use default plugin setting. err: %s", metadata.GetName(ctx), settingErr.Error()))
				return
			}

			instance.Setting = pluginSetting
			if util.GetSystemTimestamp()-startTimestamp > 100 {
				logger.Warn(ctx, fmt.Sprintf("load system plugin[%s] setting too slow, cost %d ms", metadata.GetName(ctx), util.GetSystemTimestamp()-startTimestamp))
			}

			// Init plugin BEFORE adding to instances list
			// This ensures the plugin is fully initialized before it can be queried
			m.initPlugin(util.NewTraceContext(), instance)

			m.instances = append(m.instances, instance)
		})
	}

	logger.Debug(ctx, fmt.Sprintf("finish loading system plugins, cost %d ms", util.GetSystemTimestamp()-start))
}

func (m *Manager) initPlugin(ctx context.Context, instance *Instance) {
	logger.Info(ctx, fmt.Sprintf("start init plugin: %s", instance.Metadata.GetName(ctx)))
	instance.InitStartTimestamp = util.GetSystemTimestamp()
	instance.Plugin.Init(ctx, InitParams{
		API:             instance.API,
		PluginDirectory: instance.PluginDirectory,
	})
	instance.InitFinishedTimestamp = util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("init plugin %s finished, cost %d ms", instance.Metadata.GetName(ctx), instance.InitFinishedTimestamp-instance.InitStartTimestamp))
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
	if !IsAllSupportedOS(metadata.SupportedOS) {
		return Metadata{}, fmt.Errorf("unsupported os in plugin.json file (%s), os=%s", pluginDirectory, metadata.SupportedOS)
	}

	metadata.Directory = pluginDirectory
	metadata.LoadPluginI18nFromDirectory(ctx)

	return metadata, nil
}

// ParseScriptMetadata parses metadata from script plugin file comments
// Supports two formats:
// 1. JSON block format (preferred): # { ... } with complete plugin.json structure
// 2. Legacy @wox.xxx format: individual @wox.id, @wox.name, etc. annotations
func (m *Manager) ParseScriptMetadata(ctx context.Context, scriptPath string) (Metadata, error) {
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		return Metadata{}, fmt.Errorf("failed to read script file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Parse JSON block format
	var jsonBuilder strings.Builder
	inJsonBlock := false
	jsonStartLine := -1
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Stop parsing when we reach non-comment lines (except shebang)
		if !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "#!/") {
			if trimmed != "" {
				break
			}
			continue
		}

		// Remove comment markers
		cleaned := strings.TrimPrefix(trimmed, "#")
		cleaned = strings.TrimPrefix(cleaned, "//")
		cleaned = strings.TrimSpace(cleaned)

		// Check for JSON block start
		if !inJsonBlock && cleaned == "{" {
			inJsonBlock = true
			jsonStartLine = i
			braceDepth = 1
			jsonBuilder.WriteString(cleaned)
			continue
		}

		// Collect JSON content
		if inJsonBlock {
			jsonBuilder.WriteString("\n")
			jsonBuilder.WriteString(cleaned)

			// Track brace depth
			for _, ch := range cleaned {
				if ch == '{' {
					braceDepth++
				} else if ch == '}' {
					braceDepth--
				}
			}

			// Check for JSON block end (when braces are balanced)
			if braceDepth == 0 {
				// Try to parse the collected JSON
				jsonStr := jsonBuilder.String()
				var metadata Metadata
				unmarshalErr := json.Unmarshal([]byte(jsonStr), &metadata)
				if unmarshalErr != nil {
					return Metadata{}, fmt.Errorf("failed to parse JSON metadata block (starting at line %d): %w", jsonStartLine+1, unmarshalErr)
				}

				// Set script-specific fields
				metadata.Runtime = string(PLUGIN_RUNTIME_SCRIPT)
				metadata.Entry = filepath.Base(scriptPath)
				metadata.Directory = filepath.Dir(scriptPath)
				metadata.LoadPluginI18nFromDirectory(ctx)

				// Validate and set defaults
				return m.validateAndSetScriptMetadataDefaults(metadata)
			}
		}
	}

	// No JSON block found
	return Metadata{}, fmt.Errorf("no JSON metadata block found in script file. Script plugins must define metadata as a JSON object in comments")
}

// validateAndSetScriptMetadataDefaults validates required fields and sets default values
func (m *Manager) validateAndSetScriptMetadataDefaults(metadata Metadata) (Metadata, error) {
	// Validate required fields
	if metadata.Id == "" {
		return Metadata{}, fmt.Errorf("missing required field: Id")
	}
	if metadata.GetName(context.Background()) == "" {
		return Metadata{}, fmt.Errorf("missing required field: Name")
	}
	if len(metadata.TriggerKeywords) == 0 {
		return Metadata{}, fmt.Errorf("missing required field: TriggerKeywords")
	}

	// Set default values
	if metadata.Author == "" {
		metadata.Author = "Unknown"
	}
	if metadata.GetDescription(context.Background()) == "" {
		metadata.Description = "A script plugin"
	}
	if metadata.Icon == "" {
		metadata.Icon = "emoji:📝"
	}
	if metadata.MinWoxVersion == "" {
		metadata.MinWoxVersion = "2.0.0"
	}
	if metadata.Version == "" {
		metadata.Version = "1.0.0"
	}

	// Set supported OS to all platforms by default
	if len(metadata.SupportedOS) == 0 {
		metadata.SupportedOS = []string{"Windows", "Linux", "Macos"}
	}

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
		logger.Info(ctx, fmt.Sprintf("Unloading existing script plugin instance: %s", metadata.GetName(ctx)))
		m.UnloadPlugin(ctx, existingInstance)
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
	err = m.loadHostPlugin(ctx, scriptHost, metadata)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to reload script plugin: %s", err.Error()))
		return
	}

	logger.Info(ctx, fmt.Sprintf("Successfully reloaded script plugin: %s", metadata.GetName(ctx)))
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
		logger.Info(ctx, fmt.Sprintf("Found script plugin to unload: %s", pluginToUnload.Metadata.GetName(ctx)))
		m.UnloadPlugin(ctx, pluginToUnload)
	} else {
		logger.Debug(ctx, fmt.Sprintf("No script plugin found for file: %s", fileName))
	}
}

// WaitForSystemPlugins blocks until all system plugins have finished loading and initializing.
// This is useful for tests or callers that need to ensure all plugins are ready before querying.
func (m *Manager) WaitForSystemPlugins() {
	m.systemPluginsWg.Wait()
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
	defer util.GoRecover(ctx, fmt.Sprintf("<%s> query panic", pluginInstance.GetName(ctx)), func(err error) {
		// if plugin query panic, return error result
		failedResult := m.GetResultForFailedQuery(ctx, pluginInstance.Metadata, query, err)
		results = []QueryResult{
			m.PolishResult(ctx, pluginInstance, query, failedResult),
		}
	})

	logger.Info(ctx, fmt.Sprintf("<%s> start query: %s", pluginInstance.GetName(ctx), query.RawQuery))
	start := util.GetSystemTimestamp()

	// set query env base on plugin's feature
	currentEnv := query.Env
	newEnv := QueryEnv{}
	if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQueryEnv) {
		queryEnvParams, err := pluginInstance.Metadata.GetFeatureParamsForQueryEnv()
		if err != nil {
			logger.Error(ctx, fmt.Sprintf("<%s> invalid query env config: %s", pluginInstance.GetName(ctx), err))
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
			if queryEnvParams.RequireActiveWindowIsOpenSaveDialog {
				newEnv.ActiveWindowIsOpenSaveDialog = currentEnv.ActiveWindowIsOpenSaveDialog
			}
			if queryEnvParams.RequireActiveBrowserUrl {
				newEnv.ActiveBrowserUrl = currentEnv.ActiveBrowserUrl
			}
		}
	}
	query.Env = newEnv

	results = pluginInstance.Plugin.Query(ctx, query)
	logger.Debug(ctx, fmt.Sprintf("<%s> finish query, result count: %d, cost: %dms", pluginInstance.GetName(ctx), len(results), util.GetSystemTimestamp()-start))

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
	pluginName := pluginMetadata.GetName(ctx)

	return QueryResult{
		Title:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_query_failed"), pluginName),
		SubTitle: util.EllipsisEnd(err.Error(), 20),
		Icon:     icon,
		Preview: WoxPreview{
			PreviewType: WoxPreviewTypeText,
			PreviewData: err.Error(),
		},
	}
}

func (m *Manager) getDefaultActions(ctx context.Context, pluginInstance *Instance, query Query, title, subTitle string) (defaultActions []QueryResultAction) {
	// Declare both actions first
	var addToFavoriteAction func(context.Context, ActionContext)
	var removeFromFavoriteAction func(context.Context, ActionContext)

	// Define add to favorite action
	addToFavoriteAction = func(ctx context.Context, actionContext ActionContext) {
		setting.GetSettingManager().PinResult(ctx, pluginInstance.Metadata.Id, title, subTitle)

		// Get API instance
		api := NewAPI(pluginInstance)
		api.Notify(ctx, "i18n:plugin_manager_pin_in_query_success")

		// Get current result state
		updatableResult := api.GetUpdatableResult(ctx, actionContext.ResultId)
		if updatableResult == nil {
			return // Result no longer visible
		}

		// Update the result to refresh UI
		// Note: We don't need to manually add favorite tail here because:
		// 1. GetUpdatableResult filters out system tails (including favorite icon)
		// 2. PolishUpdatableResult will automatically add favorite tail back if this is a favorite result
		// 3. This ensures the favorite tail is always managed by the system
		api.UpdateResult(ctx, *updatableResult)
	}

	// Define remove from favorite action
	removeFromFavoriteAction = func(ctx context.Context, actionContext ActionContext) {
		setting.GetSettingManager().UnpinResult(ctx, pluginInstance.Metadata.Id, title, subTitle)

		// Get API instance
		api := NewAPI(pluginInstance)
		api.Notify(ctx, "i18n:plugin_manager_unpin_in_query")

		// Get current result state
		updatableResult := api.GetUpdatableResult(ctx, actionContext.ResultId)
		if updatableResult == nil {
			return // Result no longer visible
		}

		// Update the result to refresh UI
		// Note: We don't need to manually remove favorite tail here because:
		// 1. GetUpdatableResult filters out system tails (including favorite icon)
		// 2. PolishUpdatableResult will NOT add favorite tail back if this is not a favorite result
		// 3. This ensures the favorite tail is always managed by the system
		api.UpdateResult(ctx, *updatableResult)
	}

	if setting.GetSettingManager().IsPinedResult(ctx, pluginInstance.Metadata.Id, title, subTitle) {
		defaultActions = append(defaultActions, QueryResultAction{
			Name:                   "i18n:plugin_manager_unpin_in_query",
			Icon:                   common.UnpinIcon,
			IsSystemAction:         true,
			PreventHideAfterAction: true,
			Action:                 removeFromFavoriteAction,
		})
	} else {
		defaultActions = append(defaultActions, QueryResultAction{
			Name:                   "i18n:plugin_manager_pin_in_query",
			Icon:                   common.PinIcon,
			IsSystemAction:         true,
			PreventHideAfterAction: true,
			Action:                 addToFavoriteAction,
		})
	}

	return defaultActions
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

func (m *Manager) startSessionQueryCache(query Query) {
	m.sessionQueryResultCache.Store(query.SessionId, newQueryResultSet(query))
}

func (m *Manager) getQueryResultSet(sessionId string) (*QueryResultSet, bool) {
	return m.sessionQueryResultCache.Load(sessionId)
}

func (m *Manager) getQueryResultSetForQuery(query Query) (*QueryResultSet, bool) {
	if query.Id == "" {
		return nil, false
	}
	set, ok := m.sessionQueryResultCache.Load(query.SessionId)
	if !ok {
		return nil, false
	}
	if set.Query.Id != query.Id {
		return nil, false
	}
	return set, true
}

func (m *Manager) storeQueryResult(ctx context.Context, pluginInstance *Instance, query Query, resultOriginal QueryResult) {
	if query.Id == "" {
		logger.Warn(ctx, "query id is empty, skip result cache")
		return
	}
	if query.SessionId == "" {
		logger.Warn(ctx, "query session id is empty, skip result cache")
		return
	}

	set, ok := m.getQueryResultSetForQuery(query)
	if !ok {
		return
	}

	set.Results.Store(resultOriginal.Id, &QueryResultCache{
		Result:         resultOriginal,
		PluginInstance: pluginInstance,
		Query:          query,
	})

}

func (m *Manager) findResultCacheInSession(sessionId string, queryId string, resultId string) (*QueryResultCache, bool) {
	if resultId == "" {
		return nil, false
	}
	set, found := m.sessionQueryResultCache.Load(sessionId)
	if !found {
		return nil, false
	}
	if queryId != "" && set.Query.Id != queryId {
		return nil, false
	}
	resultCache, found := set.Results.Load(resultId)
	if !found {
		return nil, false
	}
	return resultCache, true
}

func (m *Manager) findResultCacheById(resultId string) (*QueryResultCache, bool) {
	if resultId == "" {
		return nil, false
	}

	var foundCache *QueryResultCache
	m.sessionQueryResultCache.Range(func(_ string, set *QueryResultSet) bool {
		resultCache, found := set.Results.Load(resultId)
		if !found {
			return true
		}
		foundCache = resultCache
		return false
	})

	if foundCache == nil {
		return nil, false
	}

	return foundCache, true
}

func (m *Manager) findResultCacheByIdWithContext(ctx context.Context, resultId string) (*QueryResultCache, bool) {
	sessionId := util.GetContextSessionId(ctx)
	queryId := util.GetContextQueryId(ctx)
	if sessionId != "" {
		if resultCache, ok := m.findResultCacheInSession(sessionId, queryId, resultId); ok {
			return resultCache, true
		}
	}
	return m.findResultCacheById(resultId)
}

func (m *Manager) GetSessionIdByQueryId(queryId string) string {
	if queryId == "" {
		return ""
	}
	var sessionId string
	m.sessionQueryResultCache.Range(func(_ string, set *QueryResultSet) bool {
		if set.Query.Id == queryId {
			sessionId = set.Query.SessionId
			return false
		}
		return true
	})
	return sessionId
}

func (m *Manager) GetResultSessionId(resultId string) string {
	resultCache, found := m.findResultCacheById(resultId)
	if !found {
		return ""
	}
	return resultCache.Query.SessionId
}

func (m *Manager) GetQueryInfoByResultId(resultId string) (string, string) {
	resultCache, found := m.findResultCacheById(resultId)
	if !found {
		return "", ""
	}
	return resultCache.Query.SessionId, resultCache.Query.Id
}

func (m *Manager) IsCurrentQuery(sessionId string, queryId string) bool {
	if queryId == "" {
		return false
	}
	set, found := m.sessionQueryResultCache.Load(sessionId)
	if !found {
		return false
	}
	return set.Query.Id == queryId
}

func (m *Manager) buildResultUI(resultCache *QueryResultCache, queryId string) QueryResultUI {
	uiResult := resultCache.Result
	if !uiResult.Preview.IsEmpty() && uiResult.Preview.PreviewType != WoxPreviewTypeRemote && len(uiResult.Preview.PreviewData) > previewDataMaxSize {
		uiResult.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?sessionId=%s&queryId=%s&id=%s", resultCache.Query.SessionId, queryId, uiResult.Id),
		}
	}
	resultUI := uiResult.ToUI()
	resultUI.QueryId = queryId
	return resultUI
}

// BuildQueryResultsSnapshot builds a snapshot of query results for the given session and query id.
// Results are grouped by their group name, and both groups and results within groups are sorted by score.
func (m *Manager) BuildQueryResultsSnapshot(sessionId string, queryId string) []QueryResultUI {
	if queryId == "" {
		return []QueryResultUI{}
	}
	set, found := m.sessionQueryResultCache.Load(sessionId)
	if !found || set.Query.Id != queryId {
		return []QueryResultUI{}
	}

	var results []QueryResultUI
	set.Results.Range(func(_ string, resultCache *QueryResultCache) bool {
		results = append(results, m.buildResultUI(resultCache, queryId))
		return true
	})

	if len(results) == 0 {
		return []QueryResultUI{}
	}

	groupScores := map[string]int64{}
	for _, result := range results {
		if score, ok := groupScores[result.Group]; !ok || result.GroupScore > score {
			groupScores[result.Group] = result.GroupScore
		}
	}

	var groups []string
	for group := range groupScores {
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		scoreA := groupScores[groups[i]]
		scoreB := groupScores[groups[j]]
		if scoreA == scoreB {
			return groups[i] < groups[j]
		}
		return scoreA > scoreB
	})

	groupedResults := make(map[string][]QueryResultUI)
	for _, result := range results {
		groupedResults[result.Group] = append(groupedResults[result.Group], result)
	}

	for group := range groupedResults {
		groupResults := groupedResults[group]
		sort.SliceStable(groupResults, func(i, j int) bool {
			return groupResults[i].Score > groupResults[j].Score
		})
		groupedResults[group] = groupResults
	}

	finalResults := make([]QueryResultUI, 0, len(results)+len(groups))
	for _, group := range groups {
		groupResults := groupedResults[group]
		if len(groupResults) == 0 {
			continue
		}
		if group != "" {
			finalResults = append(finalResults, QueryResultUI{
				QueryId:    queryId,
				Id:         fmt.Sprintf("group:%s:%s", queryId, group),
				Title:      group,
				SubTitle:   "",
				Icon:       common.WoxImage{},
				Preview:    WoxPreview{},
				Score:      groupScores[group],
				Group:      group,
				GroupScore: groupScores[group],
				Tails:      []QueryResultTail{},
				Actions:    []QueryResultActionUI{},
				IsGroup:    true,
			})
		}
		finalResults = append(finalResults, groupResults...)
	}

	return finalResults
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
			result.Actions[actionIndex].Icon = common.ExecuteRunIcon
		}
		if result.Actions[actionIndex].Type == "" {
			if len(result.Actions[actionIndex].Form) > 0 || result.Actions[actionIndex].OnSubmit != nil {
				result.Actions[actionIndex].Type = QueryResultActionTypeForm
			} else {
				result.Actions[actionIndex].Type = QueryResultActionTypeExecute
			}
		}
	}
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
	// translate tail text and assign IDs if not present
	for i := range result.Tails {
		if result.Tails[i].Id == "" {
			result.Tails[i].Id = uuid.NewString()
		}
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
		if result.Actions[actionIndex].Type == QueryResultActionTypeForm {
			for definitionIndex := range result.Actions[actionIndex].Form {
				if result.Actions[actionIndex].Form[definitionIndex].Value != nil {
					result.Actions[actionIndex].Form[definitionIndex].Value = result.Actions[actionIndex].Form[definitionIndex].Value.Translate(pluginInstance.API.GetTranslation)
				}
			}
		}
	}
	// translate preview data if preview type is text
	if result.Preview.PreviewType == WoxPreviewTypeText || result.Preview.PreviewType == WoxPreviewTypeMarkdown {
		result.Preview.PreviewData = m.translatePlugin(ctx, pluginInstance, result.Preview.PreviewData)
	}
	// translate group name
	result.Group = m.translatePlugin(ctx, pluginInstance, result.Group)

	// set first action as default if no default action is set
	defaultActionCount := lo.CountBy(result.Actions, func(item QueryResultAction) bool {
		return item.IsDefault
	})
	if defaultActionCount == 0 {
		if len(result.Actions) > 0 {
			result.Actions[0].IsDefault = true
			result.Actions[0].Hotkey = "Enter"
		}
	}

	//move default action to first one of the actions
	sort.Slice(result.Actions, func(i, j int) bool {
		return result.Actions[i].IsDefault
	})

	// normalize hotkeys for all actions
	for actionIndex := range result.Actions {
		var action = result.Actions[actionIndex]

		// if default action's hotkey is empty, set it as Enter
		if action.IsDefault && action.Hotkey == "" {
			result.Actions[actionIndex].Hotkey = "Enter"
		}

		// normalize hotkey for platform specific modifiers
		result.Actions[actionIndex].Hotkey = normalizeHotkeyForPlatform(result.Actions[actionIndex].Hotkey)
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
	var originalPreview = result.Preview
	if !result.Preview.IsEmpty() && result.Preview.PreviewType != WoxPreviewTypeRemote && len(result.Preview.PreviewData) > previewDataMaxSize {
		result.Preview = WoxPreview{
			PreviewType: WoxPreviewTypeRemote,
			PreviewData: fmt.Sprintf("/preview?sessionId=%s&queryId=%s&id=%s", query.SessionId, query.Id, result.Id),
		}
	}

	ignoreAutoScore := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureIgnoreAutoScore)
	if !ignoreAutoScore {
		score := m.calculateResultScore(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle, query.RawQuery)
		if score > 0 {
			logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) add score: %d", pluginInstance.GetName(ctx), result.Title, score))
			result.Score += score
		}
	}
	// check if result is favorite result
	// favorite result will not be affected by ignoreAutoScore setting, so we add score here
	isFavorite := setting.GetSettingManager().IsPinedResult(ctx, pluginInstance.Metadata.Id, result.Title, result.SubTitle)
	if isFavorite {
		favScore := int64(100000)
		logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) is favorite result, add score: %d", pluginInstance.GetName(ctx), result.Title, favScore))
		result.Score += favScore

		// Add favorite icon to tails if not already present
		hasFavoriteTail := false
		for _, tail := range result.Tails {
			if tail.ContextData[favoriteTailContextDataKey] == favoriteTailContextDataValue {
				hasFavoriteTail = true
				break
			}
		}
		if !hasFavoriteTail {
			result.Tails = append(result.Tails, QueryResultTail{
				Type:         QueryResultTailTypeImage,
				Image:        common.PinIcon,
				ContextData:  common.ContextData{favoriteTailContextDataKey: favoriteTailContextDataValue}, // Use ContextData to identify favorite tail
				IsSystemTail: true,                                                                         // Mark as system tail so it will be filtered out in GetUpdatableResult
			})
		}
	}

	// Create cache at the end
	resultCopy := result
	// Because we may have replaced preview with remote preview
	// we need to restore the original preview in the cache
	resultCopy.Preview = originalPreview
	m.storeQueryResult(ctx, pluginInstance, query, resultCopy)

	return result
}

func (m *Manager) PolishUpdatableResult(ctx context.Context, pluginInstance *Instance, result UpdatableResult) UpdatableResult {
	// Get result cache to update it
	resultCache, found := m.findResultCacheByIdWithContext(ctx, result.Id)
	if !found {
		return result // Result not in cache, just return as-is
	}

	// Polish actions if they are being updated
	if result.Actions != nil {
		actions := *result.Actions

		// Set default action id and icon if not present
		for actionIndex := range actions {
			if actions[actionIndex].Id == "" {
				actions[actionIndex].Id = uuid.NewString()
			}
			if actions[actionIndex].Icon.IsEmpty() {
				actions[actionIndex].Icon = common.ExecuteRunIcon
			}
			if actions[actionIndex].Type == "" {
				if len(actions[actionIndex].Form) > 0 || actions[actionIndex].OnSubmit != nil {
					actions[actionIndex].Type = QueryResultActionTypeForm
				} else {
					actions[actionIndex].Type = QueryResultActionTypeExecute
				}
			}
		}

		// For external plugins (Node.js/Python), create proxy action callbacks
		// These callbacks will invoke the host's action method, which will then
		// call the actual cached callback in the plugin host
		if proxyCreator, ok := pluginInstance.Plugin.(ActionProxyCreator); ok {
			for actionIndex := range actions {
				// Always create proxy callback for external plugins
				// because the Action field is not serialized and will be nil
				if actions[actionIndex].Type == QueryResultActionTypeExecute && actions[actionIndex].Action == nil {
					actions[actionIndex].Action = proxyCreator.CreateActionProxy(actions[actionIndex].Id)
				}
			}
		}
		if proxyCreator, ok := pluginInstance.Plugin.(FormActionProxyCreator); ok {
			for actionIndex := range actions {
				if actions[actionIndex].Type == QueryResultActionTypeForm && actions[actionIndex].OnSubmit == nil {
					actions[actionIndex].OnSubmit = proxyCreator.CreateFormActionProxy(actions[actionIndex].Id)
				}
			}
		}

		// Set first action as default if no default action is set
		defaultActionCount := lo.CountBy(actions, func(item QueryResultAction) bool {
			return item.IsDefault
		})
		if defaultActionCount == 0 && len(actions) > 0 {
			actions[0].IsDefault = true
			actions[0].Hotkey = "Enter"
		}

		// If default action's hotkey is empty, set it as Enter
		for actionIndex := range actions {
			if actions[actionIndex].IsDefault && actions[actionIndex].Hotkey == "" {
				actions[actionIndex].Hotkey = "Enter"
			}
			// Normalize hotkey for platform specific modifiers
			actions[actionIndex].Hotkey = normalizeHotkeyForPlatform(actions[actionIndex].Hotkey)
		}

		// Move default action to first position
		sort.Slice(actions, func(i, j int) bool {
			return actions[i].IsDefault
		})

		// Add system actions (like pin/unpin)
		// System actions are added after user actions
		systemActions := m.getDefaultActions(ctx, pluginInstance, resultCache.Query, resultCache.Result.Title, resultCache.Result.SubTitle)
		actions = append(actions, systemActions...)

		// Translate action names
		for actionIndex := range actions {
			actions[actionIndex].Name = m.translatePlugin(ctx, pluginInstance, actions[actionIndex].Name)
			if actions[actionIndex].Type == QueryResultActionTypeForm {
				for definitionIndex := range actions[actionIndex].Form {
					if actions[actionIndex].Form[definitionIndex].Value != nil {
						actions[actionIndex].Form[definitionIndex].Value = actions[actionIndex].Form[definitionIndex].Value.Translate(pluginInstance.API.GetTranslation)
					}
				}
			}
		}

		result.Actions = &actions

		// Update cache: merge new actions with cached actions to preserve callbacks
		// When updating actions, we need to preserve the Action callbacks from cache
		// because callbacks cannot be serialized and may be nil in the updated actions
		for i := range actions {
			// Find matching action in cache by ID
			var cachedAction *QueryResultAction
			for j := range resultCache.Result.Actions {
				if resultCache.Result.Actions[j].Id == actions[i].Id {
					cachedAction = &resultCache.Result.Actions[j]
					break
				}
			}

			// If action callback is nil in the new action but exists in cache, preserve it
			if cachedAction != nil {
				if actions[i].Action == nil && cachedAction.Action != nil {
					actions[i].Action = cachedAction.Action
				}
				if actions[i].Type == QueryResultActionTypeForm && actions[i].OnSubmit == nil && cachedAction.OnSubmit != nil {
					actions[i].OnSubmit = cachedAction.OnSubmit
				}
			}
		}

		// Update cache with merged actions
		resultCache.Result.Actions = actions
	}

	// Translate title if present
	if result.Title != nil {
		translated := m.translatePlugin(ctx, pluginInstance, *result.Title)
		result.Title = &translated
		resultCache.Result.Title = translated
	}

	// Translate subtitle if present
	if result.SubTitle != nil {
		translated := m.translatePlugin(ctx, pluginInstance, *result.SubTitle)
		result.SubTitle = &translated
		resultCache.Result.SubTitle = translated
	}

	// Translate tails if present
	if result.Tails != nil {
		tails := *result.Tails
		for i := range tails {
			// Assign ID if not present
			if tails[i].Id == "" {
				tails[i].Id = uuid.NewString()
			}
			if tails[i].Type == QueryResultTailTypeText {
				tails[i].Text = m.translatePlugin(ctx, pluginInstance, tails[i].Text)
			}
			if tails[i].Type == QueryResultTailTypeImage {
				tails[i].Image = common.ConvertIcon(ctx, tails[i].Image, pluginInstance.PluginDirectory)
			}
		}

		// Add favorite icon to tails if this is a favorite result
		isFavorite := setting.GetSettingManager().IsPinedResult(ctx, pluginInstance.Metadata.Id, resultCache.Result.Title, resultCache.Result.SubTitle)
		if isFavorite {
			// Check if favorite tail already exists
			hasFavoriteTail := false
			for _, tail := range tails {
				if tail.ContextData[favoriteTailContextDataKey] == favoriteTailContextDataValue {
					hasFavoriteTail = true
					break
				}
			}
			if !hasFavoriteTail {
				tails = append(tails, QueryResultTail{
					Type:         QueryResultTailTypeImage,
					Image:        common.PinIcon,
					ContextData:  common.ContextData{favoriteTailContextDataKey: favoriteTailContextDataValue}, // Use ContextData to identify favorite tail
					IsSystemTail: true,                                                                         // Mark as system tail so it will be filtered out in GetUpdatableResult
				})
			}
		}

		result.Tails = &tails
		resultCache.Result.Tails = tails
	}

	// Translate preview properties if present
	if result.Preview != nil {
		preview := *result.Preview
		var previewProperties = make(map[string]string)
		for key, value := range preview.PreviewProperties {
			translatedKey := m.translatePlugin(ctx, pluginInstance, key)
			previewProperties[translatedKey] = value
		}
		preview.PreviewProperties = previewProperties
		result.Preview = &preview
		resultCache.Result.Preview = preview
	}

	// Update icon in cache if present
	if result.Icon != nil {
		convertedIcon := common.ConvertIcon(ctx, *result.Icon, pluginInstance.PluginDirectory)
		result.Icon = &convertedIcon
		resultCache.Result.Icon = *result.Icon
	}

	return result
}

func (m *Manager) GetUpdatableResult(ctx context.Context, resultId string) *UpdatableResult {
	// Try to find the result in the cache
	resultCache, found := m.findResultCacheByIdWithContext(ctx, resultId)
	if !found {
		return nil // Result not found (no longer visible)
	}

	// Construct UpdatableResult from cache
	title := resultCache.Result.Title
	subTitle := resultCache.Result.SubTitle
	icon := resultCache.Result.Icon
	preview := resultCache.Result.Preview

	// Make a copy of tails to avoid modifying cache when developer appends to it
	// Filter out system tails (they will be added back in polish)
	tails := []QueryResultTail{}
	for _, tail := range resultCache.Result.Tails {
		if !tail.IsSystemTail {
			tails = append(tails, tail)
		}
	}

	// Make a copy of actions to avoid modifying cache when developer modifies it
	// Filter out system actions (they will be added back in polish)
	actions := []QueryResultAction{}
	for _, action := range resultCache.Result.Actions {
		if !action.IsSystemAction {
			actions = append(actions, action)
		}
	}

	return &UpdatableResult{
		Id:       resultId,
		Title:    &title,
		SubTitle: &subTitle,
		Icon:     &icon,
		Preview:  &preview,
		Tails:    &tails,
		Actions:  &actions,
	}
}

func (m *Manager) Query(ctx context.Context, query Query) (resultsChan chan []QueryResultUI, doneChan chan bool) {
	resultsChan = make(chan []QueryResultUI, 10)
	doneChan = make(chan bool)

	m.startSessionQueryCache(query)

	counter := &atomic.Int32{}
	counter.Store(int32(len(m.instances)))

	for _, pluginInstance := range m.instances {
		if !m.canOperateQuery(ctx, pluginInstance, query) {
			counter.Add(-1)
			if counter.Load() == 0 {
				doneChan <- true
			}
			continue
		}

		if pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDebounce) {
			debounceParams, err := pluginInstance.Metadata.GetFeatureParamsForDebounce()
			if err == nil {
				logger.Debug(ctx, fmt.Sprintf("[%s] debounce query, will execute in %d ms", pluginInstance.GetName(ctx), debounceParams.IntervalMs))
				if v, ok := m.debounceQueryTimer.Load(pluginInstance.Metadata.Id); ok {
					if v.timer.Stop() {
						v.onStop()
					}
				}

				timer := time.AfterFunc(time.Duration(debounceParams.IntervalMs)*time.Millisecond, func() {
					m.queryParallel(ctx, pluginInstance, query, resultsChan, doneChan, counter)
				})
				onStop := func() {
					logger.Debug(ctx, fmt.Sprintf("[%s] previous debounced query cancelled", pluginInstance.GetName(ctx)))
					counter.Add(-1)
					if counter.Load() == 0 {
						doneChan <- true
					}
				}
				m.debounceQueryTimer.Store(pluginInstance.Metadata.Id, &debounceTimer{
					timer:  timer,
					onStop: onStop,
				})
				continue
			} else {
				logger.Error(ctx, fmt.Sprintf("[%s] %s, query directlly", pluginInstance.GetName(ctx), err))
			}
		}

		m.queryParallel(ctx, pluginInstance, query, resultsChan, doneChan, counter)
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
						actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, query.SessionId), query.Id)
						m.ExecuteAction(actionCtx, query.SessionId, query.Id, result.Id, action.Id)
						return true
					}
				}
			} else {
				notifier.Notify(nil, fmt.Sprintf("Silent query failed, there shouldbe only one result, but got %d", len(results)))
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
				SubTitle: string(item.Description),
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
	util.Go(ctx, fmt.Sprintf("[%s] parallel query", pluginInstance.GetName(ctx)), func() {
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

	return pluginInstance.Metadata.translate(ctx, common.I18nString(key))
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
		query.Id = plainQuery.QueryId
		query.SessionId = util.GetContextSessionId(ctx)
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveWindowIcon = m.GetUI().GetActiveWindowIcon()
		query.Env.ActiveWindowIsOpenSaveDialog = m.GetUI().GetActiveWindowIsOpenSaveDialog()
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
		return query, instance, nil
	}

	if plainQuery.QueryType == QueryTypeSelection {
		query := Query{
			Id:        plainQuery.QueryId,
			Type:      QueryTypeSelection,
			RawQuery:  plainQuery.QueryText,
			Search:    plainQuery.QueryText,
			Selection: plainQuery.QuerySelection,
		}
		query.SessionId = util.GetContextSessionId(ctx)
		query.Env.ActiveWindowTitle = m.GetUI().GetActiveWindowName()
		query.Env.ActiveWindowPid = m.GetUI().GetActiveWindowPid()
		query.Env.ActiveWindowIcon = m.GetUI().GetActiveWindowIcon()
		query.Env.ActiveWindowIsOpenSaveDialog = m.GetUI().GetActiveWindowIsOpenSaveDialog()
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

func (m *Manager) getActiveFileExplorerPath(ctx context.Context) string {
	// Supported on Windows (File Explorer) and macOS (Finder)
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return ""
	}

	// Use platform-specific implementation via util/window
	return window.GetActiveFileExplorerPath()
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

func (m *Manager) ExecuteAction(ctx context.Context, sessionId string, queryId string, resultId string, actionId string) error {
	resultCache, found := m.findResultCacheInSession(sessionId, queryId, resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (execute action): %s", resultId)
	}

	// Find the action in cache
	var actionCache *QueryResultAction
	for i := range resultCache.Result.Actions {
		if resultCache.Result.Actions[i].Id == actionId {
			actionCache = &resultCache.Result.Actions[i]
			break
		}
	}
	if actionCache == nil {
		return fmt.Errorf("action not found for result id: %s, action id: %s", resultId, actionId)
	}

	// Check if action callback is nil
	if actionCache.Action == nil {
		return fmt.Errorf("action callback is nil for result id: %s, action id: %s", resultId, actionId)
	}

	meta := resultCache.PluginInstance.Metadata
	analytics.TrackActionExecuted(ctx, meta.Id, resultCache.PluginInstance.GetName(ctx))

	actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, resultCache.Query.SessionId), resultCache.Query.Id)
	actionCache.Action(actionCtx, ActionContext{
		ResultId:       resultId,
		ResultActionId: actionId,
		ContextData:    actionCache.ContextData,
	})

	util.Go(actionCtx, fmt.Sprintf("[%s] post execute action", resultCache.PluginInstance.GetName(actionCtx)), func() {
		m.postExecuteAction(actionCtx, resultCache, actionCache.ContextData)
	})

	return nil
}

func (m *Manager) SubmitFormAction(ctx context.Context, sessionId string, queryId string, resultId string, actionId string, values map[string]string) error {
	resultCache, found := m.findResultCacheInSession(sessionId, queryId, resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (submit form action): %s", resultId)
	}

	var actionCache *QueryResultAction
	for i := range resultCache.Result.Actions {
		if resultCache.Result.Actions[i].Id == actionId && resultCache.Result.Actions[i].Type == QueryResultActionTypeForm {
			actionCache = &resultCache.Result.Actions[i]
			break
		}
	}
	if actionCache == nil {
		return fmt.Errorf("form action not found for result id: %s, action id: %s", resultId, actionId)
	}

	if actionCache.OnSubmit == nil {
		return fmt.Errorf("form action callback is nil for result id: %s, action id: %s", resultId, actionId)
	}

	meta := resultCache.PluginInstance.Metadata
	analytics.TrackActionExecuted(ctx, meta.Id, resultCache.PluginInstance.GetName(ctx))

	actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, resultCache.Query.SessionId), resultCache.Query.Id)
	actionCache.OnSubmit(actionCtx, FormActionContext{
		ActionContext: ActionContext{
			ResultId:       resultId,
			ResultActionId: actionId,
			ContextData:    actionCache.ContextData,
		},
		Values: values,
	})

	util.Go(actionCtx, fmt.Sprintf("[%s] post execute action", resultCache.PluginInstance.GetName(actionCtx)), func() {
		m.postExecuteAction(actionCtx, resultCache, actionCache.ContextData)
	})

	return nil
}

func (m *Manager) postExecuteAction(ctx context.Context, resultCache *QueryResultCache, contextData map[string]string) {
	// Add actioned result for statistics
	meta := resultCache.PluginInstance.Metadata
	setting.GetSettingManager().AddActionedResult(ctx, meta.Id, resultCache.Result.Title, resultCache.Result.SubTitle, resultCache.Query.RawQuery)

	// Add to MRU if plugin supports it
	if meta.IsSupportFeature(MetadataFeatureMRU) {
		mruItem := setting.MRUItem{
			PluginID:    meta.Id,
			Title:       resultCache.Result.Title,
			SubTitle:    resultCache.Result.SubTitle,
			Icon:        resultCache.Result.Icon,
			ContextData: contextData,
		}

		// Decide MRU identity hash based on plugin metadata feature params
		hashTitle := mruItem.Title
		hashSubTitle := mruItem.SubTitle
		if params, err := meta.GetFeatureParamsForMRU(); err == nil {
			switch params.HashBy {
			case "rawquery":
				if resultCache.Query.RawQuery != "" {
					hashTitle = resultCache.Query.RawQuery
					hashSubTitle = ""
				}
			case "search":
				if resultCache.Query.Search != "" {
					hashTitle = resultCache.Query.Search
					hashSubTitle = ""
				}
			default:
				// "title" or unknown: keep default Title/SubTitle based hash
			}
		}
		mruItem.Hash = string(setting.NewResultHash(meta.Id, hashTitle, hashSubTitle))
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

func (m *Manager) GetResultPreview(ctx context.Context, sessionId string, queryId string, resultId string) (WoxPreview, error) {
	resultCache, found := m.findResultCacheInSession(sessionId, queryId, resultId)
	if !found {
		return WoxPreview{}, fmt.Errorf("result cache not found for result id (get preview): %s", resultId)
	}

	// if preview text is too long, ellipsis it, otherwise UI maybe freeze when render
	preview := resultCache.Result.Preview
	if preview.PreviewType == WoxPreviewTypeText {
		preview.PreviewData = util.EllipsisMiddle(preview.PreviewData, 2000)
		// translate preview data if preview type is text
		preview.PreviewData = m.translatePlugin(ctx, resultCache.PluginInstance, preview.PreviewData)
	}

	return preview, nil
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

	// Replace file explorer path variable if present
	if strings.Contains(query, QueryVariableFileExplorerPath) {
		startTime := time.Now()
		explorerPath := m.getActiveFileExplorerPath(ctx)
		query = strings.ReplaceAll(query, QueryVariableFileExplorerPath, explorerPath)
		logger.Debug(ctx, fmt.Sprintf("replaced file explorer path variable in %d ms", time.Since(startTime).Milliseconds()))
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

func (m *Manager) GetAIProvider(ctx context.Context, provider common.ProviderName, alias string) (ai.Provider, error) {
	key := string(provider)
	if alias != "" {
		key = fmt.Sprintf("%s_%s", provider, alias)
	}
	if v, exist := m.aiProviders.Load(key); exist {
		return v, nil
	}

	//check if provider has setting
	aiProviderSettings := setting.GetSettingManager().GetWoxSetting(ctx).AIProviders.Get()
	providerSetting, providerSettingExist := lo.Find(aiProviderSettings, func(item setting.AIProvider) bool {
		return item.Name == provider && item.Alias == alias
	})
	if !providerSettingExist {
		return nil, fmt.Errorf("ai provider setting not found: %s (alias=%s)", provider, alias)
	}

	newProvider, newProviderErr := ai.NewProvider(ctx, providerSetting)
	if newProviderErr != nil {
		return nil, newProviderErr
	}
	m.aiProviders.Store(key, newProvider)
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

	logger.Info(ctx, fmt.Sprintf("execute deeplink for plugin: %s, callbacks: %d", pluginInstance.GetName(ctx), len(pluginInstance.DeepLinkCallbacks)))

	for _, callback := range pluginInstance.DeepLinkCallbacks {
		util.Go(ctx, fmt.Sprintf("[%s] execute deeplink callback", pluginInstance.GetName(ctx)), func() {
			callback(ctx, arguments)
		})
	}
}

func (m *Manager) QueryMRU(ctx context.Context, sessionId string, queryId string) []QueryResultUI {
	query := Query{
		Id:             queryId,
		SessionId:      sessionId,
		Type:           QueryTypeInput,
		TriggerKeyword: "mru",
	}
	m.startSessionQueryCache(query)

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
			key := fmt.Sprintf("%s|%s|%s|%s", item.PluginID, restored.Title, restored.SubTitle, serializeContextData(item.ContextData))
			if seen[key] {
				util.GetLogger().Debug(ctx, fmt.Sprintf("duplicate mru item, skip restore mru item: %s", item.Title))
				continue
			}
			seen[key] = true

			// Add "Remove from MRU" action to each MRU result
			removeMRUAction := QueryResultAction{
				Id:   uuid.NewString(),
				Name: i18n.GetI18nManager().TranslateWox(ctx, "mru_remove_action"),
				Icon: common.TrashIcon,
				Action: func(ctx context.Context, actionContext ActionContext) {
					err := setting.GetSettingManager().RemoveMRUItem(ctx, item.Hash)
					if err != nil {
						util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove MRU item: %s", err.Error()))
					} else {
						util.GetLogger().Info(ctx, fmt.Sprintf("removed MRU item: %s - %s", item.Title, item.SubTitle))
					}
				},
			}

			// Add the remove action to the result
			restored.Actions = append(restored.Actions, removeMRUAction)

			polishedResult := m.PolishResult(ctx, pluginInstance, query, *restored)
			results = append(results, polishedResult.ToUI())
		}
	}

	if len(results) == 0 {
		return results
	}

	return m.BuildQueryResultsSnapshot(query.SessionId, query.Id)
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
		if restored, err := pluginInstance.MRURestoreCallbacks[0](ctx, mruData); err == nil {
			return restored
		} else {
			util.GetLogger().Debug(ctx, fmt.Sprintf("MRU restore failed for plugin %s: %s", pluginInstance.GetName(ctx), err.Error()))
		}
	}

	// For external plugins (Python/Node.js), MRU support will be implemented later
	// Currently only Go plugins support MRU functionality
	if pluginInstance.Host != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("External plugin MRU restore not yet implemented for plugin %s", pluginInstance.GetName(ctx)))
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
