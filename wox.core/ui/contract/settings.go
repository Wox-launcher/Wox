package contract

import (
	"context"

	"wox/account"
	"wox/cloudsync"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
)

// UpdateChannelVersion describes the latest version published for one update channel.
type UpdateChannelVersion struct {
	Channel       string
	LatestVersion string
	Error         string
}

// RuntimeStatus describes the current state of one plugin runtime host.
type RuntimeStatus struct {
	Runtime           string
	IsStarted         bool
	HostVersion       string
	StatusCode        string
	StatusMessage     string
	ExecutablePath    string
	LastStartError    string
	CanRestart        bool
	InstallURL        string
	LoadedPluginCount int
	LoadedPluginNames []string
}

// AboutSettingsServices exposes product metadata used by the About page.
type AboutSettingsServices interface {
	Version(ctx context.Context, sessionID string) (string, error)
}

// UpdateSettingsServices exposes release metadata used by update settings.
type UpdateSettingsServices interface {
	UpdateChannelVersions(ctx context.Context, sessionID string) ([]UpdateChannelVersion, error)
}

// RuntimeSettingsServices exposes plugin runtime status and recovery operations.
type RuntimeSettingsServices interface {
	RuntimeStatuses(ctx context.Context, sessionID string) ([]RuntimeStatus, error)
	RestartRuntime(ctx context.Context, sessionID string, runtime string) error
}

// UsageStatsDay contains one calendar bucket in a usage report.
type UsageStatsDay struct {
	Date  string
	Count int64
}

// UsageStatsItem contains one ranked application or plugin in a usage report.
type UsageStatsItem struct {
	ID    string
	Name  string
	Count int64
	Icon  common.WoxImage
}

// UsageStats contains the report fields consumed by the embedded settings UI.
type UsageStats struct {
	Period          string
	PeriodOpened    int64
	PeriodAppLaunch int64
	PeriodAppsUsed  int64
	PeriodActions   int64
	UsageDays       int
	MostActiveHour  int
	MostActiveDay   int
	OpenedByDay     []UsageStatsDay
	TopApps         []UsageStatsItem
	TopPlugins      []UsageStatsItem
}

// UsageSettingsServices exposes usage reports without transport decoding.
type UsageSettingsServices interface {
	UsageStats(ctx context.Context, sessionID string, period string) (UsageStats, error)
}

// DataBackup describes one restorable settings backup.
type DataBackup struct {
	ID        string
	Name      string
	Timestamp int64
	Type      string
	Path      string
}

// DataSettingsServices exposes user-data, backup, and log operations.
type DataSettingsServices interface {
	DataLocation(ctx context.Context, sessionID string) (string, error)
	DataBackups(ctx context.Context, sessionID string) ([]DataBackup, error)
	CreateDataBackup(ctx context.Context, sessionID string) error
	RestoreDataBackup(ctx context.Context, sessionID string, backupID string) error
	ChangeDataLocation(ctx context.Context, sessionID string, location string) error
	ClearLogs(ctx context.Context, sessionID string) error
	OpenPath(ctx context.Context, sessionID string, path string) error
	BackupFolder(ctx context.Context, sessionID string) (string, error)
	OpenLog(ctx context.Context, sessionID string) error
}

// GlanceCatalogItem describes one translated plugin glance available for selection.
type GlanceCatalogItem struct {
	PluginID          string
	GlanceID          string
	PluginName        string
	Name              string
	Description       string
	Icon              common.WoxImage
	RefreshIntervalMs int
}

// AppearanceSettingsServices exposes core-owned appearance resources.
type AppearanceSettingsServices interface {
	SystemFontFamilies(ctx context.Context, sessionID string) ([]string, error)
	GlanceCatalog(ctx context.Context, sessionID string) ([]GlanceCatalogItem, error)
}

