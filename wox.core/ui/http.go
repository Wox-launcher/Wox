package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"wox/plugin"
	"wox/setting/definition"
	"wox/ui/dto"
	"wox/util"
	"wox/util/timetracking"

	"github.com/google/uuid"
	"github.com/olahol/melody"
	"github.com/rs/cors"
	"github.com/samber/lo"
)

var m *melody.Melody

type websocketMsgType string

const (
	WebsocketMsgTypeRequest  websocketMsgType = "WebsocketMsgTypeRequest"
	WebsocketMsgTypeResponse websocketMsgType = "WebsocketMsgTypeResponse"
)

type WebsocketMsg struct {
	RequestId     string // unique id for each request
	TraceId       string // trace id between ui and wox, used for logging
	SessionId     string // ui session id for isolating messages
	Type          websocketMsgType
	Method        string
	Success       bool
	Data          any
	SendTimestamp int64 // timestamp when message is sent (milliseconds since epoch)
}

type QueryResponse struct {
	QueryId     string                   `json:"QueryId"`
	Results     []plugin.QueryResultUI   `json:"Results"`
	Refinements []plugin.QueryRefinement `json:"Refinements"`
	Layout      plugin.QueryLayout       `json:"Layout"`
	Context     plugin.QueryContext      `json:"Context"`
	IsFinal     bool                     `json:"IsFinal"` // indicates if this is the final batch of results
}

type QueryCompletionHintPayload struct {
	QueryId        string                      `json:"QueryId"`
	CompletionHint *plugin.QueryCompletionHint `json:"CompletionHint,omitempty"`
}

type RestResponse struct {
	Success bool
	Message string
	Data    any
}

func writeSuccessResponse(w http.ResponseWriter, data any) {
	d, marshalErr := json.Marshal(RestResponse{
		Success: true,
		Message: "",
		Data:    data,
	})
	if marshalErr != nil {
		writeErrorResponse(w, marshalErr.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(d)
}

func writeErrorResponse(w http.ResponseWriter, errMsg string) {
	d, _ := json.Marshal(RestResponse{
		Success: false,
		Message: errMsg,
		Data:    "",
	})

	w.Header().Set("Content-Type", "application/json")
	w.Write(d)
}

func serveAndWait(ctx context.Context, port int) {
	m = melody.New()
	m.Config.MaxMessageSize = 1024 * 1024 * 10 // 10MB

	mux := http.NewServeMux()

	for path, callback := range routers {
		//add panic handler
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			defer util.GoRecover(ctx, "http request panic", func(err error) {
				writeErrorResponse(w, err.Error())
			})

			callback(w, r)
		})
	}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		receivedAt := util.GetSystemTimestamp()
		ctxNew := util.NewTraceContext()
		msgText := string(msg)

		if strings.Contains(msgText, string(WebsocketMsgTypeRequest)) {
			var request WebsocketMsg
			unmarshalStart := util.GetSystemTimestamp()
			unmarshalErr := json.Unmarshal(msg, &request)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
				return
			}
			requestCtx := util.WithSessionContext(
				util.NewTraceContextWith(request.TraceId),
				request.SessionId,
			)
			if request.Method == "Query" {
				tracker := timetracking.New("websocket_request_unmarshal")
				if tracker.Enabled() {
					tracker.SetRawString("queryId", websocketMsgStringParam(request, "queryId"))
					tracker.SetRawString("method", request.Method)
					tracker.SetRawString("queryType", websocketMsgStringParam(request, "queryType"))
					tracker.SetString("queryText", websocketMsgStringParam(request, "queryText"))
					tracker.SetInt("payloadBytes", len(msg))
					tracker.SetInt64("costMs", util.GetSystemTimestamp()-unmarshalStart)
					tracker.SetInt64("sinceReceiveMs", util.GetSystemTimestamp()-receivedAt)
					tracker.Log(requestCtx)
				}
			}
			dispatchStart := util.GetSystemTimestamp()
			util.Go(ctxNew, "handle ui query", func() {
				if request.Method == "Query" {
					tracker := timetracking.New("websocket_request_handler_start")
					if tracker.Enabled() {
						tracker.SetRawString("queryId", websocketMsgStringParam(request, "queryId"))
						tracker.SetRawString("method", request.Method)
						tracker.SetInt64("queuedMs", util.GetSystemTimestamp()-dispatchStart)
						tracker.SetInt64("sinceReceiveMs", util.GetSystemTimestamp()-receivedAt)
						tracker.Log(requestCtx)
					}
				}
				onUIWebsocketRequest(requestCtx, request)
			})
		} else if strings.Contains(msgText, string(WebsocketMsgTypeResponse)) {
			var response WebsocketMsg
			unmarshalErr := json.Unmarshal(msg, &response)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket response: %s", unmarshalErr.Error()))
				return
			}
			responseCtx := util.WithSessionContext(
				context.WithValue(ctxNew, util.ContextKeyTraceId, response.TraceId),
				response.SessionId,
			)
			util.Go(ctxNew, "handle ui response", func() {
				onUIWebsocketResponse(responseCtx, response)
			})
		} else {
			logger.Error(ctxNew, fmt.Sprintf("unknown websocket msg: %s", string(msg)))
		}
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://127.0.0.1:%d", port))
	handler := cors.Default().Handler(mux)
	err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), handler)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request WebsocketMsg) error {
	// Check if melody websocket server is initialized
	if m == nil {
		logger.Warn(ctx, fmt.Sprintf("websocket server not ready, skipping UI request: %s", request.Method))
		return fmt.Errorf("websocket server not initialized")
	}

	request.Type = WebsocketMsgTypeRequest
	request.Success = true
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return marshalErr
	}

	jsonData, _ := json.Marshal(request.Data)

	// some messages are too frequent, skip logging to avoid performance issue
	if request.Method != "UpdateResult" && request.Method != "ShowToolbarMsg" {
		util.GetLogger().Debug(ctx, fmt.Sprintf("[Wox -> UI] %s: %s", request.Method, jsonData))
	}
	return m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response WebsocketMsg) {
	// Check if melody websocket server is initialized
	if m == nil {
		logger.Warn(ctx, fmt.Sprintf("websocket server not ready, skipping UI response: %s", response.Method))
		return
	}

	responseStart := util.GetSystemTimestamp()
	var queryId string
	var responseCount int
	var isFinal bool
	var isQueryResponse bool
	if util.IsDev() {
		queryId, responseCount, isFinal, isQueryResponse = responseUIQueryTimingInfo(response)
	}
	response.Type = WebsocketMsgTypeResponse
	marshalStart := util.GetSystemTimestamp()
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	marshalCost := util.GetSystemTimestamp() - marshalStart
	if isQueryResponse {
		tracker := timetracking.New("response_ui_marshal")
		if tracker.Enabled() {
			tracker.SetRawString("queryId", queryId)
			tracker.SetInt("responseCount", responseCount)
			tracker.SetBool("isFinal", isFinal)
			tracker.SetInt("payloadBytes", len(marshalData))
			tracker.SetInt64("costMs", marshalCost)
			tracker.Log(ctx)
		}
	}
	broadcastStart := util.GetSystemTimestamp()
	broadcastErr := m.Broadcast(marshalData)
	broadcastCost := util.GetSystemTimestamp() - broadcastStart
	if isQueryResponse {
		tracker := timetracking.New("response_ui_broadcast")
		if tracker.Enabled() {
			tracker.SetRawString("queryId", queryId)
			tracker.SetInt("responseCount", responseCount)
			tracker.SetBool("isFinal", isFinal)
			tracker.SetInt("payloadBytes", len(marshalData))
			tracker.SetInt64("costMs", broadcastCost)
			tracker.SetInt64("totalMs", util.GetSystemTimestamp()-responseStart)
			tracker.Log(ctx)
		}
	}
	if broadcastErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to broadcast websocket response: %s", broadcastErr.Error()))
	}
}

