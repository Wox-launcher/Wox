package common

import (
	"context"
	"wox/util/selection"
)

type PlainQuery struct {
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
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context, showContext ShowContext)
	ToggleApp(ctx context.Context)
	OpenSettingWindow(ctx context.Context, windowContext SettingWindowContext)
	PickFiles(ctx context.Context, params PickFilesParams) []string
	GetActiveWindowName() string
	GetActiveWindowPid() int
	GetServerPort(ctx context.Context) int
	GetAllThemes(ctx context.Context) []Theme
	ChangeTheme(ctx context.Context, theme Theme)
	InstallTheme(ctx context.Context, theme Theme)
	UninstallTheme(ctx context.Context, theme Theme)
	RestoreTheme(ctx context.Context)
	Notify(ctx context.Context, msg NotifyMsg)
	FocusToChatInput(ctx context.Context)
	SendChatResponse(ctx context.Context, chatData AIChatData)
	UpdateResult(ctx context.Context, result UpdateableResult)
}

type ShowContext struct {
	SelectAll            bool
	AutoFocusToChatInput bool // auto focus chat input on next ui update
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

// UpdateableResult is used to update the result of a query
// Unlike Refresh, this directly updates the result instead of doing so through polling
// This is now used internally by the AI chat plugin
type UpdateableResult struct {
	Id    string
	Title *string
}
