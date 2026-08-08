package ui

import (
	"context"
	"fmt"

	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/ui/dto"
	"wox/util"

	"github.com/jinzhu/copier"
	"github.com/samber/lo"
)

func convertPluginInstanceToDto(ctx context.Context, pluginInstance *plugin.Instance) (installedPlugin dto.PluginDto, err error) {
	if err := copier.Copy(&installedPlugin, &pluginInstance.Metadata); err != nil {
		return dto.PluginDto{}, err
	}
	installedPlugin.Name = pluginInstance.GetName(ctx)
	installedPlugin.NameEn = pluginInstance.Metadata.GetNameEn(ctx)
	installedPlugin.Description = pluginInstance.GetDescription(ctx)
	installedPlugin.DescriptionEn = pluginInstance.Metadata.GetDescriptionEn(ctx)
	installedPlugin.IsSystem = pluginInstance.IsSystemPlugin
	installedPlugin.IsDev = pluginInstance.IsDevPlugin
	installedPlugin.IsInstalled = true
	installedPlugin.IsDisable = pluginInstance.Setting.Disabled.Get()
	installedPlugin.TriggerKeywords = pluginInstance.GetTriggerKeywords()
	installedPlugin.Commands = pluginInstance.GetQueryCommands()
	installedPlugin.Glances = translatePluginGlances(ctx, pluginInstance)

	storePlugin, storeErr := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginInstance.Metadata.Id)
	if storeErr == nil {
		installedPlugin.ScreenshotUrls = storePlugin.ScreenshotUrls
		installedPlugin.IsUpgradable = plugin.IsVersionUpgradable(pluginInstance.Metadata.Version, storePlugin.Version)
	} else {
		installedPlugin.ScreenshotUrls = []string{}
		installedPlugin.IsUpgradable = false
	}

	icon, iconErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
	if iconErr == nil {
		installedPlugin.Icon = icon
	} else {
		installedPlugin.Icon = common.NewWoxImageBase64(`data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAACXBIWXMAAAsTAAALEwEAmpwYAAAELUlEQVR4nO3ZW2xTdRwH8JPgkxE1XuKFQUe73rb1IriNyYOJvoiALRszvhqffHBLJjEx8Q0TlRiN0RiNrPd27boLY1wUFAQHyquJiTIYpefay7au2yiJG1/zb6Kx/ZfS055T1mS/5JtzXpr+Pufy//9P/gyzURulXIHp28Rp7H5eY/OSc6bRiiPNN9tBQs4bDsFrbN5/AQ2JANO3qRgx9dZ76I2vwingvsQhgHUK2NPQCKeAuOw7Mf72B1hPCEZu9bBrWE8IRm6RH60nBFMNQA3Eh6kVzCzzyOVu5I+HUyvqApREkOZxe5bKR+kVdQFKIcgVLwW4usyrD1ACcSsXKwm4lYvVB1Ar4r7fAWeNCPLClgIcruBFVhRQK4Jc8Vwulj/WZRQqh4i8+X5d5glGaYCDBzp/WYQ5KsJ98JDqCEZJgIO/g53nM9BHpXxMEQHuXnURjFIA0vyOHxfQMiIVxBgW4FIRwSgBcLB3YPt+DrqwWDKGEA9Xz7tlES/9nkPHuQyeP5/By3/crh9gf3wNlpMpaENC2egDHFwHSiBurqL78hLaJlNoPZaCeSIJ01gSu68sqw/YF1uDeTKF5qBQUXR+DkNFiOgbg7BOSBTAOJrIw1QD7J1dzf+Jxs/LitbL4qhjsAAROjAA67hEAQzRBLovLSkPePX6an6U2eblqorWE4en7xCFsIxJFEAfkcoiZANeufo3tMMitnq4qkIArZMp2E+k4H29COEcQPuoSAH0YQm7prPKAMhjsMXFVpUmN4f2qTRsp+dgPTUH21SyJKJtRKQALcNiSYRswLNH46gmTW4W7SfSsP8w/x/AcjIN6/EkvEWPU9AxgNaIQAF0IRFdF7O1AZ75Lg65aXKxsJxKw35mngIQlOVYoiTCHBYogDZQiJANePrbm5AT8uhYT8/hubPzdwW0HU/BMpGA5yCNMIV4CrDdL6DzQrY6wFPfxFBpSPOkabLEuBeAzAOWcYIonCcCr/XDFOQpQLOPIBblA578OoZKQprfcXYeO88tVAwg80D7mARPL40wBjgKoPHy8gFPfHUD90q++Z8W8usauQAyD7RFJbiLEfv7YfBztQMe/3IW5bLFzeYb7/g5UzXANJZE64gIdw+N0Hu52gCPfTGLu2Wrm0XHhQw6Ly7WDDCOJmCOiNQq1r+vHy0etnrAo59fR6mQcZ40Tr7GlAIYogmYhgVqFUsQOjdbHeCRz66hONt8HLqmF9E1nVUcoI9IMIUIYpBGuOLyAQ9/eg3/jybAo/tyFrsuZVUD6MMSjEGeQvj2vgMwLz4gC7D5yAy7+cgMSLYHBbzw2xK6f1Uf0DIswhDgMeQsRPAae0QW4sGP/9zz0Cd/seRTcfeVpboCdCEReh+PIUeNiHWx7dtsDxQibF6mkQpFCHLONOYGvM1Hmm+obd+NYhqg/gG2aOxED6eh5gAAAABJRU5ErkJggg==`)
	}
	installedPlugin.Icon = common.ConvertIcon(ctx, installedPlugin.Icon, pluginInstance.PluginDirectory)
	return convertPluginDto(ctx, installedPlugin, pluginInstance), nil
}

