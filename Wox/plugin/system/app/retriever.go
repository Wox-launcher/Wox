package app

import (
	"context"
	"wox/plugin"
)

type Retriever interface {
	UpdateAPI(api plugin.API)
	GetPlatform() string
	GetAppDirectories(ctx context.Context) []appDirectory
	GetAppExtensions(ctx context.Context) []string
	ParseAppInfo(ctx context.Context, path string) (appInfo, error)
	GetExtraApps(ctx context.Context) ([]appInfo, error)
	GetPid(ctx context.Context, app appInfo) int
}
