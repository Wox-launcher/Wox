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
	"wox/common"

	"github.com/disintegration/imaging"
	win "github.com/lxn/win"
	"golang.org/x/sys/windows/registry"
)

// Windows implementation using SHGetFileInfoW to retrieve the HICON for a file extension
// and converting it to PNG, cached on disk by extension.

var (
	shell32          = syscall.NewLazyDLL("shell32.dll")
	gdi32            = syscall.NewLazyDLL("gdi32.dll")
	user32           = syscall.NewLazyDLL("user32.dll")
	shGetFileInfo    = shell32.NewProc("SHGetFileInfoW")
	extractIconEx    = shell32.NewProc("ExtractIconExW")
	createDIBSection = gdi32.NewProc("CreateDIBSection")
	createSolidBrush = gdi32.NewProc("CreateSolidBrush")
	fillRect         = user32.NewProc("FillRect")
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

func getFileTypeIconImpl(ctx context.Context, ext string) (common.WoxImage, error) {
	const size = 48
	cachePath := buildCachePath(ext, size)

	// Check cache first
	if _, err := os.Stat(cachePath); err == nil {
		return common.NewWoxImageAbsolutePath(cachePath), nil
	}

	// Try registry method first (more accurate for associated file types)
	var hIcon win.HICON
	var err error

	hIcon, err = getIconFromRegistry(ext)
	if err != nil {
		// Fallback to SHGetFileInfo
		hIcon, err = getIconFromSHGetFileInfo(ext)
		if err != nil {
			return common.WoxImage{}, errors.New("failed to get file type icon for " + ext + ": " + err.Error())
		}
	}
	defer win.DestroyIcon(hIcon)

	// Convert HICON to image
	img, convErr := convertIconToImage(ctx, hIcon)
	if convErr != nil {
		return common.WoxImage{}, errors.New("failed to convert icon to image for " + ext + ": " + convErr.Error())
	}

	// Ensure cache dir exists and save
	cacheDir := filepath.Dir(cachePath)
	if mkdirErr := os.MkdirAll(cacheDir, 0o755); mkdirErr != nil {
		return common.WoxImage{}, errors.New("failed to create cache dir: " + mkdirErr.Error())
	}

	if saveErr := imaging.Save(img, cachePath); saveErr != nil {
		return common.WoxImage{}, errors.New("failed to save icon to cache for " + ext + ": " + saveErr.Error())
	}

	return common.NewWoxImageAbsolutePath(cachePath), nil
}

func convertIconToImage(ctx context.Context, hIcon win.HICON) (image.Image, error) {
	// Use a more reliable method: draw icon to a DC and capture the bitmap
	const size = 48

	hdc := win.GetDC(0)
	if hdc == 0 {
		return nil, errors.New("failed to get DC")
	}
	defer win.ReleaseDC(0, hdc)

	// Create a compatible DC and bitmap
	memDC := win.CreateCompatibleDC(hdc)
	if memDC == 0 {
		return nil, errors.New("failed to create compatible DC")
	}
	defer win.DeleteDC(memDC)

	// Create a 32-bit RGBA bitmap
	var bmi win.BITMAPINFO
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = size
	bmi.BmiHeader.BiHeight = -size // top-down
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = win.BI_RGB

	var pBits unsafe.Pointer
	ret, _, _ := createDIBSection.Call(
		uintptr(hdc),
		uintptr(unsafe.Pointer(&bmi)),
		uintptr(win.DIB_RGB_COLORS),
		uintptr(unsafe.Pointer(&pBits)),
		0,
		0,
	)
	hBitmap := win.HBITMAP(ret)
	if hBitmap == 0 {
		return nil, errors.New("failed to create DIB section")
	}
	defer win.DeleteObject(win.HGDIOBJ(hBitmap))

	// Select bitmap into DC
	oldBitmap := win.SelectObject(memDC, win.HGDIOBJ(hBitmap))
	defer win.SelectObject(memDC, oldBitmap)

	// Fill with transparent background (all zeros = transparent black)
	// This allows the icon's own transparency to show through
	clearBits := make([]byte, size*size*4)
	copy((*[1 << 30]byte)(pBits)[:size*size*4], clearBits)

	// Draw the icon
	if !win.DrawIconEx(memDC, 0, 0, hIcon, size, size, 0, 0, win.DI_NORMAL) {
		return nil, errors.New("failed to draw icon")
	}

	// Copy bitmap data to Go image
	bits := make([]byte, size*size*4)
	copy(bits, (*[1 << 30]byte)(pBits)[:size*size*4])

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	// Convert BGRA to RGBA
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			base := y*size*4 + x*4
			b := bits[base+0]
			g := bits[base+1]
			r := bits[base+2]
			a := bits[base+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img, nil
}
