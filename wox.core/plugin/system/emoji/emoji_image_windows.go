//go:build windows

package emoji

import (
	"errors"
	"image"
	"image/color"
	"syscall"
	"unsafe"
)

var (
	d2d1     = syscall.NewLazyDLL("d2d1.dll")
	dwrite   = syscall.NewLazyDLL("dwrite.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	d2d1CreateFactory     = d2d1.NewProc("D2D1CreateFactory")
	dwriteCreateFactory   = dwrite.NewProc("DWriteCreateFactory")
	coInitializeEx        = ole32.NewProc("CoInitializeEx")
	coUninitialize        = ole32.NewProc("CoUninitialize")
	coCreateInstance      = ole32.NewProc("CoCreateInstance")
	createStreamOnHGlobal = ole32.NewProc("CreateStreamOnHGlobal")
	getHGlobalFromStream  = ole32.NewProc("GetHGlobalFromStream")
	globalLock            = kernel32.NewProc("GlobalLock")
	globalUnlock          = kernel32.NewProc("GlobalUnlock")
	globalSize            = kernel32.NewProc("GlobalSize")
)

// GUIDs
var (
	iidID2D1Factory             = guid{0x06152247, 0x6f50, 0x465a, [8]byte{0x92, 0x45, 0x11, 0x8b, 0xfd, 0x3b, 0x60, 0x07}}
	iidIDWriteFactory           = guid{0xb859ee5a, 0xd838, 0x4b5b, [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48}}
	clsidWICImagingFactory      = guid{0xcacaf262, 0x9370, 0x4615, [8]byte{0xa1, 0x3b, 0x9f, 0x55, 0x39, 0xda, 0x4c, 0x0a}}
	iidIWICImagingFactory       = guid{0xec5ec8a9, 0xc395, 0x4314, [8]byte{0x9c, 0x77, 0x54, 0xd7, 0xa9, 0x35, 0xff, 0x70}}
	guidWICPixelFormat32bppBGRA = guid{0x6fddc324, 0x4e03, 0x4bfe, [8]byte{0xb1, 0x85, 0x3d, 0x77, 0x76, 0x8d, 0xc9, 0x10}}
	guidContainerFormatPng      = guid{0x1b7cfaf4, 0x713f, 0x473c, [8]byte{0xbb, 0xcd, 0x61, 0x37, 0x42, 0x5f, 0xae, 0xaf}}
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

const (
	d2d1FactoryTypeSingleThreaded = 0
	dwriteFactoryTypeShared       = 0
	coinitApartmentThreaded       = 0x2
	clsctxInprocServer            = 0x1
)

func getNativeEmojiImage(emoji string, size int) (image.Image, error) {
	if size <= 0 {
		size = 256
	}

	// Initialize COM
	coInitializeEx.Call(0, coinitApartmentThreaded)
	defer coUninitialize.Call()

	// Create D2D1 Factory
	var d2dFactory uintptr
	hr, _, _ := d2d1CreateFactory.Call(
		d2d1FactoryTypeSingleThreaded,
		uintptr(unsafe.Pointer(&iidID2D1Factory)),
		0,
		uintptr(unsafe.Pointer(&d2dFactory)),
	)
	if hr != 0 || d2dFactory == 0 {
		return nil, errors.New("failed to create D2D1 factory")
	}
	defer release(d2dFactory)

	// Create DWrite Factory
	var dwFactory uintptr
	hr, _, _ = dwriteCreateFactory.Call(
		dwriteFactoryTypeShared,
		uintptr(unsafe.Pointer(&iidIDWriteFactory)),
		uintptr(unsafe.Pointer(&dwFactory)),
	)
	if hr != 0 || dwFactory == 0 {
		return nil, errors.New("failed to create DWrite factory")
	}
	defer release(dwFactory)

	// Create WIC Factory
	var wicFactory uintptr
	hr, _, _ = coCreateInstance.Call(
		uintptr(unsafe.Pointer(&clsidWICImagingFactory)),
		0,
		clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIWICImagingFactory)),
		uintptr(unsafe.Pointer(&wicFactory)),
	)
	if hr != 0 || wicFactory == 0 {
		return nil, errors.New("failed to create WIC factory")
	}
	defer release(wicFactory)

	// Create WIC Bitmap
	wicBitmap, err := wicCreateBitmap(wicFactory, uint32(size), uint32(size))
	if err != nil {
		return nil, err
	}
	defer release(wicBitmap)

	// Create D2D render target from WIC bitmap
	rt, err := d2dCreateWicBitmapRenderTarget(d2dFactory, wicBitmap)
	if err != nil {
		return nil, err
	}
	defer release(rt)

	// Create text format
	textFormat, err := dwCreateTextFormat(dwFactory, float32(size)*0.75)
	if err != nil {
		return nil, err
	}
	defer release(textFormat)

	// Set text alignment to center
	dwTextFormatSetAlignment(textFormat)

	// Create text layout
	utf16Emoji := utf16FromString(emoji)
	textLayout, err := dwCreateTextLayout(dwFactory, utf16Emoji, textFormat, float32(size), float32(size))
	if err != nil {
		return nil, err
	}
	defer release(textLayout)

	// Create brush
	brush, err := d2dCreateSolidColorBrush(rt)
	if err != nil {
		return nil, err
	}
	defer release(brush)

	// Begin draw
	d2dBeginDraw(rt)

	// Clear with white
	d2dClear(rt)

	// Draw text with color font support
	d2dDrawTextLayout(rt, textLayout, brush)

	// End draw
	hr = d2dEndDraw(rt)
	if hr != 0 {
		return nil, errors.New("D2D EndDraw failed")
	}

	// Lock WIC bitmap and copy pixels
	pixels, err := wicCopyPixels(wicBitmap, uint32(size), uint32(size))
	if err != nil {
		return nil, err
	}

	// Convert BGRA to RGBA
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	stride := size * 4
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			offset := y*stride + x*4
			b := pixels[offset+0]
			g := pixels[offset+1]
			r := pixels[offset+2]
			a := pixels[offset+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img, nil
}

func release(obj uintptr) {
	if obj != 0 {
		vtbl := *(*uintptr)(unsafe.Pointer(obj))
		releaseMethod := *(*uintptr)(unsafe.Pointer(vtbl + 2*unsafe.Sizeof(uintptr(0))))
		syscall.SyscallN(releaseMethod, obj)
	}
}

func wicCreateBitmap(factory uintptr, width, height uint32) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(factory))
	// CreateBitmap is at index 17 in IWICImagingFactory vtable
	createBitmap := *(*uintptr)(unsafe.Pointer(vtbl + 17*unsafe.Sizeof(uintptr(0))))
	var bitmap uintptr
	hr, _, _ := syscall.SyscallN(createBitmap, factory, uintptr(width), uintptr(height),
		uintptr(unsafe.Pointer(&guidWICPixelFormat32bppBGRA)), 1, uintptr(unsafe.Pointer(&bitmap)))
	if hr != 0 {
		return 0, errors.New("WIC CreateBitmap failed")
	}
	return bitmap, nil
}

