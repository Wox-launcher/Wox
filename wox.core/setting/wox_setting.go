package setting

import (
	"fmt"
	"regexp"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/util"
	"wox/util/locale"
)

type WoxSetting struct {
	EnableAutostart      *PlatformValue[bool]
	MainHotkey           *PlatformValue[string]
	SelectionHotkey      *PlatformValue[string]
	UsePinYin            *WoxSettingValue[bool]
	SwitchInputMethodABC *WoxSettingValue[bool]
	HideOnStart          *WoxSettingValue[bool]
	HideOnLostFocus      *WoxSettingValue[bool]
	ShowTray             *WoxSettingValue[bool]
	LangCode             *WoxSettingValue[i18n.LangCode]
	QueryHotkeys         *PlatformValue[[]QueryHotkey]
	QueryShortcuts       *WoxSettingValue[[]QueryShortcut]
	LaunchMode           *WoxSettingValue[LaunchMode]
	StartPage            *WoxSettingValue[StartPage]
	ShowPosition         *WoxSettingValue[PositionType]
	AIProviders          *WoxSettingValue[[]AIProvider]
	EnableAutoBackup     *WoxSettingValue[bool]
	EnableAutoUpdate     *WoxSettingValue[bool]
	CustomPythonPath     *PlatformValue[string]
	CustomNodejsPath     *PlatformValue[string]

	// HTTP proxy settings
	HttpProxyEnabled *PlatformValue[bool]
	HttpProxyUrl     *PlatformValue[string]

	// UI related
	AppWidth       *WoxSettingValue[int]
	MaxResultCount *WoxSettingValue[int]
	ThemeId        *WoxSettingValue[string]

	// Window position for last location mode
	LastWindowX *WoxSettingValue[int]
	LastWindowY *WoxSettingValue[int]

	QueryHistories  *WoxSettingValue[[]QueryHistory]
	PinedResults    *WoxSettingValue[*util.HashMap[ResultHash, bool]]
	ActionedResults *WoxSettingValue[*util.HashMap[ResultHash, []ActionedResult]]
}

type LaunchMode = string
type StartPage = string

type PositionType string

const (
	PositionTypeMouseScreen  PositionType = "mouse_screen"
	PositionTypeActiveScreen PositionType = "active_screen"
	PositionTypeLastLocation PositionType = "last_location"
)

const (
	LaunchModeFresh    LaunchMode = "fresh"    // start fresh with empty query
	LaunchModeContinue LaunchMode = "continue" // continue with last query
)

const (
	StartPageBlank StartPage = "blank" // show blank page
	StartPageMRU   StartPage = "mru"   // show MRU (Most Recently Used) list
)

const (
	DefaultThemeId = "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03"
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

// ResultHash is a unique identifier for a result.
// It is used to store actioned results and favorite results.
type ResultHash string

func NewResultHash(pluginId, title, subTitle string) ResultHash {
	return ResultHash(util.Md5([]byte(fmt.Sprintf("%s%s%s", pluginId, title, subTitle))))
}

// ActionedResult stores the information of an actioned result.
type ActionedResult struct {
	Timestamp int64
	Query     string // Record the raw query text when the user performs action on this result
}

// QueryHistory stores the information of a query history.
type QueryHistory struct {
	Query     common.PlainQuery
	Timestamp int64
}

func NewWoxSetting(store *WoxSettingStore) *WoxSetting {
	usePinYin := false
	defaultLangCode := i18n.LangCodeEnUs
	switchInputMethodABC := false
	if locale.IsZhCN() {
		usePinYin = true
		switchInputMethodABC = true
		defaultLangCode = i18n.LangCodeZhCn
	}

	return &WoxSetting{
		MainHotkey:           NewPlatformValue(store, "MainHotkey", "alt+space", "option+space", "ctrl+space"),
		SelectionHotkey:      NewPlatformValue(store, "SelectionHotkey", "ctrl+alt+space", "command+option+space", "ctrl+shift+j"),
		UsePinYin:            NewWoxSettingValue(store, "UsePinYin", usePinYin),
		SwitchInputMethodABC: NewWoxSettingValue(store, "SwitchInputMethodABC", switchInputMethodABC),
		ShowTray:             NewWoxSettingValue(store, "ShowTray", true),
		HideOnLostFocus:      NewWoxSettingValue(store, "HideOnLostFocus", true),
		HideOnStart:          NewWoxSettingValue(store, "HideOnStart", false),
		LangCode: NewWoxSettingValueWithValidator(store, "LangCode", defaultLangCode, func(code i18n.LangCode) bool {
			return i18n.IsSupportedLangCode(string(code))
		}),
		LaunchMode:       NewWoxSettingValue(store, "LaunchMode", LaunchModeContinue),
		StartPage:        NewWoxSettingValue(store, "StartPage", StartPageMRU),
		ShowPosition:     NewWoxSettingValue(store, "ShowPosition", PositionTypeMouseScreen),
		AppWidth:         NewWoxSettingValue(store, "AppWidth", 800),
		MaxResultCount:   NewWoxSettingValue(store, "MaxResultCount", 10),
		ThemeId:          NewWoxSettingValue(store, "ThemeId", DefaultThemeId),
		EnableAutostart:  NewPlatformValue(store, "EnableAutostart", false, false, false),
		HttpProxyEnabled: NewPlatformValue(store, "HttpProxyEnabled", false, false, false),
		HttpProxyUrl:     NewPlatformValue(store, "HttpProxyUrl", "", "", ""),
		CustomPythonPath: NewPlatformValue(store, "CustomPythonPath", "", "", ""),
		CustomNodejsPath: NewPlatformValue(store, "CustomNodejsPath", "", "", ""),
		EnableAutoBackup: NewWoxSettingValue(store, "EnableAutoBackup", true),
		EnableAutoUpdate: NewWoxSettingValue(store, "EnableAutoUpdate", true),
		LastWindowX:      NewWoxSettingValue(store, "LastWindowX", -1),
		LastWindowY:      NewWoxSettingValue(store, "LastWindowY", -1),
		QueryHotkeys:     NewPlatformValue(store, "QueryHotkeys", []QueryHotkey{}, []QueryHotkey{}, []QueryHotkey{}),
		QueryShortcuts:   NewWoxSettingValue(store, "QueryShortcuts", []QueryShortcut{}),
		AIProviders:      NewWoxSettingValue(store, "AIProviders", []AIProvider{}),
		QueryHistories:   NewWoxSettingValue(store, "QueryHistories", []QueryHistory{}),
		PinedResults:     NewWoxSettingValue(store, "PinedResults", util.NewHashMap[ResultHash, bool]()),
		ActionedResults:  NewWoxSettingValue(store, "ActionedResults", util.NewHashMap[ResultHash, []ActionedResult]()),
	}
}
