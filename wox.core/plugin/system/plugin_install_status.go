package system

import (
	"context"

	"wox/plugin"
)

// showPluginInstallStatus replaces the current operation status in its owning context.
func showPluginInstallStatus(ctx context.Context, api plugin.API, message string, inProgress bool) {
	api.ShowToolbarMsg(ctx, plugin.ToolbarMsg{Id: "plugin-install-status", Title: message, Indeterminate: inProgress, IsPersist: inProgress})
}
