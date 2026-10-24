package fileicon

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
	"wox/util"
	"wox/util/imagecache"

	"github.com/disintegration/imaging"
	"github.com/go-ole/go-ole"
	win "github.com/lxn/win"
)

// GetShortcutIcon preserves custom artwork and fingerprints both the link and its resource.
func GetShortcutIcon(ctx context.Context, shortcut, target, location string) (string, string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	source := target
	if location != "" {
		if resource, _ := parseIconLocation(location); resource != "" {
			source = resource
		}
	}
	if source == "" {
		source = shortcut
	}
	linkInfo, err := os.Stat(shortcut)
	if err != nil {
		return "", source, err
	}
	if isOfflineIconSource(linkInfo) {
		return "", source, errors.New("shortcut is offline")
	}
	var sourceTime int64
	if info, err := os.Stat(source); err == nil {
		if isOfflineIconSource(info) {
			return "", source, errors.New("shortcut icon source is offline")
		}
		sourceTime = info.ModTime().UnixNano()
	}
	fingerprint := fmt.Sprintf("%s|%s|%d|%d", source, location, sourceTime, linkInfo.ModTime().UnixNano())
	// Keep the shortcut path in the stem so explicit cache cleanup can remove
	// every resource revision without knowing its old location or timestamp.
	cache := filepath.Join(util.GetLocation().GetImageCacheDirectory(), buildPathCacheStem(shortcut, util.ResultGridIconSize)+util.Md5([]byte(fingerprint))+".png")
	if info, err := os.Stat(cache); err == nil && info.Size() > 0 {
		imagecache.Touch(ctx, cache, info)
		return cache, source, nil
	}
	artwork, err := getShellFileIcon(ctx, shortcut, util.ResultGridIconSize)
	if err != nil {
		return "", source, err
	}
	if err := os.MkdirAll(filepath.Dir(cache), 0o755); err != nil {
		return "", source, err
	}
	if err := imaging.Save(artwork, cache); err != nil {
		return "", source, err
	}
	return cache, source, nil
}

// expandIconEnvironment uses Windows expansion rules, including case-insensitive variable names.
func expandIconEnvironment(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "\"")
	input, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return value
	}
	needed, _, _ := expandEnvironmentStrings.Call(uintptr(unsafe.Pointer(input)), 0, 0)
	if needed == 0 || needed > 32768 {
		return value
	}
	output := make([]uint16, needed)
	written, _, _ := expandEnvironmentStrings.Call(uintptr(unsafe.Pointer(input)), uintptr(unsafe.Pointer(&output[0])), uintptr(len(output)))
	if written == 0 || written > uintptr(len(output)) {
		return value
	}
	return syscall.UTF16ToString(output)
}

var shellCreateItem = shell32.NewProc("SHCreateItemFromParsingName")
var shellImageFactoryIID = ole.NewGUID("{BCC18B79-BA16-442F-80C4-8A59C30C463B}")
var expandEnvironmentStrings = syscall.NewLazyDLL("kernel32.dll").NewProc("ExpandEnvironmentStringsW")

// getShellFileIcon asks Windows for the item's artwork, including custom shortcut icons.
// COM stays on one OS thread, and shell extensions are never entered concurrently.
func getShellFileIcon(ctx context.Context, path string, size int) (image.Image, error) {
	// Wait before pinning a thread: parallel indexing can queue many Shell requests.
	shellIconMu.Lock()
	defer shellIconMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		oleErr, ok := err.(*ole.OleError)
		if !ok || (oleErr.Code() != 1 && oleErr.Code() != 0x80010106) {
			return nil, err
		}
		if oleErr.Code() == 1 {
			defer ole.CoUninitialize()
		}
	} else {
		defer ole.CoUninitialize()
	}
	path16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var factory *struct{ vtable *[4]uintptr }
	hr, _, _ := shellCreateItem.Call(uintptr(unsafe.Pointer(path16)), 0, uintptr(unsafe.Pointer(shellImageFactoryIID)), uintptr(unsafe.Pointer(&factory)))
	if factory != nil {
		defer syscall.SyscallN(factory.vtable[2], uintptr(unsafe.Pointer(factory)))
	}
	if int32(hr) < 0 || factory == nil {
		return nil, fmt.Errorf("create shell image factory: HRESULT %#x", hr)
	}
	size = max(16, min(size, 256))
	var bitmap win.HBITMAP
	// SIZE is passed by value: two stack words on x86, one packed word on 64-bit Windows.
	if unsafe.Sizeof(uintptr(0)) == 8 {
		packed := uint64(uint32(size)) | uint64(uint32(size))<<32
		hr, _, _ = syscall.SyscallN(factory.vtable[3], uintptr(unsafe.Pointer(factory)), uintptr(packed), 0x4|0x1, uintptr(unsafe.Pointer(&bitmap)))
	} else {
		hr, _, _ = syscall.SyscallN(factory.vtable[3], uintptr(unsafe.Pointer(factory)), uintptr(size), uintptr(size), 0x4|0x1, uintptr(unsafe.Pointer(&bitmap)))
	}
	if bitmap != 0 {
		defer win.DeleteObject(win.HGDIOBJ(bitmap))
	}
	if int32(hr) < 0 || bitmap == 0 {
		return nil, fmt.Errorf("get shell icon: HRESULT %#x", hr)
	}
	bits, width, height, err := readIconBitmap(bitmap)
	if err != nil {
		return nil, err
	}
	// ImageFactory may return an opaque RGB bitmap without an alpha channel.
	return iconPixels(bits, nil, width, height, false)
}

