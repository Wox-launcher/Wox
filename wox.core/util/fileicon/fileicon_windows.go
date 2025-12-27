package fileicon

import (
	"context"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
	"wox/util"

	"github.com/disintegration/imaging"
	win "github.com/lxn/win"
	"golang.org/x/sys/windows/registry"
)

// Windows implementation using SHGetFileInfoW to retrieve the HICON for a file extension
// and converting it to PNG, cached on disk by extension.

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	user32           = syscall.NewLazyDLL("user32.dll")
	shGetFileInfo    = shell32.NewProc("SHGetFileInfoW")
	extractIconEx    = shell32.NewProc("ExtractIconExW")
	privateExtractIcons = user32.NewProc("PrivateExtractIconsW")
)

type shFileInfo struct {
	HIcon         win.HICON
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

const (
	SHGFI_ICON              = 0x000000100
	SHGFI_LARGEICON         = 0x000000000
	SHGFI_USEFILEATTRIBUTES = 0x000000010
	FILE_ATTRIBUTE_NORMAL   = 0x00000080
)

// expandEnvVars expands Windows environment variables in a string
func expandEnvVars(s string) string {
	s = strings.ReplaceAll(s, "%SystemRoot%", os.Getenv("SystemRoot"))
	s = strings.ReplaceAll(s, "%ProgramFiles%", os.Getenv("ProgramFiles"))
	s = strings.ReplaceAll(s, "%ProgramFiles(x86)%", os.Getenv("ProgramFiles(x86)"))
	return s
}

// parseIconLocation parses icon location string like "C:\path\to\file.dll,-123"
func parseIconLocation(location string) (path string, index int) {
	parts := strings.Split(location, ",")
	if len(parts) == 0 {
		return "", 0
	}

	path = expandEnvVars(strings.TrimSpace(parts[0]))

	if len(parts) > 1 {
		indexStr := strings.TrimSpace(parts[1])
		index, _ = strconv.Atoi(indexStr)
	}

	return path, index
}

// getIconFromRegistry tries to get icon from registry DefaultIcon key
func getIconFromRegistry(ext string) (win.HICON, error) {
	// Open HKEY_CLASSES_ROOT\.ext
	key, err := registry.OpenKey(registry.CLASSES_ROOT, ext, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer key.Close()

	// Get the ProgID
	progID, _, err := key.GetStringValue("")
	if err != nil {
		return 0, err
	}

	// Open HKEY_CLASSES_ROOT\ProgID\DefaultIcon
	iconKey, err := registry.OpenKey(registry.CLASSES_ROOT, progID+`\DefaultIcon`, registry.QUERY_VALUE)
	if err != nil {
		return 0, err
	}
	defer iconKey.Close()

	// Get the default icon location
	iconLocation, _, err := iconKey.GetStringValue("")
	if err != nil {
		return 0, err
	}

	// Parse and extract icon
	iconPath, iconIndex := parseIconLocation(iconLocation)
	lpIconPath, err := syscall.UTF16PtrFromString(iconPath)
	if err != nil {
		return 0, err
	}

	var hIconLarge win.HICON
	ret, _, _ := extractIconEx.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		uintptr(iconIndex),
		uintptr(unsafe.Pointer(&hIconLarge)),
		0, // don't need small icon
		1,
	)

	if ret == 0 || ret == 0xFFFFFFFF || hIconLarge == 0 {
		return 0, errors.New("ExtractIconEx failed")
	}

	return hIconLarge, nil
}

// getIconFromSHGetFileInfo fallback method using SHGetFileInfo
func getIconFromSHGetFileInfo(ext string) (win.HICON, error) {
	tmp, err := os.CreateTemp("", "wox_ext_*"+ext)
	if err != nil {
		return 0, err
	}
	tmp.Close()
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	lpPath, err := syscall.UTF16PtrFromString(tmpPath)
	if err != nil {
		return 0, err
	}

	var shfi shFileInfo
	ret, _, _ := shGetFileInfo.Call(
		uintptr(unsafe.Pointer(lpPath)),
		0,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		uintptr(SHGFI_ICON|SHGFI_LARGEICON),
	)

	if ret == 0 || shfi.HIcon == 0 {
		return 0, errors.New("SHGetFileInfo failed")
	}

	return shfi.HIcon, nil
}

func getFileIconImpl(ctx context.Context, filePath string) (string, error) {
	const size = 48
	cachePath := buildPathCachePath(filePath, size)

	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("empty path")
	}
	if _, err := os.Stat(filePath); err != nil {
		return "", err
	}

	img, err := getHighResIcon(ctx, filePath)
	if err != nil {
		img, err = getIconUsingExtractIconEx(ctx, filePath)
	}
	if err != nil {
		img, err = getWindowsDefaultIcon(ctx)
	}
	if err != nil {
		return "", err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkdirErr != nil {
		return "", errors.New("failed to create cache dir: " + mkdirErr.Error())
	}
	if saveErr := imaging.Save(img, cachePath); saveErr != nil {
		return "", errors.New("failed to save icon to cache for " + filePath + ": " + saveErr.Error())
	}

	return cachePath, nil
}

func getIconUsingExtractIconEx(ctx context.Context, filePath string) (image.Image, error) {
	lpIconPath, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		return nil, err
	}

	var largeIcon win.HICON
	var smallIcon win.HICON
	ret, _, _ := extractIconEx.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		uintptr(unsafe.Pointer(&smallIcon)),
		1,
	)
	if ret == 0 || largeIcon == 0 {
		return nil, errors.New("no icons found in file")
	}
	defer win.DestroyIcon(largeIcon)

	return convertIconToImage(ctx, largeIcon)
}

