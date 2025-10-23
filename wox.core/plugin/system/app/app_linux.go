package app

import (
	"context"
	"errors"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"
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

func (a *LinuxRetriever) GetPid(ctx context.Context, app appInfo) int {
	return 0
}

func (a *LinuxRetriever) GetProcessStat(ctx context.Context, app appInfo) (*ProcessStat, error) {
	return nil, errors.New("not implemented")
}

func (a *LinuxRetriever) OpenAppFolder(ctx context.Context, app appInfo) error {
	return shell.OpenFileInFolder(app.Path)
}
