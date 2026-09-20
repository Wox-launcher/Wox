package plugin

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"wox/common"
	"wox/util"
)

type pluginToolRegistration struct {
	descriptor   PluginToolDescriptor
	handler      PluginToolHandler
	inputSchema  *compiledPluginToolSchema
	outputSchema *compiledPluginToolSchema
}

type pluginToolCallFrame struct {
	PluginId string
	Name     string
}

type pluginToolCallState struct {
	callerPluginId string
	frames         []pluginToolCallFrame
}

type pluginToolCallStateKey struct{}

type pluginToolActiveCall struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// beginPluginToolCall atomically admits a handler against the unload gate.
func (i *Instance) beginPluginToolCall(ctx context.Context, name string) (context.Context, *pluginToolRegistration, func(), *PluginToolError) {
	i.pluginToolsMu.Lock()
	defer i.pluginToolsMu.Unlock()
	initialized, initErr := i.initStatus()
	if i.pluginToolsStopping || !initialized || initErr != nil || pluginInstanceDisabled(i) {
		return ctx, nil, nil, newPluginToolError(PluginToolErrorPluginUnavailable, "plugin is unavailable")
	}
	registration := i.pluginTools[name]
	if registration == nil {
		return ctx, nil, nil, newPluginToolError(PluginToolErrorNotFound, "tool not found: "+name)
	}
	ctx, cancel := context.WithCancel(ctx)
	call := &pluginToolActiveCall{cancel: cancel, done: make(chan struct{})}
	if i.pluginToolCalls == nil {
		i.pluginToolCalls = make(map[*pluginToolActiveCall]struct{})
	}
	i.pluginToolCalls[call] = struct{}{}
	return ctx, registration, func() {
		cancel()
		i.pluginToolsMu.Lock()
		delete(i.pluginToolCalls, call)
		close(call.done)
		i.pluginToolsMu.Unlock()
	}, nil
}

// stopPluginTools waits for handlers to exit before unload callbacks release resources.
func (i *Instance) stopPluginTools() {
	i.pluginToolsMu.Lock()
	i.pluginToolsStopping = true
	calls := make([]*pluginToolActiveCall, 0, len(i.pluginToolCalls))
	for call := range i.pluginToolCalls {
		calls = append(calls, call)
	}
	i.pluginToolsMu.Unlock()
	for _, call := range calls {
		call.cancel()
	}
	for _, call := range calls {
		<-call.done
	}
}

func (i *Instance) storePluginTool(reg *pluginToolRegistration) *PluginToolError {
	if i == nil {
		return newPluginToolError(PluginToolErrorPluginUnavailable, "plugin instance is nil")
	}
	i.pluginToolsMu.Lock()
	defer i.pluginToolsMu.Unlock()
	if i.pluginToolsStopping {
		return newPluginToolError(PluginToolErrorPluginUnavailable, "plugin is unloading")
	}
	if i.pluginTools == nil {
		i.pluginTools = map[string]*pluginToolRegistration{}
	}
	if _, exists := i.pluginTools[reg.descriptor.Name]; exists {
		return newPluginToolError(PluginToolErrorAlreadyRegistered, fmt.Sprintf("tool already registered: %s", reg.descriptor.Name))
	}
	i.pluginTools[reg.descriptor.Name] = reg
	return nil
}

func (i *Instance) deletePluginTool(name string) {
	if i == nil {
		return
	}
	i.pluginToolsMu.Lock()
	defer i.pluginToolsMu.Unlock()
	delete(i.pluginTools, name)
}

func (i *Instance) snapshotPluginTools() []*pluginToolRegistration {
	if i == nil {
		return nil
	}
	i.pluginToolsMu.RLock()
	defer i.pluginToolsMu.RUnlock()
	if len(i.pluginTools) == 0 {
		return nil
	}
	snapshot := make([]*pluginToolRegistration, 0, len(i.pluginTools))
	for _, registration := range i.pluginTools {
		snapshot = append(snapshot, registration)
	}
	return snapshot
}

