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
	// Load shell32.dll instead of user32.dll
	shell32 = syscall.NewLazyDLL("shell32.dll")
	// Get the address of ExtractIconExW from shell32.dll
	extractIconEx = shell32.NewProc("ExtractIconExW")
)

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
	}
}

func (a *WindowsRetriever) GetAppExtensions(ctx context.Context) []string {
	return []string{"exe", "lnk"}
}

func (a *WindowsRetriever) ParseAppInfo(ctx context.Context, path string) (appInfo, error) {
	if strings.HasSuffix(path, ".lnk") {
		return a.parseShortcut(ctx, path)
	}
	if strings.HasSuffix(path, ".exe") {
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

	if targetPath == "" || !strings.HasSuffix(targetPath, ".exe") {
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

	return appInfo{
		Name: strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath)),
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

	return appInfo{
		Name: strings.TrimSuffix(filepath.Base(appPath), filepath.Ext(appPath)),
		Path: filepath.Clean(appPath),
		Icon: icon,
		Type: AppTypeDesktop, // 使用常量
	}, nil
}

func (a *WindowsRetriever) GetAppIcon(ctx context.Context, path string) (image.Image, error) {
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

	// Get icon information
	var iconInfo win.ICONINFO
	if win.GetIconInfo(largeIcon, &iconInfo) == false {
		return nil, fmt.Errorf("failed to get icon info")
	}
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmColor))
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmMask))

	// Create device-independent bitmap (DIB) to receive image data
	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	var bmpInfo win.BITMAPINFO
	bmpInfo.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmpInfo.BmiHeader))
	bmpInfo.BmiHeader.BiWidth = int32(iconInfo.XHotspot * 2)
	bmpInfo.BmiHeader.BiHeight = -int32(iconInfo.YHotspot * 2) // Negative value indicates top-down DIB
	bmpInfo.BmiHeader.BiPlanes = 1
	bmpInfo.BmiHeader.BiBitCount = 32
	bmpInfo.BmiHeader.BiCompression = win.BI_RGB

	// Allocate memory to store bitmap data
	bits := make([]byte, iconInfo.XHotspot*2*iconInfo.YHotspot*2*4)
	if win.GetDIBits(hdc, win.HBITMAP(iconInfo.HbmColor), 0, uint32(iconInfo.YHotspot*2), &bits[0], &bmpInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, fmt.Errorf("failed to get DIB bits")
	}

	width := int(iconInfo.XHotspot * 2)
	height := int(iconInfo.YHotspot * 2)
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
				Type: AppTypeUWP, // 使用常量
			}

			// Get app icon
			icon, err := a.GetUWPAppIcon(ctx, appID)
			if err == nil {
				app.Icon = icon
				a.uwpIconCache[appID] = icon.ImageData
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
		return common.NewWoxImageAbsolutePath(iconPath), nil
	}

	powershellCmd := fmt.Sprintf(`
		[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
		$packageFamilyName = ($('%s' -split '!')[0])
		$package = Get-AppxPackage | Where-Object { $_.PackageFamilyName -eq $packageFamilyName }
		if ($package) {
			$manifest = Get-AppxPackageManifest $package
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
			if ($logo) {
				$logoPath = Join-Path $package.InstallLocation $logo
				$directory = Split-Path $logoPath
				$filename = Split-Path $logoPath -Leaf
				$baseFilename = [System.IO.Path]::GetFileNameWithoutExtension($filename)
				$extension = [System.IO.Path]::GetExtension($filename)
				
				# Add more scaling versions and target sizes
				$scales = @('scale-200', 'scale-100', 'scale-150', 'scale-125', 'scale-400', '')
				$targetSizes = @('44', '48', '24', '32', '64', '256', '16')
				
				# First try filenames with sizes
				foreach ($size in $targetSizes) {
					foreach ($scale in $scales) {
						$targetPath = if ($scale) {
							Join-Path $directory "$baseFilename.targetsize-$size.$scale$extension"
						} else {
							Join-Path $directory "$baseFilename.targetsize-$size$extension"
						}
						if (Test-Path $targetPath) {
							Write-Output "Found icon: $targetPath"
							Write-Output $targetPath
							exit
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
						Write-Output "Found icon: $scaledPath"
						Write-Output $scaledPath
						exit
					}
				}
				
				# If nothing found, return original path
				if (Test-Path $logoPath) {
					Write-Output "Using original path: $logoPath"
					Write-Output $logoPath
				}
			}
			Write-Output "Package info: $($package.PackageFullName)"
			Write-Output "Install location: $($package.InstallLocation)"
		}
	`, appID)

	output, err := shell.RunOutput("powershell", "-Command", powershellCmd)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Error running powershell command: %v", err))
		return common.WoxImage{}, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		util.GetLogger().Error(ctx, "No icon path found")
		return common.WoxImage{}, fmt.Errorf("No icon path found")
	}

	// Last line is the icon path
	iconPath := strings.TrimSpace(lines[len(lines)-1])
	if iconPath == "" {
		util.GetLogger().Error(ctx, "Icon path is empty")
		return common.WoxImage{}, fmt.Errorf("Icon path is empty")
	}

	return common.NewWoxImageAbsolutePath(iconPath), nil
}
