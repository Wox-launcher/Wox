package plugin

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"wox/common"
	"wox/database"
	"wox/setting"
)

func TestPluginToolRegisterListInvoke(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	echoSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string"},
		},
		"required": []any{"text"},
	}
	registered := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{
			Name:         "echo_text",
			Description:  "Echo the supplied text",
			InputSchema:  echoSchema,
			OutputSchema: echoSchema,
		},
		Handler: func(_ context.Context, option InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: map[string]any{"text": option.Arguments["text"]}}
		},
	})
	if registered.Error != nil {
		t.Fatalf("register: %v", registered.Error)
	}

	listed := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{})
	if listed.Error != nil || len(listed.Tools) != 1 {
		t.Fatalf("list = %#v", listed)
	}
	if listed.Tools[0].PluginId != "beta" || listed.Tools[0].Tool.Name != "echo_text" {
		t.Fatalf("catalog item = %#v", listed.Tools[0])
	}

	invoked := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{
		PluginId:  "beta",
		Name:      "echo_text",
		Arguments: map[string]any{"text": "hello"},
	})
	if invoked.Error != nil || invoked.Output["text"] != "hello" {
		t.Fatalf("invoke = %#v", invoked)
	}
}

func TestPluginToolRejectsInvalidRegistrationAndDuplicateName(t *testing.T) {
	manager, _, target := newPluginToolTestEnv("alpha", "beta")
	emptyObject := map[string]any{"type": "object"}
	handler := func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
		return InvokePluginToolHandlerResult{Output: map[string]any{}}
	}

	invalidName := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool:    PluginToolDescriptor{Name: "Echo Text", Description: "bad name", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: handler,
	})
	if invalidName.Error == nil || invalidName.Error.Code != PluginToolErrorInvalidRegistration {
		t.Fatalf("invalid name = %#v", invalidName)
	}

	remoteRef := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{
			Name:         "remote_ref",
			Description:  "uses a remote ref",
			InputSchema:  map[string]any{"type": "object", "$ref": "https://example.com/schema.json"},
			OutputSchema: emptyObject,
		},
		Handler: handler,
	})
	if remoteRef.Error == nil || remoteRef.Error.Code != PluginToolErrorInvalidRegistration {
		t.Fatalf("remote ref = %#v", remoteRef)
	}

	first := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool:    PluginToolDescriptor{Name: "ping", Description: "Ping", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: handler,
	})
	if first.Error != nil {
		t.Fatalf("first register: %v", first.Error)
	}
	duplicate := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool:    PluginToolDescriptor{Name: "ping", Description: "Ping again", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: handler,
	})
	if duplicate.Error == nil || duplicate.Error.Code != PluginToolErrorAlreadyRegistered {
		t.Fatalf("duplicate = %#v", duplicate)
	}
}

func TestPluginToolRejectsUnknownInputFieldsAndInvalidOutput(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	var handlerCalls atomic.Int32
	registered := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{
			Name:        "echo_text",
			Description: "Echo the supplied text",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"text": map[string]any{"type": "string"}},
				"required":   []any{"text"},
			},
			OutputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"text": map[string]any{"type": "string"}},
				"required":   []any{"text"},
			},
		},
		Handler: func(_ context.Context, option InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			handlerCalls.Add(1)
			if option.Arguments["forceBadOutput"] == true {
				return InvokePluginToolHandlerResult{Output: map[string]any{}}
			}
			return InvokePluginToolHandlerResult{Output: map[string]any{"text": option.Arguments["text"]}}
		},
	})
	if registered.Error != nil {
		t.Fatalf("register: %v", registered.Error)
	}

	unknownField := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{
		PluginId:  "beta",
		Name:      "echo_text",
		Arguments: map[string]any{"text": "ok", "extra": "nope"},
	})
	if unknownField.Error == nil || unknownField.Error.Code != PluginToolErrorInvalidArguments || handlerCalls.Load() != 0 {
		t.Fatalf("unknown field = %#v calls=%d", unknownField, handlerCalls.Load())
	}

	missingRequired := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{
		PluginId: "beta",
		Name:     "echo_text",
	})
	if missingRequired.Error == nil || missingRequired.Error.Code != PluginToolErrorInvalidArguments {
		t.Fatalf("missing required = %#v", missingRequired)
	}

	manager.UnregisterPluginTool(t.Context(), target, UnregisterPluginToolOption{Name: "echo_text"})
	badOutput := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{
			Name:        "echo_text",
			Description: "Echo the supplied text",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"text": map[string]any{"type": "string"}, "forceBadOutput": map[string]any{"type": "boolean"}},
				"required":   []any{"text"},
			},
			OutputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"text": map[string]any{"type": "string"}},
				"required":   []any{"text"},
			},
		},
		Handler: func(_ context.Context, option InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: map[string]any{}}
		},
	})
	if badOutput.Error != nil {
		t.Fatalf("re-register: %v", badOutput.Error)
	}
	invalidOutput := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{
		PluginId:  "beta",
		Name:      "echo_text",
		Arguments: map[string]any{"text": "ok"},
	})
	if invalidOutput.Error == nil || invalidOutput.Error.Code != PluginToolErrorInvalidOutput {
		t.Fatalf("invalid output = %#v", invalidOutput)
	}
}

