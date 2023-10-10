package system

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"wox/plugin"
	"wox/util"
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
	Icon string
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

	appCache, cacheErr := i.loadAppCache(ctx)
	if cacheErr == nil {
		i.apps = appCache
	}

	util.Go(ctx, "index apps", func() {
		i.indexApps(util.NewTraceContext())
	})
}

func (i *AppPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	for _, info := range i.apps {
		if util.StringContains(info.Name, query.Search) {
			results = append(results, plugin.QueryResult{
				Id:       uuid.NewString(),
				Title:    info.Name,
				SubTitle: info.Path,
				Icon:     plugin.WoxImage{},
				Action: func() {
					runErr := exec.Command("open", info.Path).Run()
					if runErr != nil {
						i.api.Log(ctx, fmt.Sprintf("error openning app %s: %s", info.Path, runErr.Error()))
					}
				},
			})
		}
	}

	return results
}

func (i *AppPlugin) indexApps(ctx context.Context) {
	startTimestamp := util.GetSystemTimestamp()
	var apps []appInfo
	if strings.ToLower(runtime.GOOS) == "darwin" {
		apps = i.getMacApps(ctx)
	}

	if len(apps) > 0 {
		i.api.Log(ctx, fmt.Sprintf("indexed %d apps", len(i.apps)))
		i.apps = apps

		var cachePath = i.getAppCachePath()
		cacheContent, marshalErr := json.Marshal(apps)
		if marshalErr != nil {
			i.api.Log(ctx, fmt.Sprintf("error marshalling app cache: %s", marshalErr.Error()))
			return
		}
		writeErr := os.WriteFile(cachePath, cacheContent, 0644)
		if writeErr != nil {
			i.api.Log(ctx, fmt.Sprintf("error writing app cache: %s", writeErr.Error()))
			return
		}
		i.api.Log(ctx, fmt.Sprintf("wrote app cache to %s", cachePath))
	}

	i.api.Log(ctx, fmt.Sprintf("indexed %d apps, cost %d ms", len(i.apps), util.GetSystemTimestamp()-startTimestamp))
}

func (i *AppPlugin) getAppCachePath() string {
	return path.Join(os.TempDir(), "wox-app-cache.json")
}

func (i *AppPlugin) loadAppCache(ctx context.Context) ([]appInfo, error) {
	startTimestamp := util.GetSystemTimestamp()
	i.api.Log(ctx, "start to load app cache")
	var cachePath = i.getAppCachePath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		i.api.Log(ctx, "app cache file not found")
		return nil, err
	}

	cacheContent, readErr := os.ReadFile(cachePath)
	if readErr != nil {
		i.api.Log(ctx, fmt.Sprintf("error reading app cache file: %s", readErr.Error()))
		return nil, readErr
	}

	var apps []appInfo
	unmarshalErr := json.Unmarshal(cacheContent, &apps)
	if unmarshalErr != nil {
		i.api.Log(ctx, fmt.Sprintf("error unmarshalling app cache file: %s", unmarshalErr.Error()))
		return nil, unmarshalErr
	}

	i.api.Log(ctx, fmt.Sprintf("loaded %d apps from cache, cost %d ms", len(apps), util.GetSystemTimestamp()-startTimestamp))
	return apps, nil
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

	info := appInfo{
		Name: strings.TrimSpace(string(out)),
		Path: path,
	}
	icon, iconErr := i.getMacAppIcon(ctx, path)
	if iconErr != nil {
		i.api.Log(ctx, fmt.Sprintf("failed to get app icon: %s", iconErr.Error()))
	}
	info.Icon = icon.String()

	return info, nil
}

func (i *AppPlugin) getMacAppIcon(ctx context.Context, appPath string) (plugin.WoxImage, error) {
	// md5 iconPath
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(appPath)))
	iconCachePath := path.Join(os.TempDir(), fmt.Sprintf("%s.png", iconPathMd5))
	if _, err := os.Stat(iconCachePath); err == nil {
		return plugin.WoxImage{
			ImageType: plugin.WoxImageTypeAbsolutePath,
			ImageData: iconCachePath,
		}, nil
	}

	i.api.Log(ctx, fmt.Sprintf("start to get app icon: %s", appPath))
	out, err := exec.Command("defaults", "read", fmt.Sprintf(`"%s"`, path.Join(appPath, "Contents", "Info.plist")), "CFBundleIconFile").Output()
	if err != nil {
		msg := fmt.Sprintf("failed to get app icon name from CFBundleIconFile(%s): %s", appPath, err.Error())
		if out != nil {
			msg = fmt.Sprintf("%s, output: %s", msg, string(out))
		}
		return plugin.WoxImage{}, errors.New(msg)
	}

	//TODO: some app may not have Info.plist file, instead they have a PkgInfo file

	iconName := strings.TrimSpace(string(out))
	if iconName == "" {
		iconName = "AppIcon.icns"
	}
	if !strings.HasSuffix(iconName, ".icns") {
		iconName = iconName + ".icns"
	}

	iconPath := path.Join(appPath, "Contents", "Resources", iconName)
	if _, statErr := os.Stat(iconPath); os.IsNotExist(statErr) {
		return plugin.WoxImage{}, fmt.Errorf("icon file %s not found", iconPath)
	}

	//use sips to convert icns to png
	//sips -s format png /Applications/Calculator.app/Contents/Resources/AppIcon.icns --out /tmp/wox-app-icon.png
	out, err = exec.Command("sips", "-s", "format", "png", iconPath, "--out", iconCachePath).Output()
	if err != nil {
		msg := fmt.Sprintf("failed to convert icns to png: %s", err.Error())
		if out != nil {
			msg = fmt.Sprintf("%s, output: %s", msg, string(out))
		}
		return plugin.WoxImage{}, errors.New(msg)
	}

	i.api.Log(ctx, fmt.Sprintf("app icon cache created: %s", iconCachePath))

	return plugin.WoxImage{
		ImageType: plugin.WoxImageTypeAbsolutePath,
		ImageData: iconCachePath,
	}, nil
}
