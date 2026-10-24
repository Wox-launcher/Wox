package fileicon

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
	"wox/util"
	"wox/util/imagecache"

	"github.com/disintegration/imaging"
	win "github.com/lxn/win"
	"golang.org/x/sys/windows/registry"
)

// Windows implementation using SHGetFileInfoW to retrieve the HICON for a file extension
// and converting it to PNG, cached on disk by extension.

var (
	shell32       = syscall.NewLazyDLL("shell32.dll")
	user32        = syscall.NewLazyDLL("user32.dll")
	shGetFileInfo = shell32.NewProc("SHGetFileInfoW")
	extractIconEx = shell32.NewProc("ExtractIconExW")
	shellIconMu   sync.Mutex

	privateExtractIcons iconExtractionProcedure = user32.NewProc("PrivateExtractIconsW")
)

// iconExtractionProcedure keeps the native return-value and handle-ownership contract testable.
type iconExtractionProcedure interface {
	Find() error
	Call(...uintptr) (uintptr, uintptr, error)
}

type shFileInfo struct {
	HIcon         win.HICON
	IIcon         int32
	DwAttributes  uint32
	SzDisplayName [260]uint16
	SzTypeName    [80]uint16
}

const (
	SHGFI_ICON                           = 0x000000100
	SHGFI_LARGEICON                      = 0x000000000
	SHGFI_USEFILEATTRIBUTES              = 0x000000010
	FILE_ATTRIBUTE_NORMAL                = 0x00000080
	FILE_ATTRIBUTE_OFFLINE               = 0x00001000
	FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = 0x00400000
	FILE_ATTRIBUTE_RECALL_ON_OPEN        = 0x00040000
)

// parseIconLocation parses icon location string like "C:\path\to\file.dll,-123"
func parseIconLocation(location string) (path string, index int) {
	// Only the final comma separates an optional resource index from the path.
	path = strings.TrimSpace(location)
	if comma := strings.LastIndex(path, ","); comma >= 0 {
		if parsed, err := strconv.Atoi(strings.TrimSpace(path[comma+1:])); err == nil {
			index = parsed
			path = path[:comma]
		}
	}
	return expandIconEnvironment(path), index
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
	shellIconMu.Lock()
	defer shellIconMu.Unlock()
	// Bug fix: application indexing can request many file-type icons in
	// parallel during smoke startup. Serializing the shell icon fallback avoids
	// concurrent SHGetFileInfoW access that can terminate the Windows process
	// before Go error handling can recover.
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

func getFileIconImpl(ctx context.Context, filePath string, size int) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", errors.New("empty path")
	}

	fi, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	cachePath := buildPathCachePath(filePath, size, fi.ModTime().UnixNano())
	if info, err := os.Stat(cachePath); err == nil {
		imagecache.Touch(ctx, cachePath, info)
		return cachePath, nil
	}

	// Check if the file is an offline file (e.g. OneDrive, Phone Link)
	// If it is, avoid reading its contents to extract high res icons, to prevent triggering an automatic file download.
	if isOfflineIconSource(fi) {
		util.GetLogger().Debug(ctx, "File is offline, falling back to extension icon to avoid download: "+filePath)
		return getFileTypeIconImpl(ctx, filepath.Ext(filePath), size)
	}

	img, err := getHighResIcon(ctx, filePath)
	if err != nil {
		img, err = getIconUsingExtractIconEx(ctx, filePath)
	}
	// Ask the Shell for per-item artwork before collapsing to a file-type icon.
	if err != nil {
		img, err = getShellFileIcon(ctx, filePath, size)
	}
	if err != nil {
		util.GetLogger().Debug(ctx, "No embedded file icon found, fallback to associated file type icon: "+filePath)
		return GetFileTypeIconWithSize(ctx, filepath.Ext(filePath), size)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkdirErr != nil {
		return "", errors.New("failed to create cache dir: " + mkdirErr.Error())
	}
	if saveErr := imaging.Save(img, cachePath); saveErr != nil {
		return "", errors.New("failed to save icon to cache for " + filePath + ": " + saveErr.Error())
	}

	return cachePath, nil
}

// isOfflineIconSource prevents native extraction from hydrating cloud placeholders.
func isOfflineIconSource(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&(FILE_ATTRIBUTE_OFFLINE|FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS|FILE_ATTRIBUTE_RECALL_ON_OPEN) != 0
}

