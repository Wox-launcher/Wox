package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	win "github.com/lxn/win"
)

var (
	// Load shell32.dll and user32.dll
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")
	// Get the address of APIs
	extractIconEx       = shell32.NewProc("ExtractIconExW")
	privateExtractIcons = user32.NewProc("PrivateExtractIconsW")
	shGetFileInfo       = shell32.NewProc("SHGetFileInfoW")

	// Load version.dll for file version info
	version                = syscall.NewLazyDLL("version.dll")
	getFileVersionInfoSize = version.NewProc("GetFileVersionInfoSizeW")
	getFileVersionInfo     = version.NewProc("GetFileVersionInfoW")
	verQueryValue          = version.NewProc("VerQueryValueW")
)

// Windows constants for icon extraction
const (
	SHGFI_ICON          = 0x000000100
	SHGFI_DISPLAYNAME   = 0x000000200
	SHGFI_LARGEICON     = 0x000000000
	SHGFI_SMALLICON     = 0x000000001
	SHGFI_SYSICONINDEX  = 0x000004000
	SHGFI_SHELLICONSIZE = 0x000000004
	IMAGE_ICON          = 1
	LR_DEFAULTSIZE      = 0x00000040
	LR_LOADFROMFILE     = 0x00000010
)

// SHFILEINFO structure for SHGetFileInfo
type SHFILEINFO struct {
	HIcon         win.HICON
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

var appRetriever = &WindowsRetriever{}

type WindowsRetriever struct {
	api plugin.API

	uwpIconCache map[string]string // appID -> icon path
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
			Path:           usr.HomeDir + "\\AppData\\Local",
			Recursive:      true,
			RecursiveDepth: 4,
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
	// Use PowerShell + COM to resolve shortcut with proper Unicode support
	targetPath, resolveErr := a.resolveShortcutWithAPI(ctx, appPath)
	if resolveErr != nil {
		return appInfo{}, fmt.Errorf("failed to resolve shortcut %s: %v", appPath, resolveErr)
	}

	a.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Resolved shortcut %s -> %s", appPath, targetPath))

	if targetPath == "" || !strings.HasSuffix(strings.ToLower(targetPath), ".exe") {
		return appInfo{}, errors.New("no target path found or not an exe file")
	}

	// use default icon if no icon is found
	icon := appIcon
	img, iconErr := a.GetAppIcon(ctx, targetPath)
	if iconErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s, use default icon: %s", targetPath, iconErr.Error()))
	} else {
		woxIcon, imgErr := common.NewWoxImage(img)
		if imgErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error converting icon for %s: %s", targetPath, imgErr.Error()))
		} else {
			icon = woxIcon
		}
	}

	// Try to get display name from target exe file version info
	displayName := a.getFileDisplayName(ctx, targetPath)
	if displayName == "" {
		// Fallback to shortcut filename if no display name found
		displayName = strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath))
		a.api.Log(ctx, plugin.LogLevelDebug, fmt.Sprintf("Using shortcut filename as display name: %s", displayName))
	}

	return appInfo{
		Name: displayName,
		Path: filepath.Clean(targetPath),
		Icon: icon,
		Type: AppTypeDesktop,
	}, nil
}

func (a *WindowsRetriever) parseExe(ctx context.Context, appPath string) (appInfo, error) {
	// use default icon if no icon is found
	icon := appIcon
	img, iconErr := a.GetAppIcon(ctx, appPath)
	if iconErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error getting icon for %s: %s", appPath, iconErr.Error()))
	} else {
		woxIcon, imgErr := common.NewWoxImage(img)
		if imgErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error converting icon for %s: %s", appPath, imgErr.Error()))
		} else {
			icon = woxIcon
		}
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

func (a *WindowsRetriever) GetAppIcon(ctx context.Context, path string) (image.Image, error) {
	// Priority 1: Try to get high resolution icon using PrivateExtractIconsW (best quality)
	if icon, err := a.getHighResIcon(ctx, path); err == nil {
		return icon, nil
	}

	// Priority 2: Try to get large icon using SHGetFileInfo (public API fallback)
	if icon, err := a.getIconUsingSHGetFileInfo(ctx, path); err == nil {
		return icon, nil
	}

	// Priority 3: Try ExtractIconEx
	if icon, err := a.getIconUsingExtractIconEx(ctx, path); err == nil {
		return icon, nil
	}

	// Priority 4: Final fallback to Windows default executable icon
	return a.getWindowsDefaultIcon(ctx)
}

