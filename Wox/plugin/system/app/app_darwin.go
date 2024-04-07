package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

const unsigned char *GetPrefPaneIcon(const char *prefPanePath, size_t *length);
*/
import "C"
import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/mitchellh/go-homedir"
	"howett.net/plist"
	"image"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"unsafe"
	"wox/plugin"
	"wox/util"
)

var appRetriever = &MacRetriever{}

type MacRetriever struct {
	api plugin.API
}

func (a *MacRetriever) UpdateAPI(api plugin.API) {
	a.api = api
}

func (a *MacRetriever) GetPlatform() string {
	return util.PlatformMacOS
}

func (a *MacRetriever) GetAppDirectories(ctx context.Context) []appDirectory {
	userHomeApps, _ := homedir.Expand("~/Applications")
	return []appDirectory{
		{
			Path: userHomeApps, Recursive: false,
		},
		{
			Path: "/Applications", Recursive: false,
		},
		{
			Path: "/Applications/Utilities", Recursive: false,
		},
		{
			Path: "/System/Applications", Recursive: false,
		},
		{
			Path: "/System/Library/PreferencePanes", Recursive: false,
		},
	}
}

func (a *MacRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"app", "prefPane"}
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
	for _, extension := range a.GetAppExtensions(ctx) {
		if strings.HasSuffix(appName, "."+extension) {
			appName = appName[:len(appName)-len(extension)-1]
		}
	}

	info := appInfo{
		Name: appName,
		Path: path,
	}
	icon, iconErr := a.getMacAppIcon(ctx, path)
	if iconErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, iconErr.Error())
	}
	info.Icon = icon

	return info, nil
}

func (a *MacRetriever) getMacAppIcon(ctx context.Context, appPath string) (plugin.WoxImage, error) {
	if v, ok := iconsMap[appPath]; ok {
		return v, nil
	}

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

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app icon cache created: %s", iconCachePath))
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
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("get icon from info.plist fail, try to parse with cgo path=%s, err=%s", appPath, infoPlistErr.Error()))

	iconPath2, cgoErr := a.parseMacAppIconFromCgo(ctx, appPath)
	if cgoErr == nil {
		return iconPath2, nil
	} else {
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("get icon from cgo fail, return default icon path=%s, err=%s", appPath, cgoErr.Error()))
	}

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

	// handle CFBundleIconFile
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
	}

	// handle CFBundleIcons if not found above
	icons, cfBundleIconsExist := plistData["CFBundleIcons"].(map[string]any)
	if cfBundleIconsExist {
		primaryIcon, cfBundlePrimaryIconExist := icons["CFBundlePrimaryIcon"].(map[string]any)
		if cfBundlePrimaryIconExist {
			iconFiles, cfBundleIconFilesExist := primaryIcon["CFBundleIconFiles"].([]any)
			if cfBundleIconFilesExist {
				lastIconName := iconFiles[len(iconFiles)-1].(string)
				iconPath := ""
				files, readDirErr := os.ReadDir(path.Dir(plistPath))
				if readDirErr == nil {
					for _, file := range files {
						if strings.HasPrefix(file.Name(), lastIconName) {
							iconPath = path.Join(path.Dir(plistPath), file.Name())
							break
						}
					}
				}
				if iconPath != "" {
					return iconPath, nil
				}
			}
		}
	}

	return "", fmt.Errorf("info plist doesn't have CFBundleIconFile property")
}

func (a *MacRetriever) parseMacAppIconFromCgo(ctx context.Context, appPath string) (string, error) {
	cPath := C.CString(appPath)
	defer C.free(unsafe.Pointer(cPath))

	var length C.size_t
	cIcon := C.GetPrefPaneIcon(cPath, &length)
	if cIcon != nil {
		defer C.free(unsafe.Pointer(cIcon))
		pngBytes := C.GoBytes(unsafe.Pointer(cIcon), C.int(length))
		imgReader := bytes.NewReader(pngBytes)
		img, _, err := image.Decode(imgReader)
		if err != nil {
			return "", fmt.Errorf("failed to decode icon image with system api: %v", err)
		}

		iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(appPath)))
		iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("app_cgo_%s.png", iconPathMd5))
		saveErr := imaging.Save(img, iconCachePath)
		if saveErr != nil {
			return "", saveErr
		}

		return iconCachePath, nil
	}

	return "", errors.New("no icon found with system api")
}