// readIconBitmap reads a top-down BGRA copy without selecting the owned bitmap into a DC.
func readIconBitmap(bitmap win.HBITMAP) ([]byte, int, int, error) {
	var info win.BITMAP
	if bitmap == 0 || win.GetObject(win.HGDIOBJ(bitmap), uintptr(unsafe.Sizeof(info)), unsafe.Pointer(&info)) == 0 {
		return nil, 0, 0, errors.New("cannot inspect icon bitmap")
	}
	width, height := int(info.BmWidth), int(info.BmHeight)
	if width <= 0 || height <= 0 || width > 4096 || height > 8192 {
		return nil, 0, 0, errors.New("invalid icon dimensions")
	}
	// A screen DC must be released on the same thread that acquired it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	dc := win.GetDC(0)
	if dc == 0 {
		return nil, 0, 0, errors.New("cannot acquire icon DC")
	}
	defer win.ReleaseDC(0, dc)
	var dib win.BITMAPINFO
	dib.BmiHeader = win.BITMAPINFOHEADER{BiSize: uint32(unsafe.Sizeof(dib.BmiHeader)), BiWidth: int32(width), BiHeight: -int32(height), BiPlanes: 1, BiBitCount: 32, BiCompression: win.BI_RGB}
	bits := make([]byte, width*height*4)
	if win.GetDIBits(dc, bitmap, 0, uint32(height), &bits[0], &dib, win.DIB_RGB_COLORS) != int32(height) {
		return nil, 0, 0, errors.New("cannot read icon pixels")
	}
	return bits, width, height, nil
}

// convertWindowsIcon preserves alpha icons and reconstructs legacy transparency from the AND mask.
func convertWindowsIcon(icon win.HICON) (image.Image, error) {
	var info win.ICONINFO
	if !win.GetIconInfo(icon, &info) {
		return nil, errors.New("cannot inspect icon")
	}
	if info.HbmColor != 0 {
		defer win.DeleteObject(win.HGDIOBJ(info.HbmColor))
	}
	if info.HbmMask != 0 {
		defer win.DeleteObject(win.HGDIOBJ(info.HbmMask))
	}
	if info.HbmColor == 0 {
		mask, width, height, err := readIconBitmap(info.HbmMask)
		if err != nil {
			return nil, err
		}
		if height%2 != 0 {
			return nil, errors.New("invalid monochrome icon mask")
		}
		height /= 2
		// Monochrome icons store the AND mask above the XOR color plane.
		return iconPixels(mask[width*height*4:], mask[:width*height*4], width, height, true)
	}
	bits, width, height, err := readIconBitmap(info.HbmColor)
	if err != nil {
		return nil, err
	}
	hasAlpha := false
	for i := 3; i < len(bits); i += 4 {
		hasAlpha = hasAlpha || bits[i] != 0
	}
	if hasAlpha {
		return iconPixels(bits, nil, width, height, false)
	}
	mask, mw, mh, err := readIconBitmap(info.HbmMask)
	if err != nil {
		return nil, err
	}
	if mw != width || mh != height {
		return nil, errors.New("icon mask dimensions differ")
	}
	return iconPixels(bits, mask, width, height, false)
}

// iconPixels accepts opaque black artwork and rejects genuinely transparent images.
// Invert-only monochrome pixels cannot be represented in PNG and remain transparent.
func iconPixels(bits, mask []byte, width, height int, monochrome bool) (image.Image, error) {
	if width <= 0 || height <= 0 || len(bits) != width*height*4 || (mask != nil && len(mask) != len(bits)) {
		return nil, errors.New("invalid icon pixel buffer")
	}
	hasAlpha := false
	if !monochrome {
		for i := 3; i < len(bits); i += 4 {
			hasAlpha = hasAlpha || bits[i] != 0
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	visible := false
	for i := 0; i < len(bits); i += 4 {
		alpha := bits[i+3]
		if !hasAlpha {
			alpha = 255
			if mask != nil && mask[i] != 0 {
				alpha = 0
			}
		}
		if alpha != 0 {
			visible = true
			img.SetRGBA((i/4)%width, (i/4)/width, color.RGBA{R: bits[i+2], G: bits[i+1], B: bits[i], A: alpha})
		}
	}
	if !visible {
		return nil, errors.New("extracted icon is fully transparent")
	}
	return img, nil
}
