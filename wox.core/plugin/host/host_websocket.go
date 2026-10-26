package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"wox/network"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

type WebsocketHost struct {
	ws             *util.WebsocketClient
	host           plugin.Host
	requestMap     *util.HashMap[string, chan JsonRpcResponse]
	hostProcess    *hostProcess
	statusLock     sync.RWMutex
	lifecycle      sync.Mutex
	offlineRelease func()
	// Opaque tokens bind nested tool RPCs to a live parent on this host and plugin.
	pluginToolCalls sync.Map

	// Runtime status needs the exact executable and startup error. The previous
	// IsStarted-only state collapsed missing interpreters, process launch
	// failures, and websocket connection failures into the same stopped state.
	executablePath string
	lastStartError string
}

func (w *WebsocketHost) getHostName(ctx context.Context) string {
	return fmt.Sprintf("%s Host Impl", w.host.GetRuntime(ctx))
}

func (w *WebsocketHost) StartHost(ctx context.Context, executablePath string, entry string, envs []string, executableArgs ...string) error {
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	if w.IsHostStarted(ctx) {
		return nil
	}
	if w.offlineRelease != nil {
		w.offlineRelease()
		w.offlineRelease = nil
	}
	ctx, release, policyErr := network.Default.Begin(context.WithoutCancel(ctx))
	if policyErr != nil {
		return policyErr
	}
	started := false
	defer func() {
		if !started {
			release()
		}
	}()
	w.setStartState(executablePath, "")

	port, portErr := util.GetAvailableTcpPort(ctx)
	if portErr != nil {
		startErr := fmt.Errorf("failed to get available port: %w", portErr)
		w.setStartState(executablePath, startErr.Error())
		return startErr
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> starting host on port %d", w.getHostName(ctx), port))
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host path: %s", w.getHostName(ctx), executablePath))
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host entry: %s", w.getHostName(ctx), entry))
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host args: %s", w.getHostName(ctx), strings.Join(executableArgs, " ")))
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host log directory: %s", w.getHostName(ctx), util.GetLocation().GetLogHostsDirectory()))
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> wox pid: %d", w.getHostName(ctx), os.Getpid()))

	var args []string
	args = append(args, executableArgs...)
	args = append(args, entry, fmt.Sprintf("%d", port), util.GetLocation().GetLogHostsDirectory(), fmt.Sprintf("%d", os.Getpid()))

	stderr := &hostStderr{}
	cmd, err := shell.RunWithEnvLifetimeBoundOutput(ctx, executablePath, envs, util.GetLogger().GetWriter(), io.MultiWriter(util.GetLogger().GetWriter(), stderr), args...)
	if err != nil {
		startErr := fmt.Errorf("failed to start host process with %s: %w", executablePath, err)
		w.setStartState(executablePath, startErr.Error())
		return startErr
	}
	stopOffline := context.AfterFunc(ctx, func() { shell.TerminateProcessTree(cmd.Process.Pid) })
	defer func() {
		if !started {
			stopOffline()
		}
	}()
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host pid: %d", w.getHostName(ctx), cmd.Process.Pid))

	// Keep this context alive with the host: WebsocketClient uses it for the connected transport too.
	hostCtx, cancelHost := context.WithCancelCause(ctx)
	process := watchHostProcess(cmd, stderr, cancelHost)
	if connectErr := w.startWebsocketServer(hostCtx, port); connectErr != nil {
		// Snapshot the cause before terminating an otherwise live host on connection timeout.
		startErr := fmt.Errorf("host process started but websocket connection failed: %w", connectErr)
		select {
		case <-process.done:
			startErr = process.exitError()
		default:
			if detail := stderr.detail(); detail != "" {
				startErr = fmt.Errorf("%w: %s", startErr, detail)
			}
		}
		cancelHost(startErr)
		if client := w.websocketClient(); client != nil {
			client.Close(ctx)
		}
		w.terminateHostProcess(ctx, process)
		w.setStartState(executablePath, startErr.Error())
		return startErr
	}

	w.statusLock.Lock()
	w.hostProcess = process
	started = true
	w.offlineRelease = func() { stopOffline(); release() }
	w.statusLock.Unlock()
	w.setStartState(executablePath, "")
	return nil
}

func (w *WebsocketHost) StopHost(ctx context.Context) {
	w.lifecycle.Lock()
	defer w.lifecycle.Unlock()
	if w.offlineRelease != nil {
		w.offlineRelease()
		w.offlineRelease = nil
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> stopping host", w.getHostName(ctx)))
	// Detach state before closing it because in-flight RPCs may observe this host.
	w.statusLock.Lock()
	client := w.ws
	w.ws = nil
	hostProcess := w.hostProcess
	w.hostProcess = nil
	w.statusLock.Unlock()
	if client != nil {
		client.Close(ctx)
	}
	if hostProcess != nil {
		w.terminateHostProcess(ctx, hostProcess)
	}
}

// ProcessID returns the active shared runtime host PID for release diagnostics.
func (w *WebsocketHost) ProcessID() int {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	if w.hostProcess == nil {
		return 0
	}
	return w.hostProcess.cmd.Process.Pid
}

// terminateHostProcess kills a host and waits for Windows to release its file handles.
func (w *WebsocketHost) terminateHostProcess(ctx context.Context, process *hostProcess) {
	if process == nil {
		return
	}

	select {
	case <-process.done:
		return
	default:
	}
	pid := process.cmd.Process.Pid
	// Kill the host tree, not only the host. A Python plugin that shells out to uv leaves
	// uv.exe behind if we only TerminateProcess the interpreter.
	shell.TerminateProcessTree(pid)
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> killed host process tree(%d)", w.getHostName(ctx), pid))

	<-process.done
}

func (w *WebsocketHost) IsHostStarted(ctx context.Context) bool {
	client := w.websocketClient()
	return client != nil && client.IsConnected()
}

// websocketClient snapshots the connection while mode switches can detach it.
func (w *WebsocketHost) websocketClient() *util.WebsocketClient {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()
	return w.ws
}

func (w *WebsocketHost) GetExecutablePath() string {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()

	return w.executablePath
}

func (w *WebsocketHost) GetLastStartError() string {
	w.statusLock.RLock()
	defer w.statusLock.RUnlock()

	return w.lastStartError
}

func (w *WebsocketHost) setStartState(executablePath string, lastStartError string) {
	w.statusLock.Lock()
	defer w.statusLock.Unlock()

	w.executablePath = executablePath
	w.lastStartError = lastStartError
}

func (w *WebsocketHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("start loading %s plugin, directory: %s", metadata.GetName(ctx), pluginDirectory))
	_, loadPluginErr := w.invokeMethod(ctx, metadata, "loadPlugin", map[string]string{
		"PluginId":        metadata.Id,
		"PluginDirectory": pluginDirectory,
		"Entry":           metadata.Entry,
		"EntryMode":       plugin.PluginEntryMode(metadata),
	})
	if loadPluginErr != nil {
		return nil, loadPluginErr
	}

	return NewWebsocketPlugin(metadata, w), nil
}

func (w *WebsocketHost) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	_, unloadPluginErr := w.invokeMethod(ctx, metadata, "unloadPlugin", map[string]string{
		"PluginId": metadata.Id,
	})
	if unloadPluginErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to unload %s plugin: %s", metadata.GetName(ctx), unloadPluginErr))
	}
}

