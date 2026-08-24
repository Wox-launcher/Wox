//go:build windows

package woxui

import (
	"context"
	"fmt"
	"image"
	"unsafe"

	"wox/util"

	xdraw "golang.org/x/image/draw"

	"github.com/lxn/win"
)

const (
	windowsIconSmall = 0
	windowsIconBig   = 1
)

// applyWindowIcon publishes a per-window HICON so the taskbar does not keep the process Wox glyph.
func (w *platformWindow) applyWindowIcon() {
	if w == nil || w.hwnd == 0 || w.options.Icon == nil {
		return
	}
	small, err := windowsHICONFromImage(w.options.Icon, windowsSmallIconSize())
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to create Windows small window icon: %s", err.Error()))
		return
	}
	big, err := windowsHICONFromImage(w.options.Icon, windowsBigIconSize(w.options.Icon))
	if err != nil {
		win.DestroyIcon(small)
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to create Windows large window icon: %s", err.Error()))
		return
	}
	w.destroyWindowIcons()
	w.smallIcon = small
	w.bigIcon = big
	win.SendMessage(w.hwnd, win.WM_SETICON, windowsIconSmall, uintptr(small))
	win.SendMessage(w.hwnd, win.WM_SETICON, windowsIconBig, uintptr(big))
}

func (w *platformWindow) destroyWindowIcons() {
	if w == nil {
		return
	}
	if w.smallIcon != 0 {
		win.DestroyIcon(w.smallIcon)
		w.smallIcon = 0
	}
	if w.bigIcon != 0 {
		win.DestroyIcon(w.bigIcon)
		w.bigIcon = 0
	}
}

// windowsSmallIconSize uses the shell's current small-icon metric for title-bar and Alt+Tab.
func windowsSmallIconSize() int {
	size := int(win.GetSystemMetrics(win.SM_CXSMICON))
	if size <= 0 {
		return 16
	}
	if size > 256 {
		return 256
	}
	return size
}

// windowsBigIconSize keeps the source resolution so the taskbar can downscale a sharp glyph.
func windowsBigIconSize(source *Image) int {
	if source == nil {
		return 32
	}
	size := source.Width
	if source.Height > size {
		size = source.Height
	}
	if size < 32 {
		return 32
	}
	if size > 256 {
		return 256
	}
	return size
}

// windowsHICONFromImage rasterizes a premultiplied RGBA image into a 32-bit alpha HICON.
func windowsHICONFromImage(source *Image, size int) (win.HICON, error) {
	rgba, err := windowsIconRGBA(source, size)
	if err != nil {
		return 0, err
	}
	header := win.BITMAPINFOHEADER{
		BiSize: uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})), BiWidth: int32(size), BiHeight: -int32(size),
		BiPlanes: 1, BiBitCount: 32, BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	color := win.CreateDIBSection(0, &header, win.DIB_RGB_COLORS, &bits, 0, 0)
	if color == 0 || bits == nil {
		return 0, fmt.Errorf("create window icon bitmap failed")
	}
	pixels := unsafe.Slice((*byte)(bits), size*size*4)
	for offset := 0; offset < len(pixels); offset += 4 {
		pixels[offset] = rgba.Pix[offset+2]
		pixels[offset+1] = rgba.Pix[offset+1]
		pixels[offset+2] = rgba.Pix[offset]
		pixels[offset+3] = rgba.Pix[offset+3]
	}
	maskStride := ((size + 31) / 32) * 4
	maskBits := make([]byte, maskStride*size)
	mask := win.CreateBitmap(int32(size), int32(size), 1, 1, unsafe.Pointer(&maskBits[0]))
	if mask == 0 {
		win.DeleteObject(win.HGDIOBJ(color))
		return 0, fmt.Errorf("create window icon mask failed")
	}
	icon := win.CreateIconIndirect(&win.ICONINFO{FIcon: win.TRUE, HbmMask: mask, HbmColor: color})
	win.DeleteObject(win.HGDIOBJ(color))
	win.DeleteObject(win.HGDIOBJ(mask))
	if icon == 0 {
		return 0, fmt.Errorf("create window icon failed")
	}
	return icon, nil
}

func windowsIconRGBA(source *Image, size int) (*image.RGBA, error) {
	if source == nil || size <= 0 || source.Width <= 0 || source.Height <= 0 || len(source.pixels) < source.Width*source.Height*4 {
		return nil, fmt.Errorf("window icon image is empty")
	}
	src := &image.RGBA{Pix: source.pixels, Stride: source.Width * 4, Rect: image.Rect(0, 0, source.Width, source.Height)}
	if source.Width == size && source.Height == size {
		return src, nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst, nil
}
