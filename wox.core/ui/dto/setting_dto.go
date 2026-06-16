package dto

import (
	"wox/i18n"
	"wox/setting"
)

type WoxSettingDto struct {
	EnableAutostart      bool
	MainHotkey           string
	SelectionHotkey      string
	IgnoredHotkeyApps    []setting.IgnoredHotkeyApp
	LogLevel             string
	UsePinYin            bool
	SwitchInputMethodABC bool
	HideOnStart          bool
	// OnboardingFinished is sent with the regular settings DTO so Flutter can
	// update the guide completion flag through the existing key-value API and
	// avoid a separate first-run state endpoint.
	OnboardingFinished        bool
	HideOnLostFocus           bool
	ShowTray                  bool
	LangCode                  i18n.LangCode
	QueryHotkeys              []setting.QueryHotkey
	QueryShortcuts            []setting.QueryShortcut
	TrayQueries               []setting.TrayQuery
	LaunchMode                setting.LaunchMode
	StartPage                 setting.StartPage
	AIProviders               []setting.AIProvider
	HttpProxyEnabled          bool
	HttpProxyUrl              string
	ShowPosition              setting.PositionType
	IsLinuxWaylandSession     bool
	EnableAutoBackup          bool
	EnableAutoUpdate          bool
	ReleaseChannel            setting.ReleaseChannel
	EnableAnonymousUsageStats bool
	CustomPythonPath          string
	CustomNodejsPath          string
	CloudSyncServerUrl        string
	CloudSyncDisabledPlugins  []string

	// UI related
	AppWidth       int
	MaxResultCount int
	// UiDensity is a compact enum rather than per-control dimensions because
	// backend window sizing and Flutter rendering both derive their local
	// metrics from the same three scale buckets.
	UiDensity                 setting.UiDensity
	ThemeId                   string
	AppFontFamily             string
	EnableQueryCompletionHint bool
	EnableGlance              bool
	PrimaryGlance             setting.GlanceRef
	// HideGlanceIcon is kept beside the Glance selection because Flutter needs
	// it with the rest of the UI settings to render the query-box accessory.
	HideGlanceIcon bool

	// Debug display switches are only shown by the dev UI, but the DTO keeps
	// them beside other settings so backend tail rendering and Flutter toggles
	// stay synchronized through the existing settings API.
	ShowScoreTail                      bool
	ShowPerformanceTail                bool
	ShowPerformanceTailBatch           bool
	ShowPerformanceTailPluginQuery     bool
	ShowPerformanceTailBackendPrepared bool
	ShowPerformanceTailUiReceived      bool
}
