package dto

import (
	"wox/common"
	"wox/plugin"
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
	PluginDirectory    string // only available when plugin is installed
	ScreenshotUrls     []string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []plugin.MetadataCommand
	SupportedOS        []string
	SettingDefinitions definition.PluginSettingDefinitions // only available when plugin is installed
	Setting            PluginSettingDto                    // only available when plugin is installed
	Features           []plugin.MetadataFeature            // only available when plugin is installed
	IsSystem           bool
	IsDev              bool
	IsInstalled        bool
	IsDisable          bool // only available when plugin is installed
}

type PluginSettingDto struct {
	Disabled        bool
	TriggerKeywords []string
	QueryCommands   []PluginQueryCommandDto
	Settings        map[string]string
}

type PluginQueryCommandDto struct {
	Command     string
	Description string
}