func getHighResIcon(ctx context.Context, filePath string) (img image.Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			util.GetLogger().Debug(ctx, "PrivateExtractIconsW caused panic (API may not be available)")
			img = nil
			err = errors.New("PrivateExtractIconsW panic")
		}
	}()

	if err = privateExtractIcons.Find(); err != nil {
		return nil, err
	}

	lpIconPath, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		return nil, err
	}

	sizes := []int{256, 128, 64, 48}
	for _, size := range sizes {
		var hIcon win.HICON
		ret, _, callErr := privateExtractIcons.Call(
			uintptr(unsafe.Pointer(lpIconPath)),
			0,
			uintptr(size),
			uintptr(size),
			uintptr(unsafe.Pointer(&hIcon)),
			0,
			1,
			0,
		)
		if callErr != nil &&
			callErr.Error() != "The operation completed successfully." &&
			callErr.Error() != "User stopped resource enumeration." {
			continue
		}

		if ret > 0 && hIcon != 0 {
			defer win.DestroyIcon(hIcon)
			util.GetLogger().Info(ctx, "Successfully extracted high-res icon using PrivateExtractIconsW")
			return convertIconToImage(ctx, hIcon)
		}
	}

	return nil, errors.New("no icons found in file")
}

func getWindowsDefaultIcon(ctx context.Context) (image.Image, error) {
	if icon, err := getHighResDefaultIcon(ctx); err == nil {
		return icon, nil
	}
	return getStandardDefaultIcon(ctx)
}

func getHighResDefaultIcon(ctx context.Context) (image.Image, error) {
	shell32Path, err := syscall.UTF16PtrFromString("shell32.dll")
	if err != nil {
		return nil, err
	}

	if err := privateExtractIcons.Find(); err != nil {
		return nil, err
	}

	sizes := []int{256, 128, 64, 48}
	for _, size := range sizes {
		var hIcon win.HICON
		ret, _, callErr := privateExtractIcons.Call(
			uintptr(unsafe.Pointer(shell32Path)),
			2,
			uintptr(size),
			uintptr(size),
			uintptr(unsafe.Pointer(&hIcon)),
			0,
			1,
			0,
		)
		if callErr != nil &&
			callErr.Error() != "The operation completed successfully." &&
			callErr.Error() != "User stopped resource enumeration." {
			continue
		}
		if ret > 0 && hIcon != 0 {
			defer win.DestroyIcon(hIcon)
			util.GetLogger().Info(ctx, "Successfully extracted default icon from shell32.dll")
			return convertIconToImage(ctx, hIcon)
		}
	}

	return nil, errors.New("failed to extract high resolution default icon from shell32.dll")
}

