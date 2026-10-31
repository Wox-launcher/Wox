package plugin

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"wox/util"

	"github.com/fsnotify/fsnotify"
	"github.com/samber/lo"
)

// loadScriptPlugins loads script plugins from the user script plugins directory
func (l *pluginLifecycle) loadScriptPlugins(ctx context.Context) ([]Metadata, error) {
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
		metadata, metadataErr := l.ParseScriptMetadata(ctx, scriptPath)
		if metadataErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse script plugin metadata for %s: %s", entry.Name(), metadataErr.Error()))
			continue
		}

		metaDataList = append(metaDataList, metadata)
	}

	logger.Debug(ctx, fmt.Sprintf("found %d script plugins", len(metaDataList)))
	return metaDataList, nil
}

// startScriptPluginMonitoring starts monitoring the user script plugins directory for changes
func (l *pluginLifecycle) startScriptPluginMonitoring(ctx context.Context) {
	userScriptPluginDirectory := util.GetLocation().GetUserScriptPluginsDirectory()

	watch, err := util.WatchDirectory(userScriptPluginDirectory, func(event fsnotify.Event) {
		l.handleScriptPluginEvent(ctx, event)
	}, func(watchErr error) {
		logger.Error(ctx, fmt.Sprintf("Script plugin watcher error: %s", watchErr.Error()))
	})
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to watch script plugin directory: %s", err.Error()))
		return
	}
	l.scriptPluginWatch = watch
	logger.Info(ctx, fmt.Sprintf("Started monitoring script plugins directory: %s", userScriptPluginDirectory))
}

// handleScriptPluginEvent handles file system events for script plugins
func (l *pluginLifecycle) handleScriptPluginEvent(ctx context.Context, event fsnotify.Event) {
	// Skip non-script files and temporary files
	fileName := filepath.Base(event.Name)
	if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, "~") || strings.HasSuffix(fileName, ".tmp") {
		return
	}
	if l.isArtifactWatchIgnored(event.Name) {
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
		l.debounceScriptPluginReload(ctx, event.Name, "file changed")
	case fsnotify.Remove, fsnotify.Rename:
		// File deleted or renamed - unload the plugin
		l.unloadScriptPluginByPath(ctx, event.Name)
	case fsnotify.Chmod:
		// File permissions changed - ignore for now
		logger.Debug(ctx, fmt.Sprintf("Script plugin file permissions changed: %s", event.Name))
	}
}

// debounceScriptPluginReload debounces script plugin reload to avoid multiple reloads in a short time
func (l *pluginLifecycle) debounceScriptPluginReload(ctx context.Context, scriptPath, reason string) {
	fileName := filepath.Base(scriptPath)

	// Cancel existing timer if any
	if timer, exists := l.scriptReloadTimers.Load(fileName); exists {
		timer.Stop()
	}

	// Create new timer
	timer := time.AfterFunc(2*time.Second, func() {
		if l.isArtifactWatchIgnored(scriptPath) {
			l.scriptReloadTimers.Delete(fileName)
			return
		}
		l.reloadScriptPlugin(util.NewTraceContext(), scriptPath, reason)
		l.scriptReloadTimers.Delete(fileName)
	})

	l.scriptReloadTimers.Store(fileName, timer)
}

// reloadScriptPlugin reloads a script plugin
func (l *pluginLifecycle) reloadScriptPlugin(ctx context.Context, scriptPath, reason string) {
	logger.Info(ctx, fmt.Sprintf("Reloading script plugin: %s, reason: %s", scriptPath, reason))

	// Check if file still exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		logger.Warn(ctx, fmt.Sprintf("Script plugin file no longer exists: %s", scriptPath))
		return
	}

	// Parse metadata from the script file
	metadata, err := l.ParseScriptMetadata(ctx, scriptPath)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to parse script plugin metadata: %s", err.Error()))
		return
	}

	// Find and unload existing plugin instance if any
	existingInstance, exists := lo.Find(l.pluginInstancesSnapshot(), func(instance *Instance) bool {
		return instance.Metadata.Id == metadata.Id
	})
	if exists {
		logger.Info(ctx, fmt.Sprintf("Unloading existing script plugin instance: %s", metadata.GetName(ctx)))
		l.UnloadPlugin(ctx, existingInstance)
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
	err = l.loadHostPlugin(ctx, scriptHost, metadata)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to reload script plugin: %s", err.Error()))
		return
	}

	logger.Info(ctx, fmt.Sprintf("Successfully reloaded script plugin: %s", metadata.GetName(ctx)))
}

// unloadScriptPluginByPath unloads a script plugin by its file path
func (l *pluginLifecycle) unloadScriptPluginByPath(ctx context.Context, scriptPath string) {
	fileName := filepath.Base(scriptPath)
	logger.Info(ctx, fmt.Sprintf("Unloading script plugin: %s", fileName))

	// Find plugin instance by script file name
	var pluginToUnload *Instance
	for _, instance := range l.pluginInstancesSnapshot() {
		if instance.Metadata.Runtime == string(PLUGIN_RUNTIME_SCRIPT) && instance.Metadata.Entry == fileName {
			pluginToUnload = instance
			break
		}
	}

	if pluginToUnload != nil {
		logger.Info(ctx, fmt.Sprintf("Found script plugin to unload: %s", pluginToUnload.Metadata.GetName(ctx)))
		l.UnloadPlugin(ctx, pluginToUnload)
	} else {
		logger.Debug(ctx, fmt.Sprintf("No script plugin found for file: %s", fileName))
	}
}
