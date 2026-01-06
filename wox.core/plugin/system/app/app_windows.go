package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/fileicon"
	"wox/util/shell"
)

var (
	// Load version.dll for file version info
	version                = syscall.NewLazyDLL("version.dll")
	getFileVersionInfoSize = version.NewProc("GetFileVersionInfoSizeW")
	getFileVersionInfo     = version.NewProc("GetFileVersionInfoW")
	verQueryValue          = version.NewProc("VerQueryValueW")

	// Load psapi.dll for process enumeration
	psapi                = syscall.NewLazyDLL("psapi.dll")
	enumProcesses        = psapi.NewProc("EnumProcesses")
	getModuleFileNameExW = psapi.NewProc("GetModuleFileNameExW")
	getProcessMemoryInfo = psapi.NewProc("GetProcessMemoryInfo")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	openProcess          = kernel32.NewProc("OpenProcess")
	closeHandle          = kernel32.NewProc("CloseHandle")
	getProcessTimes      = kernel32.NewProc("GetProcessTimes")
)

const (
	// Process access rights
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
)

// PROCESS_MEMORY_COUNTERS_EX structure for GetProcessMemoryInfo
type PROCESS_MEMORY_COUNTERS_EX struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
	PrivateUsage               uintptr
}

var appRetriever = &WindowsRetriever{}

type processInfo struct {
	Pid  int
	Path string
}

type cpuSample struct {
	kernelTime int64
	userTime   int64
	timestamp  int64
}

type WindowsRetriever struct {
	api plugin.API

	uwpIconCache          sync.Map // map[string]string: appID -> icon path
	runningProcesses      []processInfo
	runningProcessesMutex sync.RWMutex // protects runningProcesses and lastProcessUpdateTime
	lastProcessUpdateTime int64
	cpuSamples            sync.Map // map[string]cpuSample: app path -> last CPU sample
}

func (a *WindowsRetriever) UpdateAPI(api plugin.API) {
	a.api = api
}

func (a *WindowsRetriever) GetPlatform() string {
	return util.PlatformWindows
}

func (a *WindowsRetriever) GetAppDirectories(ctx context.Context) []appDirectory {
	// get the start menu and program files directories for current user
	usr, _ := user.Current()
	return []appDirectory{
		{
			Path:           usr.HomeDir + "\\AppData\\Roaming\\Microsoft\\Windows\\Start Menu\\Programs",
			Recursive:      true,
			RecursiveDepth: 2,
		},
		{
			Path:              usr.HomeDir + "\\AppData\\Local",
			RecursiveExcludes: []string{"Temp"},
			Recursive:         true,
			RecursiveDepth:    4,
		},
		{
			Path:           "C:\\ProgramData\\Microsoft\\Windows\\Start Menu\\Programs",
			Recursive:      true,
			RecursiveDepth: 2,
		},
		{
			Path:           "C:\\Program Files",
			Recursive:      true,
			RecursiveDepth: 2,
		},
		{
			Path:           "C:\\Program Files (x86)",
			Recursive:      true,
			RecursiveDepth: 2,
		},
		{
			Path:           usr.HomeDir + "\\Desktop",
			Recursive:      false,
			RecursiveDepth: 0,
		},
	}
}

func (a *WindowsRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"exe", "lnk"}
}

func (a *WindowsRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	lowerPath := strings.ToLower(path)
	if strings.HasSuffix(lowerPath, ".lnk") {
		return a.parseShortcut(ctx, path)
	}
	if strings.HasSuffix(lowerPath, ".exe") {
		return a.parseExe(ctx, path)
	}

	return appInfo{}, errors.New("not implemented")
}

