package plugin

import (
	"context"
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