type d2d1RenderTargetProperties struct {
	Type        uint32
	PixelFormat struct {
		Format    uint32
		AlphaMode uint32
	}
	DpiX     float32
	DpiY     float32
	Usage    uint32
	MinLevel uint32
}

func d2dCreateWicBitmapRenderTarget(factory uintptr, wicBitmap uintptr) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(factory))
	// CreateWicBitmapRenderTarget is at index 13 in ID2D1Factory vtable
	createRT := *(*uintptr)(unsafe.Pointer(vtbl + 13*unsafe.Sizeof(uintptr(0))))

	props := d2d1RenderTargetProperties{
		Type: 0, // D2D1_RENDER_TARGET_TYPE_DEFAULT
		PixelFormat: struct {
			Format    uint32
			AlphaMode uint32
		}{
			Format:    87, // DXGI_FORMAT_B8G8R8A8_UNORM
			AlphaMode: 1,  // D2D1_ALPHA_MODE_PREMULTIPLIED
		},
		DpiX:     0,
		DpiY:     0,
		Usage:    0,
		MinLevel: 0,
	}

	var rt uintptr
	hr, _, _ := syscall.SyscallN(createRT, factory, wicBitmap, uintptr(unsafe.Pointer(&props)), uintptr(unsafe.Pointer(&rt)))
	if hr != 0 {
		return 0, errors.New("D2D CreateWicBitmapRenderTarget failed")
	}
	return rt, nil
}

func dwCreateTextFormat(factory uintptr, fontSize float32) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(factory))
	// CreateTextFormat is at index 15 in IDWriteFactory vtable
	createTextFormat := *(*uintptr)(unsafe.Pointer(vtbl + 15*unsafe.Sizeof(uintptr(0))))

	fontFamily := utf16FromString("Segoe UI Emoji")
	locale := utf16FromString("en-us")

	var textFormat uintptr
	hr, _, _ := syscall.SyscallN(createTextFormat, factory,
		uintptr(unsafe.Pointer(&fontFamily[0])), 0,
		400, 0, 5, // weight=normal, style=normal, stretch=normal
		uintptr(*(*uint32)(unsafe.Pointer(&fontSize))),
		uintptr(unsafe.Pointer(&locale[0])),
		uintptr(unsafe.Pointer(&textFormat)))
	if hr != 0 {
		return 0, errors.New("DWrite CreateTextFormat failed")
	}
	return textFormat, nil
}

func dwTextFormatSetAlignment(textFormat uintptr) {
	vtbl := *(*uintptr)(unsafe.Pointer(textFormat))
	// SetTextAlignment is at index 3 in IDWriteTextFormat vtable
	setTextAlignment := *(*uintptr)(unsafe.Pointer(vtbl + 3*unsafe.Sizeof(uintptr(0))))
	syscall.SyscallN(setTextAlignment, textFormat, 2) // DWRITE_TEXT_ALIGNMENT_CENTER

	// SetParagraphAlignment is at index 4 in IDWriteTextFormat vtable
	setParagraphAlignment := *(*uintptr)(unsafe.Pointer(vtbl + 4*unsafe.Sizeof(uintptr(0))))
	syscall.SyscallN(setParagraphAlignment, textFormat, 1) // DWRITE_PARAGRAPH_ALIGNMENT_CENTER
}