func responseUISuccessWithData(ctx context.Context, request WebsocketMsg, data any) {
	responseUI(ctx, WebsocketMsg{
		RequestId: request.RequestId,
		TraceId:   util.GetContextTraceId(ctx),
		SessionId: request.SessionId,
		Type:      WebsocketMsgTypeResponse,
		Method:    request.Method,
		Success:   true,
		Data:      data,
	})
}

func responseUIQueryResults(ctx context.Context, request WebsocketMsg, queryId string, results []plugin.QueryResultUI, isFinal bool) {
	responseUIQueryResponse(ctx, request, queryId, plugin.QueryResponseUI{Results: results}, isFinal)
}

func responseUIQueryResponse(ctx context.Context, request WebsocketMsg, queryId string, response plugin.QueryResponseUI, isFinal bool) {
	payloadStart := util.GetSystemTimestamp()
	sendTimestamp := util.GetSystemTimestamp()
	queryPayload := QueryResponse{
		QueryId:     queryId,
		Results:     response.Results,
		Refinements: response.Refinements,
		Layout:      response.Layout,
		Context:     response.Context,
		IsFinal:     isFinal,
	}
	if tracker := timetracking.New("response_ui_query_payload"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetInt("responseCount", len(response.Results))
		tracker.SetBool("isFinal", isFinal)
		tracker.SetInt64("buildCostMs", util.GetSystemTimestamp()-payloadStart)
		tracker.SetInt64("sendTimestamp", sendTimestamp)
		tracker.Log(ctx)
	}
	responseUI(ctx, WebsocketMsg{
		RequestId:     request.RequestId,
		TraceId:       util.GetContextTraceId(ctx),
		SessionId:     request.SessionId,
		Type:          WebsocketMsgTypeResponse,
		Method:        request.Method,
		Success:       true,
		SendTimestamp: sendTimestamp, // Only set timestamp for Query responses
		Data:          queryPayload,
	})
}

// websocketMsgStringParam reads lightweight query diagnostics from websocket data before the normal typed handler parses it.
func websocketMsgStringParam(msg WebsocketMsg, key string) string {
	dataMap, ok := msg.Data.(map[string]any)
	if !ok {
		return ""
	}
	if value, exists := dataMap[key]; exists {
		return fmt.Sprint(value)
	}
	if len(key) > 0 {
		upperKey := strings.ToUpper(key[:1]) + key[1:]
		if value, exists := dataMap[upperKey]; exists {
			return fmt.Sprint(value)
		}
	}
	return ""
}

