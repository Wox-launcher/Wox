package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"wox/i18n"
	"wox/plugin"
	"wox/ui/contract"
	"wox/util"
	"wox/util/shell"
)

// getRuntimeStatuses prepares canonical runtime state for typed UI services and compatibility adapters.
func getRuntimeStatuses(ctx context.Context) []contract.RuntimeStatus {
	instances := plugin.GetPluginManager().GetPluginInstances()

	statuses := make([]contract.RuntimeStatus, 0, len(plugin.AllHosts))
	for _, runtimeHost := range plugin.AllHosts {
		runtime := string(runtimeHost.GetRuntime(ctx))

		var pluginNames []string
		for _, instance := range instances {
			if strings.EqualFold(instance.Metadata.Runtime, runtime) {
				pluginNames = append(pluginNames, instance.GetName(ctx))
			}
		}
		sort.Strings(pluginNames)
		runtimeStatus := runtimeHost.RuntimeStatus(ctx)

		statuses = append(statuses, contract.RuntimeStatus{
			Runtime:           runtime,
			IsStarted:         runtimeHost.IsStarted(ctx),
			HostVersion:       getRuntimeHostVersion(ctx, runtime, runtimeStatus.ExecutablePath),
			StatusCode:        string(runtimeStatus.StatusCode),
			StatusMessage:     localizeRuntimeStatusMessage(ctx, runtime, runtimeStatus),
			ExecutablePath:    runtimeStatus.ExecutablePath,
			LastStartError:    runtimeStatus.LastStartError,
			CanRestart:        runtimeStatus.CanRestart,
			InstallURL:        runtimeStatus.InstallUrl,
			LoadedPluginCount: len(pluginNames),
			LoadedPluginNames: pluginNames,
		})
	}

	sort.SliceStable(statuses, func(i, j int) bool {
		return statuses[i].Runtime < statuses[j].Runtime
	})
	return statuses
}

// localizeRuntimeStatusMessage converts host state into user-facing settings text.
func localizeRuntimeStatusMessage(ctx context.Context, runtime string, status plugin.RuntimeHostStatus) string {
	runtimeName := runtime
	switch strings.ToUpper(runtime) {
	case string(plugin.PLUGIN_RUNTIME_NODEJS):
		runtimeName = "Node.js"
	case string(plugin.PLUGIN_RUNTIME_PYTHON):
		runtimeName = "Python"
	}

	// Keep raw resolver details out of localized settings text. LastStartError
	// remains available separately for true host startup failures.
	switch status.StatusCode {
	case plugin.RuntimeHostStatusRunning:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_running")
	case plugin.RuntimeHostStatusExecutableMissing:
		return strings.ReplaceAll(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_executable_missing_detail"), "{runtime}", runtimeName)
	case plugin.RuntimeHostStatusUnsupportedVersion:
		return strings.ReplaceAll(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_unsupported_version_detail"), "{runtime}", runtimeName)
	case plugin.RuntimeHostStatusStartFailed:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_start_failed_detail")
	case plugin.RuntimeHostStatusStopped:
		return i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_status_stopped")
	default:
		return status.StatusMessage
	}
}

// getRuntimeHostVersion probes the configured executable for supported external runtimes.
func getRuntimeHostVersion(ctx context.Context, runtime string, executablePath string) string {
	if executablePath == "" {
		return ""
	}

	switch strings.ToUpper(runtime) {
	case string(plugin.PLUGIN_RUNTIME_NODEJS):
		return getNodejsHostVersion(ctx, executablePath)
	case string(plugin.PLUGIN_RUNTIME_PYTHON):
		return getPythonHostVersion(ctx, executablePath)
	default:
		return ""
	}
}

// getNodejsHostVersion reads the version reported by the configured Node.js executable.
func getNodejsHostVersion(ctx context.Context, nodePath string) string {
	versionOutput, err := shell.RunOutput(nodePath, "-v")
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get nodejs host version: %s", err))
		return ""
	}
	return strings.TrimSpace(string(versionOutput))
}

// getPythonHostVersion reads the configured Python version with a fallback for unusual launchers.
func getPythonHostVersion(ctx context.Context, pythonPath string) string {
	versionOutput, err := shell.RunOutput(pythonPath, "--version")
	version := strings.TrimSpace(string(versionOutput))
	if err != nil || version == "" {
		versionOutput, err = shell.RunOutput(pythonPath, "-c", "import sys;print(sys.version.split()[0])")
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get python host version: %s", err))
			return ""
		}
		version = strings.TrimSpace(string(versionOutput))
	}
	return strings.TrimPrefix(version, "Python ")
}

// restartRuntimeHost validates settings-owned restart operations before invoking the plugin manager.
func restartRuntimeHost(ctx context.Context, runtimeName string) error {
	runtime := plugin.ConvertToRuntime(runtimeName)
	if runtime != plugin.PLUGIN_RUNTIME_NODEJS && runtime != plugin.PLUGIN_RUNTIME_PYTHON {
		return fmt.Errorf("runtime %s does not support restart from settings", runtime)
	}
	return plugin.GetPluginManager().RestartHostForRuntime(ctx, runtime, nil, nil)
}
