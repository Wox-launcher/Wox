package clipboard

import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/image/bmp"
)

var (
	user32                     = syscall.MustLoadDLL("user32")
	openClipboard              = user32.MustFindProc("OpenClipboard")
	closeClipboard             = user32.MustFindProc("CloseClipboard")
	emptyClipboard             = user32.MustFindProc("EmptyClipboard")
	getClipboardData           = user32.MustFindProc("GetClipboardData")
	setClipboardData           = user32.MustFindProc("SetClipboardData")
	isClipboardFormatAvailable = user32.MustFindProc("IsClipboardFormatAvailable")
	getClipboardSequenceNumber = user32.MustFindProc("GetClipboardSequenceNumber")
	registerClipboardFormat    = user32.MustFindProc("RegisterClipboardFormatW")

	kernel32 = syscall.NewLazyDLL("kernel32")
	gLock    = kernel32.NewProc("GlobalLock")
	gUnlock  = kernel32.NewProc("GlobalUnlock")
	gAlloc   = kernel32.NewProc("GlobalAlloc")
	gFree    = kernel32.NewProc("GlobalFree")
	memMove  = kernel32.NewProc("RtlMoveMemory")

	shell32       = syscall.NewLazyDLL("shell32.dll")
	dragQueryFile = shell32.NewProc("DragQueryFileW")
)

type bitmapHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	PLanes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

const (
	cFmtUnicodeText = 13
	gmemMoveable    = 0x0002
	cFmtHdrop       = 15
)

var lastSeqNum uint32

func readText() (string, error) {
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return "", fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	hMem, _, err := getClipboardData.Call(cFmtUnicodeText)
	if hMem == 0 {
		return "", fmt.Errorf("failed to get clipboard data: %w", err)
	}

	p, _, err := gLock.Call(hMem)
	if p == 0 {
		return "", fmt.Errorf("failed to lock global memory: %w", err)
	}
	defer gUnlock.Call(hMem)

	var buf []uint16
	for i := 0; ; i++ {
		ch := *(*uint16)(unsafe.Pointer(p + uintptr(i*2)))
		if ch == 0 {
			buf = make([]uint16, i)
			copy(buf, (*[1 << 20]uint16)(unsafe.Pointer(p))[:i:i])
			break
		}
	}

	return string(utf16.Decode(buf)), nil
}

func writeTextData(text string) error {
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	r, _, err = emptyClipboard.Call()
	if r == 0 {
		return fmt.Errorf("failed to clear clipboard: %w", err)
	}

	if len(text) == 0 {
		return nil
	}

	s, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("failed to convert string to UTF16: %w", err)
	}

	hMem, _, err := gAlloc.Call(gmemMoveable, uintptr(len(s)*int(unsafe.Sizeof(s[0]))))
	if hMem == 0 {
		return fmt.Errorf("failed to allocate global memory: %w", err)
	}

	p, _, err := gLock.Call(hMem)
	if p == 0 {
		gFree.Call(hMem)
		return fmt.Errorf("failed to lock global memory: %w", err)
	}
	defer gUnlock.Call(hMem)

	memMove.Call(p, uintptr(unsafe.Pointer(&s[0])), uintptr(len(s)*int(unsafe.Sizeof(s[0]))))

	v, _, err := setClipboardData.Call(cFmtUnicodeText, hMem)
	if v == 0 {
		gFree.Call(hMem)
		return fmt.Errorf("failed to set clipboard data: %w", err)
	}

	return nil
}