func (a *WindowsRetriever) getIconUsingSHGetFileInfo(ctx context.Context, path string) (image.Image, error) {
	// Convert file path to UTF16
	lpIconPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	var shfi SHFILEINFO
	ret, _, _ := shGetFileInfo.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		0,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		SHGFI_ICON|SHGFI_LARGEICON,
	)

	if ret == 0 || shfi.HIcon == 0 {
		return nil, fmt.Errorf("failed to get icon using SHGetFileInfo")
	}
	defer win.DestroyIcon(shfi.HIcon)

	return a.convertIconToImage(ctx, shfi.HIcon)
}

func (a *WindowsRetriever) getIconUsingExtractIconEx(ctx context.Context, path string) (image.Image, error) {
	// Convert file path to UTF16
	lpIconPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	// Get icon handle using ExtractIconEx
	var largeIcon win.HICON
	var smallIcon win.HICON
	ret, _, _ := extractIconEx.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if ret == 0 {
		return nil, fmt.Errorf("no icons found in file")
	}
	defer win.DestroyIcon(largeIcon) // Ensure icon resources are released

	return a.convertIconToImage(ctx, largeIcon)
}

func (a *WindowsRetriever) getHighResIcon(ctx context.Context, path string) (image.Image, error) {
	// Safely try to use PrivateExtractIconsW (undocumented API, but provides best quality)
	defer func() {
		if r := recover(); r != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("PrivateExtractIconsW caused panic (API may not be available): %v", r))
		}
	}()

	// Check if PrivateExtractIconsW is available
	if err := privateExtractIcons.Find(); err != nil {
		return nil, fmt.Errorf("PrivateExtractIconsW not available: %v", err)
	}

	// Convert file path to UTF16
	lpIconPath, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("failed to convert path to UTF16: %v", err)
	}

	// Try different icon sizes: 256, 128, 64, 48 (prioritize larger sizes)
	sizes := []int{256, 128, 64, 48}

	for _, size := range sizes {
		var hIcon win.HICON

		// Use a safe call wrapper
		ret, _, callErr := func() (uintptr, uintptr, error) {
			defer func() {
				if r := recover(); r != nil {
					util.GetLogger().Debug(ctx, fmt.Sprintf("PrivateExtractIconsW call panicked for size %d: %v", size, r))
				}
			}()

			return privateExtractIcons.Call(
				uintptr(unsafe.Pointer(lpIconPath)),
				0,             // icon index
				uintptr(size), // cx - desired width
				uintptr(size), // cy - desired height
				uintptr(unsafe.Pointer(&hIcon)),
				0, // icon IDs (not needed)
				1, // number of icons to extract
				0, // flags
			)
		}()

		// Check for system call errors (ignore "operation completed successfully" and "user stopped resource enumeration")
		if callErr != nil &&
			callErr.Error() != "The operation completed successfully." &&
			callErr.Error() != "User stopped resource enumeration." {
			continue
		}

		if ret > 0 && hIcon != 0 {
			defer win.DestroyIcon(hIcon)
			util.GetLogger().Info(ctx, fmt.Sprintf("Successfully extracted %dx%d high-res icon from %s using PrivateExtractIconsW", size, size, path))
			return a.convertIconToImage(ctx, hIcon)
		}
	}

	return nil, fmt.Errorf("failed to extract high resolution icon using PrivateExtractIconsW")
}

