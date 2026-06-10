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
	EnableHyperKey       *PlatformValue[bool]
	IgnoredHotkeyApps    *PlatformValue[[]IgnoredHotkeyApp]
	LogLevel             *WoxSettingValue[string]
	UsePinYin            *WoxSettingValue[bool]
	SwitchInputMethodABC *WoxSettingValue[bool]
	HideOnStart          *WoxSettingValue[bool]
	// OnboardingFinished records whether this user data directory has already
	// seen the first-run guide. This is independent of account age because old
	// users who never saw the guide should still get one skippable pass.
	OnboardingFinished *WoxSettingValue[bool]
	HideOnLostFocus    *WoxSettingValue[bool]
	ShowTray           *WoxSettingValue[bool]
	LangCode           *WoxSettingValue[i18n.LangCode]
	QueryHotkeys       *PlatformValue[[]QueryHotkey]
	QueryShortcuts     *WoxSettingValue[[]QueryShortcut]
	TrayQueries        *WoxSettingValue[[]TrayQuery]
	LaunchMode         *WoxSettingValue[LaunchMode]
	StartPage          *WoxSettingValue[StartPage]
	ShowPosition       *WoxSettingValue[PositionType]
	AIProviders        *WoxSettingValue[[]AIProvider]
	EnableAutoBackup   *WoxSettingValue[bool]
	EnableAutoUpdate   *WoxSettingValue[bool]
	ReleaseChannel     *WoxSettingValue[ReleaseChannel]
	CustomPythonPath   *PlatformValue[string]
	CustomNodejsPath   *PlatformValue[string]

	// HTTP proxy settings
	HttpProxyEnabled *PlatformValue[bool]
	HttpProxyUrl     *PlatformValue[string]

	// UI related
	AppWidth       *WoxSettingValue[int]
	MaxResultCount *WoxSettingValue[int]
	// UiDensity keeps launcher text and control sizing in one user preference.
	// The setting is stored as an enum instead of individual dimensions so Go
	// window estimates and Flutter rendering can derive the same compact,
	// normal, and comfortable sizes without expanding the settings DTO.
	UiDensity                 *WoxSettingValue[UiDensity]
	ThemeId                   *WoxSettingValue[string]
	AppFontFamily             *PlatformValue[string]
	EnableQueryCompletionHint *WoxSettingValue[bool]
	EnableGlance              *WoxSettingValue[bool]
	PrimaryGlance             *WoxSettingValue[GlanceRef]
	// HideGlanceIcon is a presentation-only switch for the query-box glance.
	// Glance providers still return icons for metadata and future surfaces, but
	// the launcher can render a quieter text-only accessory when users prefer it.
	HideGlanceIcon *WoxSettingValue[bool]

	// Development-only debug display switches. Score and performance tails were
	// previously hard-coded around dev-only code paths, so storing the switches
	// here gives the settings UI and backend rendering one shared source of truth.
	ShowScoreTail                      *WoxSettingValue[bool]
	ShowPerformanceTail                *WoxSettingValue[bool]
	ShowPerformanceTailBatch           *WoxSettingValue[bool]
	ShowPerformanceTailPluginQuery     *WoxSettingValue[bool]
	ShowPerformanceTailBackendPrepared *WoxSettingValue[bool]
	ShowPerformanceTailUiReceived      *WoxSettingValue[bool]

	// Window position for last location mode
	LastWindowX *WoxSettingValue[int]
	LastWindowY *WoxSettingValue[int]

	QueryHistories           *WoxSettingValue[[]QueryHistory]
	QueryCompletionFeedbacks *WoxSettingValue[[]QueryCompletionFeedback]
	PinedResults             *WoxSettingValue[*util.HashMap[ResultHash, bool]]
	ActionedResults          *WoxSettingValue[*util.HashMap[ResultHash, []ActionedResult]]

	// Anonymous usage statistics
	EnableAnonymousUsageStats *WoxSettingValue[bool]
}

type LaunchMode = string
type StartPage = string

type UiDensity string
type ReleaseChannel string

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
	UiDensityCompact     UiDensity = "compact"
	UiDensityNormal      UiDensity = "normal"
	UiDensityComfortable UiDensity = "comfortable"
)

const (
	ReleaseChannelStable ReleaseChannel = "stable"
	ReleaseChannelBeta   ReleaseChannel = "beta"
)

const (
	DefaultThemeId = "44a933d5-e6de-4c1f-8ee5-b2305c6abdf3"
)

const (
	LogLevelInfo  = "INFO"
	LogLevelDebug = "DEBUG"
)

type QueryShortcut struct {
	Shortcut string // support index placeholder, e.g. shortcut "wi" => "wpm install {0} to {1}", when user input "wi 1 2", the query will be "wpm install 1 to 2"
	Query    string
	Disabled bool
}

type IgnoredHotkeyApp struct {
	Name     string
	Identity string
	Path     string
	Icon     common.WoxImage
}

func (q *QueryShortcut) HasPlaceholder() bool {
	return strings.Contains(q.Query, "{0}")
}