func (a *WindowsRetriever) parseShortcut(ctx context.Context, appPath string) (appInfo, error) {
	targetPath, resolveErr := a.resolveShortcutWithAPI(ctx, appPath)
	if resolveErr != nil {
		return appInfo{}, fmt.Errorf("failed to resolve shortcut %s: %v", appPath, resolveErr)
	}

	icon := appIcon
	if targetPath != "" && strings.HasSuffix(strings.ToLower(targetPath), ".exe") {
		if iconPath, iconErr := fileicon.GetFileIconByPath(ctx, targetPath); iconErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s, use default icon: %s", targetPath, iconErr.Error()))
		} else {
			icon = common.NewWoxImageAbsolutePath(iconPath)
		}
	} else if iconPath, iconErr := fileicon.GetFileIconByPath(ctx, appPath); iconErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s, use default icon: %s", appPath, iconErr.Error()))
	} else {
		icon = common.NewWoxImageAbsolutePath(iconPath)
	}

	displayName := strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))

	return appInfo{
		Name: displayName,
		Path: filepath.Clean(appPath),
		Icon: icon,
		Type: AppTypeDesktop,
	}, nil
}

func (a *WindowsRetriever) parseExe(ctx context.Context, appPath string) (appInfo, error) {
	// use default icon if no icon is found
	icon := appIcon
	if iconPath, iconErr := fileicon.GetFileIconByPath(ctx, appPath); iconErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s: %s", appPath, iconErr.Error()))
	} else {
		icon = common.NewWoxImageAbsolutePath(iconPath)
	}

	// Try to get display name from exe file version info
	displayName := a.getFileDisplayName(ctx, appPath)
	if displayName == "" {
		// Fallback to exe filename if no display name found
		displayName = strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
		util.GetLogger().Debug(ctx, fmt.Sprintf("Using exe filename as display name: %s", displayName))
	}

	return appInfo{
		Name: displayName,
		Path: filepath.Clean(appPath),
		Icon: icon,
		Type: AppTypeDesktop,
	}, nil
}

// resolveShortcutWithAPI uses Windows API to resolve shortcut target path with proper Unicode support
func (a *WindowsRetriever) resolveShortcutWithAPI(ctx context.Context, shortcutPath string) (string, error) {
	// Resolve via in-process COM only. No PowerShell fallback to avoid extra processes.
	targetPath, nativeErr := resolveShortcutTarget(ctx, shortcutPath)
	if nativeErr != nil {
		return "", fmt.Errorf("failed to resolve shortcut: %w", nativeErr)
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", fmt.Errorf("failed to resolve shortcut: empty target path")
	}
	return targetPath, nil
}

// getFileDisplayName gets the display name from file version info
func (a *WindowsRetriever) getFileDisplayName(ctx context.Context, filePath string) string {
	// Convert file path to UTF16
	lpFileName, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Failed to convert file path to UTF16: %s", err.Error()))
		return ""
	}

	// Get version info size
	size, _, _ := getFileVersionInfoSize.Call(uintptr(unsafe.Pointer(lpFileName)), 0)
	if size == 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf("No version info found for file: %s", filePath))
		return ""
	}

	// Allocate buffer for version info
	buffer := make([]byte, size)

	// Get version info
	ret, _, _ := getFileVersionInfo.Call(
		uintptr(unsafe.Pointer(lpFileName)),
		0,
		uintptr(size),
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if ret == 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Failed to get version info for file: %s", filePath))
		return ""
	}

	// Try to get FileDescription first, then ProductName
	displayNames := []string{
		"\\StringFileInfo\\040904e4\\FileDescription",
		"\\StringFileInfo\\040904e4\\ProductName",
		"\\StringFileInfo\\040904b0\\FileDescription", // Simplified Chinese
		"\\StringFileInfo\\040904b0\\ProductName",
	}

	for _, queryPath := range displayNames {
		name := a.queryVersionString(ctx, buffer, queryPath)
		if name != "" {
			util.GetLogger().Debug(ctx, fmt.Sprintf("Found display name '%s' for file: %s", name, filePath))
			return name
		}
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("No display name found in version info for file: %s", filePath))
	return ""
}

// queryVersionString queries a string value from version info buffer
func (a *WindowsRetriever) queryVersionString(ctx context.Context, buffer []byte, queryPath string) string {
	lpSubBlock, err := syscall.UTF16PtrFromString(queryPath)
	if err != nil {
		return ""
	}

	var lpBuffer uintptr
	var puLen uint32

	ret, _, _ := verQueryValue.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(lpSubBlock)),
		uintptr(unsafe.Pointer(&lpBuffer)),
		uintptr(unsafe.Pointer(&puLen)),
	)

	if ret == 0 || puLen == 0 {
		return ""
	}

	// Convert UTF16 string to Go string
	// puLen is already the number of UTF16 characters (not bytes)
	utf16Length := puLen
	if utf16Length == 0 {
		return ""
	}

	// Create a slice with the exact length needed
	utf16Slice := (*[1024]uint16)(unsafe.Pointer(lpBuffer))[:utf16Length]
	result := syscall.UTF16ToString(utf16Slice)

	return result
}

