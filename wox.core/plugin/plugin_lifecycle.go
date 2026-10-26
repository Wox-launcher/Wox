package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"wox/i18n"
	"wox/network"
	"wox/util"

	"github.com/samber/lo"
)

// pluginLifecycle owns installed instances and their load, host and file-watch lifetimes.
// Query scheduling and UI routing remain outside this owner. Locks stay with the
// operations they protect; releasePresentation clears plugin-owned UI messages and callbacks.
type pluginLifecycle struct {
	instances []*Instance
	// Offline transitions wait for in-flight loads before suspending instances.
	offlineLifecycle       sync.RWMutex
	instancesMu            sync.RWMutex
	systemPluginsReady     chan struct{}
	scriptPluginWatch      *util.DirectoryWatch
	scriptReloadTimers     *util.HashMap[string, *time.Timer]
	singleFilePluginWatch  *util.DirectoryWatch
	singleFileReloadTimers *util.HashMap[string, *time.Timer]
	singleFileWatchIgnored *util.HashMap[string, int64]
	// Manual recovery and watchdog restarts share this lock and backoff state.
	hostRestartMu         sync.Mutex
	hostWatchdogStates    map[string]*hostWatchdogState
	hostWatchdogStop      chan struct{}
	hostWatchdogDone      chan struct{}
	hostWatchdogLifecycle sync.Mutex
	hostWatchdogStarted   bool
	// Startup, store installs and reloads of the same ID must not publish duplicates.
	pluginLoadLocks     sync.Map
	releasePresentation func(string)
}

// newPluginLifecycle creates runtime state before any asynchronous load or watcher starts.
func newPluginLifecycle() pluginLifecycle {
	return pluginLifecycle{
		systemPluginsReady:     make(chan struct{}),
		scriptReloadTimers:     util.NewHashMap[string, *time.Timer](),
		singleFileReloadTimers: util.NewHashMap[string, *time.Timer](),
		singleFileWatchIgnored: util.NewHashMap[string, int64](),
		hostWatchdogStates:     map[string]*hostWatchdogState{},
		hostWatchdogStop:       make(chan struct{}),
		hostWatchdogDone:       make(chan struct{}),
	}
}

// Stop releases monitoring before intentionally shutting down shared runtime hosts.
func (l *pluginLifecycle) Stop(ctx context.Context) {
	// Stop host health monitoring before tearing down hosts so the watchdog
	// cannot mistake an intentional shutdown for a dead host.
	l.stopHostWatchdog()

	// Stop script plugin monitoring
	l.scriptPluginWatch.Close()
	l.singleFilePluginWatch.Close()
	l.scriptReloadTimers.Range(func(_ string, timer *time.Timer) bool {
		timer.Stop()
		return true
	})
	l.scriptReloadTimers.Clear()
	l.singleFileReloadTimers.Range(func(_ string, timer *time.Timer) bool {
		timer.Stop()
		return true
	})
	l.singleFileReloadTimers.Clear()

	for _, host := range AllHosts {
		host.Stop(ctx)
	}
	if l.releasePresentation != nil {
		for _, instance := range l.pluginInstancesSnapshot() {
			l.releasePresentation(instance.Metadata.Id)
		}
	}
}

// pluginInstancesSnapshot returns a stable slice for concurrent readers.
func (l *pluginLifecycle) pluginInstancesSnapshot() []*Instance {
	l.instancesMu.RLock()
	defer l.instancesMu.RUnlock()
	return append([]*Instance(nil), l.instances...)
}

// appendPluginInstance adds instance when its plugin ID is new.
// Empty IDs are allowed so tests can register anonymous host instances.
func (l *pluginLifecycle) appendPluginInstance(instance *Instance) bool {
	l.instancesMu.Lock()
	defer l.instancesMu.Unlock()
	if instance != nil {
		id := strings.TrimSpace(instance.Metadata.Id)
		if id != "" {
			for _, existing := range l.instances {
				if existing != nil && strings.EqualFold(existing.Metadata.Id, id) {
					return false
				}
			}
		}
	}
	l.instances = append(l.instances, instance)
	return true
}

