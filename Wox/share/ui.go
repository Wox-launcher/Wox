package share

import (
	"context"
	"wox/util"
)

type ChangedQuery struct {
	QueryType      string
	QueryText      string
	QuerySelection util.Selection
}

func (c ChangedQuery) IsEmpty() bool {
	return c.QueryText == "" && c.QuerySelection.String() == ""
}

func (c ChangedQuery) String() string {
	if c.QueryText != "" {
		return c.QueryText
	}

	return c.QuerySelection.String()
}

type UI interface {
	ChangeQuery(ctx context.Context, query ChangedQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context, showContext ShowContext)
	ToggleApp(ctx context.Context)
	Notify(ctx context.Context, title string, description string)
	GetServerPort(ctx context.Context) int
	ChangeTheme(ctx context.Context, theme Theme)
	OpenSettingWindow(ctx context.Context)
	GetAllThemes(ctx context.Context) []Theme
	PickFiles(ctx context.Context, params PickFilesParams) []string
}

type ShowContext struct {
	SelectAll bool
}

type PickFilesParams struct {
	IsDirectory bool
}

var ExitApp func(ctx context.Context)
