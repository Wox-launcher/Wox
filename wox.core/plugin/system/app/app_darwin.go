package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework UniformTypeIdentifiers
#include <stdlib.h>
#include <sys/sysctl.h>

const unsigned char *GetPrefPaneIcon(const char *prefPanePath, size_t *length);
const unsigned char *GenerateSFSymbolIcon(const char *symbolName, const char *colorName, const char *iconStyle, size_t *length);
char* GetLocalizedAppDisplayName(const char *appPath);
char* GetPreferredLanguages(void);
int get_process_list(struct kinfo_proc **procList, size_t *procCount);
char* get_process_path(pid_t pid);
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unsafe"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/fileicon"
	"wox/util/imagecache"
	"wox/util/shell"

	"github.com/mitchellh/go-homedir"
	"github.com/struCoder/pidusage"
	"github.com/tidwall/gjson"
	"howett.net/plist"
)

var appRetriever = &MacRetriever{}

var defaultAppIcon = "/System/Library/CoreServices/CoreTypes.bundle/Contents/Resources/GenericApplicationIcon.icns"

var macPreferredLanguages = sync.OnceValue(func() string {
	cLanguages := C.GetPreferredLanguages()
	if cLanguages == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cLanguages))
	return C.GoString(cLanguages)
})

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
	}
}

func (a *MacRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"app"}
}

// GetAppModifiedUnix fingerprints bundle metadata that can change without touching the .app directory.
func (a *MacRetriever) GetAppModifiedUnix(appPath string, fileInfo os.FileInfo) int64 {
	sources := make(map[string]os.FileInfo)
	if fileInfo != nil {
		sources[appPath] = fileInfo
	}
	addSource := func(sourcePath string) {
		if info, err := os.Stat(sourcePath); err == nil {
			sources[sourcePath] = info
		}
	}
	for _, contentsDir := range macAppContentsDirectories(appPath) {
		addSource(filepath.Join(contentsDir, "Info.plist"))
		resourcesDir := filepath.Join(contentsDir, "Resources")
		addSource(filepath.Join(resourcesDir, "InfoPlist.loctable"))
		for _, localizedFile := range macAppLocalizedStringsFiles(resourcesDir) {
			addSource(localizedFile)
		}
	}
	if iconPath, err := fileicon.ResolveMacAppBundleIconPath(appPath); err == nil {
		addSource(iconPath)
	}

	sourcePaths := make([]string, 0, len(sources))
	for sourcePath := range sources {
		sourcePaths = append(sourcePaths, sourcePath)
	}
	sort.Strings(sourcePaths)
	fingerprint := fnv.New64a()
	_, _ = fmt.Fprintf(fingerprint, "languages\x00%s\x00", macPreferredLanguages())
	for _, sourcePath := range sourcePaths {
		info := sources[sourcePath]
		_, _ = fmt.Fprintf(fingerprint, "%s\x00%d\x00%d\x00", sourcePath, info.ModTime().UnixNano(), info.Size())
	}
	value := int64(fingerprint.Sum64())
	if value == 0 {
		return 1
	}
	return value
}

func (a *MacRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	appName, err := a.getAppNameFromMdls(path)
	if err != nil || appName == "(null)" || strings.TrimSpace(appName) == "" {
		// Spotlight/mdls unavailable or returned invalid value, fallback to localized bundle name, Info.plist, then filename.
		if err != nil {
			a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("failed to get app name from mdls(%s): %s, falling back to localized bundle name/Info.plist/filename", path, err.Error()))
		} else {
			a.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("mdls returned empty/(null) for %s, falling back to localized bundle name/Info.plist/filename", path))
		}

		nameFromPlist, plistErr := a.getAppNameFromPlist(ctx, path)
		if localizedName := strings.TrimSpace(a.getLocalizedAppName(path)); localizedName != "" {
			appName = localizedName
		} else if plistErr == nil && strings.TrimSpace(nameFromPlist) != "" {
			appName = nameFromPlist
		} else {
			base := filepath.Base(path)
			appName = base
			a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("using filename as app name for %s (plistErr=%v)", path, plistErr))
		}
	}

	// Strip .app extension
	if strings.HasSuffix(appName, ".app") {
		appName = appName[:len(appName)-4]
	}

	info := appInfo{
		Name:            appName,
		Path:            path,
		SearchableNames: a.getAppSearchableNames(ctx, path, appName),
	}
	icon, iconErr := a.getMacAppIcon(ctx, path)
	if iconErr != nil {
		a.api.Log(ctx, plugin.LogLevelError, iconErr.Error())
	}
	info.Icon = icon
	if iconSourcePath, iconSourceErr := fileicon.ResolveMacAppBundleIconPath(path); iconSourceErr == nil {
		info.IconSourcePath = iconSourcePath
	} else {
		info.IsDefaultIcon = true
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("app %s has no dedicated bundle icon: %s", path, iconSourceErr.Error()))
	}
	if info.Icon.ImageData == defaultAppIcon {
		info.IsDefaultIcon = true
	}

	return info, nil
}

