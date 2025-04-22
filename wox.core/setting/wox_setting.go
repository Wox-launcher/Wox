package setting

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/util/shell"
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
	EnableAutoBackup     bool // Enable automatic data backup
	EnableAutoUpdate     bool // Enable automatic update check and download

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
	if isZhCN() {
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
		EnableAutoBackup: true,
		EnableAutoUpdate: true,
	}
}

func isZhCN() bool {
	lang, locale := getLocale()
	return strings.ToLower(lang) == "zh" && strings.ToLower(locale) == "cn"
}

func getLocale() (string, string) {
	osHost := runtime.GOOS
	defaultLang := "en"
	defaultLoc := "US"
	switch osHost {
	case "windows":
		// Exec powershell Get-Culture on Windows.
		output, err := shell.RunOutput("powershell", "Get-Culture | select -exp Name")
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "-")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	case "darwin":
		// Exec shell Get-Culture on MacOS.
		output, err := shell.RunOutput("osascript", "-e", "user locale of (get system info)")
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "_")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	case "linux":
		envlang, ok := os.LookupEnv("LANG")
		if ok {
			langLocRaw := strings.TrimSpace(envlang)
			langLocRaw = strings.Split(envlang, ".")[0]
			langLoc := strings.Split(langLocRaw, "_")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	}
	return defaultLang, defaultLoc
}