func (w *WebsocketHost) invokeMethod(ctx context.Context, metadata plugin.Metadata, method string, params map[string]string) (result any, err error) {
	// Releasing host-owned callbacks is local housekeeping and never invokes plugin
	// code. It must still run while offline mode blocks ordinary plugin calls.
	if method != "releaseQueryResults" {
		var release func()
		ctx, release, err = network.Default.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer release()
	}
	client := w.websocketClient()
	if client == nil || !client.IsConnected() {
		return "", fmt.Errorf("host is not connected")
	}

	request := JsonRpcRequest{
		SessionId:  util.GetContextSessionId(ctx),
		QueryId:    util.GetContextQueryId(ctx),
		TraceId:    util.GetContextTraceId(ctx),
		Id:         uuid.NewString(),
		PluginId:   metadata.Id,
		PluginName: metadata.GetName(ctx),
		Method:     method,
		Type:       JsonRpcTypeRequest,
		Params:     params,
	}
	if method == "query" || method == "onMRURestore" {
		if register, managed := plugin.QueryResultLifetime(ctx); managed {
			request.ManagedQuery = true
			sessionID, queryID := request.SessionId, request.QueryId
			// Register only after this RPC exits: even canceled/failed queries have
			// already been sent before their cleanup can be sent on this connection.
			defer register(w, func() {
				util.Go(util.NewTraceContext(), "release host query callbacks", func() {
					// A restarted host owns fresh caches; never send an old release to it.
					if w.websocketClient() != client {
						return
					}
					releaseCtx := util.WithQueryIdContext(util.WithSessionContext(util.NewTraceContext(), sessionID), queryID)
					if _, err := w.invokeMethod(releaseCtx, metadata, "releaseQueryResults", nil); err != nil {
						util.GetLogger().Debug(releaseCtx, "failed to release host query callbacks: "+err.Error())
					}
				})
			})
		}
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("<Wox -> %s> inovke plugin <%s> method: %s, request id: %s", w.getHostName(ctx), metadata.GetName(ctx), method, request.Id))

	jsonData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		return "", marshalErr
	}

	// Buffer the response so a cancellation racing with delivery cannot strand
	// the websocket reader after the caller removes this request from the map.
	resultChan := make(chan JsonRpcResponse, 1)
	w.requestMap.Store(request.Id, resultChan)
	defer w.requestMap.Delete(request.Id)

	startTimestamp := util.GetSystemTimestamp()
	sendErr := client.Send(ctx, jsonData)
	if sendErr != nil {
		return "", sendErr
	}

	timer := time.NewTimer(time.Second * 30)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("request canceled, request id: %s: %w", request.Id, context.Cause(ctx))
	case <-timer.C:
		util.GetLogger().Error(ctx, fmt.Sprintf("invoke %s response timeout, response time: %dms", metadata.GetName(ctx), util.GetSystemTimestamp()-startTimestamp))
		return "", fmt.Errorf("request timeout, request id: %s", request.Id)
	case response := <-resultChan:
		util.GetLogger().Debug(ctx, fmt.Sprintf("inovke plugin <%s> method: %s finished, response time: %dms", metadata.GetName(ctx), method, util.GetSystemTimestamp()-startTimestamp))
		if response.Error != "" {
			return "", errors.New(response.Error)
		} else {
			return response.Result, nil
		}
	}
}

