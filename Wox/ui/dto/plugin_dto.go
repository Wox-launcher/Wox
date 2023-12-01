package dto

import (
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
)

type StorePlugin struct {
	Id             string
	Name           string
	Author         string
	Version        string
	MinWoxVersion  string
	Runtime        string
	Description    string
	Icon           plugin.WoxImage
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	DateCreated    string
	DateUpdated    string
	IsInstalled    bool
}

type InstalledPlugin struct {
	Id                 string
	Name               string
	Author             string
	Version            string
	MinWoxVersion      string
	Runtime            string
	Description        string
	Icon               plugin.WoxImage
	Website            string
	Entry              string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []plugin.MetadataCommand
	SupportedOS        []string
	SettingDefinitions definition.PluginSettingDefinitions
	Settings           setting.PluginSetting
	IsSystem           bool
}
