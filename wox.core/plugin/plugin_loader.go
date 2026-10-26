package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"wox/network"
	"wox/setting"
	"wox/util"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
)

func (l *pluginLifecycle) loadPlugins(ctx context.Context) error {
	logger.Info(ctx, "start loading plugins")

	// load system plugin first
	l.loadSystemPlugins(ctx)
	return l.loadUserPlugins(ctx, false)
}

// loadUserPlugins restores installed plugins without changing their persisted enablement.
func (l *pluginLifecycle) loadUserPlugins(ctx context.Context, wait bool) error {
	if util.IsThirdPartyPluginsDisabled() {
		logger.Info(ctx, "troubleshooting mode: skipping third-party plugins and runtime hosts")
		return nil
	}

	logger.Debug(ctx, "start loading user plugin metadata")
	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to read plugin directory (%s), continue without user plugins: %s", basePluginDirectory, readErr.Error()))
		pluginDirectories = []os.DirEntry{}
	}

	var metaDataList []Metadata
	for _, entry := range pluginDirectories {
		if entry.Name() == ".DS_Store" || IsReservedUserPluginDirectory(entry.Name()) {
			continue
		}
		if !entry.IsDir() {
			continue
		}

		pluginDirectory := path.Join(basePluginDirectory, entry.Name())
		metadata, metadataErr := l.ParseMetadata(ctx, pluginDirectory)
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
	scriptMetaDataList, err := l.loadScriptPlugins(ctx)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load script plugins: %s", err.Error()))
	} else {
		metaDataList = append(metaDataList, scriptMetaDataList...)
	}

	singleFileMetaDataList, err := l.loadSingleFilePlugins(ctx)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to load single-file plugins: %s", err.Error()))
	} else {
		metaDataList = append(metaDataList, singleFileMetaDataList...)
	}

	for _, host := range AllHosts {
		catalog, ok := host.(ExternalCatalog)
		if !ok {
			continue
		}
		discovered, discoverErr := catalog.DiscoverMetadata(ctx)
		if discoverErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to load %s plugins: %s", host.GetRuntime(ctx), discoverErr.Error()))
			continue
		}
		metaDataList = append(metaDataList, discovered...)
	}

	uniqueMetadata := make([]Metadata, 0, len(metaDataList))
	seenPluginIDs := make(map[string]Metadata, len(metaDataList))
	for _, instance := range l.pluginInstancesSnapshot() {
		seenPluginIDs[strings.ToLower(instance.Metadata.Id)] = instance.Metadata
	}
	for _, metadata := range metaDataList {
		key := strings.ToLower(metadata.Id)
		if existing, exists := seenPluginIDs[key]; exists {
			logger.Error(ctx, fmt.Sprintf("skip plugin %s from %s: ID is already used by %s from %s", metadata.Id, metadata.Directory, existing.Id, existing.Directory))
			continue
		}
		seenPluginIDs[key] = metadata
		uniqueMetadata = append(uniqueMetadata, metadata)
	}
	metaDataList = uniqueMetadata

	logger.Info(ctx, fmt.Sprintf("start loading user plugins, found %d user plugins", len(metaDataList)))

	var pending sync.WaitGroup
	var loadMu sync.Mutex
	var loadErrors []error
	for _, host := range AllHosts {
		if network.IsOffline() {
			for _, metadata := range metaDataList {
				if strings.EqualFold(metadata.Runtime, string(host.GetRuntime(ctx))) {
					if err := l.registerOfflinePlugin(ctx, host, metadata); err != nil {
						util.GetLogger().Warn(ctx, err.Error())
					}
				}
			}
			continue
		}
		if wait {
			hasPending := false
			for _, metadata := range metaDataList {
				if strings.EqualFold(metadata.Runtime, string(host.GetRuntime(ctx))) {
					hasPending = true
					break
				}
			}
			if !hasPending {
				continue
			}
		}
		pending.Add(1)
		util.Go(ctx, fmt.Sprintf("[%s] start host", host.GetRuntime(ctx)), func() {
			defer pending.Done()
			newCtx := util.NewTraceContext()
			recordError := func(err error) {
				loadMu.Lock()
				loadErrors = append(loadErrors, err)
				loadMu.Unlock()
				if !network.IsOffline() {
					util.GetLogger().Error(newCtx, err.Error())
				}
			}
			if !host.IsStarted(newCtx) {
				if err := host.Start(newCtx); err != nil {
					recordError(err)
					return
				}
			}
			for _, metadata := range metaDataList {
				if !strings.EqualFold(metadata.Runtime, string(host.GetRuntime(newCtx))) {
					continue
				}
				if err := l.loadHostPlugin(newCtx, host, metadata); err != nil {
					recordError(err)
				}
			}
		})
	}
	if wait {
		pending.Wait()
		return errors.Join(loadErrors...)
	}

	return nil
}

