package host

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"strings"
	"time"
	"wox/plugin"
	"wox/util"
)

type WebsocketHost struct {
	ws          *util.WebsocketClient
	host        plugin.Host
	requestMap  *util.HashMap[string, chan JsonRpcResponse]
	hostProcess *os.Process
}

func (w *WebsocketHost) logIdentity(ctx context.Context) string {
	return fmt.Sprintf("[%s HOST]", w.host.GetRuntime(ctx))
}

func (w *WebsocketHost) StartHost(ctx context.Context, executablePath string, entry string, executableArgs ...string) error {
	port, portErr := util.GetAvailableTcpPort(ctx)
	if portErr != nil {
		return fmt.Errorf("failed to get available port: %w", portErr)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("%s starting host on port %d", w.logIdentity(ctx), port))
	util.GetLogger().Info(ctx, fmt.Sprintf("%s host path: %s", w.logIdentity(ctx), executablePath))
	util.GetLogger().Info(ctx, fmt.Sprintf("%s host entry: %s", w.logIdentity(ctx), entry))
	util.GetLogger().Info(ctx, fmt.Sprintf("%s host args: %s", w.logIdentity(ctx), strings.Join(executableArgs, " ")))
	util.GetLogger().Info(ctx, fmt.Sprintf("%s host log directory: %s", w.logIdentity(ctx), util.GetLocation().GetLogHostsDirectory()))

	var args []string
	args = append(args, executableArgs...)
	args = append(args, entry, fmt.Sprintf("%d", port), util.GetLocation().GetLogHostsDirectory(), fmt.Sprintf("%d", os.Getpid()))

	cmd, err := util.ShellRun(executablePath, args...)
	if err != nil {
		return fmt.Errorf("failed to start host: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("%s host pid: %d", w.logIdentity(ctx), cmd.Process.Pid))

	time.Sleep(time.Second) // wait for host to start
	w.startWebsocketServer(ctx, port)

	w.hostProcess = cmd.Process
	return nil
}

func (w *WebsocketHost) StopHost(ctx context.Context) {
	util.GetLogger().Info(ctx, fmt.Sprintf("%s stopping host", w.logIdentity(ctx)))
	if w.hostProcess != nil {
		var pid = w.hostProcess.Pid
		killErr := w.hostProcess.Kill()
		if killErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to kill host process(%d): %s", w.logIdentity(ctx), pid, killErr))
		} else {
			util.GetLogger().Info(ctx, fmt.Sprintf("%s killed host process(%d)", w.logIdentity(ctx), pid))
		}
	}
}

func (w *WebsocketHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	util.GetLogger().Info(ctx, fmt.Sprintf("[%s] start loading plugin, directory: %s", metadata.Name, pluginDirectory))
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
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to unload plugin: %s", metadata.Name, unloadPluginErr))
	}
}

func (w *WebsocketHost) invokeMethod(ctx context.Context, metadata plugin.Metadata, method string, params map[string]string) (result any, err error) {
	request := JsonRpcRequest{
		Id:         uuid.NewString(),
		PluginId:   metadata.Id,
		PluginName: metadata.Name,
		Method:     method,
		Type:       JsonRpcTypeRequest,
		Params:     params,
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("[%s] inovke method: %s, request id: %s", metadata.Name, method, request.Id))

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
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] response timeout, response time: %dms", metadata.Name, util.GetSystemTimestamp()-startTimestamp))
		return "", fmt.Errorf("request timeout, request id: %s", request.Id)
	case response := <-resultChan:
		util.GetLogger().Info(ctx, fmt.Sprintf("[%s] got response, response time: %dms", metadata.Name, util.GetSystemTimestamp()-startTimestamp))
		if response.Error != "" {
			return "", fmt.Errorf(response.Error)
		} else {
			return response.Result, nil
		}
	}
}

