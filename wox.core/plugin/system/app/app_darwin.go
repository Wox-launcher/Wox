package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>
#include <sys/sysctl.h>

const unsigned char *GetPrefPaneIcon(const char *prefPanePath, size_t *length);
int get_process_list(struct kinfo_proc **procList, size_t *procCount);
char* get_process_path(pid_t pid);
*/
import "C"
import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	"github.com/disintegration/imaging"
	"github.com/mitchellh/go-homedir"
	"github.com/struCoder/pidusage"
	"github.com/tidwall/gjson"
	"howett.net/plist"
)

var appRetriever = &MacRetriever{}

var defaultAppIcon = "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns"

type processInfo struct {
	Pid  int
	Path string
}

type MacRetriever struct {
	runningProcesses      []processInfo
	lastProcessUpdateTime int64
	api                   plugin.API
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
			Path: "/Applications", Recursive: true, RecursiveDepth: 2,
		},
		{
			Path: "/System/Applications", Recursive: true, RecursiveDepth: 2,
		},
		{
			Path: "/System/Library/CoreServices/Applications", Recursive: false,
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
	var appName string
	var err error

	if strings.HasSuffix(path, ".prefPane") {
		appName, err = a.getPrefPaneName(path)
	} else {
		appName, err = a.getAppNameFromMdls(path)
	}

	if err != nil {
		return appInfo{}, err
	}

	if appName == "(null)" {
		appName = filepath.Base(path)
		a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to get app name from mdls(%s), using filename instead", path))
	}
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

func (a *MacRetriever) getPrefPaneName(path string) (string, error) {
	plistPath := filepath.Join(path, "Contents", "Info.plist")
	plistFile, err := os.Open(plistPath)
	if err != nil {
		return "", err
	}
	defer plistFile.Close()

	var plistData map[string]interface{}
	decoder := plist.NewDecoder(plistFile)
	if err := decoder.Decode(&plistData); err != nil {
		return "", err
	}

	if name, ok := plistData["CFBundleName"].(string); ok && name != "" {
		return name, nil
	}

	if name, ok := plistData["NSPrefPaneIconLabel"].(string); ok && name != "" {
		return name, nil
	}

	return filepath.Base(path), nil
}

func (a *MacRetriever) getAppNameFromMdls(path string) (string, error) {
	out, err := shell.RunOutput("mdls", "-name", "kMDItemDisplayName", "-raw", path)
	if err != nil {
		msg := fmt.Sprintf("failed to get app name from mdls(%s): %s", path, err.Error())
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			msg = fmt.Sprintf("failed to get app name from mdls(%s): %s", path, exitError.Stderr)
		}
		return "", errors.New(msg)
	}

	return strings.TrimSpace(string(out)), nil
}