func (q *QueryShortcut) PlaceholderCount() int {
	return len(regexp.MustCompile(`(?m){\d}`).FindAllString(q.Query, -1))
}

type AIProvider struct {
	Name   common.ProviderName // see ai.ProviderName
	Alias  string              // optional, used to distinguish multiple configs for the same provider
	ApiKey string
	Host   string
}

type QueryHotkey struct {
	Name              string
	Hotkey            string
	Query             string // Support plugin.QueryVariable
	IsSilentExecution bool   // If true, the query will be executed without showing the query in the input box
	HideQueryBox      bool
	HideToolbar       bool
	Width             int
	MaxResultCount    int
	Position          QueryHotkeyPosition
	Disabled          bool
}

func (q QueryHotkey) DisplayName() string {
	if strings.TrimSpace(q.Name) != "" {
		return strings.TrimSpace(q.Name)
	}

	return q.Query
}

type QueryHotkeyPosition string

const (
	QueryHotkeyPositionSystemDefault QueryHotkeyPosition = "system_default"
	QueryHotkeyPositionTopLeft       QueryHotkeyPosition = "top_left"
	QueryHotkeyPositionTopCenter     QueryHotkeyPosition = "top_center"
	QueryHotkeyPositionTopRight      QueryHotkeyPosition = "top_right"
	QueryHotkeyPositionCenter        QueryHotkeyPosition = "center"
	QueryHotkeyPositionBottomLeft    QueryHotkeyPosition = "bottom_left"
	QueryHotkeyPositionBottomCenter  QueryHotkeyPosition = "bottom_center"
	QueryHotkeyPositionBottomRight   QueryHotkeyPosition = "bottom_right"
)

type TrayQuery struct {
	Icon           common.WoxImage
	Query          string
	Width          int `json:",omitempty"`
	MaxResultCount int `json:",omitempty"`
	HideQueryBox   bool
	HideToolbar    bool
	Disabled       bool
}

type GlanceRef struct {
	// PluginId plus GlanceId forms the persisted global identity so plugins can
	// reuse simple local ids without colliding with other providers.
	PluginId string
	GlanceId string
}

func (g GlanceRef) IsEmpty() bool {
	return g.PluginId == "" || g.GlanceId == ""
}

// ResultHash is a unique identifier for a result.
// It is used to store actioned results and favorite results.
type ResultHash string

func NewResultHash(pluginId, title, subTitle string) ResultHash {
	return ResultHash(util.Md5([]byte(fmt.Sprintf("%s%s%s", pluginId, title, subTitle))))
}

// NormalizeUiDensity converts missing or stale stored values to normal. The
// density setting is user-editable, so normalization keeps old config files and
// manual edits from pushing unsupported sizing states into the launcher.
func NormalizeUiDensity(value string) UiDensity {
	switch UiDensity(strings.ToLower(strings.TrimSpace(value))) {
	case UiDensityCompact:
		return UiDensityCompact
	case UiDensityComfortable:
		return UiDensityComfortable
	default:
		return UiDensityNormal
	}
}

// IsValidUiDensity lets lazy setting loading fall back to normal when a stored
// value is not one of the three supported scale buckets.
func IsValidUiDensity(value UiDensity) bool {
	return value == UiDensityCompact || value == UiDensityNormal || value == UiDensityComfortable
}

// NormalizeReleaseChannel converts missing or unsupported channel values to stable.
func NormalizeReleaseChannel(value string) ReleaseChannel {
	switch ReleaseChannel(strings.ToLower(strings.TrimSpace(value))) {
	case ReleaseChannelBeta:
		return ReleaseChannelBeta
	default:
		return ReleaseChannelStable
	}
}

