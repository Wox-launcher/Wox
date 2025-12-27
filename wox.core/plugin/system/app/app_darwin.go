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
	"context"
	"errors"
	"fmt"
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
	"wox/util/fileicon"
	"wox/util/shell"

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
		if err != nil || appName == "(null)" || strings.TrimSpace(appName) == "" {
			// Spotlight/mdls unavailable or returned invalid value, fallback to Info.plist then filename
			if err != nil {
				a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to get app name from mdls(%s): %s, falling back to Info.plist/filename", path, err.Error()))
			} else {
				a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("mdls returned empty/(null) for %s, falling back to Info.plist/filename", path))
			}

			if nameFromPlist, err2 := a.getAppNameFromPlist(ctx, path); err2 == nil && strings.TrimSpace(nameFromPlist) != "" {
				appName = nameFromPlist
			} else {
				base := filepath.Base(path)
				appName = base
				a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("using filename as app name for %s (plistErr=%v)", path, err2))
			}
		}
	}

	// Strip extension suffix (.app/.prefPane)
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

func (a *MacRetriever) getAppNameFromPlist(ctx context.Context, appPath string) (string, error) {
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
	if err := decoder.Decode(&plistData); err != nil {
		return "", fmt.Errorf("failed to decode Info.plist: %s", err.Error())
	}

	// Prefer CFBundleDisplayName, then CFBundleName, then CFBundleExecutable
	if name, ok := plistData["CFBundleDisplayName"].(string); ok && strings.TrimSpace(name) != "" {
		return name, nil
	}
	if name, ok := plistData["CFBundleName"].(string); ok && strings.TrimSpace(name) != "" {
		return name, nil
	}
	if name, ok := plistData["CFBundleExecutable"].(string); ok && strings.TrimSpace(name) != "" {
		return name, nil
	}

	return "", fmt.Errorf("no suitable display name keys in Info.plist")
}

func (a *MacRetriever) getMacAppIcon(ctx context.Context, appPath string) (common.WoxImage, error) {
	if iconPath, err := fileicon.GetFileIconByPath(ctx, appPath); err == nil {
		return common.NewWoxImageAbsolutePath(iconPath), nil
	}

	return common.WoxImage{
		ImageType: common.WoxImageTypeAbsolutePath,
		ImageData: defaultAppIcon,
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
