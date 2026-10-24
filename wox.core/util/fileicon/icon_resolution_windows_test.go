package fileicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
	"unsafe"
	"wox/util"

	"github.com/disintegration/imaging"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	win "github.com/lxn/win"
)

// TestNativeIconMasks exercises real HICON conversion, including legacy zero-alpha artwork.
func TestNativeIconMasks(t *testing.T) {
	for _, tc := range []struct {
		name              string
		alpha             byte
		transparent, mono bool
	}{
		{"opaque-black", 255, false, false}, {"legacy-black", 0, false, false},
		{"legacy-transparent", 0, true, false}, {"monochrome-black", 0, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const size = 16
			pixels := make([]byte, size*size*4)
			for i := 3; i < len(pixels); i += 4 {
				pixels[i] = tc.alpha
			}
			maskHeight := int32(size)
			if tc.mono {
				maskHeight *= 2
			}
			maskBits := make([]byte, int(maskHeight)*2)
			if tc.transparent {
				for i := range maskBits {
					maskBits[i] = 255
				}
			}
			mask := win.CreateBitmap(size, maskHeight, 1, 1, unsafe.Pointer(&maskBits[0]))
			if mask == 0 {
				t.Fatal("mask allocation failed")
			}
			defer win.DeleteObject(win.HGDIOBJ(mask))
			var bitmap win.HBITMAP
			if !tc.mono {
				bitmap = win.CreateBitmap(size, size, 1, 32, unsafe.Pointer(&pixels[0]))
				if bitmap == 0 {
					t.Fatal("color allocation failed")
				}
				defer win.DeleteObject(win.HGDIOBJ(bitmap))
			}
			info := win.ICONINFO{FIcon: 1, HbmMask: mask, HbmColor: bitmap}
			icon := win.CreateIconIndirect(&info)
			if icon == 0 {
				t.Fatal("icon allocation failed")
			}
			defer win.DestroyIcon(icon)
			img, err := convertIconToImage(context.Background(), icon)
			if tc.transparent {
				if err == nil {
					t.Fatal("transparent icon accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			r, g, b, a := img.At(8, 8).RGBA()
			if r != 0 || g != 0 || b != 0 || a != 65535 {
				t.Fatalf("black opaque pixel lost: %d %d %d %d", r, g, b, a)
			}
		})
	}
}

// TestIconPixelsPreservesAlpha prevents the legacy fallback from overriding modern alpha.
func TestIconPixelsPreservesAlpha(t *testing.T) {
	img, err := iconPixels([]byte{0, 0, 128, 128, 0, 0, 0, 0}, []byte{255, 255, 255, 0, 0, 0, 0, 0}, 2, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	r, _, _, a := img.At(0, 0).RGBA()
	if r != 32896 || a != 32896 {
		t.Fatalf("premultiplied alpha changed: %d %d", r, a)
	}
	_, _, _, a = img.At(1, 0).RGBA()
	if a != 0 {
		t.Fatal("transparent pixel became opaque")
	}
}

// TestShellShortcutCustomIcon verifies artwork from the shortcut, not its executable target.
func TestShellShortcutCustomIcon(t *testing.T) {
	// Initialize the process-wide crash log before redirecting disposable caches.
	util.GetLogger()
	dir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	iconPath := filepath.Join(dir, "custom,red.ico")
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	// Store one PNG-backed ICO frame, leaving production files untouched.
	header := make([]byte, 22)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], 1)
	header[6], header[7] = 32, 32
	binary.LittleEndian.PutUint16(header[10:], 1)
	binary.LittleEndian.PutUint16(header[12:], 32)
	binary.LittleEndian.PutUint32(header[14:], uint32(pngData.Len()))
	binary.LittleEndian.PutUint32(header[18:], 22)
	if err := os.WriteFile(iconPath, append(header, pngData.Bytes()...), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "test.lnk")
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		t.Fatal(err)
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		t.Fatal(err)
	}
	defer unknown.Release()
	dispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		t.Fatal(err)
	}
	defer dispatch.Release()
	shortcut, err := oleutil.CallMethod(dispatch, "CreateShortcut", link)
	if err != nil {
		t.Fatal(err)
	}
	defer shortcut.Clear()
	for key, value := range map[string]string{"TargetPath": filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe"), "IconLocation": iconPath + ",0"} {
		v, err := oleutil.PutProperty(shortcut.ToIDispatch(), key, value)
		if err != nil {
			t.Fatal(err)
		}
		if v != nil {
			v.Clear()
		}
	}
	saved, err := oleutil.CallMethod(shortcut.ToIDispatch(), "Save")
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		saved.Clear()
	}
	location, err := oleutil.GetProperty(shortcut.ToIDispatch(), "IconLocation")
	if err != nil {
		t.Fatal(err)
	}
	if got, index := parseIconLocation(location.ToString()); got != iconPath || index != 0 {
		t.Fatalf("shortcut fixture lost its resource path: %s", location.ToString())
	}
	location.Clear()
	// Warm the Shell before measuring resource ownership over repeated extraction.
	if embedded, err := getHighResIcon(context.Background(), iconPath); err != nil {
		t.Fatalf("custom ICO extraction failed: %v", err)
	} else {
		r, g, b, a := embedded.At(embedded.Bounds().Dx()/2, embedded.Bounds().Dy()/2).RGBA()
		if r < 60000 || g != 0 || b != 0 || a < 60000 {
			t.Fatalf("invalid ICO fixture: %d %d %d %d", r, g, b, a)
		}
	}
	artwork, err := getShellFileIcon(context.Background(), link, 96)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := artwork.At(artwork.Bounds().Dx()/2, artwork.Bounds().Dy()/2).RGBA()
	if r < 60000 || g > 1000 || b > 1000 || a < 60000 {
		t.Fatalf("custom red logo not used: %d %d %d %d", r, g, b, a)
	}
	resources := syscall.NewLazyDLL("user32.dll").NewProc("GetGuiResources")
	before, _, _ := resources.Call(^uintptr(0), 0)
	for i := 0; i < 100; i++ {
		if _, err := getShellFileIcon(context.Background(), link, 96); err != nil {
			t.Fatal(err)
		}
	}
	after, _, _ := resources.Call(^uintptr(0), 0)
	t.Logf("Shell GDI before=%d after=%d", before, after)
	if after > before+8 {
		t.Fatal("Shell icon extraction leaked GDI objects")
	}
	ctx := context.Background()
	target := filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe")
	first, source, err := GetShortcutIcon(ctx, link, target, iconPath+",0")
	if err != nil || source != iconPath {
		t.Fatalf("shortcut icon: source=%q err=%v", source, err)
	}
	if dedicated, err := HasDedicatedImageIcon(ctx, first); err != nil || !dedicated {
		t.Fatalf("custom shortcut artwork was not dedicated: %v", err)
	}
	second, _, err := GetShortcutIcon(ctx, link, target, iconPath+",0")
	if err != nil || second != first {
		t.Fatalf("unchanged shortcut cache was not reused: %q err=%v", second, err)
	}
	changed := time.Now().Add(time.Hour)
	if err := os.Chtimes(iconPath, changed, changed); err != nil {
		t.Fatal(err)
	}
	third, _, err := GetShortcutIcon(ctx, link, target, iconPath+",0")
	if err != nil || third == first {
		t.Fatalf("changed icon resource reused stale cache: %q err=%v", third, err)
	}
	if err := os.Chtimes(link, changed, changed); err != nil {
		t.Fatal(err)
	}
	fourth, _, err := GetShortcutIcon(ctx, link, target, iconPath+",0")
	if err != nil || fourth == third {
		t.Fatalf("changed shortcut reused stale cache: %q err=%v", fourth, err)
	}
	if err := CleanFileIconCache(ctx, link); err != nil {
		t.Fatal(err)
	}
	for _, cached := range []string{first, third, fourth} {
		if _, err := os.Stat(cached); !os.IsNotExist(err) {
			t.Fatalf("explicit cleanup retained %s: %v", cached, err)
		}
	}
	// Keep the generic case in the same COM apartment as the custom shortcut.
	genericTarget := filepath.Join(dir, "helper.exe")
	if err := os.WriteFile(genericTarget, []byte("no embedded icon"), 0o600); err != nil {
		t.Fatal(err)
	}
	genericLink := filepath.Join(dir, "helper.lnk")
	genericShortcut, err := oleutil.CallMethod(dispatch, "CreateShortcut", genericLink)
	if err != nil {
		t.Fatal(err)
	}
	defer genericShortcut.Clear()
	value, err := oleutil.PutProperty(genericShortcut.ToIDispatch(), "TargetPath", genericTarget)
	if value != nil {
		value.Clear()
	}
	if err != nil {
		t.Fatal(err)
	}
	saved, err = oleutil.CallMethod(genericShortcut.ToIDispatch(), "Save")
	if saved != nil {
		saved.Clear()
	}
	if err != nil {
		t.Fatal(err)
	}
	cached, _, err := GetShortcutIcon(ctx, genericLink, genericTarget, "")
	if err != nil {
		t.Fatal(err)
	}
	if dedicated, err := HasDedicatedImageIcon(ctx, cached); err != nil || dedicated {
		t.Fatalf("generic Shell shortcut artwork classified as dedicated: dedicated=%t err=%v", dedicated, err)
	}
}