func (a *WindowsRetriever) GetExtraApps(ctx context.Context) ([]appInfo, error) {
	settingsApps := a.getWindowsSettingsApps(ctx)
	if len(settingsApps) > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("Loaded %d Windows Settings items", len(settingsApps)))
	}

	uwpApps := a.GetUWPApps(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf("Found %d UWP apps", len(uwpApps)))

	return append(settingsApps, uwpApps...), nil
}

// getPrivateWorkingSet calculates the private (non-shared) working set size for a process
func getPrivateWorkingSet(handle syscall.Handle) (uintptr, error) {
	type PSAPI_WORKING_SET_BLOCK struct {
		Flags uintptr
	}

	type PSAPI_WORKING_SET_INFORMATION struct {
		NumberOfEntries uintptr
		WorkingSetInfo  [1]PSAPI_WORKING_SET_BLOCK
	}

	psapi := syscall.NewLazyDLL("psapi.dll")
	queryWorkingSet := psapi.NewProc("QueryWorkingSet")

	// First call to get the required buffer size
	var wsInfo PSAPI_WORKING_SET_INFORMATION
	queryWorkingSet.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&wsInfo)),
		unsafe.Sizeof(wsInfo),
	)

	if wsInfo.NumberOfEntries == 0 || wsInfo.NumberOfEntries > 1000000 {
		return 0, fmt.Errorf("invalid number of entries: %d", wsInfo.NumberOfEntries)
	}

	// Allocate buffer for all entries
	bufferSize := unsafe.Sizeof(uintptr(0)) + wsInfo.NumberOfEntries*unsafe.Sizeof(PSAPI_WORKING_SET_BLOCK{})
	buffer := make([]byte, bufferSize)

	// Second call to get actual data
	ret, _, _ := queryWorkingSet.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&buffer[0])),
		bufferSize,
	)

	if ret == 0 {
		return 0, fmt.Errorf("QueryWorkingSet failed")
	}

	// Parse working set entries and count private pages
	actualEntries := *(*uintptr)(unsafe.Pointer(&buffer[0]))
	pageSize := uintptr(4096)
	var privateBytes uintptr

	// Bit 8 of flags indicates if page is shared (1) or private (0)
	offset := unsafe.Sizeof(uintptr(0))
	for i := uintptr(0); i < actualEntries; i++ {
		flags := *(*uintptr)(unsafe.Pointer(&buffer[offset+i*unsafe.Sizeof(uintptr(0))]))
		isShared := (flags & (1 << 8)) != 0
		if !isShared {
			privateBytes += pageSize
		}
	}

	return privateBytes, nil
}

