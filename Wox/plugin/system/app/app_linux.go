package app

import (
	"context"
	"errors"
	"wox/plugin"
	"wox/util"
)

var appRetriever = &LinuxRetriever{}

type LinuxRetriever struct {
	api plugin.API
}

func (a *LinuxRetriever) UpdateAPI(api plugin.API) {
	a.api = api
}

func (a *LinuxRetriever) GetPlatform() string {
	return util.PlatformLinux
}

func (a *LinuxRetriever) GetAppDirectories(ctx context.Context) []appDirectory {
	return []appDirectory{
		{},
	}
}

func (a *LinuxRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{}
}

func (a *LinuxRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	return appInfo{}, errors.New("not implemented")
}

func (a *LinuxRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	return []appInfo{}, nil
}