func (i *Instance) clearPluginTools() {
	if i == nil {
		return
	}
	i.pluginToolsMu.Lock()
	defer i.pluginToolsMu.Unlock()
	i.pluginTools = nil
}

func (i *Instance) pluginToolsCallable() bool {
	if i == nil {
		return false
	}
	i.pluginToolsMu.RLock()
	defer i.pluginToolsMu.RUnlock()
	initialized, initErr := i.initStatus()
	return !i.pluginToolsStopping && initialized && initErr == nil && !pluginInstanceDisabled(i)
}

func (a *APIImpl) RegisterPluginTool(ctx context.Context, option RegisterPluginToolOption) RegisterPluginToolResult {
	return GetPluginManager().RegisterPluginTool(ctx, a.pluginInstance, option)
}

func (a *APIImpl) UnregisterPluginTool(ctx context.Context, option UnregisterPluginToolOption) UnregisterPluginToolResult {
	return GetPluginManager().UnregisterPluginTool(ctx, a.pluginInstance, option)
}

func (a *APIImpl) ListPluginTools(ctx context.Context, option ListPluginToolsOption) ListPluginToolsResult {
	return GetPluginManager().ListPluginTools(ctx, a.pluginInstance, option)
}

func (a *APIImpl) InvokePluginTool(ctx context.Context, option InvokePluginToolOption) InvokePluginToolResult {
	return GetPluginManager().InvokePluginTool(ctx, a.pluginInstance, option)
}

func (m *Manager) RegisterPluginTool(ctx context.Context, owner *Instance, option RegisterPluginToolOption) RegisterPluginToolResult {
	if owner == nil {
		return RegisterPluginToolResult{Error: newPluginToolError(PluginToolErrorPluginUnavailable, "plugin instance is nil")}
	}
	if err := validatePluginToolName(option.Tool.Name); err != nil {
		return RegisterPluginToolResult{Error: err}
	}
	if err := validatePluginToolDescription(option.Tool.Description); err != nil {
		return RegisterPluginToolResult{Error: err}
	}
	if option.Handler == nil {
		return RegisterPluginToolResult{Error: newPluginToolError(PluginToolErrorInvalidRegistration, "tool handler is required")}
	}

	inputSchema, inputErr := compilePluginToolSchema(option.Tool.InputSchema, true)
	if inputErr != nil {
		return RegisterPluginToolResult{Error: inputErr}
	}
	outputSchema, outputErr := compilePluginToolSchema(option.Tool.OutputSchema, false)
	if outputErr != nil {
		return RegisterPluginToolResult{Error: outputErr}
	}

	registration := &pluginToolRegistration{
		descriptor: PluginToolDescriptor{
			Name:         option.Tool.Name,
			Description:  option.Tool.Description,
			InputSchema:  inputSchema.raw,
			OutputSchema: outputSchema.raw,
			Annotations:  option.Tool.Annotations,
		},
		handler:      option.Handler,
		inputSchema:  inputSchema,
		outputSchema: outputSchema,
	}
	if err := owner.storePluginTool(registration); err != nil {
		return RegisterPluginToolResult{Error: err}
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("registered plugin tool: plugin=%s name=%s", owner.Metadata.Id, option.Tool.Name))
	return RegisterPluginToolResult{}
}

func (m *Manager) UnregisterPluginTool(ctx context.Context, owner *Instance, option UnregisterPluginToolOption) UnregisterPluginToolResult {
	if owner == nil {
		return UnregisterPluginToolResult{Error: newPluginToolError(PluginToolErrorPluginUnavailable, "plugin instance is nil")}
	}
	name := strings.TrimSpace(option.Name)
	if name == "" {
		return UnregisterPluginToolResult{Error: newPluginToolError(PluginToolErrorInvalidRegistration, "tool name is required")}
	}
	owner.deletePluginTool(name)
	util.GetLogger().Info(ctx, fmt.Sprintf("unregistered plugin tool: plugin=%s name=%s", owner.Metadata.Id, name))
	return UnregisterPluginToolResult{}
}