func getStandardDefaultIcon(ctx context.Context) (image.Image, error) {
	exeExtension, err := syscall.UTF16PtrFromString(".exe")
	if err != nil {
		return nil, err
	}

	var shfi shFileInfo
	ret, _, _ := shGetFileInfo.Call(
		uintptr(unsafe.Pointer(exeExtension)),
		FILE_ATTRIBUTE_NORMAL,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		SHGFI_ICON|SHGFI_LARGEICON|SHGFI_USEFILEATTRIBUTES,
	)
	if ret == 0 || shfi.HIcon == 0 {
		return nil, errors.New("failed to get default Windows executable icon")
	}
	defer win.DestroyIcon(shfi.HIcon)

	util.GetLogger().Info(ctx, "Using Windows standard default executable icon as fallback")
	return convertIconToImage(ctx, shfi.HIcon)
}

func getFileTypeIconImpl(ctx context.Context, ext string) (string, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)

	// Check cache first
	if _, err := os.Stat(cachePath); err == nil {
		return cachePath, nil
	}

	// Try registry method first (more accurate for associated file types)
	var hIcon win.HICON
	var err error

	hIcon, err = getIconFromRegistry(ext)
	if err != nil {
		// Fallback to SHGetFileInfo
		hIcon, err = getIconFromSHGetFileInfo(ext)
		if err != nil {
			return "", errors.New("failed to get file type icon for " + ext + ": " + err.Error())
		}
	}
	defer win.DestroyIcon(hIcon)

	// Convert HICON to image
	img, convErr := convertIconToImage(ctx, hIcon)
	if convErr != nil {
		return "", errors.New("failed to convert icon to image for " + ext + ": " + convErr.Error())
	}

	// Ensure cache dir exists and save
	cacheDir := filepath.Dir(cachePath)
	if mkdirErr := os.MkdirAll(cacheDir, 0o755); mkdirErr != nil {
		return "", errors.New("failed to create cache dir: " + mkdirErr.Error())
	}

	if saveErr := imaging.Save(img, cachePath); saveErr != nil {
		return "", errors.New("failed to save icon to cache for " + ext + ": " + saveErr.Error())
	}

	return cachePath, nil
}

func convertIconToImage(ctx context.Context, hIcon win.HICON) (image.Image, error) {
	var iconInfo win.ICONINFO
	if !win.GetIconInfo(hIcon, &iconInfo) {
		return nil, errors.New("failed to get icon info")
	}
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmColor))
	defer win.DeleteObject(win.HGDIOBJ(iconInfo.HbmMask))

	hdc := win.GetDC(0)
	defer win.ReleaseDC(0, hdc)

	var bitmap win.BITMAP
	if win.GetObject(win.HGDIOBJ(iconInfo.HbmColor), uintptr(unsafe.Sizeof(bitmap)), unsafe.Pointer(&bitmap)) == 0 {
		return nil, errors.New("failed to get bitmap object")
	}

	width := int(bitmap.BmWidth)
	height := int(bitmap.BmHeight)

	var bmpInfo win.BITMAPINFO
	bmpInfo.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmpInfo.BmiHeader))
	bmpInfo.BmiHeader.BiWidth = int32(width)
	bmpInfo.BmiHeader.BiHeight = -int32(height)
	bmpInfo.BmiHeader.BiPlanes = 1
	bmpInfo.BmiHeader.BiBitCount = 32
	bmpInfo.BmiHeader.BiCompression = win.BI_RGB

	bits := make([]byte, width*height*4)
	if win.GetDIBits(hdc, win.HBITMAP(iconInfo.HbmColor), 0, uint32(height), &bits[0], &bmpInfo, win.DIB_RGB_COLORS) == 0 {
		return nil, errors.New("failed to get DIB bits")
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			base := y*width*4 + x*4
			b := bits[base+0]
			g := bits[base+1]
			r := bits[base+2]
			a := bits[base+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

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
		return nil, errors.New("extracted icon is empty or fully transparent")
	}

	return img, nil
}