func writeImageData(img image.Image) error {
	const (
		cFmtDIB       = 8
		fileHeaderLen = 14 // BMP file header length to skip
	)

	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	r, _, err = emptyClipboard.Call()
	if r == 0 {
		return fmt.Errorf("failed to clear clipboard: %w", err)
	}

	// Write PNG format for transparency support
	pngBuf := new(bytes.Buffer)
	if err := png.Encode(pngBuf, img); err == nil {
		pngFormatName, _ := syscall.UTF16PtrFromString("PNG")
		pngFormat, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(pngFormatName)))
		if pngFormat != 0 {
			pngData := pngBuf.Bytes()
			hMemPng, _, _ := gAlloc.Call(gmemMoveable, uintptr(len(pngData)))
			if hMemPng != 0 {
				pMemPng, _, _ := gLock.Call(hMemPng)
				if pMemPng != 0 {
					memMove.Call(pMemPng, uintptr(unsafe.Pointer(&pngData[0])), uintptr(len(pngData)))
					gUnlock.Call(hMemPng)
					setClipboardData.Call(pngFormat, hMemPng)
				}
			}
		}
	}

	// Also write DIB format for compatibility with apps that don't support PNG
	buf := new(bytes.Buffer)
	err = bmp.Encode(buf, img)
	if err != nil {
		return fmt.Errorf("failed to encode image: %w", err)
	}

	// CF_DIB format expects DIB data without the BMP file header (14 bytes)
	bmpData := buf.Bytes()
	if len(bmpData) <= fileHeaderLen {
		return fmt.Errorf("invalid BMP data: too short")
	}
	dibData := bmpData[fileHeaderLen:]

	hMem, _, err := gAlloc.Call(gmemMoveable, uintptr(len(dibData)))
	if hMem == 0 {
		return fmt.Errorf("failed to allocate global memory: %w", err)
	}

	pMem, _, err := gLock.Call(hMem)
	if pMem == 0 {
		gFree.Call(hMem)
		return fmt.Errorf("failed to lock global memory: %w", err)
	}

	memMove.Call(pMem, uintptr(unsafe.Pointer(&dibData[0])), uintptr(len(dibData)))
	gUnlock.Call(hMem)

	ret, _, err := setClipboardData.Call(cFmtDIB, hMem)
	if ret == 0 {
		gFree.Call(hMem)
		return fmt.Errorf("failed to set clipboard data: %w", err)
	}

	return nil
}

func readFilePaths() ([]string, error) {
	var fileNames []string

	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	hDrop, _, err := getClipboardData.Call(cFmtHdrop)
	if hDrop == 0 {
		return nil, fmt.Errorf("failed to get clipboard data: %w", err)
	}

	hGlobal, _, err := gLock.Call(hDrop)
	if hGlobal == 0 {
		return nil, fmt.Errorf("failed to lock global memory: %w", err)
	}
	defer gUnlock.Call(hDrop)

	count, _, _ := dragQueryFile.Call(hGlobal, 0xFFFFFFFF, 0, 0)
	for i := uint(0); i < uint(count); i++ {
		len, _, _ := dragQueryFile.Call(hGlobal, uintptr(i), 0, 0)
		buffer := make([]uint16, len+1)
		dragQueryFile.Call(hGlobal, uintptr(i), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len+1))
		fileNames = append(fileNames, syscall.UTF16ToString(buffer))
	}

	return fileNames, nil
}

func readImage() (image.Image, error) {
	return readBmpImage()
}

