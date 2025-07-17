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
	UsePinYin            *Value[bool]
	SwitchInputMethodABC *Value[bool]
	HideOnStart          *Value[bool]
	HideOnLostFocus      *Value[bool]
	ShowTray             *Value[bool]
	LangCode             *Value[i18n.LangCode]
	QueryHotkeys         *PlatformValue[[]QueryHotkey]
	QueryShortcuts       *Value[[]QueryShortcut]
	LastQueryMode        *Value[LastQueryMode]
	ShowPosition         *Value[PositionType]
	AIProviders          *Value[[]AIProvider]
	EnableAutoBackup     *Value[bool]
	EnableAutoUpdate     *Value[bool]
	CustomPythonPath     *PlatformValue[string]
	CustomNodejsPath     *PlatformValue[string]

	// HTTP proxy settings
	HttpProxyEnabled *PlatformValue[bool]
	HttpProxyUrl     *PlatformValue[string]

	// UI related
	AppWidth       *Value[int]
	MaxResultCount *Value[int]
	ThemeId        *Value[string]

	// Window position for last location mode
	LastWindowX *Value[int]
	LastWindowY *Value[int]

	// Data that was previously in WoxAppData
	QueryHistories  *Value[[]QueryHistory]
	FavoriteResults *Value[*util.HashMap[ResultHash, bool]]
	ActionedResults *Value[*util.HashMap[ResultHash, []ActionedResult]]
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

func NewWoxSetting(store WoxSettingStore) *WoxSetting {
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
		UsePinYin:            NewValue(store, "UsePinYin", usePinYin),
		SwitchInputMethodABC: NewValue(store, "SwitchInputMethodABC", switchInputMethodABC),
		ShowTray:             NewValue(store, "ShowTray", true),
		HideOnLostFocus:      NewValue(store, "HideOnLostFocus", true),
		HideOnStart:          NewValue(store, "HideOnStart", false),
		LangCode: NewValueWithValidator(store, "LangCode", defaultLangCode, func(code i18n.LangCode) bool {
			return i18n.IsSupportedLangCode(string(code))
		}),
		LastQueryMode:    NewValue(store, "LastQueryMode", LastQueryModeEmpty),
		ShowPosition:     NewValue(store, "ShowPosition", PositionTypeMouseScreen),
		AppWidth:         NewValue(store, "AppWidth", 800),
		MaxResultCount:   NewValue(store, "MaxResultCount", 10),
		ThemeId:          NewValue(store, "ThemeId", DefaultThemeId),
		EnableAutostart:  NewPlatformValue(store, "EnableAutostart", false, false, false),
		HttpProxyEnabled: NewPlatformValue(store, "HttpProxyEnabled", false, false, false),
		HttpProxyUrl:     NewPlatformValue(store, "HttpProxyUrl", "", "", ""),
		CustomPythonPath: NewPlatformValue(store, "CustomPythonPath", "", "", ""),
		CustomNodejsPath: NewPlatformValue(store, "CustomNodejsPath", "", "", ""),
		EnableAutoBackup: NewValue(store, "EnableAutoBackup", true),
		EnableAutoUpdate: NewValue(store, "EnableAutoUpdate", true),
		LastWindowX:      NewValue(store, "LastWindowX", -1),
		LastWindowY:      NewValue(store, "LastWindowY", -1),
		QueryHotkeys:     NewPlatformValue(store, "QueryHotkeys", []QueryHotkey{}, []QueryHotkey{}, []QueryHotkey{}),
		QueryShortcuts:   NewValue(store, "QueryShortcuts", []QueryShortcut{}),
		AIProviders:      NewValue(store, "AIProviders", []AIProvider{}),
		QueryHistories:   NewValue(store, "QueryHistories", []QueryHistory{}),
		FavoriteResults:  NewValue(store, "FavoriteResults", util.NewHashMap[ResultHash, bool]()),
		ActionedResults:  NewValue(store, "ActionedResults", util.NewHashMap[ResultHash, []ActionedResult]()),
	}
}
