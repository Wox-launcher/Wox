package setting

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/util"
	"wox/util/locale"
)

type WoxSetting struct {
	EnableAutostart           *PlatformValue[bool]
	MainHotkey                *PlatformValue[string]
	SelectionHotkey           *PlatformValue[string]
	ActionPanelHotkey         *PlatformValue[string] // Action Hotkey; new default primary+K, existing users keep J via migration.
	IgnoreHotkeysOnFullscreen *PlatformValue[bool]
	IgnoredHotkeyApps         *PlatformValue[[]IgnoredHotkeyApp]
	LogLevel                  *WoxSettingValue[string]
	UsePinYin                 *WoxSettingValue[bool]
	SwitchInputMethodABC      *WoxSettingValue[bool]
	HideOnStart               *WoxSettingValue[bool]
	// OnboardingFinished records whether this user data directory has already
	// seen the first-run guide. This is independent of account age because old
	// users who never saw the guide should still get one skippable pass.
	OnboardingFinished *WoxSettingValue[bool]
	HideOnLostFocus    *WoxSettingValue[bool]
	ShowTray           *WoxSettingValue[bool]
	LangCode           *WoxSettingValue[i18n.LangCode]
	QueryHotkeys       *PlatformValue[[]QueryHotkey]
	// ResultBindings stores result-level hotkeys and aliases independently of
	// homepage MRU. PlatformValue keeps same-OS Cloud Sync. Sync apply must
	// persist the payload as-is and must not restore-validate or drop failed
	// bindings.
	ResultBindings         *PlatformValue[[]ResultBinding]
	QueryAliases           *WoxSettingValue[[]QueryAlias]
	TrayQueries            *WoxSettingValue[[]TrayQuery]
	LaunchMode             *WoxSettingValue[LaunchMode]
	StartPage              *WoxSettingValue[StartPage]
	ShowPosition           *WoxSettingValue[PositionType]
	AIProviders            *WoxSettingValue[[]AIProvider]
	AIMCPServers           *WoxSettingValue[[]common.AIChatMCPServerConfig]
	AISkills               *WoxSettingValue[[]common.Skill]
	AIDisabledBuiltinTools *WoxSettingValue[[]string]
	EnableAutoBackup       *WoxSettingValue[bool]
	EnableAutoUpdate       *WoxSettingValue[bool]
	ReleaseChannel         *WoxSettingValue[ReleaseChannel]
	CustomPythonPath       *PlatformValue[string]
	CustomNodejsPath       *PlatformValue[string]

	// CloudSyncServerUrl is a local-only development override. It must not be
	// synced because each device may target a different test server.
	CloudSyncServerUrl       *WoxSettingValue[string]
	CloudSyncDisabledPlugins *WoxSettingValue[[]string]

	// HTTP proxy settings
	HttpProxyEnabled *PlatformValue[bool]
	HttpProxyUrl     *PlatformValue[string]

	// UI related
	AppWidth       *WoxSettingValue[int]
	MaxResultCount *WoxSettingValue[int]
	// UiDensity keeps launcher, chat, and text-overlay sizing in one user preference.
	// The setting is stored as an enum instead of individual dimensions so Go
	// window estimates and UI rendering can derive the same compact,
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

	// IgnoredDoctorChecks stores doctor check types the user has dismissed.
	// Ignored checks are skipped in the toolbar but still visible in the
	// doctor query with an Unignore action.
	IgnoredDoctorChecks *WoxSettingValue[[]string]
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
	DefaultThemeId      = "44a933d5-e6de-4c1f-8ee5-b2305c6abdf3"
	DefaultAutoThemeId  = "532238bc-6eda-4011-a080-c365b67486fc"
	DefaultLightThemeId = "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860"
	DefaultDarkThemeId  = "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03"
)

const (
	DefaultActionPanelHotkeyWindows = "ctrl+k"
	DefaultActionPanelHotkeyMac     = "command+k"
	DefaultActionPanelHotkeyLinux   = "ctrl+k"
	LegacyActionPanelHotkeyWindows  = "ctrl+j"
	LegacyActionPanelHotkeyMac      = "command+j"
	LegacyActionPanelHotkeyLinux    = "ctrl+j"
)