func readBmpImage() (image.Image, error) {
	const (
		fileHeaderLen = 14
		cFmtDIB       = 8
	)

	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	hClipDat, _, err := getClipboardData.Call(cFmtDIB)
	if err != nil && hClipDat == 0 {
		return nil, errors.New("not dib format data: " + err.Error())
	}
	if hClipDat == 0 {
		return nil, errors.New("getClipboardData returned 0")
	}

	pMemBlk, _, err := gLock.Call(hClipDat)
	if pMemBlk == 0 {
		return nil, errors.New("failed to call global lock: " + err.Error())
	}
	defer gUnlock.Call(hClipDat)

	// DIB data starts with the header
	bmpHeader := (*bitmapHeader)(unsafe.Pointer(pMemBlk))

	// Manual Decoder for 32-bit Images (Common in modern Windows apps like Chrome/Edge)
	// The standard Go bmp.Decode often fails with "unsupported BMP image" for these.
	if bmpHeader.BitCount == 32 {
		width := int(bmpHeader.Width)
		height := int(bmpHeader.Height)
		isTopDown := height < 0
		if isTopDown {
			height = -height
		}

		// Validation: prevent unreasonable dimensions
		if width <= 0 || height <= 0 || width > 32768 || height > 32768 {
			return nil, fmt.Errorf("invalid image dimensions: %dx%d", width, height)
		}

		// Calculate offset to pixel data
		headerSize := bmpHeader.Size
		offset := headerSize

		// Handle BI_BITFIELDS (Compression=3) with BITMAPINFOHEADER (Size=40)
		// In this case, 3 DWORD color masks follow the header.
		if bmpHeader.Compression == 3 && headerSize == 40 {
			offset += 12
		}

		// Pointer to pixel data
		pixelsAddr := uintptr(unsafe.Pointer(pMemBlk)) + uintptr(offset)

		img := image.NewNRGBA(image.Rect(0, 0, width, height))

		// 32bpp stride is always width * 4
		stride := width * 4

		// Construct slice for pixel data (careful with bounds)
		dataSize := stride * height
		pixelData := (*[1 << 30]byte)(unsafe.Pointer(pixelsAddr))[:dataSize:dataSize]

		for y := 0; y < height; y++ {
			// DIBs are usually bottom-up
			destY := y
			if !isTopDown {
				destY = height - 1 - y
			}

			srcRow := y * stride
			destRow := destY * img.Stride

			for x := 0; x < width; x++ {
				// Input is BGRA or BGRX
				b := pixelData[srcRow+x*4+0]
				g := pixelData[srcRow+x*4+1]
				r := pixelData[srcRow+x*4+2]
				a := pixelData[srcRow+x*4+3]

				// Set to NRGBA
				img.Pix[destRow+x*4+0] = r
				img.Pix[destRow+x*4+1] = g
				img.Pix[destRow+x*4+2] = b
				img.Pix[destRow+x*4+3] = a
			}
		}
		return img, nil
	}

	// Fallback for non-32bpp images (e.g. 24bpp) where standard decoder might still work,
	// or properly constructing the file header for them.
	// Re-implementing the BMP file construction for fallback.

	// Get the total size of the DIB data (including header, palette, and pixel data)
	imageSize := bmpHeader.SizeImage
	if imageSize == 0 && bmpHeader.Compression == 0 { // BI_RGB
		stride := (int(bmpHeader.Width)*int(bmpHeader.BitCount) + 31) / 32 * 4
		imageSize = uint32(stride * int(map[bool]int32{true: bmpHeader.Height, false: -bmpHeader.Height}[bmpHeader.Height > 0]))
	}

	// Offset Calculation Logic for File Header
	offset := uint32(fileHeaderLen) + bmpHeader.Size

	// + Palette/Masks:
	if bmpHeader.Compression == 3 && bmpHeader.Size == 40 {
		offset += 12
	}

	// If BitCount <= 8, a color table follows.
	if bmpHeader.BitCount <= 8 {
		colors := bmpHeader.ClrUsed
		if colors == 0 {
			colors = 1 << bmpHeader.BitCount
		}
		offset += colors * 4
	}

	// Total file size
	fileSize := offset + imageSize

	// Construct BMP file in memory
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint16('B')|(uint16('M')<<8)) // bfType
	binary.Write(buf, binary.LittleEndian, uint32(fileSize))             // bfSize
	binary.Write(buf, binary.LittleEndian, uint32(0))                    // bfReserved1, bfReserved2
	binary.Write(buf, binary.LittleEndian, uint32(offset))               // bfOffBits

	// Write the rest of the DIB data
	dibSize := fileSize - fileHeaderLen
	dibData := (*[1 << 30]byte)(unsafe.Pointer(pMemBlk))[:dibSize:dibSize]
	buf.Write(dibData)

	return bmp.Decode(buf)
}

func isClipboardChanged() bool {
	r, _, _ := getClipboardSequenceNumber.Call()
	if r == 0 {
		return false
	}

	seqNum := uint32(r)
	if seqNum != lastSeqNum {
		lastSeqNum = seqNum
		return true
	}

	return false
}