func (l *pluginLifecycle) loadHostPlugin(ctx context.Context, host Host, metadata Metadata) error {
	unlock := l.lockPluginLoad(metadata.Id)
	defer unlock()
	return l.loadHostPluginLocked(ctx, host, metadata)
}

func (l *pluginLifecycle) loadHostPluginLocked(ctx context.Context, host Host, metadata Metadata) error {
	l.offlineLifecycle.RLock()
	defer l.offlineLifecycle.RUnlock()
	// Installs and reloads must not bypass the startup-only troubleshooting mode.
	if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
		return err
	}
	// Plugin loading is the final shared gate for startup, local installs, and dev
	// reloads. Install-time checks can be bypassed by existing files on disk, so
	// keep this guard here to prevent incompatible plugins from entering runtime hosts.
	if err := ensureWoxVersionSupported(metadata.GetName(ctx), metadata.MinWoxVersion); err != nil {
		return err
	}

	if strings.TrimSpace(metadata.Id) != "" && l.GetPluginInstanceById(metadata.Id) != nil {
		// Startup and store install can both try to load the same on-disk plugin.
		// Treat the second call as success so install does not roll back the file.
		logger.Info(ctx, fmt.Sprintf("[%s HOST] plugin %s (%s) is already loaded, skip duplicate load", host.GetRuntime(ctx), metadata.GetName(ctx), metadata.Id))
		return nil
	}

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
		RuntimeLoaded:         true,
		LoadStartTimestamp:    loadStartTimestamp,
		LoadFinishedTimestamp: loadFinishTimestamp,
		IsDevPlugin:           metadata.IsDev,
		DevPluginDirectory:    metadata.DevPluginDirectory,
	}
	instance.beginInitCycle()
	instance.API = NewAPI(instance)
	pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
	if settingErr != nil {
		instance.API.Log(ctx, LogLevelError, fmt.Errorf("[SYS] failed to load plugin[%s] setting: %w", metadata.GetName(ctx), settingErr).Error())
		instance.finishInit(false, settingErr)
		return settingErr
	}
	instance.Setting = pluginSetting

	if !l.appendPluginInstance(instance) {
		logger.Info(ctx, fmt.Sprintf("[%s HOST] skip duplicate plugin instance %s (%s): ID is already loaded", host.GetRuntime(ctx), metadata.GetName(ctx), metadata.Id))
		instance.finishInit(false, nil)
		return nil
	}

	if pluginSetting.Disabled.Get() {
		logger.Info(ctx, fmt.Errorf("[%s HOST] plugin is disabled by user, skip init: %s", host.GetRuntime(ctx), metadata.GetName(ctx)).Error())
		instance.API.Log(ctx, LogLevelWarning, fmt.Sprintf("[SYS] plugin is disabled by user, skip init: %s", metadata.GetName(ctx)))
		instance.finishInit(false, nil)
		return nil
	}

	util.Go(ctx, fmt.Sprintf("[%s] init plugin", metadata.GetName(ctx)), func() {
		l.initPlugin(ctx, instance)
	})

	return nil
}