func (w *WebsocketHost) decodeHostResult(result any, target any) error {
	if result == nil {
		return fmt.Errorf("result is nil")
	}

	var raw []byte
	switch typed := result.(type) {
	case string:
		raw = []byte(typed)
	default:
		marshalResult, marshalErr := json.Marshal(typed)
		if marshalErr != nil {
			return fmt.Errorf("failed to marshal host result: %w", marshalErr)
		}
		raw = marshalResult
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("failed to unmarshal host result: %w", err)
	}

	return nil
}

func (w *WebsocketHost) startWebsocketServer(ctx context.Context, port int) error {
	client := util.NewWebsocketClient(fmt.Sprintf("ws://127.0.0.1:%d", port))
	w.statusLock.Lock()
	w.ws = client
	w.statusLock.Unlock()
	client.OnMessage(ctx, func(data []byte) {
		util.Go(ctx, fmt.Sprintf("<%s> onMessage", w.getHostName(ctx)), func() {
			w.onMessage(string(data))
		})
	})
	const maxAttempts = 30
	const baseDelay = 200 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		connErr := client.Connect(ctx)
		if connErr == nil {
			return nil
		}
		lastErr = connErr
		if attempt == maxAttempts {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to connect to host after %d attempts: %s", w.getHostName(ctx), maxAttempts, connErr))
			break
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("<%s> failed to connect to host (attempt %d/%d): %s", w.getHostName(ctx), attempt, maxAttempts, connErr))
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(time.Duration(attempt) * baseDelay):
		}
	}

	return fmt.Errorf("failed to connect to host websocket on port %d after %d attempts: %w", port, maxAttempts, lastErr)
}

func (w *WebsocketHost) onMessage(data string) {
	ctx := util.NewTraceContext()
	var envelope struct {
		Id      string
		Method  string
		TraceId string
		Type    JsonRpcType
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		util.GetLogger().Error(ctx, "failed to unmarshal host message envelope: "+err.Error())
		return
	}

	if envelope.Type == JsonRpcTypeSystemLog {
		traceId := gjson.Get(data, "TraceId").String()
		level := gjson.Get(data, "Level").String()
		msg := gjson.Get(data, "Message").String()

		logCtx := util.WithComponentContext(util.NewTraceContextWith(traceId), fmt.Sprintf("%s HOST", w.host.GetRuntime(ctx)))
		if level == "error" {
			util.GetLogger().Error(logCtx, msg)
		}
		if level == "warn" || level == "warning" {
			util.GetLogger().Warn(logCtx, msg)
		}
		if level == "info" {
			util.GetLogger().Info(logCtx, msg)
		}
		if level == "debug" {
			util.GetLogger().Debug(logCtx, msg)
		}
	} else if envelope.Type == JsonRpcTypeRequest {
		var request JsonRpcRequest
		unmarshalErr := json.Unmarshal([]byte(data), &request)
		if unmarshalErr != nil {
			// The envelope remains usable when Params has the wrong wire type.
			w.sendPluginResponse(util.NewTraceContextWith(envelope.TraceId), JsonRpcRequest{Id: envelope.Id, Method: envelope.Method}, nil, fmt.Errorf("failed to unmarshal request: %w", unmarshalErr))
			return
		}

		w.handleRequestFromPlugin(util.NewTraceContextWith(request.TraceId), request)
	} else if envelope.Type == JsonRpcTypeResponse {
		var response JsonRpcResponse
		unmarshalErr := json.Unmarshal([]byte(data), &response)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to unmarshal response: %s", w.getHostName(ctx), unmarshalErr))
			return
		}

		w.handleResponseFromPlugin(util.NewTraceContextWith(response.TraceId), response)
	} else {
		util.GetLogger().Error(ctx, fmt.Sprintf("<%s> unknown message type: %s", w.getHostName(ctx), data))
	}
}

func (w *WebsocketHost) handleResponseFromPlugin(ctx context.Context, response JsonRpcResponse) {
	resultChan, exist := w.requestMap.Load(response.Id)
	if !exist {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to find request id: %s", w.getHostName(ctx), response.Id))
		return
	}

	resultChan <- response
}
