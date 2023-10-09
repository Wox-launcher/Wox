package system

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"wox/plugin"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AppPlugin{})
}

type AppPlugin struct {
	api  plugin.API
	apps []appInfo
}

type appInfo struct {
	Name string
	Path string
}

func (i *AppPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "ea2b6859-14bc-4c89-9c88-627da7379141",
		Name:          "App",
		Author:        "Wox Launcher",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Nodejs",
		Description:   "Search app installed on your computer",
		Icon:          "",
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (i *AppPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API

	if runtime.GOOS == "darwin" {
		i.apps = i.getMacApps(ctx)
	}

	i.api.Log(ctx, fmt.Sprintf("found %d apps", len(i.apps)))
}

func (i *AppPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, info := range i.apps {
		if strings.Contains(strings.ToLower(info.Name), strings.ToLower(query.Search)) {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    info.Name,
				SubTitle: info.Path,
				Icon:     plugin.WoxImage{},
				Action: func() bool {
					//plugin.GetStoreManager().Install(ctx, info)
					return false
				},
			})
		}
	}

	return results
}

func (i *AppPlugin) getMacApps(ctx context.Context) []appInfo {
	i.api.Log(ctx, "start to get mac apps")

	var appDirectories = []string{
		"/Applications",
		"/Applications/Utilities",
		"/System/Applications",
		"/System/Library/PreferencePanes",
	}

	var appDirectoryPaths []string
	for _, appDirectory := range appDirectories {
		// get all .app directories in appDirectory
		appDir, readErr := os.ReadDir(appDirectory)
		if readErr != nil {
			i.api.Log(ctx, fmt.Sprintf("error reading directory %s: %s", appDirectory, readErr.Error()))
			continue
		}

		for _, entry := range appDir {
			if strings.HasSuffix(entry.Name(), ".app") || strings.HasSuffix(entry.Name(), ".prefPane") {
				appDirectoryPaths = append(appDirectoryPaths, path.Join(appDirectory, entry.Name()))
			}
		}
	}

	var appInfos []appInfo
	for _, directoryPath := range appDirectoryPaths {
		info, getErr := i.getMacAppInfo(ctx, directoryPath)
		if getErr != nil {
			i.api.Log(ctx, fmt.Sprintf("error getting app info for %s: %s", directoryPath, getErr.Error()))
			continue
		}

		appInfos = append(appInfos, info)
	}

	return appInfos
}

func (i *AppPlugin) getMacAppInfo(ctx context.Context, path string) (appInfo, error) {
	out, err := exec.Command("mdls", "-name", "kMDItemDisplayName", "-raw", path).Output()
	if err != nil {
		return appInfo{}, fmt.Errorf("failed to get app name: %w", err)
	}

	return appInfo{
		Name: strings.TrimSpace(string(out)),
		Path: path,
	}, nil
}