func TestPluginToolHiddenUntilInitializedAndAfterDisable(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := &Manager{}
	caller := newReadyPluginToolInstance("alpha", "Alpha")
	target := &Instance{Metadata: Metadata{Id: "beta", Name: common.I18nString("Beta")}}
	target.beginInitCycle()
	manager.appendPluginInstance(caller)
	manager.appendPluginInstance(target)

	emptyObject := map[string]any{"type": "object"}
	registered := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "ready_check", Description: "Ready check", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: map[string]any{}}
		},
	})
	if registered.Error != nil {
		t.Fatalf("register during init: %v", registered.Error)
	}
	if tools := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools; len(tools) != 0 {
		t.Fatalf("tools visible before init: %#v", tools)
	}
	unavailable := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "ready_check"})
	if unavailable.Error == nil || unavailable.Error.Code != PluginToolErrorPluginUnavailable {
		t.Fatalf("invoke before init = %#v", unavailable)
	}

	target.finishInit(true, nil)
	if tools := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools; len(tools) != 1 {
		t.Fatalf("tools after init = %#v", tools)
	}

	target.Setting = setting.NewPluginSetting(setting.NewPluginSettingStore(database.GetDB(), "beta"), nil)
	if err := target.Setting.Disabled.Set(true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if tools := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools; len(tools) != 0 {
		t.Fatalf("tools visible after disable: %#v", tools)
	}
	disabled := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "ready_check"})
	if disabled.Error == nil || disabled.Error.Code != PluginToolErrorPluginUnavailable {
		t.Fatalf("invoke after disable = %#v", disabled)
	}
}

func TestPluginToolClearOnUnloadAndIdempotentUnregister(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	emptyObject := map[string]any{"type": "object"}
	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "ready_check", Description: "Ready check", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: map[string]any{}}
		},
	}); result.Error != nil {
		t.Fatalf("register: %v", result.Error)
	}
	if result := manager.UnregisterPluginTool(t.Context(), target, UnregisterPluginToolOption{Name: "ready_check"}); result.Error != nil {
		t.Fatalf("unregister: %v", result.Error)
	}
	if result := manager.UnregisterPluginTool(t.Context(), target, UnregisterPluginToolOption{Name: "ready_check"}); result.Error != nil {
		t.Fatalf("repeat unregister: %v", result.Error)
	}
	if tools := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools; len(tools) != 0 {
		t.Fatalf("tools after unregister: %#v", tools)
	}

	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "ready_check", Description: "Ready check", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: map[string]any{}}
		},
	}); result.Error != nil {
		t.Fatalf("re-register: %v", result.Error)
	}
	manager.clearRuntimeCallbacks(target)
	if tools := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools; len(tools) != 0 {
		t.Fatalf("tools after clear: %#v", tools)
	}
}

func TestPluginToolBusinessErrorPanicAndRecursion(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	emptyObject := map[string]any{"type": "object"}
	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "fail_now", Description: "Business failure", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Error: newPluginToolError("NOTE_NOT_FOUND", "missing")}
		},
	}); result.Error != nil {
		t.Fatalf("register fail_now: %v", result.Error)
	}
	failed := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "fail_now"})
	if failed.Error == nil || failed.Error.Code != "NOTE_NOT_FOUND" || failed.Output != nil {
		t.Fatalf("business error = %#v", failed)
	}

	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "boom", Description: "Panic", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			panic("boom")
		},
	}); result.Error != nil {
		t.Fatalf("register boom: %v", result.Error)
	}
	panicked := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "boom"})
	if panicked.Error == nil || panicked.Error.Code != PluginToolErrorExecutionFailed || strings.Contains(panicked.Error.Message, "goroutine") {
		t.Fatalf("panic result = %#v", panicked)
	}

	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "loop", Description: "Recursive", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(ctx context.Context, _ InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			nested := manager.InvokePluginTool(ctx, target, InvokePluginToolOption{PluginId: "beta", Name: "loop"})
			return InvokePluginToolHandlerResult{Output: nested.Output, Error: nested.Error}
		},
	}); result.Error != nil {
		t.Fatalf("register loop: %v", result.Error)
	}
	recursive := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "loop"})
	if recursive.Error == nil || recursive.Error.Code != PluginToolErrorPermissionDenied {
		t.Fatalf("recursive = %#v", recursive)
	}
}

