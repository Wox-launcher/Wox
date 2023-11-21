package app

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/mitchellh/go-homedir"
	"howett.net/plist"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"wox/plugin"
	"wox/util"
)

type MacRetriever struct {
	api plugin.API
}

func (a *MacRetriever) GetPlatform() string {
	return util.PlatformMacOS
}

func (a *MacRetriever) GetAppDirectories(ctx context.Context) []string {
	userHomeApps, _ := homedir.Expand("~/Applications")
	return []string{
		userHomeApps,
		"/Applications",
		"/Applications/Utilities",
		"/System/Applications",
		"/System/Library/PreferencePanes",
		"/System/Library/CoreServices",
	}
}

func (a *MacRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"app"}
}

func (a *MacRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	out, err := util.ShellRunOutput("mdls", "-name", "kMDItemDisplayName", "-raw", path)
	if err != nil {
		msg := fmt.Sprintf("failed to get app name from mdls(%s): %s", path, err.Error())
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			msg = fmt.Sprintf("failed to get app name from mdls(%s): %s", path, exitError.Stderr)
		}
		return appInfo{}, errors.New(msg)
	}

	appName := strings.TrimSpace(string(out))
	if strings.HasSuffix(appName, ".app") {
		appName = appName[:len(appName)-4]
	}

	info := appInfo{
		Name: appName,
		Path: path,
	}
	icon, iconErr := a.getMacAppIcon(ctx, path)
	if iconErr != nil {
		a.api.Log(ctx, iconErr.Error())
	}
	info.Icon = icon

	return info, nil
}

func (a *MacRetriever) getMacAppIcon(ctx context.Context, appPath string) (plugin.WoxImage, error) {
	// md5 iconPath
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(appPath)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("app_%s.png", iconPathMd5))
	if _, err := os.Stat(iconCachePath); err == nil {
		return plugin.WoxImage{
			ImageType: plugin.WoxImageTypeAbsolutePath,
			ImageData: iconCachePath,
		}, nil
	}

	rawImagePath, iconErr := a.getMacAppIconImagePath(ctx, appPath)
	if iconErr != nil {
		return plugin.WoxImage{}, iconErr
	}

	if strings.HasSuffix(rawImagePath, ".icns") {
		//use sips to convert icns to png
		//sips -s format png /Applications/Calculator.app/Contents/Resources/AppIcon.icns --out /tmp/wox-app-icon.png
		out, openErr := util.ShellRunOutput("sips", "-s", "format", "png", rawImagePath, "--out", iconCachePath)
		if openErr != nil {
			msg := fmt.Sprintf("failed to convert icns to png: %s", openErr.Error())
			if out != nil {
				msg = fmt.Sprintf("%s, output: %s", msg, string(out))
			}
			return plugin.WoxImage{}, errors.New(msg)
		}
	} else {
		originF, originErr := os.Open(rawImagePath)
		if originErr != nil {
			return plugin.WoxImage{}, fmt.Errorf("can't open origin image file: %s", originErr.Error())
		}

		//copy image to cache
		destF, destErr := os.Create(iconCachePath)
		if destErr != nil {
			return plugin.WoxImage{}, fmt.Errorf("can't create cache file: %s", destErr.Error())
		}
		defer destF.Close()

		if _, err := io.Copy(destF, originF); err != nil {
			return plugin.WoxImage{}, fmt.Errorf("can't copy image to cache: %s", err.Error())
		}
	}

	a.api.Log(ctx, fmt.Sprintf("app icon cache created: %s", iconCachePath))
	return plugin.WoxImage{
		ImageType: plugin.WoxImageTypeAbsolutePath,
		ImageData: iconCachePath,
	}, nil
}

func (a *MacRetriever) getMacAppIconImagePath(ctx context.Context, appPath string) (string, error) {
	iconPath, infoPlistErr := a.parseMacAppIconFromInfoPlist(ctx, appPath)
	if infoPlistErr == nil {
		return iconPath, nil
	}
	a.api.Log(ctx, fmt.Sprintf("get icon from info.plist fail, path=%s, err=%s", appPath, infoPlistErr.Error()))

	//return default icon
	return "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns", nil
}

func (a *MacRetriever) parseMacAppIconFromInfoPlist(ctx context.Context, appPath string) (string, error) {
	plistPath := path.Join(appPath, "Contents", "Info.plist")
	plistFile, openErr := os.Open(plistPath)
	if openErr != nil {
		plistPath = path.Join(appPath, "WrappedBundle", "Info.plist")
		plistFile, openErr = os.Open(plistPath)
		if openErr != nil {
			return "", fmt.Errorf("can't find Info.plist in this app: %s", openErr.Error())
		}
	}
	defer plistFile.Close()

	decoder := plist.NewDecoder(plistFile)
	var plistData map[string]any
	decodeErr := decoder.Decode(&plistData)
	if decodeErr != nil {
		return "", fmt.Errorf("failed to decode Info.plist: %s", decodeErr.Error())
	}
	iconName, exist := plistData["CFBundleIconFile"].(string)
	if exist {
		if !strings.HasSuffix(iconName, ".icns") {
			iconName = iconName + ".icns"
		}
		iconPath := path.Join(appPath, "Contents", "Resources", iconName)
		if _, statErr := os.Stat(iconPath); os.IsNotExist(statErr) {
			return "", fmt.Errorf("icon file not found: %s", iconPath)
		}

		return iconPath, nil
	} else {
		return "", fmt.Errorf("info plist doesnt have CFBundleIconFile property")
	}
}