func (w *WebsocketHost) startWebsocketServer(ctx context.Context, port int) {
	w.ws = util.NewWebsocketClient(fmt.Sprintf("ws://localhost:%d", port))
	w.ws.OnMessage(ctx, func(data []byte) {
		util.Go(ctx, fmt.Sprintf("%s onMessage", w.logIdentity(ctx)), func() {
			w.onMessage(util.NewTraceContext(), string(data))
		})
	})
	connErr := w.ws.Connect(ctx)
	if connErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to connect to host: %s", w.logIdentity(ctx), connErr))
		return
	}
}

func (w *WebsocketHost) onMessage(ctx context.Context, data string) {
	//util.GetLogger().Debug(ctx, fmt.Sprintf("%s received message: %s", w.logIdentity(ctx), data))
	if strings.Contains(data, string(JsonRpcTypeRequest)) {
		w.handleRequestFromPlugin(ctx, data)
	} else if strings.Contains(data, string(JsonRpcTypeResponse)) {
		w.handleResponseFromPlugin(ctx, data)
	} else {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s unknown message type: %s", w.logIdentity(ctx), data))
	}
}

func (w *WebsocketHost) handleRequestFromPlugin(ctx context.Context, data string) {
	var request JsonRpcRequest
	unmarshalErr := json.Unmarshal([]byte(data), &request)
	if unmarshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to unmarshal request: %s", w.logIdentity(ctx), unmarshalErr))
		return
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("[%s] got request from plugin, method: %s", request.PluginName, request.Method))

	var pluginInstance *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == request.PluginId {
			pluginInstance = instance
			break
		}
	}
	if pluginInstance.Plugin == nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[%s] failed to find plugin instance", request.PluginName))
		return
	}

	switch request.Method {
	case "HideApp":
		pluginInstance.API.HideApp(ctx)
		w.sendResponse(ctx, request, "")
	case "ShowApp":
		pluginInstance.API.ShowApp(ctx)
		w.sendResponse(ctx, request, "")
	case "ChangeQuery":
		query, exist := request.Params["query"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ChangeQuery method must have a query parameter", request.PluginName))
			return
		}
		pluginInstance.API.ChangeQuery(ctx, query)
		w.sendResponse(ctx, request, "")
	case "ShowMsg":
		title, exist := request.Params["title"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] ShowMsg method must have a title parameter", request.PluginName))
			return
		}
		description := request.Params["description"]
		iconPath := request.Params["iconPath"]
		pluginInstance.API.ShowMsg(ctx, title, description, iconPath)
		w.sendResponse(ctx, request, "")
	case "Log":
		msg, exist := request.Params["msg"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] Log method must have a msg parameter", request.PluginName))
			return
		}
		pluginInstance.API.Log(ctx, msg)
		w.sendResponse(ctx, request, "")
	case "GetTranslation":
		key, exist := request.Params["key"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] GetTranslation method must have a key parameter", request.PluginName))
			return
		}
		result := pluginInstance.API.GetTranslation(ctx, key)
		w.sendResponse(ctx, request, result)
	case "GetSetting":
		key, exist := request.Params["key"]
		if !exist {
			util.GetLogger().Error(ctx, fmt.Sprintf("[%s] GetSetting method must have a key parameter", request.PluginName))
			return
		}

		result := pluginInstance.API.GetSetting(ctx, key)
		w.sendResponse(ctx, request, result)
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
		w.sendResponse(ctx, request, "")
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
		w.sendResponse(ctx, request, "")
	}
}

func (w *WebsocketHost) handleResponseFromPlugin(ctx context.Context, data string) {
	var response JsonRpcResponse
	unmarshalErr := json.Unmarshal([]byte(data), &response)
	if unmarshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to unmarshal response: %s", w.logIdentity(ctx), unmarshalErr))
		return
	}

	resultChan, exist := w.requestMap.Load(response.Id)
	if !exist {
		util.GetLogger().Error(ctx, fmt.Sprintf("%s failed to find request id: %s", w.logIdentity(ctx), response.Id))
		return
	}

	resultChan <- response
}

func (w *WebsocketHost) sendResponse(ctx context.Context, request JsonRpcRequest, result string) {
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