func TestPluginToolReentrancyAndLockIsNotHeldDuringHandler(t *testing.T) {
	manager := &Manager{}
	pluginA := newReadyPluginToolInstance("plugin-a", "A")
	pluginB := newReadyPluginToolInstance("plugin-b", "B")
	manager.appendPluginInstance(pluginA)
	manager.appendPluginInstance(pluginB)
	emptyObject := map[string]any{"type": "object"}

	started := make(chan struct{})
	release := make(chan struct{})
	if result := manager.RegisterPluginTool(t.Context(), pluginB, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "hold", Description: "Hold until released", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			close(started)
			<-release
			return InvokePluginToolHandlerResult{Output: map[string]any{}}
		},
	}); result.Error != nil {
		t.Fatalf("register hold: %v", result.Error)
	}
	if result := manager.RegisterPluginTool(t.Context(), pluginA, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "call_b", Description: "Call B", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(ctx context.Context, _ InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			nested := manager.InvokePluginTool(ctx, pluginA, InvokePluginToolOption{PluginId: "plugin-b", Name: "hold"})
			return InvokePluginToolHandlerResult{Output: nested.Output, Error: nested.Error}
		},
	}); result.Error != nil {
		t.Fatalf("register call_b: %v", result.Error)
	}

	var invokeResult InvokePluginToolResult
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		invokeResult = manager.InvokePluginTool(t.Context(), pluginA, InvokePluginToolOption{PluginId: "plugin-a", Name: "call_b"})
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	listed := make(chan ListPluginToolsResult, 1)
	go func() {
		listed <- manager.ListPluginTools(t.Context(), pluginA, ListPluginToolsOption{})
	}()
	select {
	case result := <-listed:
		if result.Error != nil || len(result.Tools) != 2 {
			t.Fatalf("list during handler = %#v", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("list blocked while handler ran")
	}
	close(release)
	wg.Wait()
	if invokeResult.Error != nil {
		t.Fatalf("reentrant invoke: %v", invokeResult.Error)
	}
}

func TestPluginToolStableCatalogSort(t *testing.T) {
	manager := &Manager{}
	caller := newReadyPluginToolInstance("caller", "Caller")
	zeta := newReadyPluginToolInstance("zeta", "Zeta")
	alpha := newReadyPluginToolInstance("alpha", "Alpha")
	manager.appendPluginInstance(caller)
	manager.appendPluginInstance(zeta)
	manager.appendPluginInstance(alpha)
	emptyObject := map[string]any{"type": "object"}
	handler := func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
		return InvokePluginToolHandlerResult{Output: map[string]any{}}
	}
	for _, item := range []struct {
		instance *Instance
		name     string
	}{
		{zeta, "second"},
		{zeta, "first"},
		{alpha, "middle"},
	} {
		if result := manager.RegisterPluginTool(t.Context(), item.instance, RegisterPluginToolOption{
			Tool:    PluginToolDescriptor{Name: item.name, Description: item.name, InputSchema: emptyObject, OutputSchema: emptyObject},
			Handler: handler,
		}); result.Error != nil {
			t.Fatalf("register %s: %v", item.name, result.Error)
		}
	}

	listed := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{})
	got := make([]string, 0, len(listed.Tools))
	for _, item := range listed.Tools {
		got = append(got, item.PluginId+"/"+item.Tool.Name)
	}
	want := []string{"alpha/middle", "zeta/first", "zeta/second"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("catalog order = %v, want %v", got, want)
	}
}

func TestPluginToolTimeout(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	emptyObject := map[string]any{"type": "object"}
	if result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "sleep", Description: "Sleep", InputSchema: emptyObject, OutputSchema: emptyObject},
		Handler: func(ctx context.Context, _ InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			<-ctx.Done()
			return InvokePluginToolHandlerResult{Error: pluginToolErrorFromContext(ctx.Err())}
		},
	}); result.Error != nil {
		t.Fatalf("register: %v", result.Error)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	timedOut := manager.InvokePluginTool(ctx, caller, InvokePluginToolOption{PluginId: "beta", Name: "sleep"})
	if timedOut.Error == nil || timedOut.Error.Code != PluginToolErrorTimeout {
		t.Fatalf("timeout = %#v", timedOut)
	}
}