// DefaultActionPanelHotkey is the Action Hotkey for new installs.
func DefaultActionPanelHotkey() string {
	if util.IsWindows() {
		return DefaultActionPanelHotkeyWindows
	}
	if util.IsMacOS() {
		return DefaultActionPanelHotkeyMac
	}
	return DefaultActionPanelHotkeyLinux
}

// LegacyActionPanelHotkey is the pre-customization Action Hotkey.
func LegacyActionPanelHotkey() string {
	if util.IsWindows() {
		return LegacyActionPanelHotkeyWindows
	}
	if util.IsMacOS() {
		return LegacyActionPanelHotkeyMac
	}
	return LegacyActionPanelHotkeyLinux
}

// LegacyActionPanelHotkeyForPlatform returns the pre-customization shortcut for one OS.
func LegacyActionPanelHotkeyForPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case util.PlatformMacOS:
		return LegacyActionPanelHotkeyMac
	case util.PlatformLinux:
		return LegacyActionPanelHotkeyLinux
	default:
		return LegacyActionPanelHotkeyWindows
	}
}

const (
	LogLevelInfo  = "INFO"
	LogLevelDebug = "DEBUG"
)

type QueryAlias struct {
	// Alias is stored as "Shortcut" so existing settings and Cloud Sync payloads stay readable.
	Alias    string `json:"Shortcut"` // first-token expansion, e.g. "wi" => "wpm install {0} to {1}"
	Query    string
	Disabled bool
}

type IgnoredHotkeyApp struct {
	Name     string
	Identity string
	Path     string
	Icon     common.WoxImage
}

func (q *QueryAlias) HasPlaceholder() bool {
	return strings.Contains(q.Query, "{0}")
}

func (q *QueryAlias) PlaceholderCount() int {
	return len(regexp.MustCompile(`(?m){\d}`).FindAllString(q.Query, -1))
}

type AIProvider struct {
	Name            common.ProviderName // see ai.ProviderName
	Alias           string              // optional, used to distinguish multiple configs for the same provider
	ApiKey          string
	Host            string
	Executable      string `json:",omitempty"` // Optional installed CLI path; empty uses executable discovery.
	ReasoningEffort string `json:",omitempty"` // Empty preserves the installed provider's model default.
}

const (
	DefaultAIWebSearchResultCount        = 5
	MaxAIWebSearchResultCount            = 10
	DefaultAIWebSearchFetchMaxCharacters = 12000
	MaxAIWebSearchFetchMaxCharacters     = 50000
	DefaultAIWebSearchExaEndpoint        = "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa"
)

