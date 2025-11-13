package dto

import (
	"wox/i18n"
	"wox/setting"
)

type WoxSettingDto struct {
	EnableAutostart      bool
	MainHotkey           string
	SelectionHotkey      string
	UsePinYin            bool
	SwitchInputMethodABC bool
	HideOnStart          bool
	HideOnLostFocus      bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         []setting.QueryHotkey
	QueryShortcuts       []setting.QueryShortcut
	LaunchMode           setting.LaunchMode
	StartPage            setting.StartPage
	AIProviders          []setting.AIProvider
	HttpProxyEnabled     bool
	HttpProxyUrl         string
	ShowPosition         setting.PositionType
	EnableAutoBackup     bool
	EnableAutoUpdate     bool
	CustomPythonPath     string
	CustomNodejsPath     string

	// UI related
	AppWidth       int
	MaxResultCount int
	ThemeId        string
}
