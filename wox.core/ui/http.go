package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"wox/plugin"
	"wox/setting/definition"
	"wox/ui/dto"
	"wox/util"

	"github.com/samber/lo"
)

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

// newRouterMux exposes the same core-owned HTTP API to loopback and in-process callers.
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
