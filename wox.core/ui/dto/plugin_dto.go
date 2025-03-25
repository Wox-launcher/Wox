package dto

import (
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
)

type PluginDto struct {
	Id                 string
	Name               string
	Author             string
	Version            string
	MinWoxVersion      string
	Runtime            string
	Description        string
	Icon               common.WoxImage
	Website            string
	Entry              string
	ScreenshotUrls     []string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []plugin.MetadataCommand
	SupportedOS        []string
	SettingDefinitions definition.PluginSettingDefinitions // only available when plugin is installed
	Setting            setting.PluginSetting               // only available when plugin is installed
	Features           []plugin.MetadataFeature            // only available when plugin is installed
	IsSystem           bool
	IsDev              bool
	IsInstalled        bool
	IsDisable          bool // only available when plugin is installed
}