func (a *WindowsRetriever) convertIconToImage(ctx context.Context, hIcon win.HICON) (image.Image, error) {

	// Get icon information
	var iconInfo win.ICONINFO
	if !win.GetIconInfo(hIcon, &iconInfo) {
		return nil, fmt.Errorf("failed to get icon info")
	}
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmColor))
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmMask))

	// Get actual bitmap dimensions
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	// Get bitmap info to determine actual size
	var bitmap win.BITMAP
	if win.GetObject(win.HGDIOBJ(iconInfo.HbmColor), uintptr(unsafe.Sizeof(bitmap)), unsafe.Pointer(&bitmap)) == 0 {
		return nil, fmt.Errorf("failed to get bitmap object")
	}

	width := int(bitmap.BmWidth)
	height := int(bitmap.BmHeight)

	var bmpInfo win.BITMAPINFO
	bmpInfo.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmpInfo.BmiHeader))
	bmpInfo.BmiHeader.BiWidth = int32(width)
	bmpInfo.BmiHeader.BiHeight = -int32(height) // Negative value indicates top-down DIB
	bmpInfo.BmiHeader.BiPlanes = 1
	bmpInfo.BmiHeader.BiBitCount = 32
	bmpInfo.BmiHeader.BiCompression = win.BI_RGB

	// Allocate memory to store bitmap data
	bits := make([]byte, width*height*4)
	if win.GetDIBits(hdc, win.HBITMAP(iconInfo.HbmColor), 0, uint32(height), &bits[0], &bmpInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, fmt.Errorf("failed to get DIB bits")
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Copy the bitmap data into the img.Pix slice.
	// Note: Windows bitmaps are stored in BGR format, so we need to swap the red and blue channels.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			base := y*width*4 + x*4
			// The bitmap data in bits is in BGRA format.
			b := bits[base+0]
			g := bits[base+1]
			r := bits[base+2]
			a := bits[base+3]
			// Set the pixel in the image.
			// Note: image.RGBA expects data in RGBA format, so we swap R and B.
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	// Validate that the image has actual content (not just transparent pixels)
	hasContent := false
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y && !hasContent; y++ {
		for x := bounds.Min.X; x < bounds.Max.X && !hasContent; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a > 0 && (r > 0 || g > 0 || b > 0) {
				hasContent = true
			}
		}
	}

	if !hasContent {
		return nil, fmt.Errorf("extracted icon is empty or fully transparent")
	}

	return img, nil
}

// resolveShortcutWithAPI uses Windows API to resolve shortcut target path with proper Unicode support
func (a *WindowsRetriever) resolveShortcutWithAPI(ctx context.Context, shortcutPath string) (string, error) {
	// Use PowerShell to resolve the shortcut with proper Unicode handling
	powershellCmd := fmt.Sprintf(`
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		$shell = New-Object -ComObject WScript.Shell
		$shortcut = $shell.CreateShortcut('%s')
		Write-Output $shortcut.TargetPath
	`, shortcutPath)

	output, err := shell.RunOutput("powershell", "-Command", powershellCmd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve shortcut: %v", err)
	}

	targetPath := strings.TrimSpace(string(output))
	if targetPath == "" {
		return "", fmt.Errorf("empty target path")
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
	uwpApps := a.GetUWPApps(ctx)
	util.GetLogger().Info(ctx, fmt.Sprintf("Found %d UWP apps", len(uwpApps)))

	return uwpApps, nil
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
	// preload icon cache
	if a.uwpIconCache == nil {
		a.uwpIconCache = make(map[string]string)
	}
	iconCachePath := filepath.Join(util.GetLocation().GetCacheDirectory(), "app-uwp-icons.json")
	if _, err := os.Stat(iconCachePath); !os.IsNotExist(err) {
		iconCache, err := os.ReadFile(iconCachePath)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("Error reading uwp icon cache: %v", err))
		} else {
			// parse json
			jsonErr := json.Unmarshal(iconCache, &a.uwpIconCache)
			if jsonErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Error parsing uwp icon cache: %v", jsonErr))
			} else {
				util.GetLogger().Info(ctx, fmt.Sprintf("Loaded %d uwp icon cache", len(a.uwpIconCache)))
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
				a.uwpIconCache[appID] = icon.ImageData
			} else {
				util.GetLogger().Error(ctx, fmt.Sprintf("Error getting UWP icon for %s (%s), using default icon: %s", name, appID, err.Error()))
				// Keep using default appIcon when UWP icon fails
			}

			apps = append(apps, app)
			util.GetLogger().Info(ctx, fmt.Sprintf("Found UWP app: %s, AppID: %s", name, appID))
		}
	}

	// save icon cache
	iconCache, err := json.Marshal(a.uwpIconCache)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error marshalling uwp icon cache: %v", err))
	} else {
		os.WriteFile(iconCachePath, iconCache, 0644)
		util.GetLogger().Info(ctx, fmt.Sprintf("Saved %d uwp icon cache", len(a.uwpIconCache)))
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("Found %d UWP apps", len(apps)))
	return apps
}