func getIconUsingExtractIconEx(ctx context.Context, filePath string) (image.Image, error) {
	lpIconPath, err := syscall.UTF16PtrFromString(filePath)
	if err != nil {
		return nil, err
	}

	var largeIcon win.HICON
	ret, _, _ := extractIconEx.Call(
		uintptr(unsafe.Pointer(lpIconPath)),
		0,
		uintptr(unsafe.Pointer(&largeIcon)),
		0, // Do not allocate an unused small icon.
		1,
	)
	if largeIcon != 0 {
		defer win.DestroyIcon(largeIcon)
	}
	if ret == 0 || ret == 0xFFFFFFFF || largeIcon == 0 {
		return nil, errors.New("no icons found in file")
	}

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
		ret, _, _ := privateExtractIcons.Call(
			uintptr(unsafe.Pointer(lpIconPath)),
			0,
			uintptr(size),
			uintptr(size),
			uintptr(unsafe.Pointer(&hIcon)),
			0,
			1,
			0,
		)
		// Last-error can be stale or localized even on success; own every returned handle.
		if hIcon != 0 {
			defer win.DestroyIcon(hIcon)
		}

		if ret > 0 && ret != 0xFFFFFFFF && hIcon != 0 {
			util.GetLogger().Info(ctx, "Successfully extracted high-res icon using PrivateExtractIconsW")
			if converted, convertErr := convertIconToImage(ctx, hIcon); convertErr == nil {
				return converted, nil
			}
			// A broken large representation must not hide a usable smaller icon.
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
		ret, _, _ := privateExtractIcons.Call(
			uintptr(unsafe.Pointer(shell32Path)),
			2,
			uintptr(size),
			uintptr(size),
			uintptr(unsafe.Pointer(&hIcon)),
			0,
			1,
			0,
		)
		// Last-error can be stale or localized even on success; own every returned handle.
		if hIcon != 0 {
			defer win.DestroyIcon(hIcon)
		}
		if ret > 0 && ret != 0xFFFFFFFF && hIcon != 0 {
			util.GetLogger().Info(ctx, "Successfully extracted default icon from shell32.dll")
			if converted, convertErr := convertIconToImage(ctx, hIcon); convertErr == nil {
				return converted, nil
			}
			// A broken large representation must not hide a usable smaller icon.
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

func getFileTypeIconImpl(ctx context.Context, ext string, size int) (string, error) {
	cachePath := buildCachePath(ext, size)

	// Check cache first
	if info, err := os.Stat(cachePath); err == nil {
		imagecache.Touch(ctx, cachePath, info)
		return cachePath, nil
	}

	// Try registry method first (more accurate for associated file types)
	var hIcon win.HICON
	var err error
	var img image.Image

	hIcon, err = getIconFromRegistry(ext)
	if err != nil {
		// Bug fix: SHGetFileInfoW can terminate the Windows smoke process for
		// some transient extension lookups before Go can recover. When registry
		// association lookup fails, use Wox's default shell icon extraction
		// instead of the unsafe shell file-type fallback.
		img, err = getHighResDefaultIcon(ctx)
		if err != nil {
			return "", errors.New("failed to get fallback file type icon for " + ext + ": " + err.Error())
		}
	} else {
		defer win.DestroyIcon(hIcon)
		var convErr error
		// Convert HICON to image
		img, convErr = convertIconToImage(ctx, hIcon)
		if convErr != nil {
			return "", errors.New("failed to convert icon to image for " + ext + ": " + convErr.Error())
		}
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

// convertIconToImage preserves Windows alpha and legacy mask semantics.
func convertIconToImage(ctx context.Context, hIcon win.HICON) (image.Image, error) {
	return convertWindowsIcon(hIcon)
}

// HasDedicatedExecutableIcon reports whether an executable icon differs from the generic Windows .exe icon.
func HasDedicatedExecutableIcon(ctx context.Context, filePath string) (bool, error) {
	if !strings.EqualFold(filepath.Ext(filePath), ".exe") {
		return true, nil
	}
	return HasDedicatedFileIcon(ctx, filePath)
}

// HasDedicatedFileIcon checks the same embedded artwork used by file icon
// conversion, without accepting a successful generic file-association fallback.
func HasDedicatedFileIcon(ctx context.Context, filePath string) (bool, error) {
	fileIcon, err := getHighResIcon(ctx, filePath)
	if err != nil {
		fileIcon, err = getIconUsingExtractIconEx(ctx, filePath)
		if err != nil {
			return false, nil
		}
	}

	return isDedicatedWindowsIcon(ctx, fileIcon)
}

// HasDedicatedImageIcon classifies already resolved artwork without reopening
// the executable or DLL and losing the resource selection made by the Shell.
func HasDedicatedImageIcon(ctx context.Context, iconPath string) (bool, error) {
	icon, err := imaging.Open(iconPath)
	if err != nil {
		return false, err
	}
	return isDedicatedWindowsIcon(ctx, icon)
}

// isDedicatedWindowsIcon excludes generic executable artwork from visual app grids.
func isDedicatedWindowsIcon(ctx context.Context, icon image.Image) (bool, error) {
	defaultIcon, err := getWindowsDefaultIcon(ctx)
	if err != nil {
		return false, err
	}

	return !iconsEqual(icon, defaultIcon), nil
}

func iconsEqual(left image.Image, right image.Image) bool {
	if left == nil || right == nil {
		return left == right
	}

	leftIcon := normalizeIcon(left)
	rightIcon := normalizeIcon(right)
	bounds := leftIcon.Bounds()
	if bounds != rightIcon.Bounds() {
		return false
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// PNG stores straight alpha, so caching premultiplied Windows pixels
			// can round their channels. Compare visible values within one 8-bit step.
			lr, lg, lb, la := leftIcon.At(x, y).RGBA()
			rr, rg, rb, ra := rightIcon.At(x, y).RGBA()
			for _, channel := range [4][2]uint32{{lr, rr}, {lg, rg}, {lb, rb}, {la, ra}} {
				if max(channel[0], channel[1])-min(channel[0], channel[1]) > 257 {
					return false
				}
			}
		}
	}

	return true
}

func normalizeIcon(icon image.Image) *image.NRGBA {
	const iconSize = 48
	return imaging.Resize(imaging.Clone(icon), iconSize, iconSize, imaging.Lanczos)
}