func dwCreateTextLayout(factory uintptr, text []uint16, textFormat uintptr, maxWidth, maxHeight float32) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(factory))
	// CreateTextLayout is at index 18 in IDWriteFactory vtable
	createTextLayout := *(*uintptr)(unsafe.Pointer(vtbl + 18*unsafe.Sizeof(uintptr(0))))

	var textLayout uintptr
	hr, _, _ := syscall.SyscallN(createTextLayout, factory,
		uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1), textFormat,
		uintptr(*(*uint32)(unsafe.Pointer(&maxWidth))),
		uintptr(*(*uint32)(unsafe.Pointer(&maxHeight))),
		uintptr(unsafe.Pointer(&textLayout)))
	if hr != 0 {
		return 0, errors.New("DWrite CreateTextLayout failed")
	}
	return textLayout, nil
}

func d2dCreateSolidColorBrush(rt uintptr) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(rt))
	// CreateSolidColorBrush is at index 8 in ID2D1RenderTarget vtable
	createBrush := *(*uintptr)(unsafe.Pointer(vtbl + 8*unsafe.Sizeof(uintptr(0))))

	black := [4]float32{0, 0, 0, 1} // RGBA
	var brush uintptr
	hr, _, _ := syscall.SyscallN(createBrush, rt, uintptr(unsafe.Pointer(&black)), 0, uintptr(unsafe.Pointer(&brush)))
	if hr != 0 {
		return 0, errors.New("D2D CreateSolidColorBrush failed")
	}
	return brush, nil
}

func d2dBeginDraw(rt uintptr) {
	vtbl := *(*uintptr)(unsafe.Pointer(rt))
	// BeginDraw is at index 48 in ID2D1RenderTarget vtable
	beginDraw := *(*uintptr)(unsafe.Pointer(vtbl + 48*unsafe.Sizeof(uintptr(0))))
	syscall.SyscallN(beginDraw, rt)
}

func d2dClear(rt uintptr) {
	vtbl := *(*uintptr)(unsafe.Pointer(rt))
	// Clear is at index 47 in ID2D1RenderTarget vtable
	clear := *(*uintptr)(unsafe.Pointer(vtbl + 47*unsafe.Sizeof(uintptr(0))))
	// Transparent background (RGBA = 0, 0, 0, 0)
	transparent := [4]float32{0, 0, 0, 0}
	syscall.SyscallN(clear, rt, uintptr(unsafe.Pointer(&transparent)))
}

func d2dDrawTextLayout(rt uintptr, textLayout uintptr, brush uintptr) {
	vtbl := *(*uintptr)(unsafe.Pointer(rt))
	// DrawTextLayout is at index 28 in ID2D1RenderTarget vtable
	drawTextLayout := *(*uintptr)(unsafe.Pointer(vtbl + 28*unsafe.Sizeof(uintptr(0))))
	// D2D1_POINT_2F origin = {0.0f, 0.0f} - pack two float32 into one uint64
	var originX float32 = 0
	var originY float32 = 0
	origin := uint64(*(*uint32)(unsafe.Pointer(&originX))) | (uint64(*(*uint32)(unsafe.Pointer(&originY))) << 32)
	// D2D1_DRAW_TEXT_OPTIONS_ENABLE_COLOR_FONT = 4
	syscall.SyscallN(drawTextLayout, rt, uintptr(origin), textLayout, brush, 4)
}

func d2dEndDraw(rt uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(rt))
	// EndDraw is at index 49 in ID2D1RenderTarget vtable
	endDraw := *(*uintptr)(unsafe.Pointer(vtbl + 49*unsafe.Sizeof(uintptr(0))))
	hr, _, _ := syscall.SyscallN(endDraw, rt, 0, 0)
	return hr
}

func wicCopyPixels(bitmap uintptr, width, height uint32) ([]byte, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(bitmap))
	// CopyPixels is at index 7 in IWICBitmapSource vtable
	copyPixels := *(*uintptr)(unsafe.Pointer(vtbl + 7*unsafe.Sizeof(uintptr(0))))

	stride := width * 4
	bufferSize := stride * height
	pixels := make([]byte, bufferSize)

	hr, _, _ := syscall.SyscallN(copyPixels, bitmap, 0, uintptr(stride), uintptr(bufferSize), uintptr(unsafe.Pointer(&pixels[0])))
	if hr != 0 {
		return nil, errors.New("WIC CopyPixels failed")
	}
	return pixels, nil
}

func utf16FromString(s string) []uint16 {
	runes := []rune(s)
	result := make([]uint16, 0, len(runes)*2+1)
	for _, r := range runes {
		if r <= 0xFFFF {
			result = append(result, uint16(r))
		} else {
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)))
			result = append(result, uint16(0xDC00+(r&0x3FF)))
		}
	}
	result = append(result, 0)
	return result
}
