package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"os/exec"

	"bytes"
	"path/filepath"
	"strings"
	"wox/plugin"
	"wox/util"

	"github.com/adrg/xdg"

	"github.com/rkoesters/xdg/desktop"
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
	var appDirs []appDirectory
	xdgDataDirs := xdg.ApplicationDirs
	for _, dir := range xdgDataDirs {
		dir = strings.TrimSpace(dir)
		if strings.HasPrefix(dir, "/nix") || strings.HasPrefix(dir, "/etc") {
			continue
		}
		if strings.HasSuffix(dir, "/share/applications") {
			appDirs = append(appDirs, appDirectory{Path: dir, Recursive: true, RecursiveDepth: 2})
		}
	}
	return appDirs
}

func (a *LinuxRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"desktop"}
}

func (a *LinuxRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	if !strings.HasSuffix(path, ".desktop") {
		return appInfo{}, fmt.Errorf("not a desktop file: %s", path)
	}
	content, err := os.Open(path)
	if err != nil {
		return appInfo{}, err
	}
	defer content.Close()
	entry, err := desktop.New(content)
	if err != nil {
		return appInfo{}, err
	}
	var iconPath string = ""
	if filepath.IsAbs(entry.Icon) {
		if _, err := os.Stat(entry.Icon); err == nil {
			iconPath = entry.Icon
		}
	}
	if iconPath == "" {
		iconName := strings.TrimSuffix(entry.Icon, filepath.Ext(entry.Icon))
		cmd := exec.Command("geticons", iconName)
		var out bytes.Buffer
		cmd.Stdout = &out
		err = cmd.Run()
		if err == nil {
			iconPaths := strings.Split(strings.TrimSpace(out.String()), "\n")
			if len(iconPaths) != 0 {
				iconPath = iconPaths[len(iconPaths)-1]
			}
		}
	}
	icon := appIcon
	if iconPath != "" {
		switch strings.ToLower(filepath.Ext(iconPath)) {
		case ".png":
			icon = plugin.WoxImage{
				ImageType: plugin.WoxImageTypeAbsolutePath,
				ImageData: iconPath,
			}
		case ".svg":
			svgIcon, err := os.ReadFile(iconPath)
			if err == nil {
				svgString := string(svgIcon)
				icon = plugin.WoxImage{
					ImageType: plugin.WoxImageTypeSvg,
					ImageData: svgString,
				}
			}
		}
	}
	info := appInfo{
		Name: entry.Name,
		Path: path,
		Icon: icon,
		Type: AppTypeDesktop,
	}
	return info, nil
}

func (a *LinuxRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	return []appInfo{}, nil
}

func (a *LinuxRetriever) GetPid(ctx context.Context, app appInfo) int {
	return 0
}

func (a *LinuxRetriever) OpenAppFolder(ctx context.Context, app appInfo) error {
	return shell.OpenFileInFolder(app.Path)
}