func (a *WindowsRetriever) GetPid(ctx context.Context, app appInfo) int {
	// Get pid of the app
	return 0
}

func (a *WindowsRetriever) GetUWPAppIcon(ctx context.Context, appID string) (common.WoxImage, error) {
	if iconPath, ok := a.uwpIconCache[appID]; ok {
		// Verify cached path still exists
		if _, err := os.Stat(iconPath); err == nil {
			return common.NewWoxImageAbsolutePath(iconPath), nil
		} else {
			// Remove invalid cache entry
			delete(a.uwpIconCache, appID)
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

func (a *WindowsRetriever) getWindowsDefaultIcon(ctx context.Context) (image.Image, error) {
	// Try to get high resolution default icon using PrivateExtractIconsW first
	if icon, err := a.getHighResDefaultIcon(ctx); err == nil {
		return icon, nil
	}

	// Fallback to standard SHGetFileInfo method
	return a.getStandardDefaultIcon(ctx)
}

func (a *WindowsRetriever) getHighResDefaultIcon(ctx context.Context) (image.Image, error) {
	// Try to extract high-res icon from shell32.dll (contains default icons)
	shell32Path, err := syscall.UTF16PtrFromString("shell32.dll")
	if err != nil {
		return nil, fmt.Errorf("failed to convert shell32.dll path to UTF16: %v", err)
	}

	// Check if PrivateExtractIconsW is available
	if err := privateExtractIcons.Find(); err != nil {
		return nil, fmt.Errorf("PrivateExtractIconsW not available: %v", err)
	}

	// Try different icon sizes: 256, 128, 64, 48
	sizes := []int{256, 128, 64, 48}

	for _, size := range sizes {
		var hIcon win.HICON

		// Extract icon index 2 from shell32.dll (default executable icon)
		ret, _, callErr := privateExtractIcons.Call(
			uintptr(unsafe.Pointer(shell32Path)),
			2,             // icon index 2 is typically the default executable icon
			uintptr(size), // cx - desired width
			uintptr(size), // cy - desired height
			uintptr(unsafe.Pointer(&hIcon)),
			0, // icon IDs (not needed)
			1, // number of icons to extract
			0, // flags
		)

		// Check for system call errors (ignore "operation completed successfully" and "user stopped resource enumeration")
		if callErr != nil &&
			callErr.Error() != "The operation completed successfully." &&
			callErr.Error() != "User stopped resource enumeration." {
			continue
		}

		if ret > 0 && hIcon != 0 {
			defer win.DestroyIcon(hIcon)
			util.GetLogger().Info(ctx, fmt.Sprintf("Successfully extracted %dx%d default icon from shell32.dll", size, size))
			return a.convertIconToImage(ctx, hIcon)
		}
	}

	return nil, fmt.Errorf("failed to extract high resolution default icon from shell32.dll")
}

func (a *WindowsRetriever) getStandardDefaultIcon(ctx context.Context) (image.Image, error) {
	// Get the default icon for .exe files from Windows
	// This will return the standard Windows executable file icon
	exeExtension, err := syscall.UTF16PtrFromString(".exe")
	if err != nil {
		return nil, fmt.Errorf("failed to convert .exe extension to UTF16: %v", err)
	}

	var shfi SHFILEINFO
	ret, _, _ := shGetFileInfo.Call(
		uintptr(unsafe.Pointer(exeExtension)),
		0x80, // FILE_ATTRIBUTE_NORMAL
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		SHGFI_ICON|SHGFI_LARGEICON|0x000000010, // SHGFI_USEFILEATTRIBUTES
	)

	if ret == 0 || shfi.HIcon == 0 {
		return nil, fmt.Errorf("failed to get default Windows executable icon")
	}
	defer win.DestroyIcon(shfi.HIcon)

	util.GetLogger().Info(ctx, "Using Windows standard default executable icon as fallback")
	return a.convertIconToImage(ctx, shfi.HIcon)
}
