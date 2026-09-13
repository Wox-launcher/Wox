package app

import (
	"context"
	"os"
	"wox/plugin"
)

type ProcessStat struct {
	CPU    float64 // CPU usage percentage
	Memory float64 // Memory usage in bytes
}

type Retriever interface {
	UpdateAPI(api plugin.API)
	GetPlatform() string
	GetAppDirectories(ctx context.Context) []appDirectory
	GetAppExtensions(ctx context.Context) []string
	ParseAppInfo(ctx context.Context, path string) (appInfo, error)
	GetExtraApps(ctx context.Context) ([]appInfo, error)
	PrepareExtraApps(ctx context.Context, apps []appInfo) []appInfo
	GetPid(ctx context.Context, app appInfo) int
	GetProcessStat(ctx context.Context, app appInfo) (*ProcessStat, error)
	OpenAppFolder(ctx context.Context, app appInfo) error
}

// extraAppPathRetriever lets platforms route discovered app paths through the shared app cache.
type extraAppPathRetriever interface {
	GetExtraAppPaths(ctx context.Context) ([]string, error)
}

// extraAppLister returns the current AppsFolder-backed extras without a full
// directory scan. existingByPath lets the platform reuse cached icons.
type extraAppLister interface {
	ListExtraApps(ctx context.Context, existingByPath map[string]appInfo) ([]appInfo, error)
}

// appModifiedUnixRetriever includes platform-specific metadata sources in cache freshness.
type appModifiedUnixRetriever interface {
	GetAppModifiedUnix(appPath string, fileInfo os.FileInfo) int64
}