func macAppContentsDirectories(appPath string) []string {
	return []string{
		filepath.Join(appPath, "Contents"),
		filepath.Join(appPath, "WrappedBundle"),
	}
}

func macAppLocalizedStringsFiles(resourcesDir string) []string {
	localizedFiles := []string{filepath.Join(resourcesDir, "InfoPlist.strings")}
	entries, _ := os.ReadDir(resourcesDir)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".lproj") {
			localizedFiles = append(localizedFiles, filepath.Join(resourcesDir, entry.Name(), "InfoPlist.strings"))
		}
	}
	return localizedFiles
}

func resolveAppIdentityForPlatform(ctx context.Context, info appInfo) string {
	lowerPath := strings.ToLower(strings.TrimSpace(info.Path))
	if !strings.HasSuffix(lowerPath, ".app") {
		return ""
	}

	bundleID, err := getBundleIdentifierFromAppPath(info.Path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(bundleID)
}

func getBundleIdentifierFromAppPath(appPath string) (string, error) {
	plistPaths := []string{
		path.Join(appPath, "Contents", "Info.plist"),
		path.Join(appPath, "WrappedBundle", "Info.plist"),
	}

	for _, plistPath := range plistPaths {
		plistFile, err := os.Open(plistPath)
		if err != nil {
			continue
		}

		var plistData map[string]any
		decodeErr := plist.NewDecoder(plistFile).Decode(&plistData)
		_ = plistFile.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("failed to decode Info.plist: %w", decodeErr)
		}

		if bundleID, ok := plistData["CFBundleIdentifier"].(string); ok && strings.TrimSpace(bundleID) != "" {
			return bundleID, nil
		}
	}

	return "", fmt.Errorf("bundle identifier not found")
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

func (a *MacRetriever) getLocalizedAppName(appPath string) string {
	cPath := C.CString(appPath)
	defer C.free(unsafe.Pointer(cPath))
	cName := C.GetLocalizedAppDisplayName(cPath)
	if cName != nil {
		defer C.free(unsafe.Pointer(cName))
		if name := strings.TrimSpace(C.GoString(cName)); name != "" {
			return name
		}
	}
	names := a.getLocalizedAppNames(appPath)
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func (a *MacRetriever) getLocalizedAppNames(appPath string) []string {
	var names []string
	for _, contentsDir := range macAppContentsDirectories(appPath) {
		names = append(names, readBundleDisplayNames(filepath.Join(contentsDir, "Info.plist"), false)...)
		resourcesDir := filepath.Join(contentsDir, "Resources")
		names = append(names, readBundleDisplayNames(filepath.Join(resourcesDir, "InfoPlist.loctable"), true)...)
		for _, localizedFile := range macAppLocalizedStringsFiles(resourcesDir) {
			names = append(names, readBundleDisplayNames(localizedFile, false)...)
		}
	}
	return util.UniqueStrings(names)
}

// readBundleDisplayNames parses XML, binary, or OpenStep plist dictionaries without creating NSBundle caches.
func readBundleDisplayNames(plistPath string, nested bool) []string {
	plistFile, err := os.Open(plistPath)
	if err != nil {
		return nil
	}
	defer plistFile.Close()

	var values map[string]any
	if err := plist.NewDecoder(plistFile).Decode(&values); err != nil {
		return nil
	}
	var names []string
	names = append(names, bundleDisplayNames(values)...)
	if nested {
		keys := make([]string, 0, len(values))
		for key := range values {
			if key != "LocProvenance" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			switch localizedValues := values[key].(type) {
			case map[string]any:
				names = append(names, bundleDisplayNames(localizedValues)...)
			case map[string]string:
				for _, nameKey := range []string{"CFBundleDisplayName", "CFBundleName"} {
					names = append(names, localizedValues[nameKey])
				}
			}
		}
	}
	return util.UniqueStrings(names)
}

// bundleDisplayNames returns the two plist keys Finder uses for localized app names.
func bundleDisplayNames(values map[string]any) []string {
	var names []string
	for _, key := range []string{"CFBundleDisplayName", "CFBundleName"} {
		if name, ok := values[key].(string); ok && strings.TrimSpace(name) != "" {
			names = append(names, strings.TrimSpace(name))
		}
	}
	return names
}

func (a *MacRetriever) getAppSearchableNames(ctx context.Context, appPath string, primaryName string) []string {
	searchableNames := []string{
		primaryName,
	}
	// Bug fix: a single current-locale bundle name is not enough when Spotlight
	// is disabled. macOS stores display names such as Korean Calculator in
	// InfoPlist.loctable/InfoPlist.strings resources, so every localized bundle
	// alias must be indexed.
	searchableNames = append(searchableNames, a.getLocalizedAppNames(appPath)...)

	if plistName, err := a.getAppNameFromPlist(ctx, appPath); err == nil {
		searchableNames = append(searchableNames, plistName)
	}

	baseName := filepath.Base(appPath)
	searchableNames = append(searchableNames, baseName)
	if strings.HasSuffix(baseName, ".app") {
		searchableNames = append(searchableNames, strings.TrimSuffix(baseName, ".app"))
	}

	var filtered []string
	for _, name := range searchableNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		filtered = append(filtered, name)
	}

	return util.UniqueStrings(filtered)
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

func (a *MacRetriever) generateSFSymbolIconBytes(symbolName, colorName, iconStyle string) (*C.uchar, C.size_t) {
	csymbol := C.CString(symbolName)
	defer C.free(unsafe.Pointer(csymbol))
	ccolor := C.CString(colorName)
	defer C.free(unsafe.Pointer(ccolor))
	cstyle := C.CString(iconStyle)
	defer C.free(unsafe.Pointer(cstyle))

	var length C.size_t
	bytesPtr := C.GenerateSFSymbolIcon(csymbol, ccolor, cstyle, &length)
	return bytesPtr, length
}

func (a *MacRetriever) getPrefPaneIconBytes(prefPanePath string) (*C.uchar, C.size_t) {
	cpath := C.CString(prefPanePath)
	defer C.free(unsafe.Pointer(cpath))

	var length C.size_t
	bytesPtr := C.GetPrefPaneIcon(cpath, &length)
	return bytesPtr, length
}

func (a *MacRetriever) getPrefPaneIconCachePath(prefPanePath string) string {
	return filepath.Join(util.GetLocation().GetCacheDirectory(), "images", fmt.Sprintf("prefpane_%s.png", filepath.Base(prefPanePath)))
}

func (a *MacRetriever) getCachedPrefPaneIconPath(prefPanePath string) (string, bool) {
	cachePath := a.getPrefPaneIconCachePath(prefPanePath)
	if info, err := os.Stat(cachePath); err == nil {
		imagecache.Touch(util.NewTraceContext(), cachePath, info)
		return cachePath, true
	}
	return cachePath, false
}

func (a *MacRetriever) savePrefPaneIconToCache(ctx context.Context, prefPanePath string, iconBytes *C.uchar, length C.size_t) (string, error) {
	if iconBytes != nil {
		defer C.free(unsafe.Pointer(iconBytes))
	}
	if iconBytes == nil || length == 0 {
		return "", errors.New("empty preference pane icon bytes")
	}

	cachePath, exists := a.getCachedPrefPaneIconPath(prefPanePath)
	if exists {
		return cachePath, nil
	}

	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return "", err
	}

	data := C.GoBytes(unsafe.Pointer(iconBytes), C.int(length))
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return "", err
	}

	return cachePath, nil
}

