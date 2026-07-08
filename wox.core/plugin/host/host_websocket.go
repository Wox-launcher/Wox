package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/selection"
	"wox/util/shell"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

type WebsocketHost struct {
	ws          *util.WebsocketClient
	host        plugin.Host
	requestMap  *util.HashMap[string, chan JsonRpcResponse]
	hostProcess *os.Process
	statusLock  sync.RWMutex

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

	cmd, err := shell.RunWithEnv(executablePath, envs, args...)
	if err != nil {
		startErr := fmt.Errorf("failed to start host process with %s: %w", executablePath, err)
		w.setStartState(executablePath, startErr.Error())
		return startErr
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host pid: %d", w.getHostName(ctx), cmd.Process.Pid))

	time.Sleep(time.Second) // wait for host to start
	if connectErr := w.startWebsocketServer(ctx, port); connectErr != nil {
		if killErr := cmd.Process.Kill(); killErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to kill disconnected host process(%d): %s", w.getHostName(ctx), cmd.Process.Pid, killErr))
		}
		startErr := fmt.Errorf("host process started but websocket connection failed: %w", connectErr)
		w.setStartState(executablePath, startErr.Error())
		return startErr
	}

	w.hostProcess = cmd.Process
	w.setStartState(executablePath, "")
	return nil
}

func (w *WebsocketHost) StopHost(ctx context.Context) {
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> stopping host", w.getHostName(ctx)))
	if w.hostProcess != nil {
		var pid = w.hostProcess.Pid
		killErr := w.hostProcess.Kill()
		if killErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to kill host process(%d): %s", w.getHostName(ctx), pid, killErr))
		} else {
			util.GetLogger().Info(ctx, fmt.Sprintf("<%s> killed host process(%d)", w.getHostName(ctx), pid))
		}
	}
	// Bug fix: StopHost used to leave the websocket client object in place, so
	// status checks could briefly report a killed host as still connected. Clear
	// local process state immediately; a fresh StartHost creates a new client.
	w.hostProcess = nil
	w.ws = nil
}

func (w *WebsocketHost) IsHostStarted(ctx context.Context) bool {
	return w.ws != nil && w.ws.IsConnected()
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
	if w.ws == nil || !w.ws.IsConnected() {
		return "", fmt.Errorf("host is not connected")
	}

	request := JsonRpcRequest{
		TraceId:    util.GetContextTraceId(ctx),
		Id:         uuid.NewString(),
		PluginId:   metadata.Id,
		PluginName: metadata.GetName(ctx),
		Method:     method,
		Type:       JsonRpcTypeRequest,
		Params:     params,
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("<Wox -> %s> inovke plugin <%s> method: %s, request id: %s", w.getHostName(ctx), metadata.GetName(ctx), method, request.Id))

	jsonData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		return "", marshalErr
	}

	resultChan := make(chan JsonRpcResponse)
	w.requestMap.Store(request.Id, resultChan)
	defer w.requestMap.Delete(request.Id)

	startTimestamp := util.GetSystemTimestamp()
	sendErr := w.ws.Send(ctx, jsonData)
	if sendErr != nil {
		return "", sendErr
	}

	select {
	case <-time.NewTimer(time.Second * 30).C:
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

// isEmptyDynamicSettingHostResult maps host callback placeholders such as None,
// null, "", or {} to the same hidden dynamic setting contract used by Go plugins.
func isEmptyDynamicSettingHostResult(result any) bool {
	if result == nil {
		return true
	}

	var raw []byte
	switch typed := result.(type) {
	case string:
		raw = []byte(typed)
	default:
		marshalResult, marshalErr := json.Marshal(typed)
		if marshalErr != nil {
			return false
		}
		raw = marshalResult
	}

	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null" || trimmed == "{}"
}

func (w *WebsocketHost) startWebsocketServer(ctx context.Context, port int) error {
	w.ws = util.NewWebsocketClient(fmt.Sprintf("ws://127.0.0.1:%d", port))
	w.ws.OnMessage(ctx, func(data []byte) {
		util.Go(ctx, fmt.Sprintf("<%s> onMessage", w.getHostName(ctx)), func() {
			w.onMessage(string(data))
		})
	})
	const maxAttempts = 30
	const baseDelay = 200 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		connErr := w.ws.Connect(ctx)
		if connErr == nil {
			return nil
		}
		lastErr = connErr
		if attempt == maxAttempts {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to connect to host after %d attempts: %s", w.getHostName(ctx), maxAttempts, connErr))
			break
		}
		util.GetLogger().Info(ctx, fmt.Sprintf("<%s> failed to connect to host (attempt %d/%d): %s", w.getHostName(ctx), attempt, maxAttempts, connErr))
		time.Sleep(time.Duration(attempt) * baseDelay)
	}

	return fmt.Errorf("failed to connect to host websocket on port %d after %d attempts: %w", port, maxAttempts, lastErr)
}

