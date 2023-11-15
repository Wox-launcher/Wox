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

type UI interface {
	ChangeQuery(ctx context.Context, query ChangedQuery)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context, showContext ShowContext)
	ToggleApp(ctx context.Context)
	ShowMsg(ctx context.Context, title string, description string, icon string)
	GetServerPort(ctx context.Context) int
	ChangeTheme(ctx context.Context, theme string)
}

type ShowContext struct {
	SelectAll bool
}

var ExitApp func(ctx context.Context)