func (a *MacRetriever) getMacAppIcon(ctx context.Context, appPath string) (common.WoxImage, error) {
	if v, ok := iconsMap[appPath]; ok {
		return v, nil
	}

	// md5 iconPath
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(appPath)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("app_%s.png", iconPathMd5))
	if _, err := os.Stat(iconCachePath); err == nil {
		return common.WoxImage{
			ImageType: common.WoxImageTypeAbsolutePath,
			ImageData: iconCachePath,
		}, nil
	}

	rawImagePath, iconErr := a.getMacAppIconImagePath(ctx, appPath)
	if iconErr != nil {
		// use default icon if no icon is found, and don't cache
		a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to get app icon for path: %s, %s", appPath, iconErr.Error()))
		return common.WoxImage{
			ImageType: common.WoxImageTypeAbsolutePath,
			ImageData: defaultAppIcon,
		}, nil
	}

	if strings.HasSuffix(rawImagePath, ".icns") {
		//use sips to convert icns to png
		//sips -s format png /Applications/Calculator.app/Contents/Resources/AppIcon.icns --out /tmp/wox-app-icon.png
		out, openErr := shell.RunOutput("sips", "-s", "format", "png", rawImagePath, "--out", iconCachePath)
		if openErr != nil {
			msg := fmt.Sprintf("failed to convert icns to png: %s", openErr.Error())
			if out != nil {
				msg = fmt.Sprintf("%s, output: %s", msg, string(out))
			}
			return common.WoxImage{}, errors.New(msg)
		}
	} else if strings.HasSuffix(strings.ToLower(rawImagePath), ".png") {
		// Check if it's a CgBI PNG (Apple's optimized PNG format)
		// CgBI PNGs can't be displayed properly in browsers/Flutter
		isCgbi, detectErr := isCgbiPNG(rawImagePath)
		if detectErr == nil && isCgbi {
			// Convert CgBI PNG to standard PNG using sips
			out, convErr := shell.RunOutput("sips", "-s", "format", "png", rawImagePath, "--out", iconCachePath)
			if convErr != nil {
				msg := fmt.Sprintf("failed to convert CgBI PNG to standard PNG: %s", convErr.Error())
				if out != nil {
					msg = fmt.Sprintf("%s, output: %s", msg, string(out))
				}
				return common.WoxImage{}, errors.New(msg)
			}
		} else {
			// Regular PNG, just copy
			originF, originErr := os.Open(rawImagePath)
			if originErr != nil {
				return common.WoxImage{}, fmt.Errorf("can't open origin image file: %s", originErr.Error())
			}
			defer originF.Close()

			destF, destErr := os.Create(iconCachePath)
			if destErr != nil {
				return common.WoxImage{}, fmt.Errorf("can't create cache file: %s", destErr.Error())
			}
			defer destF.Close()

			if _, err := io.Copy(destF, originF); err != nil {
				return common.WoxImage{}, fmt.Errorf("can't copy image to cache: %s", err.Error())
			}
		}
	} else {
		// Other image formats, just copy
		originF, originErr := os.Open(rawImagePath)
		if originErr != nil {
			return common.WoxImage{}, fmt.Errorf("can't open origin image file: %s", originErr.Error())
		}
		defer originF.Close()

		destF, destErr := os.Create(iconCachePath)
		if destErr != nil {
			return common.WoxImage{}, fmt.Errorf("can't create cache file: %s", destErr.Error())
		}
		defer destF.Close()

		if _, err := io.Copy(destF, originF); err != nil {
			return common.WoxImage{}, fmt.Errorf("can't copy image to cache: %s", err.Error())
		}
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("app icon cache created: %s", iconCachePath))
	return common.WoxImage{
		ImageType: common.WoxImageTypeAbsolutePath,
		ImageData: iconCachePath,
	}, nil
}