// GeneralSettings contains the shared launcher settings consumed by the embedded UI.
type GeneralSettings struct {
	EnableAutostart                    bool
	MainHotkey                         string
	MainHotkeyRegistrationFailed       bool
	SelectionHotkey                    string
	IgnoredHotkeyApps                  []setting.IgnoredHotkeyApp
	LogLevel                           string
	UsePinYin                          bool
	SwitchInputMethodABC               bool
	HideOnStart                        bool
	OnboardingFinished                 bool
	HideOnLostFocus                    bool
	ShowTray                           bool
	LangCode                           i18n.LangCode
	QueryHotkeys                       []setting.QueryHotkey
	QueryShortcuts                     []setting.QueryShortcut
	TrayQueries                        []setting.TrayQuery
	LaunchMode                         setting.LaunchMode
	StartPage                          setting.StartPage
	AIProviders                        []setting.AIProvider
	AIMCPServers                       []common.AIChatMCPServerConfig
	AISkills                           []common.Skill
	HTTPProxyEnabled                   bool
	HTTPProxyURL                       string
	ShowPosition                       setting.PositionType
	IsLinuxWaylandSession              bool
	IsEvdevReadAvailable               bool
	EnableAutoBackup                   bool
	EnableAutoUpdate                   bool
	ReleaseChannel                     setting.ReleaseChannel
	EnableAnonymousUsageStats          bool
	EnablePrivacyMode                  bool
	CustomPythonPath                   string
	CustomNodejsPath                   string
	CloudSyncServerURL                 string
	CloudSyncDisabledPlugins           []string
	AppWidth                           int
	MaxResultCount                     int
	UIDensity                          setting.UiDensity
	ThemeID                            string
	AppFontFamily                      string
	EnableQueryCompletionHint          bool
	EnableGlance                       bool
	PrimaryGlance                      setting.GlanceRef
	HideGlanceIcon                     bool
	ShowScoreTail                      bool
	ShowPerformanceTail                bool
	ShowPerformanceTailBatch           bool
	ShowPerformanceTailPluginQuery     bool
	ShowPerformanceTailBackendPrepared bool
	ShowPerformanceTailUIReceived      bool
}

// GeneralSettingsServices exposes the shared settings snapshot and language catalog.
type GeneralSettingsServices interface {
	GeneralSettings(ctx context.Context, sessionID string) (GeneralSettings, error)
	AvailableLanguages(ctx context.Context, sessionID string) ([]i18n.Lang, error)
	LanguageJSON(ctx context.Context, sessionID string, langCode i18n.LangCode) (string, error)
	UpdateGeneralSetting(ctx context.Context, sessionID string, key string, value string) error
}

// MacOSPermissionStatus contains passive onboarding permission checks.
type MacOSPermissionStatus struct {
	Accessibility  string
	FullDiskAccess string
}

// OnboardingSettingsServices exposes first-run permission state and actions.
type OnboardingSettingsServices interface {
	MacOSPermissionStatus(ctx context.Context, sessionID string) (MacOSPermissionStatus, error)
	OpenMacOSPermission(ctx context.Context, sessionID string, permissionType string) error
}

// HotkeyApp describes one application that can be excluded from launcher hotkeys.
type HotkeyApp struct {
	Name     string
	Identity string
	Path     string
	Icon     common.WoxImage
}

// HotkeySettingsServices exposes platform application identities used by hotkey settings.
type HotkeySettingsServices interface {
	HotkeyAppCandidates(ctx context.Context, sessionID string) ([]HotkeyApp, error)
}

// HotkeyRecordingCapability describes raw-recorder and local fallback support.
type HotkeyRecordingCapability struct {
	RawRecorderAvailable bool
	FallbackAllowedKinds []string
	UnavailableReason    string
}

// HotkeyAvailability describes one configured or system hotkey conflict.
type HotkeyAvailability struct {
	Available     bool
	ConflictType  string
	ConflictValue string
}

// HotkeyInteractionSettingsServices exposes the recorder lifecycle used by settings forms.
type HotkeyInteractionSettingsServices interface {
	StartHotkeyRecording(ctx context.Context, sessionID string, purpose string, allowedKinds []string) (HotkeyRecordingCapability, error)
	StopHotkeyRecording(ctx context.Context, sessionID string) error
	SubmitHotkeyRecordingCandidate(ctx context.Context, sessionID string, hotkey string) error
	CheckHotkeyAvailability(ctx context.Context, sessionID string, hotkey string) (HotkeyAvailability, error)
}

// WindowManagerSettingsServices exposes browser integration used by workspace layouts.
type WindowManagerSettingsServices interface {
	BrowserExtensionConnected(ctx context.Context, sessionID string) (bool, error)
}

// AIProvider describes one built-in provider option.
type AIProvider struct {
	Name        string
	Icon        common.WoxImage
	DefaultHost string
}

// AIModel describes one model exposed by a configured provider.
type AIModel struct {
	Name          string
	Provider      string
	ProviderAlias string
}

// AICommandTemplate contains the editable command fields exposed by the shared template store.
type AICommandTemplate struct {
	ID            string
	Category      string
	Name          string
	Description   string
	Command       string
	Prompt        string
	ThinkingMode  string
	DefaultAction string
	Vision        bool
}

// AICommandTemplateServices exposes the optional command-template catalog used by plugin settings.
type AICommandTemplateServices interface {
	AICommandTemplates(ctx context.Context, sessionID string) ([]AICommandTemplate, error)
	DefaultAIModel(ctx context.Context, sessionID string) (AIModel, error)
}

