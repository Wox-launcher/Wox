package plugin

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	PluginToolErrorInvalidRegistration = "INVALID_REGISTRATION"
	PluginToolErrorAlreadyRegistered   = "TOOL_ALREADY_REGISTERED"
	PluginToolErrorNotFound            = "TOOL_NOT_FOUND"
	PluginToolErrorPluginUnavailable   = "PLUGIN_UNAVAILABLE"
	PluginToolErrorInvalidArguments    = "INVALID_ARGUMENTS"
	PluginToolErrorPermissionDenied    = "PERMISSION_DENIED"
	PluginToolErrorCancelled           = "CANCELLED"
	PluginToolErrorTimeout             = "TIMEOUT"
	PluginToolErrorExecutionFailed     = "EXECUTION_FAILED"
	PluginToolErrorInvalidOutput       = "INVALID_OUTPUT"

	pluginToolMaxNameLength     = 64
	pluginToolMaxSchemaBytes    = 64 * 1024
	pluginToolMaxSchemaDepth    = 16
	pluginToolMaxArgumentsBytes = 1 * 1024 * 1024
	pluginToolDefaultTimeout    = 30 * time.Second
	pluginToolCallIDLogKey      = "pluginToolCallId"
)

var pluginToolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// PluginToolDescriptor is the serializable catalog entry for one plugin tool.
type PluginToolDescriptor struct {
	Name         string
	Description  string
	InputSchema  map[string]any
	OutputSchema map[string]any
	Annotations  PluginToolAnnotations
}

// PluginToolAnnotations describe side effects. They are hints, not permissions.
type PluginToolAnnotations struct {
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	RequiresUI  bool
}

// PluginToolError is the structured failure returned to tool callers.
type PluginToolError struct {
	Code    string
	Message string
}

func (e *PluginToolError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// PluginToolHandler executes a registered tool. It must not assume it holds any registry lock.
type PluginToolHandler func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult

type RegisterPluginToolOption struct {
	Tool    PluginToolDescriptor
	Handler PluginToolHandler
}

type RegisterPluginToolResult struct {
	Error *PluginToolError
}

type UnregisterPluginToolOption struct {
	Name string
}

type UnregisterPluginToolResult struct {
	Error *PluginToolError
}

type ListPluginToolsOption struct {
	PluginId string
}

type ListPluginToolsResult struct {
	Tools []PluginToolListItem
	Error *PluginToolError
}

// PluginToolListItem is one catalog row, including the owning plugin identity.
type PluginToolListItem struct {
	PluginId   string
	PluginName string
	Tool       PluginToolDescriptor
}

type InvokePluginToolOption struct {
	PluginId  string
	Name      string
	Arguments map[string]any
}

type InvokePluginToolResult struct {
	Output map[string]any
	Error  *PluginToolError
}

type InvokePluginToolHandlerOption struct {
	Arguments map[string]any
}

type InvokePluginToolHandlerResult struct {
	Output map[string]any
	Error  *PluginToolError
}

func newPluginToolError(code string, message string) *PluginToolError {
	return &PluginToolError{Code: code, Message: message}
}

func pluginToolResultFromError(err *PluginToolError) InvokePluginToolResult {
	return InvokePluginToolResult{Error: err}
}

func validatePluginToolName(name string) *PluginToolError {
	if !pluginToolNamePattern.MatchString(name) || len(name) > pluginToolMaxNameLength {
		return newPluginToolError(PluginToolErrorInvalidRegistration, "tool name must be snake_case ASCII starting with a letter, at most 64 characters")
	}
	return nil
}

func validatePluginToolDescription(description string) *PluginToolError {
	if strings.TrimSpace(description) == "" {
		return newPluginToolError(PluginToolErrorInvalidRegistration, "tool description is required")
	}
	return nil
}

// InvokePluginToolAndNotify invokes a tool and reports failures through the caller API.
func InvokePluginToolAndNotify(ctx context.Context, api API, option InvokePluginToolOption) InvokePluginToolResult {
	if api == nil {
		return pluginToolResultFromError(newPluginToolError(PluginToolErrorExecutionFailed, "plugin API is not initialized"))
	}
	result := api.InvokePluginTool(ctx, option)
	if result.Error != nil {
		api.Log(ctx, LogLevelError, fmt.Sprintf("failed to invoke plugin tool %s/%s: %s", option.PluginId, option.Name, result.Error.Error()))
		api.Notify(ctx, result.Error.Message)
	}
	return result
}