func translatePluginGlances(ctx context.Context, pluginInstance *plugin.Instance) []plugin.MetadataGlance {
	glances := make([]plugin.MetadataGlance, 0, len(pluginInstance.Metadata.Glances))
	for _, glance := range pluginInstance.Metadata.Glances {
		// Settings consume translated metadata while plugin.json keeps its i18n keys.
		glance.Name = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Name))
		glance.Description = common.I18nString(pluginInstance.TranslateMetadataText(ctx, glance.Description))
		glances = append(glances, glance)
	}
	return glances
}

func convertPluginDto(ctx context.Context, pluginDto dto.PluginDto, pluginInstance *plugin.Instance) dto.PluginDto {
	if pluginInstance == nil {
		return pluginDto
	}

	logger.Debug(ctx, fmt.Sprintf("get plugin setting: %s", pluginInstance.GetName(ctx)))
	pluginDto.PluginDirectory = pluginInstance.PluginDirectory
	pluginDto.SettingDefinitions = lo.Filter(pluginInstance.Metadata.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
		return !lo.Contains(item.DisabledInPlatforms, util.GetCurrentPlatform())
	})

	var removedKeys []string
	for i, settingDefinition := range pluginDto.SettingDefinitions {
		if settingDefinition.Type != definition.PluginSettingDefinitionTypeDynamic {
			continue
		}
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
			removedKeys = append(removedKeys, settingDefinition.Value.GetKey())
		}
	}

	pluginDto.SettingDefinitions = lo.Filter(pluginDto.SettingDefinitions, func(item definition.PluginSettingDefinitionItem, _ int) bool {
		return item.Value == nil || !lo.Contains(removedKeys, item.Value.GetKey())
	})
	for i := range pluginDto.SettingDefinitions {
		if pluginDto.SettingDefinitions[i].Value != nil {
			pluginDto.SettingDefinitions[i].Value = pluginDto.SettingDefinitions[i].Value.Translate(pluginInstance.API.GetTranslation)
		}
	}

	nonDynamicSettings := make(map[string]string)
	for _, item := range pluginDto.SettingDefinitions {
		if item.Value != nil {
			nonDynamicSettings[item.Value.GetKey()] = pluginInstance.API.GetSetting(ctx, item.Value.GetKey())
		}
	}
	pluginDto.Setting = dto.PluginSettingDto{Disabled: pluginInstance.Setting.Disabled.Get(), TriggerKeywords: pluginInstance.Setting.TriggerKeywords.Get(), Settings: nonDynamicSettings}
	pluginDto.Features = pluginInstance.Metadata.Features
	pluginDto.TriggerKeywords = pluginInstance.GetTriggerKeywords()
	pluginDto.Name = pluginInstance.GetName(ctx)
	pluginDto.Description = pluginInstance.GetDescription(ctx)
	pluginDto.Commands = pluginInstance.GetQueryCommands()
	return pluginDto
}