// AISkill describes one discovered AI skill consumed by launcher selection surfaces.
type AISkill struct {
	ID           string
	Name         string
	Description  string
	Path         string
	ManifestPath string
	Source       string
	SourceName   string
	SourceURL    string
	Error        string
	Enabled      bool
}

// AICatalogSettingsServices exposes provider, model, and skill catalogs.
type AICatalogSettingsServices interface {
	AIProviders(ctx context.Context, sessionID string) ([]AIProvider, error)
	AIModels(ctx context.Context, sessionID string) ([]AIModel, error)
	AISkills(ctx context.Context, sessionID string) ([]AISkill, error)
}

// AIOperationSettingsServices exposes settings-owned AI skill mutations.
type AIOperationSettingsServices interface {
	CloneAISkills(ctx context.Context, sessionID string, sourceURL string) ([]AISkill, error)
}

// ManagedModelKind identifies one downloadable model family.
type ManagedModelKind string

const (
	ManagedModelDictation ManagedModelKind = "dictation"
	ManagedModelOCR       ManagedModelKind = "ocr"
)

// ManagedModelStatus contains runtime download state merged into settings metadata.
type ManagedModelStatus struct {
	ID               string
	DisplayName      string
	Description      string
	Languages        string
	Recommended      bool
	Status           string
	DownloadProgress int
	SizeMB           int
	Error            string
}

// ManagedModelEngineStatus contains runtime state for a model inference engine.
type ManagedModelEngineStatus struct {
	State    string
	Progress int
	Error    string
	Ready    bool
}

// ManagedModelOperation identifies a download or deletion action.
type ManagedModelOperation string

const (
	ManagedModelOperationDownload       ManagedModelOperation = "download"
	ManagedModelOperationDelete         ManagedModelOperation = "delete"
	ManagedModelOperationDownloadEngine ManagedModelOperation = "download-engine"
)

// ModelManagementSettingsServices exposes downloadable model and engine state.
type ModelManagementSettingsServices interface {
	ManagedModelStatuses(ctx context.Context, sessionID string, kind ManagedModelKind) ([]ManagedModelStatus, error)
	ManagedModelEngineStatus(ctx context.Context, sessionID string, kind ManagedModelKind) (ManagedModelEngineStatus, error)
	OperateManagedModel(ctx context.Context, sessionID string, kind ManagedModelKind, operation ManagedModelOperation, modelID string) error
}

// ThemeCatalog identifies one core-owned theme collection.
type ThemeCatalog string

const (
	ThemeCatalogInstalled ThemeCatalog = "installed"
	ThemeCatalogStore     ThemeCatalog = "store"
)

// ThemeCatalogItem contains one resolved theme and catalog-specific upgrade state.
type ThemeCatalogItem struct {
	Theme        common.Theme
	IsUpgradable bool
}

// ThemeCatalogSettingsServices exposes installed and store theme collections.
type ThemeCatalogSettingsServices interface {
	Themes(ctx context.Context, sessionID string, catalog ThemeCatalog) ([]ThemeCatalogItem, error)
}

// ThemeCurrentSettingsServices exposes the active platform-resolved theme.
type ThemeCurrentSettingsServices interface {
	CurrentTheme(ctx context.Context, sessionID string) (common.Theme, error)
}

// PluginCatalog identifies installed or store plugin collections.
type PluginCatalog string

const (
	PluginCatalogInstalled PluginCatalog = "installed"
	PluginCatalogStore     PluginCatalog = "store"
)

// PluginSetting contains persisted settings state associated with an installed plugin.
type PluginSetting struct {
	Disabled        bool
	TriggerKeywords []string
	Settings        map[string]string
}

// PluginCatalogItem contains the plugin metadata consumed across settings surfaces.
type PluginCatalogItem struct {
	ID                 string
	Name               string
	Description        string
	Author             string
	Website            string
	Version            string
	Runtime            string
	Entry              string
	PluginDirectory    string
	Icon               common.WoxImage
	ScreenshotURLs     []string
	TriggerKeywords    []string
	Commands           []plugin.MetadataCommand
	SupportedOS        []string
	Features           []plugin.MetadataFeature
	Glances            []plugin.MetadataGlance
	IsSystem           bool
	IsDev              bool
	IsInstalled        bool
	IsDisable          bool
	IsUpgradable       bool
	SettingDefinitions definition.PluginSettingDefinitions
	Setting            PluginSetting
}

// PluginCatalogSettingsServices exposes installed and store plugin collections.
type PluginCatalogSettingsServices interface {
	Plugins(ctx context.Context, sessionID string, catalog PluginCatalog) ([]PluginCatalogItem, error)
}

// PluginOperation identifies one lifecycle action supported by plugin settings.
type PluginOperation string