func (l *pluginLifecycle) loadSystemPlugins(ctx context.Context) {
	start := util.GetSystemTimestamp()
	logger.Info(ctx, fmt.Sprintf("start loading system plugins, found %d system plugins", len(AllSystemPlugin)))

	// Native startup invokes plugin loading on the UI thread before the platform
	// event loop begins. Keep system plugin initialization on that thread because
	// plugins may synchronously call mainthread; waiting here for parallel plugin
	// goroutines would deadlock with their dispatch to the blocked UI thread.
	for _, plugin := range AllSystemPlugin {
		metadata := plugin.GetMetadata()
		// System plugins use Wox's central i18n keys directly. The old path
		// flattened every central language file into every system plugin metadata,
		// which duplicated the same translation maps across all system plugins.
		// Metadata.translate already falls back to TranslateWox, so keeping the
		// central translations owned by the i18n manager preserves behavior while
		// avoiding the per-plugin live heap copy.
		instance := &Instance{
			Metadata:              metadata,
			Plugin:                plugin,
			Host:                  nil,
			IsSystemPlugin:        true,
			RuntimeLoaded:         true,
			PluginDirectory:       metadata.Directory,
			LoadStartTimestamp:    util.GetSystemTimestamp(),
			LoadFinishedTimestamp: util.GetSystemTimestamp(),
		}
		instance.beginInitCycle()
		instance.API = NewAPI(instance)

		startTimestamp := util.GetSystemTimestamp()
		pluginSetting, settingErr := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
		if settingErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to load system plugin[%s] setting, use default plugin setting. err: %s", metadata.GetName(ctx), settingErr.Error()))
			instance.finishInit(false, settingErr)
			continue
		}

		instance.Setting = pluginSetting
		if util.GetSystemTimestamp()-startTimestamp > 100 {
			logger.Warn(ctx, fmt.Sprintf("load system plugin[%s] setting too slow, cost %d ms", metadata.GetName(ctx), util.GetSystemTimestamp()-startTimestamp))
		}
		if pluginSetting.Disabled.Get() {
			logger.Info(ctx, fmt.Sprintf("system plugin is disabled by user, skip init: %s", metadata.GetName(ctx)))
			// Keep the instance discoverable so EnablePlugin can initialize it later.
			instance.finishInit(false, nil)
			l.appendPluginInstance(instance)
			continue
		}

		// Init plugin BEFORE adding to instances list
		// This ensures the plugin is fully initialized before it can be queried
		l.initPlugin(util.NewTraceContext(), instance)
		l.appendPluginInstance(instance)
	}
	close(l.systemPluginsReady)

	logger.Debug(ctx, fmt.Sprintf("finish loading system plugins, cost %d ms", util.GetSystemTimestamp()-start))
}

func (l *pluginLifecycle) initPlugin(ctx context.Context, instance *Instance) {
	instance.initLifecycleMu.Lock()
	defer instance.initLifecycleMu.Unlock()
	l.initPluginLocked(ctx, instance)
}

// initPluginLocked runs Init while the caller holds the instance lifecycle lock.
func (l *pluginLifecycle) initPluginLocked(ctx context.Context, instance *Instance) {
	if network.IsOffline() && instance.Host != nil {
		instance.finishInit(false, network.ErrOffline)
		return
	}
	logger.Info(ctx, fmt.Sprintf("start init plugin: %s", instance.Metadata.GetName(ctx)))
	instance.InitStartTimestamp = util.GetSystemTimestamp()
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("plugin init panic: %v", r)
			instance.InitFinishedTimestamp = util.GetSystemTimestamp()
			logger.Error(ctx, fmt.Sprintf("init plugin %s panicked: %s", instance.Metadata.GetName(ctx), err.Error()))
			l.clearRuntimeCallbacks(instance)
			instance.finishInit(false, err)
		}
	}()

	err := runPluginInit(ctx, instance)
	instance.InitFinishedTimestamp = util.GetSystemTimestamp()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("init plugin %s failed, cost %d ms: %s", instance.Metadata.GetName(ctx), instance.InitFinishedTimestamp-instance.InitStartTimestamp, err.Error()))
		l.clearRuntimeCallbacks(instance)
		instance.finishInit(false, err)
		return
	}

	logger.Info(ctx, fmt.Sprintf("init plugin %s finished, cost %d ms", instance.Metadata.GetName(ctx), instance.InitFinishedTimestamp-instance.InitStartTimestamp))
	instance.finishInit(true, nil)
}

// runPluginInit preserves the legacy Plugin contract while exposing host errors.
func runPluginInit(ctx context.Context, instance *Instance) error {
	params := InitParams{
		API:             instance.API,
		PluginDirectory: instance.PluginDirectory,
	}
	if initer, ok := instance.Plugin.(FallibleInit); ok {
		return initer.InitWithError(ctx, params)
	}
	instance.Plugin.Init(ctx, params)
	return nil
}