func (a *WindowsRetriever) GetProcessStat(ctx context.Context, app appInfo) (*ProcessStat, error) {
	// sync.Map doesn't need initialization

	// For multi-process apps (like Chrome), we need to sum memory from all processes with the same path
	// Update process list if needed
	a.runningProcessesMutex.RLock()
	needUpdate := util.GetSystemTimestamp()-a.lastProcessUpdateTime > 1000
	a.runningProcessesMutex.RUnlock()

	if needUpdate {
		a.runningProcessesMutex.Lock()
		// Double-check after acquiring write lock
		if util.GetSystemTimestamp()-a.lastProcessUpdateTime > 1000 {
			a.lastProcessUpdateTime = util.GetSystemTimestamp()
			a.runningProcesses = a.getRunningProcesses(ctx)
		}
		a.runningProcessesMutex.Unlock()
	}

	// Collect stats from all processes with the same path
	appPathLower := strings.ToLower(filepath.Clean(app.Path))
	var totalMemory float64
	var totalKernelTime int64
	var totalUserTime int64
	var processCount int
	currentTimestamp := util.GetSystemTimestamp()

	a.runningProcessesMutex.RLock()
	processes := a.runningProcesses
	a.runningProcessesMutex.RUnlock()

	for _, proc := range processes {
		procPathLower := strings.ToLower(filepath.Clean(proc.Path))
		if procPathLower == appPathLower {
			// Open process with query information access
			hProcess, _, _ := openProcess.Call(
				uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
				0,
				uintptr(proc.Pid),
			)

			if hProcess == 0 {
				util.GetLogger().Debug(ctx, fmt.Sprintf("Failed to open process %d for %s", proc.Pid, app.Name))
				continue // Skip processes we can't open
			}

			// Get CPU times
			var creationTime, exitTime, kernelTime, userTime syscall.Filetime
			ret, _, _ := getProcessTimes.Call(
				hProcess,
				uintptr(unsafe.Pointer(&creationTime)),
				uintptr(unsafe.Pointer(&exitTime)),
				uintptr(unsafe.Pointer(&kernelTime)),
				uintptr(unsafe.Pointer(&userTime)),
			)

			if ret != 0 {
				// Convert FILETIME to int64 (100-nanosecond intervals)
				kernelTime64 := int64(kernelTime.HighDateTime)<<32 | int64(kernelTime.LowDateTime)
				userTime64 := int64(userTime.HighDateTime)<<32 | int64(userTime.LowDateTime)
				totalKernelTime += kernelTime64
				totalUserTime += userTime64
			}

			// Try to get private working set
			privateWS, err := getPrivateWorkingSet(syscall.Handle(hProcess))

			var memCounters PROCESS_MEMORY_COUNTERS_EX
			memCounters.cb = uint32(unsafe.Sizeof(memCounters))

			ret, _, _ = getProcessMemoryInfo.Call(
				hProcess,
				uintptr(unsafe.Pointer(&memCounters)),
				uintptr(memCounters.cb),
			)

			closeHandle.Call(hProcess)

			if ret != 0 {
				// Use Private Working Set if available, otherwise fall back to Commit Size
				if err == nil && privateWS > 0 {
					totalMemory += float64(privateWS)
				} else {
					totalMemory += float64(memCounters.PagefileUsage)
				}
				processCount++
			}
		}
	}

	if processCount == 0 {
		return nil, fmt.Errorf("no running processes found for %s", app.Name)
	}

	// Calculate CPU usage
	var cpuPercent float64
	if value, exists := a.cpuSamples.Load(appPathLower); exists {
		lastSample := value.(cpuSample)
		// Calculate time elapsed in milliseconds
		timeElapsed := currentTimestamp - lastSample.timestamp
		if timeElapsed > 0 {
			// Calculate CPU time difference (in 100-nanosecond intervals)
			kernelDiff := totalKernelTime - lastSample.kernelTime
			userDiff := totalUserTime - lastSample.userTime
			totalCPUTime := kernelDiff + userDiff

			// Convert to percentage
			// CPU time is in 100-nanosecond intervals, elapsed time is in milliseconds
			// CPU% = (CPU time in ms / elapsed time in ms) * 100 / number of CPUs
			cpuTimeMs := float64(totalCPUTime) / 10000.0 // Convert 100-ns to ms
			rawPercent := (cpuTimeMs / float64(timeElapsed)) * 100.0

			// Normalize to single CPU (divide by number of logical processors)
			numCPU := runtime.NumCPU()
			cpuPercent = rawPercent / float64(numCPU)
		}
	}

	// Save current sample for next calculation
	a.cpuSamples.Store(appPathLower, cpuSample{
		kernelTime: totalKernelTime,
		userTime:   totalUserTime,
		timestamp:  currentTimestamp,
	})

	return &ProcessStat{
		CPU:    cpuPercent,
		Memory: totalMemory,
	}, nil
}