// GetExtraAppPaths discovers apps outside the normal directories without parsing their bundles.
func (a *MacRetriever) GetExtraAppPaths(ctx context.Context) ([]string, error) {
	out, err := shell.RunOutput("system_profiler", "SPApplicationsDataType", "-json")
	if err != nil {
		return nil, fmt.Errorf("failed to get extra apps: %s", err.Error())
	}

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

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("found %d extra app paths", len(appPaths)))
	return appPaths, nil
}

func (a *MacRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	return a.getSystemSettingsApps(ctx), nil
}

func (a *MacRetriever) getSystemSettingsApps(ctx context.Context) []appInfo {
	var apps []appInfo

	for key, info := range systemSettings {
		if info.URI == "" {
			continue
		}

		// Generate icon using SF Symbol
		iconStyle := info.IconStyle
		if iconStyle == "" {
			iconStyle = "filled"
		}

		var icon common.WoxImage
		// Optimization: the previous flow generated every SF Symbol icon before
		// checking the disk cache, which left large native CG image allocations in
		// the core process on every startup. Check the stable cache key first so
		// cached System Settings entries do not touch AppKit image rendering.
		cacheKey := "virtual_" + key + ".prefPane"
		if iconPath, exists := a.getCachedPrefPaneIconPath(cacheKey); exists {
			icon = common.NewWoxImageAbsolutePath(iconPath)
		} else if iconBytes, iconLen := a.generateSFSymbolIconBytes(info.SFSymbol, info.BackgroundColor, iconStyle); iconLen > 0 {
			if iconPath, err := a.savePrefPaneIconToCache(ctx, cacheKey, iconBytes, iconLen); err == nil {
				icon = common.NewWoxImageAbsolutePath(iconPath)
			}
		}

		if icon.ImageData == "" {
			icon = common.WoxIcon
		}

		// Build full URI with x-apple.systempreferences scheme
		fullURI := "x-apple.systempreferences:" + info.URI

		// Create an app entry for each display name (supports aliases)
		for _, displayName := range info.DisplayNames {
			apps = append(apps, appInfo{
				Name: displayName,
				Path: fullURI,
				Icon: icon,
			})
		}
	}

	return apps
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
