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

// loadSingleFilePlugins scans the shared single-file directory for .py and .js plugins.
func (m *Manager) loadSingleFilePlugins(ctx context.Context) ([]Metadata, error) {
	logger.Debug(ctx, "start loading single-file plugin metadata")

	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		return nil, fmt.Errorf("failed to ensure single-file plugin directory: %w", err)
	}
	entries, readErr := os.ReadDir(directory)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read single-file plugin directory: %w", readErr)
	}

	var metaDataList []Metadata
	for _, entry := range entries {
		if entry.IsDir() || shouldIgnoreSingleFileWatchName(entry.Name()) || !isSingleFilePluginSourceName(entry.Name()) {
			continue
		}
		filePath := path.Join(directory, entry.Name())
		metadata, metadataErr := m.ParseSingleFilePluginMetadata(ctx, filePath)
		if metadataErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse single-file plugin metadata for %s: %s", entry.Name(), metadataErr.Error()))
			continue
		}
		metaDataList = append(metaDataList, metadata)
	}

	logger.Debug(ctx, fmt.Sprintf("found %d single-file plugins", len(metaDataList)))
	return metaDataList, nil
}

func (m *Manager) startSingleFilePluginMonitoring(ctx context.Context) {
	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to ensure single-file plugin directory: %s", err.Error()))
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to create single-file plugin watcher: %s", err.Error()))
		return
	}
	m.singleFilePluginWatcher = watcher

	if err := watcher.Add(directory); err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to watch single-file plugin directory: %s", err.Error()))
		watcher.Close()
		return
	}

	logger.Info(ctx, fmt.Sprintf("Started monitoring single-file plugins directory: %s", directory))

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				logger.Info(ctx, "Single-file plugin watcher closed")
				return
			}
			m.handleSingleFilePluginEvent(ctx, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				logger.Info(ctx, "Single-file plugin watcher error channel closed")
				return
			}
			logger.Error(ctx, fmt.Sprintf("Single-file plugin watcher error: %s", err.Error()))
		case <-ctx.Done():
			logger.Info(ctx, "Single-file plugin monitoring stopped due to context cancellation")
			watcher.Close()
			return
		}
	}
}

func (m *Manager) handleSingleFilePluginEvent(ctx context.Context, event fsnotify.Event) {
	fileName := filepath.Base(event.Name)
	if shouldIgnoreSingleFileWatchName(fileName) || !isSingleFilePluginSourceName(fileName) {
		return
	}
	if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
		return
	}
	if m.isSingleFileWatchIgnored(event.Name) {
		return
	}

	logger.Info(ctx, fmt.Sprintf("Single-file plugin event: %s (%s)", event.Name, event.Op))

	switch {
	case event.Op&fsnotify.Create != 0 || event.Op&fsnotify.Write != 0:
		m.debounceSingleFilePluginReload(ctx, event.Name, event.Op.String())
	case event.Op&fsnotify.Remove != 0 || event.Op&fsnotify.Rename != 0:
		m.unloadSingleFilePluginByPath(ctx, event.Name)
	case event.Op&fsnotify.Chmod != 0:
		logger.Debug(ctx, fmt.Sprintf("Single-file plugin permissions changed: %s", event.Name))
	}
}

// IgnoreSingleFileWatch temporarily gives a managed writer sole ownership of reload.
func (m *Manager) IgnoreSingleFileWatch(filePath string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	m.singleFileWatchIgnored.Store(absPath, time.Now().Add(singleFileManagedWriteWindow).UnixMilli())
}

func (m *Manager) isSingleFileWatchIgnored(filePath string) bool {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	deadline, exists := m.singleFileWatchIgnored.Load(absPath)
	if !exists {
		return false
	}
	if time.Now().UnixMilli() <= deadline {
		return true
	}
	m.singleFileWatchIgnored.Delete(absPath)
	return false
}

func (m *Manager) debounceSingleFilePluginReload(ctx context.Context, filePath, reason string) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	debounceKeyedTimer(m.singleFileReloadTimers, absPath, singleFileWatchDebounce, func() {
		if m.isSingleFileWatchIgnored(absPath) {
			return
		}
		m.reloadSingleFilePlugin(util.NewTraceContext(), absPath, reason)
	})
}

func (m *Manager) reloadSingleFilePlugin(ctx context.Context, filePath, reason string) {
	logger.Info(ctx, fmt.Sprintf("Reloading single-file plugin: %s, reason: %s", filePath, reason))

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Warn(ctx, fmt.Sprintf("Single-file plugin no longer exists: %s", filePath))
		return
	}

	content, err := readFileWithRetry(filePath, singleFileReadRetryWindow)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to read single-file plugin %s: %s", filePath, err.Error()))
		return
	}

	parsed, parseErr := parseInlineMetadataContent(filePath, content)
	if parseErr != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to parse single-file plugin metadata, keeping existing instance: %s", parseErr.Error()))
		return
	}
	metadata, metadataErr := validateSingleFilePluginMetadata(ctx, filePath, parsed)
	if metadataErr != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to validate single-file plugin metadata, keeping existing instance: %s", metadataErr.Error()))
		return
	}
	if conflictErr := m.ensureSingleFilePluginIDAvailable(metadata); conflictErr != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to reload single-file plugin, keeping existing instance: %s", conflictErr.Error()))
		return
	}

	m.unloadSingleFilePluginByPath(ctx, filePath)

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), metadata.Runtime)
	})
	if !exist {
		logger.Error(ctx, fmt.Sprintf("unsupported runtime for single-file plugin: %s", metadata.Runtime))
		return
	}
	if !pluginHost.IsStarted(ctx) {
		if startErr := pluginHost.Start(ctx); startErr != nil {
			logger.Error(ctx, fmt.Sprintf("Failed to start host for single-file plugin: %s", startErr.Error()))
			return
		}
	}

	if err := m.loadHostPlugin(ctx, pluginHost, metadata); err != nil {
		logger.Error(ctx, fmt.Sprintf("Failed to reload single-file plugin: %s", err.Error()))
		return
	}
	logger.Info(ctx, fmt.Sprintf("Successfully reloaded single-file plugin: %s", metadata.GetName(ctx)))
}

func (m *Manager) unloadSingleFilePluginByPath(ctx context.Context, filePath string) {
	directory := filepath.Dir(filePath)
	entry := filepath.Base(filePath)
	logger.Info(ctx, fmt.Sprintf("Unloading single-file plugin by path: %s", filePath))

	for _, instance := range m.pluginInstancesSnapshot() {
		if !IsSingleFilePlugin(instance.Metadata) {
			continue
		}
		if !pluginPathsEqual(instance.Metadata.Directory, directory) {
			continue
		}
		if !strings.EqualFold(instance.Metadata.Entry, entry) {
			continue
		}
		logger.Info(ctx, fmt.Sprintf("Found single-file plugin to unload: %s", instance.Metadata.GetName(ctx)))
		m.UnloadPlugin(ctx, instance)
	}
}