func (a *WindowsRetriever) OpenAppFolder(ctx context.Context, app appInfo) error {
	if app.Type != AppTypeUWP {
		return shell.OpenFileInFolder(app.Path)
	}

	// Extract AppID from Path (format: "shell:AppsFolder\PackageFamilyName!AppId")
	appID := strings.TrimPrefix(app.Path, "shell:AppsFolder\\")
	if appID == "" {
		return fmt.Errorf("invalid UWP app path: %s", app.Path)
	}

	// Get app installation location using PowerShell
	output, err := shell.RunOutput("powershell", "-Command", fmt.Sprintf(`
		$packageFamilyName = ($('%s' -split '!')[0])
		$package = Get-AppxPackage | Where-Object { $_.PackageFamilyName -eq $packageFamilyName }
		if ($package) {
			Write-Output $package.InstallLocation
		}
	`, appID))
	if err != nil {
		return fmt.Errorf("failed to get UWP app install location: %v", err)
	}

	installLocation := strings.TrimSpace(string(output))
	if installLocation == "" {
		return fmt.Errorf("UWP app install location not found for: %s", appID)
	}

	return shell.OpenFileInFolder(installLocation)
}

func (a *WindowsRetriever) GetUWPApps(ctx context.Context) []appInfo {
	// preload icon cache from file
	iconCachePath := filepath.Join(util.GetLocation().GetCacheDirectory(), "app-uwp-icons.json")
	if _, err := os.Stat(iconCachePath); !os.IsNotExist(err) {
		iconCache, err := os.ReadFile(iconCachePath)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error reading uwp icon cache: %v", err))
		} else {
			// parse json
			var cacheMap map[string]string
			jsonErr := json.Unmarshal(iconCache, &cacheMap)
			if jsonErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Error parsing uwp icon cache: %v", jsonErr))
			} else {
				// Load into sync.Map
				count := 0
				for k, v := range cacheMap {
					a.uwpIconCache.Store(k, v)
					count++
				}
				util.GetLogger().Info(ctx, fmt.Sprintf("Loaded %d uwp icon cache", count))
			}
		}
	}

	var apps []appInfo

	// Modify PowerShell command, add more properties and use UTF-8 encoding
	powershellCmd := `
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		Get-StartApps | Where-Object { $_.AppID -like '*!*' } | Select-Object Name, AppID | ConvertTo-Csv -NoTypeInformation
	`

	// Set command encoding to UTF-8
	output, err := shell.RunOutput("powershell", "-Command", powershellCmd)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error running powershell command: %v", err))
		return apps
	}

	// Parse CSV output
	reader := csv.NewReader(strings.NewReader(string(output)))
	records, err := reader.ReadAll()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error parsing CSV output: %v", err))
		return apps
	}

	// Skip header row
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 2 {
			continue
		}

		name := record[0]
		appID := record[1]

		if strings.Contains(appID, "!") {
			app := appInfo{
				Name: name,
				Path: "shell:AppsFolder\\" + appID,
				Icon: appIcon,
				Type: AppTypeUWP,
			}

			// Get app icon
			icon, err := a.GetUWPAppIcon(ctx, appID)
			if err == nil {
				app.Icon = icon
				a.uwpIconCache.Store(appID, icon.ImageData)
			} else {
				util.GetLogger().Error(ctx, fmt.Sprintf("Error getting UWP icon for %s (%s), using default icon: %s", name, appID, err.Error()))
				// Keep using default appIcon when UWP icon fails
			}

			apps = append(apps, app)
			util.GetLogger().Info(ctx, fmt.Sprintf("Found UWP app: %s, AppID: %s", name, appID))
		}
	}

	// save icon cache
	cacheMap := make(map[string]string)
	count := 0
	a.uwpIconCache.Range(func(key, value interface{}) bool {
		cacheMap[key.(string)] = value.(string)
		count++
		return true
	})
	iconCache, err := json.Marshal(cacheMap)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error marshalling uwp icon cache: %v", err))
	} else {
		os.WriteFile(iconCachePath, iconCache, 0644)
		util.GetLogger().Info(ctx, fmt.Sprintf("Saved %d uwp icon cache", count))
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Found %d UWP apps", len(apps)))
	return apps
}

