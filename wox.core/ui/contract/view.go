package contract

import (
	"context"

	"wox/cloudsync"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system/shell/terminal"
)

// Position describes the launcher window origin chosen by core policy.
type Position struct {
	Type string
	X    int
	Y    int
}

// ShowOptions contains only the window behavior currently consumed by the Go UI.
type ShowOptions struct {
	SelectAll        bool
	Position         Position
	WindowWidth      int
	MaxResultCount   int
	LaunchMode       string
	StartPage        string
	HideQueryBox     bool
	HideToolbar      bool
	QueryBoxAtBottom bool
	HideOnBlur       bool
	ShowSource       string
}

// OpenInstanceOptions describes a primary handoff or an independently hosted secondary launcher.
type OpenInstanceOptions struct {
	Role         string
	InstanceName string
	Query        common.PlainQuery
	Show         ShowOptions
}

// View is the typed boundary used by core to update the embedded UI.
type View interface {
	SessionID() string
	Show(ctx context.Context, options ShowOptions) error
	Hide(ctx context.Context) error
	Toggle(ctx context.Context, options ShowOptions) error
	OpenInstance(ctx context.Context, options OpenInstanceOptions) error
	ChangeQuery(ctx context.Context, query common.PlainQuery) error
	RefreshQuery(ctx context.Context, preserveSelectedIndex bool) error
	RefreshGlance(ctx context.Context, pluginID string, ids []string) error
	UpdateDiagnosticStatus(ctx context.Context, enabled bool) error
	RecordHotkey(ctx context.Context, hotkey string, kind string) error
	ChangeTheme(ctx context.Context, theme common.Theme) error
	OpenSetting(ctx context.Context, windowContext common.SettingWindowContext) error
	FocusSetting(ctx context.Context) error
	OpenOnboarding(ctx context.Context) error
	OpenMacOSPermissionFlow(ctx context.Context, permissionType string) error
	ShowToolbarMessage(ctx context.Context, message plugin.ToolbarMsgUI) error
	ShowNotificationMessage(ctx context.Context, message common.NotifyMsg) error
	ClearToolbarMessage(ctx context.Context, toolbarMessageID string) error
	UpdateAttentionUnreadCount(ctx context.Context, unreadCount int) error
	SendChatResponse(ctx context.Context, chat common.AIChatData) error
	ReloadChatResources(ctx context.Context, resourceName string) error
	SendAIQuestion(ctx context.Context, questionID string, question string, options []common.AIQuestionOption) error
	ReloadSettingPlugins(ctx context.Context) error
	ReloadSetting(ctx context.Context) error
	ReloadSettingThemes(ctx context.Context) error
	CloudSyncProgressChanged(ctx context.Context, progress cloudsync.CloudSyncProgress) error
	RefreshAccountStatus(ctx context.Context) error
	UpdateResult(ctx context.Context, result plugin.UpdatableResult) (bool, error)
	PushResults(ctx context.Context, payload plugin.PushResultsPayload) (bool, error)
	ToggleRecordingMode(ctx context.Context) (bool, error)
	PickFiles(ctx context.Context, params common.PickFilesParams) ([]string, error)
	CaptureScreenshot(ctx context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error)
	WriteClipboardImageFile(ctx context.Context, filePath string) error
	ApplyTerminalChunk(ctx context.Context, sessionID string, chunk terminal.TerminalChunk) error
	ApplyTerminalState(ctx context.Context, sessionID string, state terminal.SessionState) error
}
