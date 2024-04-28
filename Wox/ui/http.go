package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olahol/melody"
	"github.com/rs/cors"
	"github.com/samber/lo"
	"net/http"
	"strings"
	"wox/plugin"
	"wox/setting/definition"
	"wox/ui/dto"
	"wox/util"
)

var m *melody.Melody

type websocketMsgType string

const (
	WebsocketMsgTypeRequest  websocketMsgType = "WebsocketMsgTypeRequest"
	WebsocketMsgTypeResponse websocketMsgType = "WebsocketMsgTypeResponse"
)

type WebsocketMsg struct {
	RequestId string // unique id for each request
	TraceId   string // trace id between ui and wox, used for logging
	Type      websocketMsgType
	Method    string
	Success   bool
	Data      any
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
	m.Config.MessageBufferSize = 1024 * 1024   // 1MB

	mux := http.NewServeMux()

	for path, callback := range routers {
		mux.HandleFunc(path, callback)
	}

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()

		if strings.Contains(string(msg), string(WebsocketMsgTypeRequest)) {
			var request WebsocketMsg
			unmarshalErr := json.Unmarshal(msg, &request)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
				return
			}
			util.Go(ctxNew, "handle ui query", func() {
				traceCtx := context.WithValue(ctxNew, util.ContextKeyTraceId, request.TraceId)
				onUIRequest(traceCtx, request)
			})
		} else if strings.Contains(string(msg), string(WebsocketMsgTypeResponse)) {
			var response WebsocketMsg
			unmarshalErr := json.Unmarshal(msg, &response)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket response: %s", unmarshalErr.Error()))
				return
			}
			util.Go(ctxNew, "handle ui response", func() {
				traceCtx := context.WithValue(ctxNew, util.ContextKeyTraceId, response.TraceId)
				onUIResponse(traceCtx, response)
			})
		} else {
			logger.Error(ctxNew, fmt.Sprintf("unknown websocket msg: %s", string(msg)))
		}
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	handler := cors.Default().Handler(mux)
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), handler)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request WebsocketMsg) error {
	request.Type = WebsocketMsgTypeRequest
	request.Success = true
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return marshalErr
	}

	jsonData, _ := json.Marshal(request.Data)
	util.GetLogger().Info(ctx, fmt.Sprintf("[Wox -> UI] %s: %s", request.Method, jsonData))
	return m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response WebsocketMsg) {
	response.Type = WebsocketMsgTypeResponse
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func responseUISuccessWithData(ctx context.Context, request WebsocketMsg, data any) {
	responseUI(ctx, WebsocketMsg{
		RequestId: request.RequestId,
		TraceId:   util.GetContextTraceId(ctx),
		Type:      WebsocketMsgTypeResponse,
		Method:    request.Method,
		Success:   true,
		Data:      data,
	})
}

func responseUISuccess(ctx context.Context, request WebsocketMsg) {
	responseUISuccessWithData(ctx, request, nil)
}

func responseUIError(ctx context.Context, request WebsocketMsg, errMsg string) {
	responseUI(ctx, WebsocketMsg{
		RequestId: request.RequestId,
		Type:      WebsocketMsgTypeResponse,
		Method:    request.Method,
		Success:   false,
		Data:      errMsg,
	})
}

func convertPluginDto(ctx context.Context, pluginDto dto.PluginDto, pluginInstance *plugin.Instance) dto.PluginDto {
	if pluginInstance != nil {
		logger.Debug(ctx, fmt.Sprintf("get plugin setting: %s", pluginInstance.Metadata.Name))
		pluginDto.SettingDefinitions = lo.Filter(pluginInstance.Metadata.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
			return !lo.Contains(item.DisabledInPlatforms, util.GetCurrentPlatform())
		})

		// replace dynamic setting definition
		var removedKeys []string
		for i, settingDefinition := range pluginDto.SettingDefinitions {
			if settingDefinition.Type == definition.PluginSettingDefinitionTypeDynamic {
				for _, callback := range pluginInstance.DynamicSettingCallbacks {
					newSettingDefinition := callback(settingDefinition.Value.GetKey())
					if newSettingDefinition.Value != nil && newSettingDefinition.Type != definition.PluginSettingDefinitionTypeDynamic {
						logger.Debug(ctx, fmt.Sprintf("dynamic setting replaced: %s(%s) -> %s(%s)", settingDefinition.Value.GetKey(), settingDefinition.Type, newSettingDefinition.Value.GetKey(), newSettingDefinition.Type))
						pluginDto.SettingDefinitions[i] = newSettingDefinition
					} else {
						logger.Error(ctx, fmt.Sprintf("dynamic setting not valid: %+v", newSettingDefinition))
						//remove invalid dynamic setting
						removedKeys = append(removedKeys, settingDefinition.Value.GetKey())
					}
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

		//add validator type to setting definition so that ui can render correctly
		for i := range pluginDto.SettingDefinitions {
			definitionType := pluginDto.SettingDefinitions[i].Type
			definitionValue := pluginDto.SettingDefinitions[i].Value
			if definitionValue != nil {
				if definitionType == definition.PluginSettingDefinitionTypeSelect {
					for _, validator := range definitionValue.(*definition.PluginSettingValueSelect).Validators {
						validator.SetValidatorType()
					}
				} else if definitionType == definition.PluginSettingDefinitionTypeTextBox {
					for _, validator := range definitionValue.(*definition.PluginSettingValueTextBox).Validators {
						validator.SetValidatorType()
					}
				} else if definitionType == definition.PluginSettingDefinitionTypeTable {
					for _, column := range definitionValue.(*definition.PluginSettingValueTable).Columns {
						for _, validator := range column.Validators {
							validator.SetValidatorType()
						}
					}
				}
			}
		}

		//translate setting definition labels
		for i := range pluginDto.SettingDefinitions {
			if pluginDto.SettingDefinitions[i].Value != nil {
				pluginDto.SettingDefinitions[i].Value.Translate(pluginInstance.API.GetTranslation)
			}
		}

		var definitionSettings = util.NewHashMap[string, string]()
		for _, item := range pluginDto.SettingDefinitions {
			if item.Value != nil {
				settingValue := pluginInstance.API.GetSetting(ctx, item.Value.GetKey())
				definitionSettings.Store(item.Value.GetKey(), settingValue)
			}
		}
		pluginDto.Setting = *pluginInstance.Setting
		//only return user pre-defined settings
		pluginDto.Setting.Settings = definitionSettings
	}

	return pluginDto
}
