package plugin

import (
	"context"
	"fmt"
	"wox/common"
)

// PluginCommandHandler handles plugin-to-plugin command requests.
type PluginCommandHandler func(ctx context.Context, request PluginCommandRequest) PluginCommandResult

// PluginCommandRequest identifies a command exposed by another plugin.
type PluginCommandRequest struct {
	PluginId string
	Command  string
	Data     common.ContextData
}

// PluginCommandResult reports whether a plugin command was handled.
type PluginCommandResult struct {
	Handled bool
	Message string
	Data    common.ContextData
}

// InvokePluginCommandAndNotify sends a plugin-to-plugin command and reports failures through the caller API.
func InvokePluginCommandAndNotify(ctx context.Context, api API, request PluginCommandRequest) {
	if api == nil {
		return
	}

	result, err := api.InvokePluginCommand(ctx, request)
	if err != nil {
		api.Log(ctx, LogLevelError, fmt.Sprintf("failed to invoke plugin command: %s", err.Error()))
		api.Notify(ctx, err.Error())
		return
	}
	if !result.Handled {
		message := result.Message
		if message == "" {
			message = "plugin command was not handled"
		}
		api.Log(ctx, LogLevelWarning, message)
		api.Notify(ctx, message)
		return
	}
	if result.Message != "" {
		api.Notify(ctx, result.Message)
	}
}
