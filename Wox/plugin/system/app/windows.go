package app

import (
	"context"
	"errors"
	"github.com/parsiya/golnk"
	"strings"
	"wox/plugin"
	"wox/util"
)

type WindowsRetriever struct {
	api plugin.API
}

func (a *WindowsRetriever) GetPlatform() string {
	return util.PlatformWindows
}

func (a *WindowsRetriever) GetAppDirectories(ctx context.Context) []string {
	return []string{
		"C:\\ProgramData\\Microsoft\\Windows\\Start Menu\\Programs",
	}
}

func (a *WindowsRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"exe", "lnk"}
}

func (a *WindowsRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	if strings.HasSuffix(path, ".lnk") {
		return a.parseShortcut(ctx, path)
	}

	return appInfo{}, errors.New("not implemented")
}

func (a *WindowsRetriever) parseShortcut(ctx context.Context, path string) (appInfo, error) {
	f, lnkErr := lnk.File(path)
	if lnkErr != nil {
		return appInfo{}, lnkErr
	}

	var targetPath = ""
	if f.LinkInfo.LocalBasePath != "" {
		targetPath = f.LinkInfo.LocalBasePath
	}
	if f.LinkInfo.LocalBasePathUnicode != "" {
		targetPath = f.LinkInfo.LocalBasePathUnicode
	}
	if targetPath == "" {
		return appInfo{}, errors.New("no target path found")
	}

	return appInfo{
		Name: f.StringData.NameString,
		Path: targetPath,
	}, nil
}
