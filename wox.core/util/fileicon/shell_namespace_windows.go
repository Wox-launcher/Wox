package fileicon

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
	"wox/util"
	"wox/util/imagecache"
	"wox/util/shell"

	"github.com/disintegration/imaging"
	win "github.com/lxn/win"
)

const shgfiPIDL = 0x000000008

var (
	ole32              = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx     = ole32.NewProc("CoInitializeEx")
	coUninitialize     = ole32.NewProc("CoUninitialize")
	shParseDisplayName = shell32.NewProc("SHParseDisplayName")
	ilFree             = shell32.NewProc("ILFree")
)

// GetShellNamespaceIcon returns a cached PNG for a Shell parsing name such as ::{CLSID}.
// Control Panel applets expose their artwork through this path. All Tasks children do not parse this way.
func GetShellNamespaceIcon(ctx context.Context, parsingName string) (string, error) {
	parsingName = strings.TrimSpace(parsingName)
	if parsingName == "" {
		return "", errors.New("empty shell namespace path")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cachePath := shellNamespaceCachePath(parsingName)
	if info, err := os.Stat(cachePath); err == nil && info.Size() > 0 {
		imagecache.Touch(ctx, cachePath, info)
		return cachePath, nil
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	releaseCOM := initShellCOM()
	defer releaseCOM()

	img, err := shellNamespaceIcon(parsingName)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", err
	}
	if err := imaging.Save(img, cachePath); err != nil {
		return "", err
	}
	return cachePath, nil
}

// GetShellFolderIcons returns cached PNGs for the children of a Shell folder, keyed by parsing name.
// Control Panel tasks cannot be parsed from their path strings, so their icons come from this enumeration.
func GetShellFolderIcons(ctx context.Context, parent string) (map[string]string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	icons := make(map[string]string)
	err := shell.VisitNamespaceChildren(parent, func(parsingName string, absolutePIDL uintptr) error {
		key := strings.ToLower(parsingName)
		cachePath := shellNamespaceCachePath(parsingName)
		if info, statErr := os.Stat(cachePath); statErr == nil && info.Size() > 0 {
			imagecache.Touch(ctx, cachePath, info)
			icons[key] = cachePath
			return nil
		}
		img, iconErr := shellIconFromPIDLValue(absolutePIDL)
		if iconErr != nil {
			return nil
		}
		if mkdirErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkdirErr != nil {
			return mkdirErr
		}
		if saveErr := imaging.Save(img, cachePath); saveErr != nil {
			return saveErr
		}
		icons[key] = cachePath
		return nil
	})
	if err != nil {
		return icons, err
	}
	return icons, nil
}

func shellNamespaceCachePath(parsingName string) string {
	return filepath.Join(util.GetLocation().GetImageCacheDirectory(), "shellns_"+util.Md5([]byte(strings.ToLower(parsingName)))+".png")
}

func shellIconFromPIDLValue(pidl uintptr) (image.Image, error) {
	shellIconMu.Lock()
	defer shellIconMu.Unlock()

	var shfi shFileInfo
	ret, _, _ := shGetFileInfo.Call(
		pidl,
		0,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		uintptr(shgfiPIDL|SHGFI_ICON|SHGFI_LARGEICON),
	)
	if ret == 0 || shfi.HIcon == 0 {
		return nil, errors.New("SHGetFileInfo failed for shell PIDL")
	}
	defer win.DestroyIcon(shfi.HIcon)
	return convertIconToImage(context.Background(), shfi.HIcon)
}

func initShellCOM() func() {
	hr, _, _ := coInitializeEx.Call(0, uintptr(0x2|0x4)) // COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE
	if hr == 0 || hr == 1 {
		return func() { coUninitialize.Call() }
	}
	return func() {}
}

func shellNamespaceIcon(parsingName string) (image.Image, error) {
	shellIconMu.Lock()
	defer shellIconMu.Unlock()

	pathUTF16, err := syscall.UTF16PtrFromString(parsingName)
	if err != nil {
		return nil, err
	}
	img, pathErr := shellIconFromPath(pathUTF16)
	if pathErr == nil {
		return img, nil
	}
	img, pidlErr := shellIconFromPIDL(pathUTF16)
	if pidlErr == nil {
		return img, nil
	}
	return nil, errors.Join(pathErr, pidlErr)
}

func shellIconFromPath(pathUTF16 *uint16) (image.Image, error) {
	var shfi shFileInfo
	ret, _, _ := shGetFileInfo.Call(
		uintptr(unsafe.Pointer(pathUTF16)),
		0,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		uintptr(SHGFI_ICON|SHGFI_LARGEICON),
	)
	if ret == 0 || shfi.HIcon == 0 {
		return nil, errors.New("SHGetFileInfo failed for shell path")
	}
	defer win.DestroyIcon(shfi.HIcon)
	return convertIconToImage(context.Background(), shfi.HIcon)
}

func shellIconFromPIDL(pathUTF16 *uint16) (image.Image, error) {
	var pidl uintptr
	hr, _, _ := shParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathUTF16)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		0,
	)
	if int32(hr) < 0 || pidl == 0 {
		return nil, errors.New("SHParseDisplayName failed")
	}
	defer ilFree.Call(pidl)

	var shfi shFileInfo
	ret, _, _ := shGetFileInfo.Call(
		pidl,
		0,
		uintptr(unsafe.Pointer(&shfi)),
		uintptr(unsafe.Sizeof(shfi)),
		uintptr(shgfiPIDL|SHGFI_ICON|SHGFI_LARGEICON),
	)
	if ret == 0 || shfi.HIcon == 0 {
		return nil, errors.New("SHGetFileInfo failed for shell PIDL")
	}
	defer win.DestroyIcon(shfi.HIcon)
	return convertIconToImage(context.Background(), shfi.HIcon)
}