func (m *Manager) ListPluginTools(ctx context.Context, caller *Instance, option ListPluginToolsOption) ListPluginToolsResult {
	filterID := strings.TrimSpace(option.PluginId)
	items := make([]PluginToolListItem, 0)
	for _, instance := range m.pluginInstancesSnapshot() {
		if filterID != "" && !strings.EqualFold(instance.Metadata.Id, filterID) {
			continue
		}
		if !instance.pluginToolsCallable() {
			continue
		}
		pluginName := instance.GetName(ctx)
		for _, registration := range instance.snapshotPluginTools() {
			// Catalog consumers own their maps; mutations must not change the registered contract.
			descriptor := registration.descriptor
			descriptor.InputSchema, _ = copyJSONObject(descriptor.InputSchema)
			descriptor.OutputSchema, _ = copyJSONObject(descriptor.OutputSchema)
			descriptor.Description = instance.Metadata.translate(ctx, common.I18nString(descriptor.Description))
			items = append(items, PluginToolListItem{
				PluginId:   instance.Metadata.Id,
				PluginName: pluginName,
				Tool:       descriptor,
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PluginId != items[j].PluginId {
			return items[i].PluginId < items[j].PluginId
		}
		return items[i].Tool.Name < items[j].Tool.Name
	})
	_ = caller
	return ListPluginToolsResult{Tools: items}
}

func (m *Manager) InvokePluginTool(ctx context.Context, caller *Instance, option InvokePluginToolOption) InvokePluginToolResult {
	if err := ctx.Err(); err != nil {
		return pluginToolResultFromError(pluginToolErrorFromContext(err))
	}
	startedAt := time.Now()
	callID := uuid.NewString()
	ctx = context.WithValue(ctx, pluginToolCallIDLogKey, callID)

	pluginID := strings.TrimSpace(option.PluginId)
	name := strings.TrimSpace(option.Name)
	if pluginID == "" || name == "" {
		return pluginToolResultFromError(newPluginToolError(PluginToolErrorInvalidArguments, "plugin id and tool name are required"))
	}

	target := m.GetPluginInstanceById(pluginID)
	if target == nil || !target.pluginToolsCallable() {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(newPluginToolError(PluginToolErrorPluginUnavailable, fmt.Sprintf("plugin is unavailable: %s", pluginID))))
	}

	callCtx, err := withPluginToolCall(ctx, caller, pluginID, name)
	if err != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(err))
	}
	callCtx, cancel := withPluginToolDeadline(callCtx)
	defer cancel()
	callCtx, registration, finish, admissionErr := target.beginPluginToolCall(callCtx, name)
	if admissionErr != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(admissionErr))
	}
	defer finish()

	arguments, normalizeErr := normalizeJSONObject(option.Arguments)
	if normalizeErr != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(newPluginToolError(PluginToolErrorInvalidArguments, normalizeErr.Error())))
	}
	if schemaErr := validateAgainstSchema(registration.inputSchema.resolved, arguments, PluginToolErrorInvalidArguments); schemaErr != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(schemaErr))
	}

	handlerResult := invokePluginToolHandler(callCtx, registration.handler, arguments)
	if handlerResult.Error != nil {
		if handlerResult.Error.Code == "" {
			handlerResult.Error.Code = PluginToolErrorExecutionFailed
		}
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(handlerResult.Error))
	}

	output, outputErr := normalizeJSONObject(handlerResult.Output)
	if outputErr != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(newPluginToolError(PluginToolErrorInvalidOutput, outputErr.Error())))
	}
	if schemaErr := validateAgainstSchema(registration.outputSchema.resolved, output, PluginToolErrorInvalidOutput); schemaErr != nil {
		return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, pluginToolResultFromError(schemaErr))
	}
	return m.finishPluginToolInvoke(ctx, caller, pluginID, name, startedAt, InvokePluginToolResult{Output: output})
}