func newPluginToolTestEnv(callerID, targetID string) (*Manager, *Instance, *Instance) {
	manager := &Manager{}
	caller := newReadyPluginToolInstance(callerID, callerID)
	target := newReadyPluginToolInstance(targetID, targetID)
	manager.appendPluginInstance(caller)
	manager.appendPluginInstance(target)
	return manager, caller, target
}

func newReadyPluginToolInstance(id, name string) *Instance {
	instance := &Instance{Metadata: Metadata{Id: id, Name: common.I18nString(name)}}
	instance.beginInitCycle()
	instance.finishInit(true, nil)
	return instance
}

func TestPluginToolCancelledBeforeDispatch(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	result := invokePluginToolHandler(ctx, func(context.Context, InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
		called = true
		return InvokePluginToolHandlerResult{}
	}, nil)
	if called || result.Error == nil || result.Error.Code != PluginToolErrorCancelled {
		t.Fatalf("cancelled handler ran=%v result=%+v", called, result)
	}
}

func TestPluginToolCatalogOwnsSchemasAndUsesOwnerTranslation(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	target.Metadata.I18n = map[string]map[string]string{"en_US": {"tool_description": "Translated tool"}}
	schema := map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string"}}}
	result := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "echo", Description: "i18n:tool_description", InputSchema: schema, OutputSchema: schema},
		Handler: func(_ context.Context, option InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			return InvokePluginToolHandlerResult{Output: option.Arguments}
		},
	})
	if result.Error != nil {
		t.Fatal(result.Error)
	}
	listed := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{})
	if listed.Tools[0].Tool.Description != "Translated tool" {
		t.Fatalf("description = %q", listed.Tools[0].Tool.Description)
	}
	for _, schema := range []map[string]any{listed.Tools[0].Tool.InputSchema, listed.Tools[0].Tool.OutputSchema} {
		schema["properties"].(map[string]any)["text"].(map[string]any)["type"] = "number"
	}
	again := manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{})
	for _, schema := range []map[string]any{again.Tools[0].Tool.InputSchema, again.Tools[0].Tool.OutputSchema} {
		if got := schema["properties"].(map[string]any)["text"].(map[string]any)["type"]; got != "string" {
			t.Fatalf("catalog mutation changed registered schema: %v", got)
		}
	}
	invoked := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "echo", Arguments: map[string]any{"text": "hello"}})
	if invoked.Error != nil {
		t.Fatal(invoked.Error)
	}
}

func TestPluginToolUnloadCancelsAndDrainsBeforeReleasingResources(t *testing.T) {
	manager, caller, target := newPluginToolTestEnv("alpha", "beta")
	started, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	schema := map[string]any{"type": "object"}
	registered := manager.RegisterPluginTool(t.Context(), target, RegisterPluginToolOption{
		Tool: PluginToolDescriptor{Name: "hold", Description: "Hold", InputSchema: schema, OutputSchema: schema},
		Handler: func(ctx context.Context, _ InvokePluginToolHandlerOption) InvokePluginToolHandlerResult {
			close(started)
			<-ctx.Done()
			close(cancelled)
			<-release
			return InvokePluginToolHandlerResult{}
		},
	})
	if registered.Error != nil {
		t.Fatal(registered.Error)
	}
	resourcesClosed := make(chan struct{})
	target.UnloadCallbacks = append(target.UnloadCallbacks, func(context.Context) { close(resourcesClosed) })
	invoked := make(chan InvokePluginToolResult, 1)
	go func() {
		invoked <- manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "hold"})
	}()
	<-started
	unloaded := make(chan struct{})
	go func() { manager.deactivatePlugin(t.Context(), target); close(unloaded) }()
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("unload did not cancel the handler")
	}
	blocked := manager.InvokePluginTool(t.Context(), caller, InvokePluginToolOption{PluginId: "beta", Name: "hold"})
	if blocked.Error == nil || blocked.Error.Code != PluginToolErrorPluginUnavailable {
		t.Errorf("unload accepted new call: %+v", blocked)
	}
	if len(manager.ListPluginTools(t.Context(), caller, ListPluginToolsOption{}).Tools) != 0 {
		t.Error("unloading tools remain visible")
	}
	select {
	case <-resourcesClosed:
		t.Error("unload released resources before the handler exited")
	default:
	}
	close(release)
	<-unloaded
	if result := <-invoked; result.Error == nil || result.Error.Code != PluginToolErrorCancelled {
		t.Fatalf("invoke result = %+v", result)
	}
	target.beginInitCycle()
	target.finishInit(true, nil)
	if !target.pluginToolsCallable() {
		t.Fatal("new init cycle did not reopen admission")
	}
}