func (a *WindowsRetriever) GetPid(ctx context.Context, app appInfo) int {
	// Update process list if it's been more than 1 second since last update
	a.runningProcessesMutex.RLock()
	needUpdate := util.GetSystemTimestamp()-a.lastProcessUpdateTime > 1000
	a.runningProcessesMutex.RUnlock()

	if needUpdate {
		a.runningProcessesMutex.Lock()
		// Double-check after acquiring write lock
		if util.GetSystemTimestamp()-a.lastProcessUpdateTime > 1000 {
			a.lastProcessUpdateTime = util.GetSystemTimestamp()
			a.runningProcesses = a.getRunningProcesses(ctx)
		}
		a.runningProcessesMutex.Unlock()
	}

	a.runningProcessesMutex.RLock()
	processes := a.runningProcesses
	a.runningProcessesMutex.RUnlock()

	// For desktop apps, match by path
	if app.Type == AppTypeDesktop {
		appPathLower := strings.ToLower(filepath.Clean(app.Path))
		for _, proc := range processes {
			procPathLower := strings.ToLower(filepath.Clean(proc.Path))
			if procPathLower == appPathLower {
				return proc.Pid
			}
		}
	}

	return 0
}

func (a *WindowsRetriever) getRunningProcesses(ctx context.Context) []processInfo {
	var infos []processInfo

	// Allocate buffer for process IDs (max 4096 processes)
	const maxProcesses = 4096
	pids := make([]uint32, maxProcesses)
	var bytesReturned uint32

	// Call EnumProcesses to get all process IDs
	ret, _, _ := enumProcesses.Call(
		uintptr(unsafe.Pointer(&pids[0])),
		uintptr(maxProcesses*4), // size in bytes
		uintptr(unsafe.Pointer(&bytesReturned)),
	)

	if ret == 0 {
		util.GetLogger().Error(ctx, "Failed to enumerate processes")
		return infos
	}

	// Calculate number of processes returned
	numProcesses := int(bytesReturned / 4)

	// Iterate through each process
	for i := 0; i < numProcesses; i++ {
		pid := pids[i]
		if pid == 0 {
			continue
		}

		// Open process with query information and VM read access
		hProcess, _, _ := openProcess.Call(
			uintptr(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ),
			0,
			uintptr(pid),
		)

		if hProcess == 0 {
			continue
		}

		// Get the executable path
		var exePath [syscall.MAX_PATH]uint16
		ret, _, _ := getModuleFileNameExW.Call(
			hProcess,
			0, // NULL for main executable
			uintptr(unsafe.Pointer(&exePath[0])),
			uintptr(syscall.MAX_PATH),
		)

		// Close process handle
		closeHandle.Call(hProcess)

		if ret == 0 {
			continue
		}

		// Convert path to string
		path := syscall.UTF16ToString(exePath[:])
		if path == "" {
			continue
		}

		infos = append(infos, processInfo{
			Pid:  int(pid),
			Path: path,
		})
	}

	return infos
}