// lockPluginLoad serializes load and reload for one plugin ID.
func (l *pluginLifecycle) lockPluginLoad(pluginID string) func() {
	key := strings.ToLower(strings.TrimSpace(pluginID))
	if key == "" {
		return func() {}
	}
	actual, _ := l.pluginLoadLocks.LoadOrStore(key, &sync.Mutex{})
	mu := actual.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// removePluginInstances updates the shared instance list without racing readers.
func (l *pluginLifecycle) removePluginInstances(shouldRemove func(*Instance) bool) {
	l.instancesMu.Lock()
	defer l.instancesMu.Unlock()

	remaining := l.instances[:0]
	for _, instance := range l.instances {
		if !shouldRemove(instance) {
			remaining = append(remaining, instance)
		}
	}
	l.instances = remaining
}

func (l *pluginLifecycle) ReloadPlugin(ctx context.Context, metadata Metadata) error {
	if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
		return err
	}
	logger.Info(ctx, fmt.Sprintf("start reloading dev plugin: %s", metadata.GetName(ctx)))
	unlock := l.lockPluginLoad(metadata.Id)
	defer unlock()

	if IsSingleFilePlugin(metadata) {
		if err := l.ensureSingleFilePluginIDAvailable(metadata); err != nil {
			return err
		}
	}

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}
	if IsSingleFilePlugin(metadata) {
		filePath, pathErr := singleFilePluginPath(metadata)
		if pathErr != nil {
			return pathErr
		}
		l.unloadSingleFilePluginByPath(ctx, filePath)
	}

	pluginInstance, pluginInstanceExist := lo.Find(l.pluginInstancesSnapshot(), func(item *Instance) bool {
		return strings.EqualFold(item.Metadata.Id, metadata.Id)
	})
	if pluginInstanceExist {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is loaded, unload first", metadata.GetName(ctx)))
		l.UnloadPlugin(ctx, pluginInstance)
	} else {
		logger.Info(ctx, fmt.Sprintf("plugin(%s) is not loaded, skip unload", metadata.GetName(ctx)))
	}

	return l.loadHostPluginLocked(ctx, pluginHost, metadata)
}

func (l *pluginLifecycle) LoadPlugin(ctx context.Context, pluginDirectory string) error {
	if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
		return err
	}
	metadata, parseErr := l.ParseMetadata(ctx, pluginDirectory)
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

	loadErr := l.loadHostPlugin(ctx, pluginHost, metadata)
	if loadErr != nil {
		return loadErr
	}

	return nil
}

// LoadRuntimePlugin loads metadata that did not come from ParseMetadata.
// Compatibility layers use it because their plugin manifest is not a Wox plugin.json.
func (l *pluginLifecycle) LoadRuntimePlugin(ctx context.Context, metadata Metadata) error {
	if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
		return err
	}
	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), metadata.Runtime)
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", metadata.Runtime)
	}
	if !pluginHost.IsStarted(ctx) {
		if err := pluginHost.Start(ctx); err != nil {
			return fmt.Errorf("failed to start host for runtime %s: %w", metadata.Runtime, err)
		}
	}
	return l.loadHostPlugin(ctx, pluginHost, metadata)
}

// ensureThirdPartyPluginsEnabled also stops installers before they replace files that cannot be loaded in this session.
func ensureThirdPartyPluginsEnabled(ctx context.Context) error {
	if network.IsOffline() {
		return network.ErrOffline
	}
	if util.IsThirdPartyPluginsDisabled() {
		return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_feedback_third_party_plugins_disabled"))
	}
	return nil
}

func (l *pluginLifecycle) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
	l.deactivatePlugin(ctx, pluginInstance)
	pluginInstance.closeImageCache(ctx)

	l.removePluginInstances(func(instance *Instance) bool {
		return instance.Metadata.Id == pluginInstance.Metadata.Id
	})
}

// DisablePlugin marks a plugin disabled and releases its runtime resources
// while keeping the instance visible to settings and store views.
func (l *pluginLifecycle) DisablePlugin(ctx context.Context, pluginId string) error {
	pluginInstance := l.GetPluginInstanceById(pluginId)
	if pluginInstance == nil {
		return fmt.Errorf("can't find plugin")
	}
	if err := pluginInstance.Setting.Disabled.Set(true); err != nil {
		return err
	}
	l.deactivatePlugin(ctx, pluginInstance)
	return nil
}

// EnablePlugin marks a plugin enabled and initializes its runtime if it was
// previously deactivated by DisablePlugin.
func (l *pluginLifecycle) EnablePlugin(ctx context.Context, pluginId string) error {
	if instance := l.GetPluginInstanceById(pluginId); instance != nil && instance.Host != nil {
		if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
			return err
		}
	}
	pluginInstance := l.GetPluginInstanceById(pluginId)
	if pluginInstance == nil {
		return fmt.Errorf("can't find plugin")
	}
	if err := pluginInstance.Setting.Disabled.Set(false); err != nil {
		return err
	}
	if err := l.activatePlugin(ctx, pluginInstance); err != nil {
		_ = pluginInstance.Setting.Disabled.Set(true)
		return err
	}
	return nil
}

