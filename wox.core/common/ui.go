package common

import (
	"context"
	"wox/util/selection"
)

type PlainQuery struct {
	QueryId        string
	QueryType      string
	QueryText      string
	QuerySelection selection.Selection
}

var DefaultSettingWindowContext = SettingWindowContext{Path: "/"}

type SettingWindowContext struct {
	Path  string
	Param string
}

func (c PlainQuery) IsEmpty() bool {
	return c.QueryText == "" && c.QuerySelection.String() == ""
}

func (c PlainQuery) String() string {
	if c.QueryText != "" {
		return c.QueryText
	}

	return c.QuerySelection.String()
}

// ui methods that can be invoked by plugins
// because the golang recycle dependency issue, we can't use UI interface directly from plugin, so we need to define a new interface here
type UI interface {
	ChangeQuery(ctx context.Context, query PlainQuery)
	RefreshQuery(ctx context.Context, preserveSelectedIndex bool)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context, showContext ShowContext)
	ToggleApp(ctx context.Context)
	OpenSettingWindow(ctx context.Context, windowContext SettingWindowContext)
	PickFiles(ctx context.Context, params PickFilesParams) []string
	GetActiveWindowName() string
	GetActiveWindowPid() int
	GetActiveWindowIcon() WoxImage
	GetActiveWindowIsOpenSaveDialog() bool
	GetServerPort(ctx context.Context) int
	GetAllThemes(ctx context.Context) []Theme
	ChangeTheme(ctx context.Context, theme Theme)
	InstallTheme(ctx context.Context, theme Theme)
	UninstallTheme(ctx context.Context, theme Theme)
	RestoreTheme(ctx context.Context)
	Notify(ctx context.Context, msg NotifyMsg)
	// UpdateResult updates a result that is currently displayed in the UI.
	// Returns true if the result was successfully updated (still visible in UI).
	// Returns false if the result is no longer visible (caller should stop updating).
	// The result parameter should be plugin.UpdatableResult, but we use interface{} to avoid circular dependency.
	UpdateResult(ctx context.Context, result interface{}) bool
	// PushResults pushes additional results for the current query.
	// Returns true if results were accepted by UI, false if query is no longer active.
	// The payload should be plugin.PushResultsPayload, but we use interface{} to avoid circular dependency.
	PushResults(ctx context.Context, payload interface{}) bool
	// IsVisible returns true if the Wox window is currently visible
	IsVisible(ctx context.Context) bool

	// AI chat plugin related methods
	FocusToChatInput(ctx context.Context)
	SendChatResponse(ctx context.Context, chatData AIChatData)
	ReloadChatResources(ctx context.Context, resouceName string)

	// ReloadSettingPlugins asks the UI to refresh plugin lists.
	ReloadSettingPlugins(ctx context.Context)

	// ReloadSetting asks the UI to reload Wox settings from backend.
	ReloadSetting(ctx context.Context)
}

type ShowContext struct {
	SelectAll    bool
	IsQueryFocus bool // auto focus chat input on next ui update
}

type PickFilesParams struct {
	IsDirectory bool
}

type NotifyMsg struct {
	PluginId       string // can be empty
	Icon           string // WoxImage.String(), can be empty
	Text           string // can be empty
	DisplaySeconds int    // 0 means display forever
}
