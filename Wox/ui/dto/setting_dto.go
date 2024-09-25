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
	LastQueryMode        setting.LastQueryMode
	AIProviders          []setting.AIProvider

	// UI related
	AppWidth int
	ThemeId  string
}