// TestFileTypeCacheInvalidatesLegacyArtwork prevents a stale association icon
// from surviving an extractor upgrade after app/path caches have been rebuilt.
func TestFileTypeCacheInvalidatesLegacyArtwork(t *testing.T) {
	util.GetLogger()
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, root)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(util.GetLocation().GetImageCacheDirectory(), "filetype_wox-icon-cache-test_96.png")
	if err := imaging.Save(image.NewRGBA(image.Rect(0, 0, 1, 1)), legacy); err != nil {
		t.Fatal(err)
	}
	current, err := GetFileTypeIcon(context.Background(), ".wox-icon-cache-test")
	if err != nil || current == legacy {
		t.Fatalf("file-type cache reused legacy artwork: %q err=%v", current, err)
	}
	img, err := imaging.Open(current)
	if err != nil || img.Bounds().Dx() <= 1 {
		t.Fatalf("file-type artwork was not regenerated: %v", err)
	}
	if dedicated, err := HasDedicatedImageIcon(context.Background(), current); err != nil || dedicated {
		t.Fatalf("generic resolved artwork classified as dedicated: dedicated=%t err=%v", dedicated, err)
	}
}

// TestIconLocationParsing retains comma paths, negative resource IDs and environment expansion.
func TestIconLocationParsing(t *testing.T) {
	t.Setenv("WOX_ICON_TEST_ROOT", t.TempDir())
	path, index := parseIconLocation("\"%wox_icon_test_root%\\custom,red.dll\",-42")
	if path != filepath.Join(os.Getenv("WOX_ICON_TEST_ROOT"), "custom,red.dll") || index != -42 {
		t.Fatalf("unexpected location: %q %d", path, index)
	}
}