func (w *WebsocketHost) onMessage(data string) {
	ctx := util.NewTraceContext()

	if strings.Contains(data, string(JsonRpcTypeSystemLog)) {
		traceId := gjson.Get(data, "TraceId").String()
		level := gjson.Get(data, "Level").String()
		msg := gjson.Get(data, "Message").String()

		logCtx := util.WithComponentContext(util.NewTraceContextWith(traceId), fmt.Sprintf("%s HOST", w.host.GetRuntime(ctx)))
		if level == "error" {
			util.GetLogger().Error(logCtx, msg)
		}
		if level == "info" {
			util.GetLogger().Info(logCtx, msg)
		}
		if level == "debug" {
			util.GetLogger().Debug(logCtx, msg)
		}
	} else if strings.Contains(data, string(JsonRpcTypeRequest)) {
		var request JsonRpcRequest
		unmarshalErr := json.Unmarshal([]byte(data), &request)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to unmarshal request: %s", w.getHostName(ctx), unmarshalErr))
			return
		}

		w.handleRequestFromPlugin(util.NewTraceContextWith(request.TraceId), request)
	} else if strings.Contains(data, string(JsonRpcTypeResponse)) {
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

func (w *WebsocketHost) handleRequestFromPlugin(ctx context.Context, request JsonRpcRequest) {
	if request.Method != "Log" {
		util.GetLogger().Info(ctx, fmt.Sprintf("got request from plugin <%s>, method: %s", request.PluginName, request.Method))
	}

	var pluginInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == request.PluginId {
			pluginInstance = instance
			break
		}
	}
	if pluginInstance == nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to find plugin instance", request.PluginName))
		return
	}

	switch request.Method {
	case "HideApp":
		pluginInstance.API.HideApp(ctx)
		w.sendResponseToHost(ctx, request, "")
	case "ShowApp":
		pluginInstance.API.ShowApp(ctx)
		w.sendResponseToHost(ctx, request, "")
	case "IsVisible":
		result := pluginInstance.API.IsVisible(ctx)
		w.sendResponseToHost(ctx, request, result)
	case "ChangeQuery":
		queryType, exist := request.Params["queryType"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ChangeQuery method must have a queryType parameter", request.PluginName))
			return
		}

		if queryType == plugin.QueryTypeInput {
			queryText, queryTextExist := request.Params["queryText"]
			if !queryTextExist {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ChangeQuery method must have a queryText parameter", request.PluginName))
				return
			}
			pluginInstance.API.ChangeQuery(ctx, common.PlainQuery{
				QueryType:   plugin.QueryTypeInput,
				QueryText:   queryText,
				ContextData: common.UnmarshalContextData(request.Params["queryContextData"]),
			})
		}
		if queryType == plugin.QueryTypeSelection {
			querySelection, querySelectionExist := request.Params["querySelection"]
			if !querySelectionExist {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ChangeQuery method must have a querySelection parameter", request.PluginName))
				return
			}

			var selection selection.Selection
			unmarshalSelectionErr := json.Unmarshal([]byte(querySelection), &selection)
			if unmarshalSelectionErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal selection: %s", request.PluginName, unmarshalSelectionErr))
				return
			}

			pluginInstance.API.ChangeQuery(ctx, common.PlainQuery{
				QueryType:      plugin.QueryTypeSelection,
				QuerySelection: selection,
				ContextData:    common.UnmarshalContextData(request.Params["queryContextData"]),
			})
		}

		w.sendResponseToHost(ctx, request, "")
	case "RefreshQuery":
		// Parse PreserveSelectedIndex parameter (optional, defaults to false)
		preserveSelectedIndex := false
		if preserveSelectedIndexStr, exists := request.Params["preserveSelectedIndex"]; exists {
			preserveSelectedIndex = preserveSelectedIndexStr == "true"
		}

		pluginInstance.API.RefreshQuery(ctx, plugin.RefreshQueryParam{
			PreserveSelectedIndex: preserveSelectedIndex,
		})

		w.sendResponseToHost(ctx, request, "")
	case "Copy":
		var params plugin.CopyParams
		params.Type = plugin.CopyType(request.Params["type"])
		params.Text = request.Params["text"]

		if woxImageStr, exists := request.Params["woxImage"]; exists {
			// wox image is empty, skip unmarshalling
			if strings.TrimSpace(woxImageStr) == "" {
				pluginInstance.API.Copy(ctx, params)
				w.sendResponseToHost(ctx, request, "")
				return
			}

			var woxImage common.WoxImage
			if err := json.Unmarshal([]byte(woxImageStr), &woxImage); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal woxImage: %s", request.PluginName, err))
			} else {
				params.WoxImage = &woxImage
			}
		}

		pluginInstance.API.Copy(ctx, params)
		w.sendResponseToHost(ctx, request, "")
	case "Screenshot":
		var option plugin.ScreenshotOption
		if optionStr, exists := request.Params["option"]; exists && strings.TrimSpace(optionStr) != "" {
			// The public plugin API keeps screenshot options in a single JSON object so new fields
			// can be added without growing the websocket method signature for every SDK runtime.
			if err := json.Unmarshal([]byte(optionStr), &option); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal screenshot option: %s", request.PluginName, err))
				w.sendResponseErrToHost(ctx, request, fmt.Errorf("failed to unmarshal screenshot option: %w", err))
				return
			}
		}

		result := pluginInstance.API.Screenshot(ctx, option)
		w.sendResponseToHost(ctx, request, result)
	case "Notify":
		message, exist := request.Params["message"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] Notify method must have a message parameter", request.PluginName))
			return
		}
		pluginInstance.API.Notify(ctx, message)
		w.sendResponseToHost(ctx, request, "")
	case "PushAttention":
		rawRequest, exist := request.Params["request"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] PushAttention method must have a request parameter", request.PluginName))
			return
		}

		var attentionRequest plugin.PushAttentionRequest
		if err := json.Unmarshal([]byte(rawRequest), &attentionRequest); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal push attention request: %s", request.PluginName, err))
			w.sendResponseErrToHost(ctx, request, fmt.Errorf("failed to unmarshal push attention request: %w", err))
			return
		}

		pluginInstance.API.PushAttention(ctx, attentionRequest)
		w.sendResponseToHost(ctx, request, "")
	case "ShowToolbarMsg":
		rawMsg, exist := request.Params["msg"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ShowToolbarMsg method must have a msg parameter", request.PluginName))
			return
		}

		var msg plugin.ToolbarMsg
		if err := json.Unmarshal([]byte(rawMsg), &msg); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal toolbar msg: %s", request.PluginName, err))
			return
		}

		pluginInstance.API.ShowToolbarMsg(ctx, msg)
		w.sendResponseToHost(ctx, request, "")
	case "ClearToolbarMsg":
		toolbarMsgId, exist := request.Params["toolbarMsgId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ClearToolbarMsg method must have a toolbarMsgId parameter", request.PluginName))
			return
		}
		pluginInstance.API.ClearToolbarMsg(ctx, toolbarMsgId)
		w.sendResponseToHost(ctx, request, "")
	case "Log":
		msg, exist := request.Params["msg"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] Log method must have a msg parameter", request.PluginName))
			return
		}
		level, exist := request.Params["level"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] Log method must have a level parameter", request.PluginName))
			return
		}

		pluginInstance.API.Log(ctx, level, msg)
		w.sendResponseToHost(ctx, request, "")
	case "GetTranslation":
		key, exist := request.Params["key"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] GetTranslation method must have a key parameter", request.PluginName))
			return
		}
		result := pluginInstance.API.GetTranslation(ctx, key)
		w.sendResponseToHost(ctx, request, result)
	case "GetSetting":
		key, exist := request.Params["key"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] GetSetting method must have a key parameter", request.PluginName))
			return
		}

		result := pluginInstance.API.GetSetting(ctx, key)
		w.sendResponseToHost(ctx, request, result)
	case "SaveSetting":
		key, exist := request.Params["key"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] SaveSetting method must have a key parameter", request.PluginName))
			return
		}
		value, valExist := request.Params["value"]
		if !valExist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] SaveSetting method must have a value parameter", request.PluginName))
			return
		}
		isPlatformSpecificStr, isPlatformSpecificExist := request.Params["isPlatformSpecific"]
		if !isPlatformSpecificExist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] SaveSetting method must have a isPlatformSpecific parameter", request.PluginName))
			return
		}
		isPlatformSpecific := strings.ToLower(isPlatformSpecificStr) == "true"

		pluginInstance.API.SaveSetting(ctx, key, value, isPlatformSpecific)
		w.sendResponseToHost(ctx, request, "")
	case "OnPluginSettingChanged":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnSettingChanged method must have a callbackId parameter", request.PluginName))
			return
		}
		metadata := pluginInstance.Metadata
		pluginInstance.API.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
			w.invokeMethod(callbackCtx, metadata, "onPluginSettingChange", map[string]string{
				"CallbackId": callbackId,
				"Key":        key,
				"Value":      value,
			})
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnGetDynamicSetting":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnGetDynamicSetting method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnGetDynamicSetting(ctx, func(callbackCtx context.Context, key string) definition.PluginSettingDefinitionItem {
			result, err := w.invokeMethod(callbackCtx, metadata, "onGetDynamicSetting", map[string]string{
				"CallbackId": callbackId,
				"Key":        key,
			})
			if err != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to get dynamic setting: %s", request.PluginName, err))
				return definition.PluginSettingDefinitionItem{
					Type: definition.PluginSettingDefinitionTypeLabel,
					Value: &definition.PluginSettingValueLabel{
						Content: fmt.Sprintf("failed to get dynamic setting: %s", err),
					},
				}
			}
			if isEmptyDynamicSettingHostResult(result) {
				return definition.PluginSettingDefinitionItem{}
			}

			var setting definition.PluginSettingDefinitionItem
			decodeErr := w.decodeHostResult(result, &setting)
			if decodeErr != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to decode dynamic setting: %s", request.PluginName, decodeErr))
				return definition.PluginSettingDefinitionItem{
					Type: definition.PluginSettingDefinitionTypeLabel,
					Value: &definition.PluginSettingValueLabel{
						Content: fmt.Sprintf("failed to decode dynamic setting: %s", decodeErr),
					},
				}
			}

			return setting
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnDeepLink":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnDeepLink method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnDeepLink(ctx, func(callbackCtx context.Context, arguments map[string]string) {
			args, marshalErr := json.Marshal(arguments)
			if marshalErr != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to marshal deep link arguments: %s", request.PluginName, marshalErr))
				return
			}

			w.invokeMethod(callbackCtx, metadata, "onDeepLink", map[string]string{
				"CallbackId": callbackId,
				"Arguments":  string(args),
			})
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnUnload":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnUnload method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnUnload(ctx, func(callbackCtx context.Context) {
			w.invokeMethod(callbackCtx, metadata, "onUnload", map[string]string{
				"CallbackId": callbackId,
			})
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnEnterPluginQuery":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnEnterPluginQuery method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnEnterPluginQuery(ctx, func(callbackCtx context.Context) {
			w.invokeMethod(callbackCtx, metadata, "onEnterPluginQuery", map[string]string{
				"CallbackId": callbackId,
			})
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnLeavePluginQuery":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnLeavePluginQuery method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnLeavePluginQuery(ctx, func(callbackCtx context.Context) {
			w.invokeMethod(callbackCtx, metadata, "onLeavePluginQuery", map[string]string{
				"CallbackId": callbackId,
			})
		})
		w.sendResponseToHost(ctx, request, "")
	case "OnMRURestore":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] OnMRURestore method must have a callbackId parameter", request.PluginName))
			return
		}

		metadata := pluginInstance.Metadata
		pluginInstance.API.OnMRURestore(ctx, func(callbackCtx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
			mruDataJson, marshalErr := json.Marshal(mruData)
			if marshalErr != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to marshal MRU data: %s", request.PluginName, marshalErr))
				return nil, marshalErr
			}

			result, invokeErr := w.invokeMethod(callbackCtx, metadata, "onMRURestore", map[string]string{
				"CallbackId": callbackId,
				"MRUData":    string(mruDataJson),
			})
			if invokeErr != nil {
				util.GetLogger().Error(callbackCtx, fmt.Sprintf("[%s] failed to invoke MRU restore callback: %s", request.PluginName, invokeErr))
				return nil, invokeErr
			}

			if result == nil {
				return nil, nil
			}

			var queryResult plugin.QueryResult
			decodeErr := w.decodeHostResult(result, &queryResult)
			if decodeErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to decode MRU restore result: %s", request.PluginName, decodeErr))
				return nil, decodeErr
			}

			return &queryResult, nil
		})
		w.sendResponseToHost(ctx, request, "")
	case "RegisterQueryCommands":
		var commands []plugin.MetadataCommand
		unmarshalErr := json.Unmarshal([]byte(request.Params["commands"]), &commands)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal commands: %s", request.PluginName, unmarshalErr))
			return
		}

		pluginInstance.API.RegisterQueryCommands(ctx, commands)
		w.sendResponseToHost(ctx, request, "")
	case "GetUpdatableResult":
		resultId, exist := request.Params["resultId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] GetUpdatableResult method must have a resultId parameter", request.PluginName))
			return
		}

		result := pluginInstance.API.GetUpdatableResult(ctx, resultId)
		if result == nil {
			w.sendResponseToHost(ctx, request, nil)
			return
		}

		// Marshal the result to JSON for sending to plugin host
		resultJson, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal updatable result: %s", request.PluginName, marshalErr))
			return
		}

		w.sendResponseToHost(ctx, request, string(resultJson))
	case "UpdateResult":
		resultStr, exist := request.Params["result"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] UpdateResult method must have a result parameter", request.PluginName))
			return
		}

		var result plugin.UpdatableResult
		unmarshalErr := json.Unmarshal([]byte(resultStr), &result)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal updatable result: %s", request.PluginName, unmarshalErr))
			return
		}

		success := pluginInstance.API.UpdateResult(ctx, result)
		w.sendResponseToHost(ctx, request, success)
	case "PushResults":
		queryStr, exist := request.Params["query"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] PushResults method must have a query parameter", request.PluginName))
			return
		}
		resultsStr, exist := request.Params["results"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] PushResults method must have a results parameter", request.PluginName))
			return
		}

		var query plugin.Query
		unmarshalQueryErr := json.Unmarshal([]byte(queryStr), &query)
		if unmarshalQueryErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal query: %s", request.PluginName, unmarshalQueryErr))
			return
		}

		var results []plugin.QueryResult
		unmarshalResultsErr := json.Unmarshal([]byte(resultsStr), &results)
		if unmarshalResultsErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal results: %s", request.PluginName, unmarshalResultsErr))
			return
		}

		success := pluginInstance.API.PushResults(ctx, query, results)
		w.sendResponseToHost(ctx, request, success)
	case "ExecuteToolbarMsgAction":
		toolbarMsgId, exist := request.Params["toolbarMsgId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ExecuteToolbarMsgAction method must have a toolbarMsgId parameter", request.PluginName))
			return
		}

		actionId, exist := request.Params["actionId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ExecuteToolbarMsgAction method must have an actionId parameter", request.PluginName))
			return
		}

		sessionId := util.GetContextSessionId(ctx)
		executeErr := plugin.GetPluginManager().ExecuteToolbarMsgAction(ctx, sessionId, toolbarMsgId, actionId)
		if executeErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] execute toolbar msg action failed: %s", request.PluginName, executeErr))
			w.sendResponseErrToHost(ctx, request, executeErr)
			return
		}

		w.sendResponseToHost(ctx, request, "")
	case "AIChatStream":
		callbackId, exist := request.Params["callbackId"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] AIChatStream method must have a callbackId parameter", request.PluginName))
			return
		}
		conversationsStr, exist := request.Params["conversations"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] AIChatStream method must have a conversations parameter", request.PluginName))
			return
		}
		optionsStr, exist := request.Params["options"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] AIChatStream method must have a options parameter", request.PluginName))
			return
		}

		var model common.Model
		modelStr, modelExist := request.Params["model"]
		if !modelExist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] AIChatStream method must have a model parameter", request.PluginName))
			return
		}
		unmarshalErr := json.Unmarshal([]byte(modelStr), &model)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal model: %s", request.PluginName, unmarshalErr))
			return
		}

		var conversations []common.Conversation
		unmarshalErr = json.Unmarshal([]byte(conversationsStr), &conversations)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal conversations: %s", request.PluginName, unmarshalErr))
			return
		}

		var options common.ChatOptions
		unmarshalErr = json.Unmarshal([]byte(optionsStr), &options)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal options: %s", request.PluginName, unmarshalErr))
			return
		}

		llmErr := pluginInstance.API.AIChatStream(ctx, model, conversations, options, func(streamResult common.ChatStreamData) {
			w.invokeMethod(ctx, pluginInstance.Metadata, "onLLMStream", map[string]string{
				"CallbackId": callbackId,
				"StreamType": string(streamResult.Status),
				"Data":       streamResult.Data,
				"Reasoning":  streamResult.Reasoning,
				"ToolCalls":  "", // currently we don't stream toolcalls
			})
		})
		if llmErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to start LLM stream: %s", request.PluginName, llmErr))
		}

		w.sendResponseToHost(ctx, request, "")
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

func (w *WebsocketHost) sendResponseToHost(ctx context.Context, request JsonRpcRequest, result any) {
	response := JsonRpcResponse{
		Id:     request.Id,
		Method: request.Method,
		Type:   JsonRpcTypeResponse,
		Result: result,
	}
	responseJson, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal response: %s", request.PluginName, marshalErr))
		return
	}

	sendErr := w.ws.Send(ctx, responseJson)
	if sendErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to send response: %s", request.PluginName, sendErr))
		return
	}
}

func (w *WebsocketHost) sendResponseErrToHost(ctx context.Context, request JsonRpcRequest, responseErr error) {
	response := JsonRpcResponse{
		Id:     request.Id,
		Method: request.Method,
		Type:   JsonRpcTypeResponse,
		Error:  responseErr.Error(),
	}
	responseJson, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal error response: %s", request.PluginName, marshalErr))
		return
	}

	sendErr := w.ws.Send(ctx, responseJson)
	if sendErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to send error response: %s", request.PluginName, sendErr))
		return
	}
}
