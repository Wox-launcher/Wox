package plugin

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"wox/i18n"
	"wox/util"

	"github.com/samber/lo"
)

func (l *pluginLifecycle) RestartHostForRuntime(ctx context.Context, runtime Runtime, skipPluginIDs []string, progressCallback UninstallProgressCallback) error {
	// Serialize with the host watchdog so a dead host is never restarted twice
	// concurrently (watchdog + settings/uninstall).
	l.hostRestartMu.Lock()
	defer l.hostRestartMu.Unlock()

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), string(runtime))
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	return l.restartHostInternal(ctx, pluginHost, skipPluginIDs, progressCallback)
}

// RefreshHostForRuntime re-detects a Node.js or Python interpreter after the
// user installs or upgrades it, starts the host if it is not running, and
// loads any installed plugins that were skipped because the host failed at
// startup. Script plugins stay loaded and pick up the new interpreter on the
// next query.
func (l *pluginLifecycle) RefreshHostForRuntime(ctx context.Context, runtime Runtime) error {
	if runtime != PLUGIN_RUNTIME_NODEJS && runtime != PLUGIN_RUNTIME_PYTHON {
		return fmt.Errorf("runtime %s does not support refresh from settings", runtime)
	}

	l.hostRestartMu.Lock()
	defer l.hostRestartMu.Unlock()

	if err := l.EnsureHostStarted(ctx, runtime); err != nil {
		// Feature: refresh is a re-check, not a command that must start the host.
		// A still-missing or unsupported interpreter is already shown on the
		// settings card. Returning that start error produced a red banner that
		// Reload immediately cleared, which looked like a flash.
		if status, ok := RuntimeStatusForRegisteredHost(ctx, runtime); ok && runtimeRefreshStillPending(status) {
			return nil
		}
		return err
	}
	return l.loadUnloadedUserPluginsForRuntime(ctx, runtime)
}

// loadUnloadedUserPluginsForRuntime loads installed plugins for one runtime
// that never entered the instance list because the host was missing at startup.
func (l *pluginLifecycle) loadUnloadedUserPluginsForRuntime(ctx context.Context, runtime Runtime) error {
	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), string(runtime))
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	basePluginDirectory := util.GetLocation().GetPluginDirectory()
	pluginDirectories, readErr := os.ReadDir(basePluginDirectory)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil
		}
		return fmt.Errorf("failed to read plugin directory: %w", readErr)
	}

	var loadErrors []string
	for _, entry := range pluginDirectories {
		if entry.Name() == ".DS_Store" || IsReservedUserPluginDirectory(entry.Name()) || !entry.IsDir() {
			continue
		}

		metadata, metadataErr := l.ParseMetadata(ctx, path.Join(basePluginDirectory, entry.Name()))
		if metadataErr != nil {
			continue
		}
		if !strings.EqualFold(metadata.Runtime, string(runtime)) {
			continue
		}
		if l.GetPluginInstanceById(metadata.Id) != nil {
			continue
		}
		if err := l.loadHostPlugin(ctx, pluginHost, metadata); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to load %s plugin %s(%s) after runtime refresh: %s", runtime, metadata.GetName(ctx), metadata.Version, err.Error()))
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %s", metadata.GetName(ctx), err.Error()))
		}
	}
	singleFileMetaDataList, err := l.loadSingleFilePlugins(ctx)
	if err != nil {
		return fmt.Errorf("failed to scan single-file plugins after runtime refresh: %w", err)
	}
	for _, metadata := range singleFileMetaDataList {
		if !strings.EqualFold(metadata.Runtime, string(runtime)) {
			continue
		}
		if l.GetPluginInstanceById(metadata.Id) != nil {
			continue
		}
		if err := l.loadHostPlugin(ctx, pluginHost, metadata); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to load %s plugin %s(%s) after runtime refresh: %s", runtime, metadata.GetName(ctx), metadata.Version, err.Error()))
			loadErrors = append(loadErrors, fmt.Sprintf("%s: %s", metadata.GetName(ctx), err.Error()))
		}
	}
	if len(loadErrors) > 0 {
		return fmt.Errorf("started %s host, but some plugins failed to load: %s", runtime, strings.Join(loadErrors, "; "))
	}
	return nil
}