func (a *MacRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	//use `system_profiler SPApplicationsDataType -json` to get all apps
	out, err := shell.RunOutput("system_profiler", "SPApplicationsDataType", "-json")
	if err != nil {
		return nil, fmt.Errorf("failed to get extra apps: %s", err.Error())
	}

	//parse json
	results := gjson.Get(string(out), "SPApplicationsDataType")
	if !results.Exists() {
		return nil, errors.New("failed to parse extra apps")
	}
	var appPaths []string
	for _, app := range results.Array() {
		appPath := app.Get("path").String()
		if appPath == "" {
			continue
		}
		if strings.HasPrefix(appPath, "/System/Library/CoreServices/") {
			continue
		}
		if strings.HasPrefix(appPath, "/System/Library/PrivateFrameworks/") {
			continue
		}
		if strings.HasPrefix(appPath, "/System/Library/Frameworks/") {
			continue
		}
		if !strings.HasSuffix(appPath, ".app") {
			continue
		}

		appPaths = append(appPaths, appPath)
	}

	// split into groups, so we can index apps in parallel
	var appPathGroups [][]string
	var groupSize = 25
	for i := 0; i < len(appPaths); i += groupSize {
		var end = i + groupSize
		if end > len(appPaths) {
			end = len(appPaths)
		}
		appPathGroups = append(appPathGroups, appPaths[i:end])
	}
	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("found extra %d apps in %d groups", len(appPaths), len(appPathGroups)))

	// index apps in parallel
	var appInfos []appInfo
	var waitGroup sync.WaitGroup
	var lock sync.Mutex
	waitGroup.Add(len(appPathGroups))
	for groupIndex := range appPathGroups {
		var appPathGroup = appPathGroups[groupIndex]
		util.Go(ctx, fmt.Sprintf("index extra app group: %d", groupIndex), func() {
			for _, appPath := range appPathGroup {
				info, getErr := a.ParseAppInfo(ctx, appPath)
				if getErr != nil {
					a.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("error getting extra app info for %s: %s", appPath, getErr.Error()))
					continue
				}

				lock.Lock()
				appInfos = append(appInfos, info)
				lock.Unlock()
			}
			waitGroup.Done()
		}, func() {
			waitGroup.Done()
		})
	}

	waitGroup.Wait()

	return appInfos, nil
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

	return "", fmt.Errorf("info plist err: %s, cgo err: %s", infoPlistErr.Error(), cgoErr.Error())
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

// isCgbiPNG checks if a PNG file is in Apple's CgBI format
// CgBI PNGs have a "CgBI" chunk in their header and can't be displayed properly in standard browsers
func isCgbiPNG(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 512 bytes to check for CgBI chunk
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}

	// Look for "CgBI" signature in the buffer
	cgbiSignature := []byte("CgBI")
	for i := 0; i <= n-4; i++ {
		if bytes.Equal(buffer[i:i+4], cgbiSignature) {
			return true, nil
		}
	}

	return false, nil
}

func (a *MacRetriever) GetPid(ctx context.Context, app appInfo) int {
	if util.GetSystemTimestamp()-a.lastProcessUpdateTime > 1000 {
		a.lastProcessUpdateTime = util.GetSystemTimestamp()
		a.runningProcesses = a.getRunningProcesses()
	}

	for _, proc := range a.runningProcesses {
		if strings.HasPrefix(proc.Path, app.Path) {
			return proc.Pid
		}
	}

	return 0
}

func (a *MacRetriever) getRunningProcesses() (infos []processInfo) {
	var procList *C.struct_kinfo_proc
	var procCount C.size_t

	if C.get_process_list(&procList, &procCount) == -1 {
		return
	}
	defer C.free(unsafe.Pointer(procList))

	slice := (*[1 << 30]C.struct_kinfo_proc)(unsafe.Pointer(procList))[:procCount:procCount]

	for _, proc := range slice {
		pid := proc.kp_proc.p_pid
		ppid := proc.kp_eproc.e_ppid
		if ppid > 1 {
			//only show user process
			continue
		}
		cPath := C.get_process_path(pid)
		if cPath == nil {
			continue
		}
		appPath := C.GoString(cPath)
		C.free(unsafe.Pointer(cPath))
		if appPath == "" {
			continue
		}

		infos = append(infos, processInfo{
			Pid:  int(pid),
			Path: appPath,
		})
	}

	return
}

func (a *MacRetriever) GetProcessStat(ctx context.Context, app appInfo) (*ProcessStat, error) {
	// For macOS, use pidusage library with the main process PID
	// Note: This doesn't handle multi-process apps like Chrome yet
	if app.Pid == 0 {
		return nil, fmt.Errorf("app %s is not running", app.Name)
	}

	stat, err := pidusage.GetStat(app.Pid)
	if err != nil {
		return nil, err
	}

	return &ProcessStat{
		CPU:    stat.CPU,
		Memory: stat.Memory,
	}, nil
}

func (a *MacRetriever) OpenAppFolder(ctx context.Context, app appInfo) error {
	return shell.OpenFileInFolder(app.Path)
}