// invokePluginToolHandler keeps execution on the caller so unload can drain real work, not just its waiters.
func invokePluginToolHandler(ctx context.Context, handler PluginToolHandler, arguments map[string]any) (result InvokePluginToolHandlerResult) {
	if err := ctx.Err(); err != nil {
		return InvokePluginToolHandlerResult{Error: pluginToolErrorFromContext(err)}
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("plugin tool panicked: %v\n%s", recovered, debug.Stack()))
			result = InvokePluginToolHandlerResult{Error: newPluginToolError(PluginToolErrorExecutionFailed, "plugin tool panicked")}
		}
	}()
	result = handler(ctx, InvokePluginToolHandlerOption{Arguments: arguments})
	if result.Error != nil {
		result.Output = nil
	}
	if ctx.Err() != nil {
		return InvokePluginToolHandlerResult{Error: pluginToolErrorFromContext(ctx.Err())}
	}
	return result
}

func withPluginToolDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(pluginToolDefaultTimeout)
	if existing, ok := ctx.Deadline(); ok && existing.Before(deadline) {
		deadline = existing
	}
	return context.WithDeadline(ctx, deadline)
}

func withPluginToolCall(ctx context.Context, caller *Instance, pluginID string, name string) (context.Context, *PluginToolError) {
	state, _ := ctx.Value(pluginToolCallStateKey{}).(*pluginToolCallState)
	if state == nil {
		state = &pluginToolCallState{}
		if caller != nil {
			state.callerPluginId = caller.Metadata.Id
		}
	} else {
		cloned := *state
		cloned.frames = append([]pluginToolCallFrame{}, state.frames...)
		state = &cloned
	}
	for _, frame := range state.frames {
		if strings.EqualFold(frame.PluginId, pluginID) && frame.Name == name {
			return ctx, newPluginToolError(PluginToolErrorPermissionDenied, fmt.Sprintf("recursive plugin tool call: %s/%s", pluginID, name))
		}
	}
	state.frames = append(state.frames, pluginToolCallFrame{PluginId: pluginID, Name: name})
	return context.WithValue(ctx, pluginToolCallStateKey{}, state), nil
}

func pluginToolErrorFromContext(err error) *PluginToolError {
	if err == nil {
		return newPluginToolError(PluginToolErrorCancelled, "plugin tool call cancelled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newPluginToolError(PluginToolErrorTimeout, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return newPluginToolError(PluginToolErrorCancelled, err.Error())
	}
	return newPluginToolError(PluginToolErrorExecutionFailed, err.Error())
}

// PluginToolErrorFromHost maps host RPC failures onto plugin tool error codes.
func PluginToolErrorFromHost(err error) *PluginToolError {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return newPluginToolError(PluginToolErrorTimeout, err.Error())
	}
	if errors.Is(err, context.Canceled) {
		return newPluginToolError(PluginToolErrorCancelled, err.Error())
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "request timeout"):
		return newPluginToolError(PluginToolErrorTimeout, message)
	case strings.Contains(message, "request canceled"):
		return newPluginToolError(PluginToolErrorCancelled, message)
	case strings.Contains(message, "host is not connected"):
		return newPluginToolError(PluginToolErrorPluginUnavailable, message)
	default:
		return newPluginToolError(PluginToolErrorExecutionFailed, message)
	}
}

func (m *Manager) finishPluginToolInvoke(ctx context.Context, caller *Instance, pluginID string, name string, startedAt time.Time, result InvokePluginToolResult) InvokePluginToolResult {
	callerID := ""
	if caller != nil {
		callerID = caller.Metadata.Id
	}
	callID, _ := ctx.Value(pluginToolCallIDLogKey).(string)
	elapsed := time.Since(startedAt)
	if result.Error != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("plugin tool failed: callId=%s caller=%s target=%s/%s code=%s elapsed=%s", callID, callerID, pluginID, name, result.Error.Code, elapsed))
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("plugin tool succeeded: callId=%s caller=%s target=%s/%s elapsed=%s", callID, callerID, pluginID, name, elapsed))
	}
	return result
}
