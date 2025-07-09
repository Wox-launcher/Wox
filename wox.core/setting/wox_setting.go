package setting

import (
	"context"
	"regexp"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/util/locale"
)

type WoxSetting struct {
	EnableAutostart      PlatformSettingValue[bool]
	MainHotkey           PlatformSettingValue[string]
	SelectionHotkey      PlatformSettingValue[string]
	UsePinYin            bool
	SwitchInputMethodABC bool
	HideOnStart          bool
	HideOnLostFocus      bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         PlatformSettingValue[[]QueryHotkey]
	QueryShortcuts       []QueryShortcut
	LastQueryMode        LastQueryMode
	ShowPosition         PositionType
	AIProviders          []AIProvider
	EnableAutoBackup     bool                         // Enable automatic data backup
	EnableAutoUpdate     bool                         // Enable automatic update check and download
	CustomPythonPath     PlatformSettingValue[string] // Custom Python executable path
	CustomNodejsPath     PlatformSettingValue[string] // Custom Node.js executable path

	// HTTP proxy settings
	HttpProxyEnabled PlatformSettingValue[bool]
	HttpProxyUrl     PlatformSettingValue[string]

	// UI related
	AppWidth       int
	MaxResultCount int
	ThemeId        string
}

type LastQueryMode = string

type PositionType string

const (
	PositionTypeMouseScreen  PositionType = "mouse_screen"
	PositionTypeActiveScreen PositionType = "active_screen"
	PositionTypeLastLocation PositionType = "last_location"
)

const (
	LastQueryModePreserve LastQueryMode = "preserve" // preserve last query and select all for quick modify
	LastQueryModeEmpty    LastQueryMode = "empty"    // empty last query
)

const (
	DefaultThemeId = "e4006bd3-6bfe-4020-8d1c-4c32a8e567e5"
)

type QueryShortcut struct {
	Shortcut string // support index placeholder, e.g. shortcut "wi" => "wpm install {0} to {1}", when user input "wi 1 2", the query will be "wpm install 1 to 2"
	Query    string
}

func (q *QueryShortcut) HasPlaceholder() bool {
	return strings.Contains(q.Query, "{0}")
}

func (q *QueryShortcut) PlaceholderCount() int {
	return len(regexp.MustCompile(`(?m){\d}`).FindAllString(q.Query, -1))
}

type AIProvider struct {
	Name   common.ProviderName // see ai.ProviderName
	ApiKey string
	Host   string
}

type QueryHotkey struct {
	Hotkey            string
	Query             string // Support plugin.QueryVariable
	IsSilentExecution bool   // If true, the query will be executed without showing the query in the input box
}

func GetDefaultWoxSetting(ctx context.Context) WoxSetting {
	usePinYin := false
	langCode := i18n.LangCodeEnUs
	switchInputMethodABC := false
	if locale.IsZhCN() {
		usePinYin = true
		switchInputMethodABC = true
		langCode = i18n.LangCodeZhCn
	}

	return WoxSetting{
		MainHotkey: PlatformSettingValue[string]{
			WinValue:   "alt+space",
			MacValue:   "command+space",
			LinuxValue: "ctrl+ctrl",
		},
		SelectionHotkey: PlatformSettingValue[string]{
			WinValue:   "win+alt+space",
			MacValue:   "command+option+space",
			LinuxValue: "ctrl+shift+j",
		},
		UsePinYin:            usePinYin,
		SwitchInputMethodABC: switchInputMethodABC,
		ShowTray:             true,
		HideOnLostFocus:      true,
		LangCode:             langCode,
		LastQueryMode:        LastQueryModeEmpty,
		ShowPosition:         PositionTypeMouseScreen,
		AppWidth:             800,
		MaxResultCount:       10,
		ThemeId:              DefaultThemeId,
		EnableAutostart: PlatformSettingValue[bool]{
			WinValue:   false,
			MacValue:   false,
			LinuxValue: false,
		},
		HttpProxyEnabled: PlatformSettingValue[bool]{
			WinValue:   false,
			MacValue:   false,
			LinuxValue: false,
		},
		HttpProxyUrl: PlatformSettingValue[string]{
			WinValue:   "",
			MacValue:   "",
			LinuxValue: "",
		},
		CustomPythonPath: PlatformSettingValue[string]{
			WinValue:   "",
			MacValue:   "",
			LinuxValue: "",
		},
		CustomNodejsPath: PlatformSettingValue[string]{
			WinValue:   "",
			MacValue:   "",
			LinuxValue: "",
		},
		EnableAutoBackup: true,
		EnableAutoUpdate: true,
	}
}
