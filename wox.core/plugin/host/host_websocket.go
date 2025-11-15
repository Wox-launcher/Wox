package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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
}

func (w *WebsocketHost) getHostName(ctx context.Context) string {
	return fmt.Sprintf("%s Host Impl", w.host.GetRuntime(ctx))
}

func (w *WebsocketHost) StartHost(ctx context.Context, executablePath string, entry string, envs []string, executableArgs ...string) error {
	port, portErr := util.GetAvailableTcpPort(ctx)
	if portErr != nil {
		return fmt.Errorf("failed to get available port: %w", portErr)
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
		return fmt.Errorf("failed to start host: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("<%s> host pid: %d", w.getHostName(ctx), cmd.Process.Pid))

	time.Sleep(time.Second) // wait for host to start
	w.startWebsocketServer(ctx, port)

	w.hostProcess = cmd.Process
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
}

func (w *WebsocketHost) IsHostStarted(ctx context.Context) bool {
	return w.ws != nil && w.ws.IsConnected()
}

func (w *WebsocketHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("start loading %s plugin, directory: %s", metadata.Name, pluginDirectory))
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
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to unload %s plugin: %s", metadata.Name, unloadPluginErr))
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
		PluginName: metadata.Name,
		Method:     method,
		Type:       JsonRpcTypeRequest,
		Params:     params,
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("<Wox -> %s> inovke plugin <%s> method: %s, request id: %s", w.getHostName(ctx), metadata.Name, method, request.Id))

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
		util.GetLogger().Error(ctx, fmt.Sprintf("invoke %s response timeout, response time: %dms", metadata.Name, util.GetSystemTimestamp()-startTimestamp))
		return "", fmt.Errorf("request timeout, request id: %s", request.Id)
	case response := <-resultChan:
		util.GetLogger().Debug(ctx, fmt.Sprintf("inovke plugin <%s> method: %s finished, response time: %dms", metadata.Name, method, util.GetSystemTimestamp()-startTimestamp))
		if response.Error != "" {
			return "", errors.New(response.Error)
		} else {
			return response.Result, nil
		}
	}
}

func (w *WebsocketHost) startWebsocketServer(ctx context.Context, port int) {
	w.ws = util.NewWebsocketClient(fmt.Sprintf("ws://localhost:%d", port))
	w.ws.OnMessage(ctx, func(data []byte) {
		util.Go(ctx, fmt.Sprintf("<%s> onMessage", w.getHostName(ctx)), func() {
			w.onMessage(string(data))
		})
	})
	connErr := w.ws.Connect(ctx)
	if connErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("<%s> failed to connect to host: %s", w.getHostName(ctx), connErr))
		return
	}
}

func (w *WebsocketHost) onMessage(data string) {
	ctx := util.NewTraceContext()

	if strings.Contains(data, string(JsonRpcTypeSystemLog)) {
		traceId := gjson.Get(data, "TraceId").String()
		level := gjson.Get(data, "Level").String()
		msg := gjson.Get(data, "Message").String()

		logCtx := util.NewComponentContext(util.NewTraceContextWith(traceId), fmt.Sprintf("%s HOST", w.host.GetRuntime(ctx)))
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
				QueryType: plugin.QueryTypeInput,
				QueryText: queryText,
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
	case "Notify":
		message, exist := request.Params["message"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] Notify method must have a message parameter", request.PluginName))
			return
		}
		pluginInstance.API.Notify(ctx, message)
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
		pluginInstance.API.OnSettingChanged(ctx, func(key string, value string) {
			w.invokeMethod(ctx, metadata, "onPluginSettingChange", map[string]string{
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
		pluginInstance.API.OnGetDynamicSetting(ctx, func(key string) definition.PluginSettingDefinitionItem {
			result, err := w.invokeMethod(ctx, metadata, "onGetDynamicSetting", map[string]string{
				"CallbackId": callbackId,
				"Key":        key,
			})
			if err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to get dynamic setting: %s", request.PluginName, err))
				return definition.PluginSettingDefinitionItem{
					Type: definition.PluginSettingDefinitionTypeLabel,
					Value: &definition.PluginSettingValueLabel{
						Content: fmt.Sprintf("failed to get dynamic setting: %s", err),
					},
				}
			}

			// validate the result is a valid definition.PluginSettingDefinitionItem json string
			var setting definition.PluginSettingDefinitionItem
			unmarshalErr := json.Unmarshal([]byte(result.(string)), &setting)
			if unmarshalErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal dynamic setting: %s", request.PluginName, unmarshalErr))
				return definition.PluginSettingDefinitionItem{
					Type: definition.PluginSettingDefinitionTypeLabel,
					Value: &definition.PluginSettingValueLabel{
						Content: fmt.Sprintf("failed to unmarshal dynamic setting: %s", unmarshalErr),
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
		pluginInstance.API.OnDeepLink(ctx, func(arguments map[string]string) {
			args, marshalErr := json.Marshal(arguments)
			if marshalErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal deep link arguments: %s", request.PluginName, marshalErr))
				return
			}

			w.invokeMethod(ctx, metadata, "onDeepLink", map[string]string{
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
		pluginInstance.API.OnUnload(ctx, func() {
			w.invokeMethod(ctx, metadata, "onUnload", map[string]string{
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
		pluginInstance.API.OnMRURestore(ctx, func(mruData plugin.MRUData) (*plugin.QueryResult, error) {
			mruDataJson, marshalErr := json.Marshal(mruData)
			if marshalErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to marshal MRU data: %s", request.PluginName, marshalErr))
				return nil, marshalErr
			}

			result, invokeErr := w.invokeMethod(ctx, metadata, "onMRURestore", map[string]string{
				"CallbackId": callbackId,
				"mruData":    string(mruDataJson),
			})
			if invokeErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to invoke MRU restore callback: %s", request.PluginName, invokeErr))
				return nil, invokeErr
			}

			if result == nil {
				return nil, nil
			}

			// Parse the result back to QueryResult
			var queryResult plugin.QueryResult
			resultStr, ok := result.(string)
			if !ok {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] MRU restore result is not a string", request.PluginName))
				return nil, fmt.Errorf("MRU restore result is not a string")
			}

			unmarshalErr := json.Unmarshal([]byte(resultStr), &queryResult)
			if unmarshalErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unmarshal MRU restore result: %s", request.PluginName, unmarshalErr))
				return nil, unmarshalErr
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