const (
	PluginOperationInstall   PluginOperation = "install"
	PluginOperationUninstall PluginOperation = "uninstall"
	PluginOperationEnable    PluginOperation = "enable"
	PluginOperationDisable   PluginOperation = "disable"
)

// PluginOperationSettingsServices exposes plugin lifecycle and persisted setting changes.
type PluginOperationSettingsServices interface {
	OperatePlugin(ctx context.Context, sessionID string, pluginID string, operation PluginOperation) error
	UpdatePluginSettings(ctx context.Context, sessionID string, pluginID string, values map[string]string) error
}

// ThemeOperation identifies one lifecycle action supported by theme settings.
type ThemeOperation string

const (
	ThemeOperationInstall   ThemeOperation = "install"
	ThemeOperationUninstall ThemeOperation = "uninstall"
	ThemeOperationApply     ThemeOperation = "apply"
)

// ThemeOperationSettingsServices exposes theme lifecycle changes.
type ThemeOperationSettingsServices interface {
	OperateTheme(ctx context.Context, sessionID string, themeID string, operation ThemeOperation) error
	SaveTheme(ctx context.Context, sessionID string, name string, theme common.Theme, overwrite bool) (common.Theme, error)
}

// CloudSettingsServices exposes account, sync, device, billing, and exclusion metadata.
type CloudSettingsServices interface {
	AccountStatus(ctx context.Context, sessionID string) (account.Status, error)
	CloudSyncStatus(ctx context.Context, sessionID string) (cloudsync.ServiceStatus, error)
	CloudDevices(ctx context.Context, sessionID string) (cloudsync.CloudSyncDeviceListResponse, error)
	BillingPlan(ctx context.Context, sessionID string) (account.BillingPlan, error)
}

// BillingSessionKind identifies the hosted billing flow requested by settings.
type BillingSessionKind string

const (
	BillingSessionCheckout BillingSessionKind = "checkout"
	BillingSessionPortal   BillingSessionKind = "portal"
)

// CloudBootstrapStatus describes the remote state needed to build the recovery form.
type CloudBootstrapStatus struct {
	HasRemoteData bool
	HasRemoteKey  bool
}

// CloudAccountOperationSettingsServices exposes account lifecycle and billing actions.
type CloudAccountOperationSettingsServices interface {
	LoginAccount(ctx context.Context, sessionID string, email string, password string, lang string) (account.ActionResult, error)
	RegisterAccount(ctx context.Context, sessionID string, email string, password string, lang string) (account.ActionResult, error)
	VerifyAccountEmail(ctx context.Context, sessionID string, email string, code string, lang string) (account.ActionResult, error)
	LogoutAccount(ctx context.Context, sessionID string) error
	ResendAccountVerification(ctx context.Context, sessionID string, email string, lang string) error
	RequestAccountPasswordReset(ctx context.Context, sessionID string, email string, lang string) error
	ConfirmAccountPasswordReset(ctx context.Context, sessionID string, token string, password string, lang string) error
	ChangeAccountPassword(ctx context.Context, sessionID string, currentPassword string, newPassword string, lang string) error
	BillingSession(ctx context.Context, sessionID string, kind BillingSessionKind) (account.BillingSession, error)
}

// CloudSyncOperationSettingsServices exposes bootstrap, manual sync, and device actions.
type CloudSyncOperationSettingsServices interface {
	CloudBootstrapStatus(ctx context.Context, sessionID string) (CloudBootstrapStatus, error)
	StartCloudBootstrap(ctx context.Context, sessionID string, recoveryCode string) error
	PushCloudChanges(ctx context.Context, sessionID string) error
	PullCloudChanges(ctx context.Context, sessionID string) error
	SyncCloud(ctx context.Context, sessionID string) error
	JoinCloudDevice(ctx context.Context, sessionID string) error
	RevokeCloudDevice(ctx context.Context, sessionID string, targetDeviceID string) (*cloudsync.CloudSyncDeviceRevokeResponse, error)
}

// SettingsServices combines typed settings domains exposed by core.
type SettingsServices interface {
	AboutSettingsServices
	UpdateSettingsServices
	RuntimeSettingsServices
	UsageSettingsServices
	DataSettingsServices
	AppearanceSettingsServices
	GeneralSettingsServices
	OnboardingSettingsServices
	HotkeySettingsServices
	HotkeyInteractionSettingsServices
	WindowManagerSettingsServices
	AICatalogSettingsServices
	AIOperationSettingsServices
	ModelManagementSettingsServices
	ThemeCatalogSettingsServices
	ThemeCurrentSettingsServices
	ThemeOperationSettingsServices
	PluginCatalogSettingsServices
	PluginOperationSettingsServices
	CloudSettingsServices
	CloudAccountOperationSettingsServices
	CloudSyncOperationSettingsServices
}
