package app

import (
	"context"
	"wox/plugin"
)

type appInfo struct {
	Name string
	Path string
	Icon plugin.WoxImage
}

type Retriever interface {
	GetPlatform() string
	GetAppDirectories(ctx context.Context) []string
	GetAppExtensions(ctx context.Context) []string
	ParseAppInfo(ctx context.Context, path string) (appInfo, error)
}