func normalizeAIWebSearchInt(value int, defaultValue int, minValue int, maxValue int) int {
	if value <= 0 {
		return defaultValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
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
	QueryHotkeyPositionMiddleLeft    QueryHotkeyPosition = "middle_left"
	QueryHotkeyPositionCenter        QueryHotkeyPosition = "center"
	QueryHotkeyPositionMiddleRight   QueryHotkeyPosition = "middle_right"
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

// NewResultHashFromParts resolves the stable result identity used by ranking and favorites.
func NewResultHashFromParts(pluginId, title, subTitle, scoreKey string) ResultHash {
	if strings.TrimSpace(scoreKey) != "" {
		return NewResultHash(pluginId, scoreKey, "")
	}
	return NewResultHash(pluginId, title, subTitle)
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

// UiDensityMultiplier is the exact compact/normal/comfortable factor.
// Integer window estimates multiply in float64 so halves such as 55 * 0.9 still round to 50.
func UiDensityMultiplier(density UiDensity) float64 {
	switch NormalizeUiDensity(string(density)) {
	case UiDensityCompact:
		return 0.9
	case UiDensityComfortable:
		return 1.1
	default:
		return 1
	}
}

// UiDensityScale is the shared multiplier used by launcher, chat, and text-overlay text.
func UiDensityScale(density UiDensity) float32 {
	return float32(UiDensityMultiplier(density))
}

// ScaleUiDensity rounds a normal-density size into the selected interface-size bucket.
// A normal density returns the authored size unchanged.
func ScaleUiDensity(value float32, density UiDensity) float32 {
	scale := UiDensityScale(density)
	if scale == 1 {
		return value
	}
	return float32(math.Round(float64(value * scale)))
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
	if locale.IsZhCN() {
		usePinYin = true
		defaultLangCode = i18n.LangCodeZhCn
	}

	return &WoxSetting{
		MainHotkey:                NewPlatformValue(store, "MainHotkey", "alt+space", "cmd+space", "ctrl+space"),
		SelectionHotkey:           NewPlatformValue(store, "SelectionHotkey", "win+alt+space", "command+option+space", "ctrl+shift+j"),
		ActionPanelHotkey:         NewPlatformValue(store, "ActionPanelHotkey", DefaultActionPanelHotkeyWindows, DefaultActionPanelHotkeyMac, DefaultActionPanelHotkeyLinux),
		IgnoreHotkeysOnFullscreen: NewPlatformValue(store, "IgnoreHotkeysOnFullscreen", false, false, false),
		IgnoredHotkeyApps:         NewPlatformValue(store, "IgnoredHotkeyApps", []IgnoredHotkeyApp{}, []IgnoredHotkeyApp{}, []IgnoredHotkeyApp{}),
		LogLevel: NewWoxSettingValueWithValidator(store, "LogLevel", LogLevelInfo, func(level string) bool {
			return strings.EqualFold(level, LogLevelInfo) || strings.EqualFold(level, LogLevelDebug)
		}),
		UsePinYin:            NewWoxSettingValue(store, "UsePinYin", usePinYin),
		SwitchInputMethodABC: NewWoxSettingValue(store, "SwitchInputMethodABC", false),
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
		EnableGlance:                       NewWoxSettingValue(store, "EnableGlance", true),
		PrimaryGlance:                      NewWoxSettingValue(store, "PrimaryGlance", GlanceRef{PluginId: "e3ad9f18-fbbe-4f22-8c1b-8274c751f6e6" /* system glance plugin id*/, GlanceId: "time"}),
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
		CloudSyncServerUrl:                 NewLocalWoxSettingValue(store, "CloudSyncServerUrl", ""),
		CloudSyncDisabledPlugins:           NewWoxSettingValue(store, "CloudSyncDisabledPlugins", []string{}),
		EnableAutoBackup:                   NewWoxSettingValue(store, "EnableAutoBackup", true),
		EnableAutoUpdate:                   NewWoxSettingValue(store, "EnableAutoUpdate", true),
		ReleaseChannel:                     NewWoxSettingValueWithValidator(store, "ReleaseChannel", ReleaseChannelStable, IsValidReleaseChannel),
		LastWindowX:                        NewWoxSettingValue(store, "LastWindowX", -1),
		LastWindowY:                        NewWoxSettingValue(store, "LastWindowY", -1),
		QueryHotkeys:                       NewPlatformValue(store, "QueryHotkeys", []QueryHotkey{}, []QueryHotkey{}, []QueryHotkey{}),
		ResultBindings:                     NewPlatformValue(store, "ResultBindings", []ResultBinding{}, []ResultBinding{}, []ResultBinding{}),
		QueryAliases:                       NewWoxSettingValue(store, "QueryAliases", []QueryAlias{}),
		TrayQueries:                        NewWoxSettingValue(store, "TrayQueries", []TrayQuery{}),
		AIProviders:                        NewWoxSettingValue(store, "AIProviders", []AIProvider{}),
		AIMCPServers:                       NewWoxSettingValue(store, "AIMCPServers", []common.AIChatMCPServerConfig{}),
		AISkills:                           NewWoxSettingValue(store, "AISkills", []common.Skill{}),
		AIDisabledBuiltinTools:             NewWoxSettingValue(store, "AIDisabledBuiltinTools", []string{}),
		QueryHistories:                     NewWoxSettingValue(store, "QueryHistories", []QueryHistory{}),
		QueryCompletionFeedbacks:           NewWoxSettingValue(store, "QueryCompletionFeedback", []QueryCompletionFeedback{}),
		PinedResults:                       NewWoxSettingValue(store, "PinedResults", util.NewHashMap[ResultHash, bool]()),
		ActionedResults:                    NewWoxSettingValue(store, "ActionedResults", util.NewHashMap[ResultHash, []ActionedResult]()),
		EnableAnonymousUsageStats:          NewWoxSettingValue(store, "EnableAnonymousUsageStats", true),
		IgnoredDoctorChecks:                NewWoxSettingValue(store, "IgnoredDoctorChecks", []string{}),
	}
}
