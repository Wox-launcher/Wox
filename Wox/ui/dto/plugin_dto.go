package dto

import (
	"wox/plugin"
	"wox/setting"
)

type StorePlugin struct {
	Id             string
	Name           string
	Author         string
	Version        string
	MinWoxVersion  string
	Runtime        string
	Description    string
	IconUrl        string
	Website        string
	DownloadUrl    string
	ScreenshotUrls []string
	DateCreated    string
	DateUpdated    string
	IsInstalled    bool
}

type InstalledPlugin struct {
	Id                           string
	Name                         string
	Author                       string
	Version                      string
	MinWoxVersion                string
	Runtime                      string
	Description                  string
	Icon                         string
	Website                      string
	Entry                        string
	TriggerKeywords              []string //User can add/update/delete trigger keywords
	Commands                     []plugin.MetadataCommand
	SupportedOS                  []string
	CustomizedSettingDefinitions setting.CustomizedPluginSettings
	Settings                     setting.PluginSetting
}
