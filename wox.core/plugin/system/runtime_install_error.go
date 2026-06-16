package system

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
)

func runtimeDisplayName(runtime plugin.Runtime) string {
	switch runtime {
	case plugin.PLUGIN_RUNTIME_NODEJS:
		return "Node.js"
	case plugin.PLUGIN_RUNTIME_PYTHON:
		return "Python"
	default:
		return string(runtime)
	}
}

func formatPluginInstallError(ctx context.Context, api plugin.API, runtime plugin.Runtime, pluginName string, version string, installErr error) string {
	status, hasStatus := plugin.GetPluginManager().RuntimeStatusForRuntime(ctx, runtime)
	if !hasStatus || (status.StatusCode != plugin.RuntimeHostStatusExecutableMissing && status.StatusCode != plugin.RuntimeHostStatusUnsupportedVersion && status.StatusCode != plugin.RuntimeHostStatusStartFailed) {
		return fmt.Sprintf("%s(%s): %s", pluginName, version, installErr.Error())
	}

	runtimeName := runtimeDisplayName(runtime)
	switch status.StatusCode {
	case plugin.RuntimeHostStatusExecutableMissing:
		// Bug fix: runtime-missing install failures used to surface the full wrapped
		// host startup chain. A missing executable is already diagnosed, so present a
		// short localized recovery message and leave the raw chain in logs.
		return fmt.Sprintf(api.GetTranslation(ctx, "i18n:plugin_installer_runtime_missing"), pluginName, version, runtimeName, runtimeName, runtimeName)
	case plugin.RuntimeHostStatusUnsupportedVersion:
		return fmt.Sprintf(api.GetTranslation(ctx, "i18n:plugin_installer_runtime_unsupported"), pluginName, version, runtimeName, runtimeName)
	case plugin.RuntimeHostStatusStartFailed:
		startError := strings.TrimSpace(status.LastStartError)
		if startError == "" {
			startError = strings.TrimSpace(status.StatusMessage)
		}
		if startError == "" {
			startError = installErr.Error()
		}
		return fmt.Sprintf(api.GetTranslation(ctx, "i18n:plugin_installer_runtime_start_failed"), pluginName, version, runtimeName, startError)
	default:
		return fmt.Sprintf("%s(%s): %s", pluginName, version, installErr.Error())
	}
}