// deactivatePlugin runs plugin unload callbacks and releases host runtime state
// without removing the instance from the installed plugin list.
func (l *pluginLifecycle) deactivatePlugin(ctx context.Context, pluginInstance *Instance) {
	if pluginInstance == nil {
		return
	}
	pluginInstance.initLifecycleMu.Lock()
	defer pluginInstance.initLifecycleMu.Unlock()
	pluginInstance.stopPluginTools()
	initialized, _ := pluginInstance.initStatus()
	if initialized && !(network.IsOffline() && pluginInstance.Host != nil) {
		for _, callback := range pluginInstance.UnloadCallbacks {
			callback(ctx)
		}
	}
	l.clearRuntimeCallbacks(pluginInstance)
	pluginInstance.resetInitState()

	if pluginInstance.Host != nil && pluginInstance.RuntimeLoaded {
		if !network.IsOffline() {
			pluginInstance.Host.UnloadPlugin(ctx, pluginInstance.Metadata)
		}
		pluginInstance.RuntimeLoaded = false
		pluginInstance.Plugin = nil
	}
}

// activatePlugin loads host runtime state if needed and runs Init once for the
// current enabled lifecycle.
func (l *pluginLifecycle) activatePlugin(ctx context.Context, pluginInstance *Instance) error {
	l.offlineLifecycle.RLock()
	defer l.offlineLifecycle.RUnlock()
	if pluginInstance != nil && pluginInstance.Host != nil {
		if err := ensureThirdPartyPluginsEnabled(ctx); err != nil {
			return err
		}
	}
	if pluginInstance == nil {
		return fmt.Errorf("can't find plugin")
	}
	pluginInstance.initLifecycleMu.Lock()
	defer pluginInstance.initLifecycleMu.Unlock()
	initialized, _ := pluginInstance.initStatus()
	if initialized {
		return nil
	}
	if pluginInstance.Host != nil && !pluginInstance.RuntimeLoaded {
		if !pluginInstance.Host.IsStarted(ctx) {
			if err := pluginInstance.Host.Start(ctx); err != nil {
				return fmt.Errorf("failed to start host for runtime %s: %w", pluginInstance.Metadata.Runtime, err)
			}
		}
		loadedPlugin, err := pluginInstance.Host.LoadPlugin(ctx, pluginInstance.Metadata, pluginInstance.PluginDirectory)
		if err != nil {
			return err
		}
		pluginInstance.Plugin = loadedPlugin
		pluginInstance.RuntimeLoaded = true
	}
	if pluginInstance.Plugin == nil {
		return fmt.Errorf("plugin runtime is not loaded: %s", pluginInstance.Metadata.GetName(ctx))
	}
	if pluginInstance.initCycleFinished() {
		pluginInstance.beginInitCycle()
	}
	l.initPluginLocked(ctx, pluginInstance)
	initialized, initErr := pluginInstance.initStatus()
	if initErr != nil {
		return initErr
	}
	if !initialized {
		return fmt.Errorf("plugin did not initialize: %s", pluginInstance.Metadata.GetName(ctx))
	}
	return nil
}

// clearRuntimeCallbacks drops callbacks registered during Init so a later
// enable starts from a clean plugin lifecycle.
func (l *pluginLifecycle) clearRuntimeCallbacks(pluginInstance *Instance) {
	if l.releasePresentation != nil {
		l.releasePresentation(pluginInstance.Metadata.Id)
	}
	pluginInstance.DynamicSettingCallbacks = nil
	pluginInstance.SettingChangeCallbacks = nil
	pluginInstance.DeepLinkCallbacks = nil
	pluginInstance.UnloadCallbacks = nil
	pluginInstance.MRURestoreCallbacks = nil
	pluginInstance.clearPluginTools()
	pluginInstance.EnterPluginQueryCallbacks = nil
	pluginInstance.LeavePluginQueryCallbacks = nil
	pluginInstance.DragOutCallbacks = nil
	pluginInstance.RuntimeQueryCommands = nil
	pluginInstance.setRuntimeTriggerKeywords(nil)
}

// WaitForSystemPlugins blocks until all system plugins have finished loading and initializing.
// This is useful for tests or callers that need to ensure all plugins are ready before querying.
func (l *pluginLifecycle) WaitForSystemPlugins() {
	<-l.systemPluginsReady
}

func (l *pluginLifecycle) GetPluginInstances() []*Instance {
	return l.pluginInstancesSnapshot()
}

func (l *pluginLifecycle) GetPluginInstanceById(pluginId string) *Instance {
	for _, instance := range l.pluginInstancesSnapshot() {
		if strings.EqualFold(instance.Metadata.Id, pluginId) {
			return instance
		}
	}
	return nil
}

// GetSystemPlugin returns the SystemPlugin implementation for the given plugin ID,
// or nil if the plugin is not found or not a system plugin.
func (l *pluginLifecycle) GetSystemPlugin(pluginId string) SystemPlugin {
	instance := l.GetPluginInstanceById(pluginId)
	if instance == nil || !instance.IsSystemPlugin {
		return nil
	}
	return instance.Plugin.(SystemPlugin)
}
