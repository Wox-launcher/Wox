//go:build windows

package emoji

import (
	"errors"
	"image"
	"image/color"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	createCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	deleteDC             = gdi32.NewProc("DeleteDC")
	selectObject         = gdi32.NewProc("SelectObject")
	createFontIndirectW  = gdi32.NewProc("CreateFontIndirectW")
	deleteObject         = gdi32.NewProc("DeleteObject")
	setBkMode            = gdi32.NewProc("SetBkMode")
	drawTextW            = user32.NewProc("DrawTextW")
	setTextColor         = gdi32.NewProc("SetTextColor")
	createDIBSectionProc = gdi32.NewProc("CreateDIBSection")
)

const (
	bkModeTransparent = 1
	dtCalcRect        = 0x00000400
	dtNoClip          = 0x00000100
	dtCenter          = 0x00000001
	dtVCenter         = 0x00000004
	dtSingleLine      = 0x00000020
)

type rect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type logFontW struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
}

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type bitmapInfo struct {
	Header bitmapInfoHeader
	Colors [1]uint32
}

func getNativeEmojiImage(emoji string, size int) (image.Image, error) {
	if size <= 0 {
		size = 256
	}

	// measure text size
	dc, _, err := createCompatibleDC.Call(0)
	if dc == 0 {
		return nil, errors.New("CreateCompatibleDC failed: " + err.Error())
	}
	defer deleteDC.Call(dc)

	font := createEmojiFont(size)
	if font == 0 {
		return nil, errors.New("CreateFontIndirectW failed")
	}
	defer deleteObject.Call(font)
	selectObject.Call(dc, font)
	setBkMode.Call(dc, bkModeTransparent)
	setTextColor.Call(dc, 0x00FFFFFF) // white; color fonts ignore this

	textPtr := utf16PtrFromString(emoji)
	textLen := int32(-1) // null-terminated
	var textRect rect
	drawTextW.Call(dc, uintptr(unsafe.Pointer(textPtr)), uintptr(textLen), uintptr(unsafe.Pointer(&textRect)), dtCalcRect|dtSingleLine)

	width := textRect.Right - textRect.Left
	height := textRect.Bottom - textRect.Top
	if width < int32(size) {
		width = int32(size)
	}
	if height < int32(size) {
		height = int32(size)
	}

	var bits unsafe.Pointer
	bi := bitmapInfo{
		Header: bitmapInfoHeader{
			Size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
			Width:       width,
			Height:      -height, // top-down
			Planes:      1,
			BitCount:    32,
			Compression: 0,
		},
	}
	hBitmap, _, err := createDIBSectionProc.Call(dc, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hBitmap == 0 {
		return nil, errors.New("CreateDIBSection failed: " + err.Error())
	}
	defer deleteObject.Call(hBitmap)

	selectObject.Call(dc, hBitmap)

	// center text
	var drawRect rect
	drawRect.Right = width
	drawRect.Bottom = height
	drawTextW.Call(dc, uintptr(unsafe.Pointer(textPtr)), uintptr(textLen), uintptr(unsafe.Pointer(&drawRect)), dtNoClip|dtCenter|dtVCenter|dtSingleLine)

	// copy pixels into Go image (bitmap is BGRA)
	stride := int(width) * 4
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			offset := y*stride + x*4
			ptr := uintptr(bits) + uintptr(offset)
			b := *(*byte)(unsafe.Pointer(ptr + 0))
			g := *(*byte)(unsafe.Pointer(ptr + 1))
			r := *(*byte)(unsafe.Pointer(ptr + 2))
			a := *(*byte)(unsafe.Pointer(ptr + 3))
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img, nil
}

func createEmojiFont(size int) uintptr {
	lf := logFontW{
		Height:  int32(size),
		Weight:  400,
		CharSet: 1, // DEFAULT_CHARSET
	}
	copy(lf.FaceName[:], utf16.Encode([]rune("Segoe UI Emoji")))
	h, _, _ := createFontIndirectW.Call(uintptr(unsafe.Pointer(&lf)))
	return h
}

func utf16PtrFromString(s string) *uint16 {
	encoded := utf16.Encode([]rune(s + "\x00"))
	return &encoded[0]
}