func (a *WindowsRetriever) GetUWPAppIcon(ctx context.Context, appID string) (common.WoxImage, error) {
	if value, ok := a.uwpIconCache.Load(appID); ok {
		iconPath := value.(string)
		// Verify cached path still exists
		if _, err := os.Stat(iconPath); err == nil {
			return common.NewWoxImageAbsolutePath(iconPath), nil
		} else {
			// Remove invalid cache entry
			a.uwpIconCache.Delete(appID)
		}
	}

	powershellCmd := fmt.Sprintf(`
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		try {
			$packageFamilyName = ($('%s' -split '!')[0])
			$package = Get-AppxPackage | Where-Object { $_.PackageFamilyName -eq $packageFamilyName }
			if (!$package) { exit 1 }

			$manifest = Get-AppxPackageManifest $package
			if (!$manifest) { exit 1 }

			$logo = $manifest.Package.Properties.Logo
			if (!$logo) {
				$visual = $manifest.Package.Applications.Application.VisualElements
				if ($visual.Square44x44Logo) {
					$logo = $visual.Square44x44Logo
				} elseif ($visual.Square150x150Logo) {
					$logo = $visual.Square150x150Logo
				} elseif ($visual.Logo) {
					$logo = $visual.Logo
				}
			}

			if (!$logo) { exit 1 }

			$logoPath = Join-Path $package.InstallLocation $logo
			if (!(Test-Path $package.InstallLocation)) { exit 1 }

			$directory = Split-Path $logoPath
			$filename = Split-Path $logoPath -Leaf
			$baseFilename = [System.IO.Path]::GetFileNameWithoutExtension($filename)
			$extension = [System.IO.Path]::GetExtension($filename)

			# Try different scaling versions and target sizes (prioritize larger sizes)
			$scales = @('scale-200', 'scale-400', 'scale-150', 'scale-125', 'scale-100', '')
			$targetSizes = @('256', '64', '48', '44', '32', '24', '16')

			# First try filenames with sizes
			foreach ($size in $targetSizes) {
				foreach ($scale in $scales) {
					$targetPath = if ($scale) {
						Join-Path $directory "$baseFilename.targetsize-$size.$scale$extension"
					} else {
						Join-Path $directory "$baseFilename.targetsize-$size$extension"
					}
					if (Test-Path $targetPath) {
						Write-Output $targetPath
						exit 0
					}
				}
			}

			# Then try scaled versions
			foreach ($scale in $scales) {
				$scaledPath = if ($scale) {
					Join-Path $directory "$baseFilename.$scale$extension"
				} else {
					$logoPath
				}
				if (Test-Path $scaledPath) {
					Write-Output $scaledPath
					exit 0
				}
			}

			# If nothing found, return original path if it exists
			if (Test-Path $logoPath) {
				Write-Output $logoPath
				exit 0
			}
			exit 1
		} catch {
			exit 1
		}
	`, appID)

	output, err := shell.RunOutput("powershell", "-Command", powershellCmd)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error running powershell command for UWP app %s: %v", appID, err))
		return common.WoxImage{}, err
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		util.GetLogger().Error(ctx, fmt.Sprintf("No output from PowerShell for UWP app %s", appID))
		return common.WoxImage{}, fmt.Errorf("no output from PowerShell")
	}

	// The output should be the icon path
	iconPath := outputStr
	if iconPath == "" {
		util.GetLogger().Error(ctx, fmt.Sprintf("No valid icon path found for UWP app %s", appID))
		return common.WoxImage{}, fmt.Errorf("no valid icon path found")
	}

	// Verify the path exists
	if _, err := os.Stat(iconPath); os.IsNotExist(err) {
		util.GetLogger().Error(ctx, fmt.Sprintf("Icon path does not exist for UWP app %s: %s", appID, iconPath))

		// Try to find any icon file in the app directory as fallback
		packageFamilyName := strings.Split(appID, "!")[0]
		fallbackIcon, fallbackErr := a.findFallbackUWPIcon(ctx, packageFamilyName)
		if fallbackErr == nil {
			return common.NewWoxImageAbsolutePath(fallbackIcon), nil
		}

		return common.WoxImage{}, fmt.Errorf("icon path does not exist: %s", iconPath)
	}

	return common.NewWoxImageAbsolutePath(iconPath), nil
}

func (a *WindowsRetriever) findFallbackUWPIcon(ctx context.Context, packageFamilyName string) (string, error) {
	// Use PowerShell to find any icon file in the UWP app directory
	powershellCmd := fmt.Sprintf(`
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		try {
			$package = Get-AppxPackage | Where-Object { $_.PackageFamilyName -eq '%s' }
			if (!$package) { exit 1 }
			if (!(Test-Path $package.InstallLocation)) { exit 1 }

			# Look for any icon files in common locations
			$iconExtensions = @('*.png', '*.jpg', '*.ico')
			$iconDirs = @('Assets', 'Images', '')

			foreach ($dir in $iconDirs) {
				$searchPath = if ($dir) { Join-Path $package.InstallLocation $dir } else { $package.InstallLocation }
				if (Test-Path $searchPath) {
					foreach ($ext in $iconExtensions) {
						$icons = Get-ChildItem -Path $searchPath -Filter $ext -ErrorAction SilentlyContinue | Sort-Object Length -Descending
						if ($icons) {
							# Prefer larger files (likely higher resolution)
							Write-Output $icons[0].FullName
							exit 0
						}
					}
				}
			}
			exit 1
		} catch {
			exit 1
		}
	`, packageFamilyName)

	output, err := shell.RunOutput("powershell", "-Command", powershellCmd)
	if err != nil {
		return "", fmt.Errorf("PowerShell execution failed: %v", err)
	}

	iconPath := strings.TrimSpace(string(output))
	if iconPath == "" {
		return "", fmt.Errorf("no fallback icon found")
	}

	return iconPath, nil
}