func IsValidReleaseChannel(value ReleaseChannel) bool {
	return value == ReleaseChannelStable || value == ReleaseChannelBeta
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

// QueryCompletionFeedback records accepted inline completion hints for local ranking.
type QueryCompletionFeedback struct {
	CompletionText        string
	LastInputPrefix       string
	Source                string
	AcceptCount           int
	LastAcceptedTimestamp int64
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
		MainHotkey:        NewPlatformValue(store, "MainHotkey", "alt+space", "cmd+space", "ctrl+space"),
		SelectionHotkey:   NewPlatformValue(store, "SelectionHotkey", "ctrl+alt+space", "command+option+space", "ctrl+shift+j"),
		EnableHyperKey:    NewPlatformValue(store, "EnableHyperKey", false, false, false),
		IgnoredHotkeyApps: NewPlatformValue(store, "IgnoredHotkeyApps", []IgnoredHotkeyApp{}, []IgnoredHotkeyApp{}, []IgnoredHotkeyApp{}),
		LogLevel: NewWoxSettingValueWithValidator(store, "LogLevel", LogLevelInfo, func(level string) bool {
			return strings.EqualFold(level, LogLevelInfo) || strings.EqualFold(level, LogLevelDebug)
		}),
		UsePinYin:            NewWoxSettingValue(store, "UsePinYin", usePinYin),
		SwitchInputMethodABC: NewWoxSettingValue(store, "SwitchInputMethodABC", switchInputMethodABC),
		ShowTray:             NewWoxSettingValue(store, "ShowTray", true),
		HideOnLostFocus:      NewWoxSettingValue(store, "HideOnLostFocus", false),
		HideOnStart:          NewWoxSettingValue(store, "HideOnStart", false),
		OnboardingFinished:   NewWoxSettingValue(store, "OnboardingFinished", false),
		LangCode: NewWoxSettingValueWithValidator(store, "LangCode", defaultLangCode, func(code i18n.LangCode) bool {
			return i18n.IsSupportedLangCode(string(code))
		}),
		LaunchMode:                         NewWoxSettingValue(store, "LaunchMode", LaunchModeContinue),
		StartPage:                          NewWoxSettingValue(store, "StartPage", StartPageMRU),
		ShowPosition:                       NewWoxSettingValue(store, "ShowPosition", PositionTypeMouseScreen),
		AppWidth:                           NewWoxSettingValue(store, "AppWidth", 750),
		MaxResultCount:                     NewWoxSettingValue(store, "MaxResultCount", 8),
		UiDensity:                          NewWoxSettingValueWithValidator(store, "UiDensity", UiDensityNormal, IsValidUiDensity),
		ThemeId:                            NewWoxSettingValue(store, "ThemeId", DefaultThemeId),
		AppFontFamily:                      NewPlatformValue(store, "AppFontFamily", "", "", ""),
		EnableQueryCompletionHint:          NewWoxSettingValue(store, "EnableQueryCompletionHint", false),
		EnableGlance:                       NewWoxSettingValue(store, "EnableGlance", false),
		PrimaryGlance:                      NewWoxSettingValue(store, "PrimaryGlance", GlanceRef{PluginId: "e3ad9f18-fbbe-4f22-8c1b-8274c751f6e6", GlanceId: "time"}),
		HideGlanceIcon:                     NewWoxSettingValue(store, "HideGlanceIcon", false),
		ShowScoreTail:                      NewWoxSettingValue(store, "ShowScoreTail", false),
		ShowPerformanceTail:                NewWoxSettingValue(store, "ShowPerformanceTail", false),
		ShowPerformanceTailBatch:           NewWoxSettingValue(store, "ShowPerformanceTailBatch", true),
		ShowPerformanceTailPluginQuery:     NewWoxSettingValue(store, "ShowPerformanceTailPluginQuery", true),
		ShowPerformanceTailBackendPrepared: NewWoxSettingValue(store, "ShowPerformanceTailBackendPrepared", true),
		ShowPerformanceTailUiReceived:      NewWoxSettingValue(store, "ShowPerformanceTailUiReceived", true),
		EnableAutostart:                    NewPlatformValue(store, "EnableAutostart", false, false, false),
		HttpProxyEnabled:                   NewPlatformValue(store, "HttpProxyEnabled", false, false, false),
		HttpProxyUrl:                       NewPlatformValue(store, "HttpProxyUrl", "", "", ""),
		CustomPythonPath:                   NewPlatformValue(store, "CustomPythonPath", "", "", ""),
		CustomNodejsPath:                   NewPlatformValue(store, "CustomNodejsPath", "", "", ""),
		EnableAutoBackup:                   NewWoxSettingValue(store, "EnableAutoBackup", true),
		EnableAutoUpdate:                   NewWoxSettingValue(store, "EnableAutoUpdate", true),
		ReleaseChannel:                     NewWoxSettingValueWithValidator(store, "ReleaseChannel", ReleaseChannelStable, IsValidReleaseChannel),
		LastWindowX:                        NewWoxSettingValue(store, "LastWindowX", -1),
		LastWindowY:                        NewWoxSettingValue(store, "LastWindowY", -1),
		QueryHotkeys:                       NewPlatformValue(store, "QueryHotkeys", []QueryHotkey{}, []QueryHotkey{}, []QueryHotkey{}),
		QueryShortcuts:                     NewWoxSettingValue(store, "QueryShortcuts", []QueryShortcut{}),
		TrayQueries:                        NewWoxSettingValue(store, "TrayQueries", []TrayQuery{}),
		AIProviders:                        NewWoxSettingValue(store, "AIProviders", []AIProvider{}),
		QueryHistories:                     NewWoxSettingValue(store, "QueryHistories", []QueryHistory{}),
		QueryCompletionFeedbacks:           NewWoxSettingValue(store, "QueryCompletionFeedback", []QueryCompletionFeedback{}),
		PinedResults:                       NewWoxSettingValue(store, "PinedResults", util.NewHashMap[ResultHash, bool]()),
		ActionedResults:                    NewWoxSettingValue(store, "ActionedResults", util.NewHashMap[ResultHash, []ActionedResult]()),
		EnableAnonymousUsageStats:          NewWoxSettingValue(store, "EnableAnonymousUsageStats", true),
	}
}