// restartHostInternal stops and restarts a shared runtime host process, then
// reloads every plugin of that runtime so the shared process returns to a
// consistent state. It is shared by user-triggered restarts (settings/uninstall)
// and the host watchdog, so it must be called with l.hostRestartMu held.
func (l *pluginLifecycle) restartHostInternal(ctx context.Context, pluginHost Host, skipPluginIDs []string, progressCallback UninstallProgressCallback) error {
	runtime := pluginHost.GetRuntime(ctx)

	skipPluginIDSet := make(map[string]struct{}, len(skipPluginIDs))
	for _, pluginID := range skipPluginIDs {
		skipPluginIDSet[pluginID] = struct{}{}
	}

	var reloadMetadataList []Metadata
	for _, instance := range l.pluginInstancesSnapshot() {
		if !strings.EqualFold(instance.Metadata.Runtime, string(runtime)) {
			continue
		}
		if _, shouldSkip := skipPluginIDSet[instance.Metadata.Id]; shouldSkip {
			continue
		}
		reloadMetadataList = append(reloadMetadataList, instance.Metadata)
	}

	// Bug fix: a shared runtime host can keep process-wide native modules loaded even after one
	// plugin unregisters. Restart the host so uninstall can retry with fresh process state.
	pluginHost.Stop(ctx)

	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_uninstall_progress_starting_host"))
	}
	if err := pluginHost.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart %s host: %w", runtime, err)
	}

	// Replace stale runtime instances only after the new host is available, then rebuild the
	// remaining plugins from metadata so the shared runtime returns to a consistent state.
	if l.releasePresentation != nil {
		for _, instance := range l.pluginInstancesSnapshot() {
			if strings.EqualFold(instance.Metadata.Runtime, string(runtime)) {
				l.releasePresentation(instance.Metadata.Id)
			}
		}
	}

	l.removePluginInstances(func(instance *Instance) bool {
		return strings.EqualFold(instance.Metadata.Runtime, string(runtime))
	})

	if len(reloadMetadataList) == 0 {
		return nil
	}

	var reloadErrors []string
	for _, metadata := range reloadMetadataList {
		if progressCallback != nil {
			progressCallback(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_uninstall_progress_restoring_runtime_plugin"), metadata.GetName(ctx)))
		}
		if err := l.loadHostPlugin(ctx, pluginHost, metadata); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to reload %s plugin %s(%s) after host restart: %s", runtime, metadata.GetName(ctx), metadata.Version, err.Error()))
			reloadErrors = append(reloadErrors, fmt.Sprintf("%s(%s)", metadata.GetName(ctx), metadata.Version))
		}
	}

	if len(reloadErrors) > 0 {
		return fmt.Errorf("failed to reload %s host plugins after restart: %s", runtime, strings.Join(reloadErrors, ", "))
	}

	return nil
}

func (l *pluginLifecycle) IsHostStarted(ctx context.Context, runtime Runtime) bool {
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

// EnsureHostStarted makes runtime startup an explicit, reusable preflight for
// install and load paths that need a live plugin host before mutating plugin files.
func (l *pluginLifecycle) EnsureHostStarted(ctx context.Context, runtime Runtime) error {
	if runtime == PLUGIN_RUNTIME_GO || runtime == PLUGIN_RUNTIME_SCRIPT {
		return nil
	}

	pluginHost, exist := lo.Find(AllHosts, func(item Host) bool {
		return strings.EqualFold(string(item.GetRuntime(ctx)), string(runtime))
	})
	if !exist {
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if pluginHost.IsStarted(ctx) {
		return nil
	}

	// Bug fix: install flows previously stopped at a generic "runtime is not started"
	// check, which blocked recovery after users corrected a Node.js/Python path.
	// Starting the host here preserves the explicit preflight while returning the
	// concrete startup or websocket connection error from the host layer.
	if err := pluginHost.Start(ctx); err != nil {
		return fmt.Errorf("failed to start host for runtime %s: %w", runtime, err)
	}

	return nil
}