// responseUIQueryTimingInfo extracts query response dimensions so generic websocket response logging can stay query-scoped.
func responseUIQueryTimingInfo(response WebsocketMsg) (queryId string, responseCount int, isFinal bool, ok bool) {
	if response.Method != "Query" {
		return "", 0, false, false
	}
	queryResponse, ok := response.Data.(QueryResponse)
	if !ok {
		return "", 0, false, false
	}
	return queryResponse.QueryId, len(queryResponse.Results), queryResponse.IsFinal, true
}

func responseUIQueryCompletionHint(ctx context.Context, request WebsocketMsg, queryId string, hint *plugin.QueryCompletionHint) {
	if hint == nil {
		return
	}

	responseUI(ctx, WebsocketMsg{
		RequestId:     uuid.NewString(),
		TraceId:       util.GetContextTraceId(ctx),
		SessionId:     request.SessionId,
		Type:          WebsocketMsgTypeResponse,
		Method:        "QueryCompletionHint",
		Success:       true,
		SendTimestamp: util.GetSystemTimestamp(),
		Data: QueryCompletionHintPayload{
			QueryId:        queryId,
			CompletionHint: hint,
		},
	})
}

func responseUISuccess(ctx context.Context, request WebsocketMsg) {
	responseUISuccessWithData(ctx, request, nil)
}

func responseUIError(ctx context.Context, request WebsocketMsg, errMsg string) {
	responseUI(ctx, WebsocketMsg{
		RequestId: request.RequestId,
		TraceId:   util.GetContextTraceId(ctx),
		SessionId: request.SessionId,
		Type:      WebsocketMsgTypeResponse,
		Method:    request.Method,
		Success:   false,
		Data:      errMsg,
	})
}

func convertPluginDto(ctx context.Context, pluginDto dto.PluginDto, pluginInstance *plugin.Instance) dto.PluginDto {
	if pluginInstance != nil {
		logger.Debug(ctx, fmt.Sprintf("get plugin setting: %s", pluginInstance.GetName(ctx)))
		pluginDto.PluginDirectory = pluginInstance.PluginDirectory
		pluginDto.SettingDefinitions = lo.Filter(pluginInstance.Metadata.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
			return !lo.Contains(item.DisabledInPlatforms, util.GetCurrentPlatform())
		})

		// replace dynamic setting definition
		var removedKeys []string
		for i, settingDefinition := range pluginDto.SettingDefinitions {
			if settingDefinition.Type == definition.PluginSettingDefinitionTypeDynamic {
				replaced := false
				for _, callback := range pluginInstance.DynamicSettingCallbacks {
					newSettingDefinition := callback(ctx, settingDefinition.Value.GetKey())
					if newSettingDefinition.Value != nil && newSettingDefinition.Type != definition.PluginSettingDefinitionTypeDynamic {
						logger.Debug(ctx, fmt.Sprintf("dynamic setting replaced: %s(%s) -> %s(%s)", settingDefinition.Value.GetKey(), settingDefinition.Type, newSettingDefinition.Value.GetKey(), newSettingDefinition.Type))
						pluginDto.SettingDefinitions[i] = newSettingDefinition
						replaced = true
						break
					}
				}

				if !replaced {
					logger.Error(ctx, "dynamic setting not replaced")
					//remove invalid dynamic setting
					removedKeys = append(removedKeys, settingDefinition.Value.GetKey())
				}
			}
		}

		//remove invalid dynamic setting
		pluginDto.SettingDefinitions = lo.Filter(pluginDto.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
			if item.Value == nil {
				return true
			}

			return !lo.Contains(removedKeys, item.Value.GetKey())
		})

		//translate setting definition labels
		for i := range pluginDto.SettingDefinitions {
			if pluginDto.SettingDefinitions[i].Value != nil {
				pluginDto.SettingDefinitions[i].Value = pluginDto.SettingDefinitions[i].Value.Translate(pluginInstance.API.GetTranslation)
			}
		}

		var nonDynamicSettings = make(map[string]string)
		for _, item := range pluginDto.SettingDefinitions {
			if item.Value != nil {
				settingValue := pluginInstance.API.GetSetting(ctx, item.Value.GetKey())
				nonDynamicSettings[item.Value.GetKey()] = settingValue
			}
		}
		pluginDto.Setting = dto.PluginSettingDto{
			Disabled:        pluginInstance.Setting.Disabled.Get(),
			TriggerKeywords: pluginInstance.Setting.TriggerKeywords.Get(),
			//only return user pre-defined settings
			Settings: nonDynamicSettings,
		}
		pluginDto.Features = pluginInstance.Metadata.Features
		pluginDto.TriggerKeywords = pluginInstance.GetTriggerKeywords()

		pluginDto.Name = pluginInstance.GetName(ctx)
		pluginDto.Description = pluginInstance.GetDescription(ctx)
		pluginDto.Commands = pluginInstance.GetQueryCommands()
	}

	return pluginDto
}
