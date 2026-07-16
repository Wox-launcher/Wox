package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"wox/common"
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
	QueryId             string                     `json:"QueryId"`
	Results             []plugin.QueryResultUI     `json:"Results"`
	Refinements         []plugin.QueryRefinement   `json:"Refinements"`
	Layout              plugin.QueryLayout         `json:"Layout"`
	Context             plugin.QueryContext        `json:"Context"`
	IsFinal             bool                       `json:"IsFinal"` // indicates if this is the final batch of results
	QueryStartTimestamp int64                      `json:"QueryStartTimestamp,omitempty"`
	ActionIconRefs      map[string]common.WoxImage `json:"ActionIconRefs,omitempty"`
}

const (
	queryActionIconRefType       = "iconref"
	queryActionIconRefMinDataLen = 128
)

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

// newRouterMux exposes the same core-owned HTTP API to socket and in-process callers.
func newRouterMux(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()
	for path, callback := range routers {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			defer util.GoRecover(ctx, "http request panic", func(err error) {
				writeErrorResponse(w, err.Error())
			})
			callback(w, r)
		})
	}
	return mux
}

func serveAndWait(ctx context.Context, port int) {
	m = melody.New()
	m.Config.MaxMessageSize = 1024 * 1024 * 10 // 10MB

	mux := newRouterMux(ctx)

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

// serveHTTPOnlyAndWait keeps loopback APIs available without exposing the UI WebSocket in embedded mode.
func serveHTTPOnlyAndWait(ctx context.Context, port int) {
	logger.Info(ctx, fmt.Sprintf("HTTP server start at: http://127.0.0.1:%d", port))
	handler := cors.Default().Handler(newRouterMux(ctx))
	if err := http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), handler); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request WebsocketMsg) error {
	localSink := getLocalUISink()
	if localSink == nil && m == nil {
		logger.Warn(ctx, fmt.Sprintf("UI transport not ready, skipping request: %s", request.Method))
		return fmt.Errorf("UI transport is not initialized")
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
	if localSink != nil {
		return localSink.deliverRequest(request)
	}
	return m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response WebsocketMsg) {
	localSink := getLocalUISink()
	if localSink == nil && m == nil {
		logger.Warn(ctx, fmt.Sprintf("UI transport not ready, skipping response: %s", response.Method))
		return
	}

	responseStart := util.GetSystemTimestamp()
	var queryId string
	var responseCount int
	var isFinal bool
	var isQueryResponse bool
	var queryResponse QueryResponse
	if util.IsDev() {
		queryId, responseCount, isFinal, isQueryResponse = responseUIQueryTimingInfo(response)
		if isQueryResponse {
			queryResponse, _ = response.Data.(QueryResponse)
		}
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
	var broadcastErr error
	if localSink != nil {
		broadcastErr = localSink.deliverResponse(response)
	} else {
		broadcastErr = m.Broadcast(marshalData)
	}
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
	if isQueryResponse {
		logQueryPayloadBreakdown(ctx, queryResponse, len(marshalData))
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
		QueryId:             queryId,
		Results:             response.Results,
		Refinements:         response.Refinements,
		Layout:              response.Layout,
		Context:             response.Context,
		IsFinal:             isFinal,
		QueryStartTimestamp: response.QueryStartTimestamp,
	}
	queryPayload = compactQueryActionIcons(queryPayload)
	if tracker := timetracking.New("response_ui_query_payload"); tracker.Enabled() {
		tracker.SetRawString("queryId", queryId)
		tracker.SetInt("responseCount", len(response.Results))
		tracker.SetBool("isFinal", isFinal)
		tracker.SetInt("actionIconRefCount", len(queryPayload.ActionIconRefs))
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

// compactQueryActionIcons replaces repeated large action icons with response-local references.
func compactQueryActionIcons(response QueryResponse) QueryResponse {
	type iconStats struct {
		count int
		icon  common.WoxImage
	}

	statsByKey := map[string]iconStats{}
	for _, result := range response.Results {
		for _, action := range result.Actions {
			icon := action.Icon
			if !shouldReferenceActionIcon(icon) {
				continue
			}
			key := actionIconReferenceKey(icon)
			stats := statsByKey[key]
			stats.count++
			stats.icon = icon
			statsByKey[key] = stats
		}
	}

	if len(statsByKey) == 0 {
		return response
	}

	refByKey := map[string]string{}
	iconRefs := map[string]common.WoxImage{}
	var results []plugin.QueryResultUI
	for resultIndex, result := range response.Results {
		var actions []plugin.QueryResultActionUI
		for actionIndex, action := range result.Actions {
			icon := action.Icon
			key := actionIconReferenceKey(icon)
			stats, ok := statsByKey[key]
			if !ok || stats.count < 2 {
				continue
			}

			if results == nil {
				results = append([]plugin.QueryResultUI(nil), response.Results...)
			}
			if actions == nil {
				actions = append([]plugin.QueryResultActionUI(nil), result.Actions...)
			}

			refId, exists := refByKey[key]
			if !exists {
				refId = fmt.Sprintf("a%d", len(refByKey)+1)
				refByKey[key] = refId
				iconRefs[refId] = stats.icon
			}
			actions[actionIndex].Icon = common.WoxImage{ImageType: queryActionIconRefType, ImageData: refId}
		}
		if actions != nil {
			results[resultIndex].Actions = actions
		}
	}

	if len(iconRefs) == 0 {
		return response
	}
	response.Results = results
	response.ActionIconRefs = iconRefs
	return response
}

func shouldReferenceActionIcon(icon common.WoxImage) bool {
	return !icon.IsEmpty() && len(icon.ImageData) >= queryActionIconRefMinDataLen
}

func actionIconReferenceKey(icon common.WoxImage) string {
	return string(icon.ImageType) + "\x00" + icon.ImageData
}

func logQueryPayloadBreakdown(ctx context.Context, response QueryResponse, payloadBytes int) {
	tracker := timetracking.New("response_ui_payload_breakdown")
	if !tracker.Enabled() {
		return
	}

	start := util.GetSystemTimestamp()
	resultsBytes := marshalSize(response.Results)
	refinementsBytes := marshalSize(response.Refinements)
	layoutBytes := marshalSize(response.Layout)
	contextBytes := marshalSize(response.Context)
	actionIconRefsBytes := marshalSize(response.ActionIconRefs)
	maxResultBytes := 0
	maxResultIndex := -1
	maxResultTitle := ""
	totalActions := 0
	maxActions := 0
	totalTails := 0
	maxTails := 0
	totalIconChars := 0
	totalActionIconChars := 0
	totalPreviewChars := 0
	for i, result := range response.Results {
		resultBytes := marshalSize(result)
		if resultBytes > maxResultBytes {
			maxResultBytes = resultBytes
			maxResultIndex = i
			maxResultTitle = result.Title
		}
		totalActions += len(result.Actions)
		if len(result.Actions) > maxActions {
			maxActions = len(result.Actions)
		}
		totalTails += len(result.Tails)
		if len(result.Tails) > maxTails {
			maxTails = len(result.Tails)
		}
		totalIconChars += len(result.Icon.ImageData)
		totalPreviewChars += len(result.Preview.PreviewData)
		for _, action := range result.Actions {
			totalActionIconChars += len(action.Icon.ImageData)
		}
	}

	tracker.SetRawString("queryId", response.QueryId)
	tracker.SetBool("isFinal", response.IsFinal)
	tracker.SetInt("payloadBytes", payloadBytes)
	tracker.SetInt("resultCount", len(response.Results))
	tracker.SetInt("resultsBytes", resultsBytes)
	tracker.SetInt("refinementsBytes", refinementsBytes)
	tracker.SetInt("layoutBytes", layoutBytes)
	tracker.SetInt("contextBytes", contextBytes)
	tracker.SetInt("actionIconRefsBytes", actionIconRefsBytes)
	tracker.SetInt("actionIconRefCount", len(response.ActionIconRefs))
	tracker.SetInt("maxResultBytes", maxResultBytes)
	tracker.SetInt("maxResultIndex", maxResultIndex)
	tracker.SetString("maxResultTitle", maxResultTitle)
	tracker.SetInt("totalActions", totalActions)
	tracker.SetInt("maxActions", maxActions)
	tracker.SetInt("totalTails", totalTails)
	tracker.SetInt("maxTails", maxTails)
	tracker.SetInt("totalIconChars", totalIconChars)
	tracker.SetInt("totalActionIconChars", totalActionIconChars)
	tracker.SetInt("totalPreviewChars", totalPreviewChars)
	tracker.SetInt64("costMs", util.GetSystemTimestamp()-start)
	tracker.Log(ctx)
}

func marshalSize(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(data)
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
				hidden := false
				for _, callback := range pluginInstance.DynamicSettingCallbacks {
					newSettingDefinition := callback(ctx, settingDefinition.Value.GetKey())
					if newSettingDefinition.IsEmpty() {
						hidden = true
						continue
					}
					if newSettingDefinition.Value != nil && newSettingDefinition.Type != definition.PluginSettingDefinitionTypeDynamic {
						logger.Debug(ctx, fmt.Sprintf("dynamic setting replaced: %s(%s) -> %s(%s)", settingDefinition.Value.GetKey(), settingDefinition.Type, newSettingDefinition.Value.GetKey(), newSettingDefinition.Type))
						pluginDto.SettingDefinitions[i] = newSettingDefinition
						replaced = true
						break
					}
				}

				if !replaced {
					if !hidden {
						logger.Error(ctx, "dynamic setting not replaced")
					}
					//remove hidden or invalid dynamic setting
					removedKeys = append(removedKeys, settingDefinition.Value.GetKey())
				}
			}
		}

		//remove hidden or invalid dynamic setting
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
