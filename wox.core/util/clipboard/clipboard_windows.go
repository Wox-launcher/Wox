package clipboard

/*
#cgo LDFLAGS: -luser32 -lkernel32 -lshell32 -lole32
#include <stdlib.h>
#include "clipboard_windows.h"
*/
import "C"
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"wox/util"

	"golang.org/x/image/bmp"
)

const (
	clipboardDiagLogThrottleMs = 2000
)

var lastSeqNum uint32
var lastClipboardDiagLogTs int64

// readClipboardContentType detects the current clipboard content type without reading data.
func readClipboardContentType() Type {
	t := C.clipboardGetContentType()
	switch int(t) {
	case 1:
		return ClipboardTypeText
	case 2:
		return ClipboardTypeImage
	case 3:
		return ClipboardTypeFile
	default:
		return ""
	}
}

func readText() (string, error) {
	var cText *C.wchar_t
	var cLen C.int
	ret := C.clipboardReadText(&cText, &cLen)
	if ret != 0 {
		if ret == -1 {
			return "", noDataErr
		}
		return "", fmt.Errorf("clipboard: readText failed (code=%d)", int(ret))
	}
	defer C.free(unsafe.Pointer(cText))

	if cLen == 0 {
		return "", nil
	}

	// Convert wchar_t (UTF-16) to Go string
	length := int(cLen)
	u16 := make([]uint16, length)
	for i := 0; i < length; i++ {
		u16[i] = *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(cText)) + uintptr(i)*2))
	}

	return string(utf16Decode(u16)), nil
}

func readFilePaths() ([]string, error) {
	var cPaths *C.wchar_t
	var cLen C.int
	ret := C.clipboardReadFilePaths(&cPaths, &cLen)
	if ret != 0 {
		if ret == -1 {
			return nil, noDataErr
		}
		return nil, fmt.Errorf("clipboard: readFilePaths failed (code=%d)", int(ret))
	}
	defer C.free(unsafe.Pointer(cPaths))

	totalChars := int(cLen)
	if totalChars <= 0 {
		return nil, noDataErr
	}

	// Read all wchar_t into a slice
	u16 := make([]uint16, totalChars)
	for i := 0; i < totalChars; i++ {
		u16[i] = *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(cPaths)) + uintptr(i)*2))
	}

	// Parse null-separated, double-null-terminated strings
	var paths []string
	start := 0
	for i := 0; i < len(u16); i++ {
		if u16[i] == 0 {
			if i > start {
				paths = append(paths, string(utf16Decode(u16[start:i])))
			}
			start = i + 1
			// Double null = end of list
			if start < len(u16) && u16[start] == 0 {
				break
			}
		}
	}

	if len(paths) == 0 {
		return nil, noDataErr
	}

	return paths, nil
}

func readImage() (image.Image, error) {
	var cData *C.uchar
	var cLen C.int
	var cIsPNG C.int
	var cInfo C.BitmapInfo
	ret := C.clipboardReadImage(&cData, &cLen, &cIsPNG, &cInfo)
	if ret != 0 {
		if ret == -1 {
			return nil, noDataErr
		}
		return nil, fmt.Errorf("clipboard: readImage failed (code=%d)", int(ret))
	}
	defer C.free(unsafe.Pointer(cData))

	dataLen := int(cLen)
	data := C.GoBytes(unsafe.Pointer(cData), cLen)

	// PNG format: decode directly
	if int(cIsPNG) != 0 {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("clipboard: failed to decode PNG data (%d bytes): %w", dataLen, err)
		}
		logClipboardDiagnostic(fmt.Sprintf("clipboard: decoded PNG image %dx%d (%d bytes)",
			img.Bounds().Dx(), img.Bounds().Dy(), dataLen))
		return img, nil
	}

	// DIB format: decode based on header info
	headerSize := int(cInfo.headerSize)
	width := int(cInfo.width)
	height := int(cInfo.height)
	bitCount := int(cInfo.bitCount)
	compression := int(cInfo.compression)
	clrUsed := int(cInfo.clrUsed)
	sizeImage := int(cInfo.sizeImage)

	// 32-bit images: manual decoder (common from Chrome/Edge, Go's bmp.Decode often fails)
	if bitCount == 32 {
		return decode32bppDIB(data, headerSize, width, height, compression)
	}

	// Non-32bpp: construct BMP file header and use standard decoder
	return decodeNon32bppDIB(data, headerSize, width, height, bitCount, compression, sizeImage, clrUsed)
}

// decode32bppDIB manually decodes a 32-bit per pixel DIB from raw data.
func decode32bppDIB(data []byte, headerSize, width, height, compression int) (image.Image, error) {
	isTopDown := height < 0
	if isTopDown {
		height = -height
	}

	if width <= 0 || height <= 0 || width > 32768 || height > 32768 {
		return nil, fmt.Errorf("clipboard: invalid 32bpp image dimensions: %dx%d", width, height)
	}

	// Calculate pixel data offset
	offset := headerSize
	// Handle BI_BITFIELDS (Compression=3) with BITMAPINFOHEADER (Size=40):
	// 3 DWORD color masks follow the header
	if compression == 3 && headerSize == 40 {
		offset += 12
	}

	if offset > len(data) {
		return nil, fmt.Errorf("clipboard: invalid 32bpp DIB pixel offset: offset=%d dataLen=%d", offset, len(data))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	stride := width * 4
	pixelData := data[offset:]
	truncatedRows := 0

	for y := 0; y < height; y++ {
		destY := y
		if !isTopDown {
			destY = height - 1 - y
		}

		srcRow := y * stride
		destRow := destY * img.Stride

		if srcRow+width*4 > len(pixelData) {
			truncatedRows++
			continue
		}

		for x := 0; x < width; x++ {
			// Input is BGRA/BGRX
			b := pixelData[srcRow+x*4+0]
			g := pixelData[srcRow+x*4+1]
			r := pixelData[srcRow+x*4+2]
			a := pixelData[srcRow+x*4+3]

			// Output is NRGBA
			img.Pix[destRow+x*4+0] = r
			img.Pix[destRow+x*4+1] = g
			img.Pix[destRow+x*4+2] = b
			img.Pix[destRow+x*4+3] = a
		}
	}

	if truncatedRows > 0 {
		logClipboardDiagnostic(fmt.Sprintf("clipboard: decoded 32bpp image with truncated rows=%d width=%d height=%d pixelDataLen=%d",
			truncatedRows, width, height, len(pixelData)))
	}
	logClipboardDiagnostic(fmt.Sprintf("clipboard: decoded 32bpp image width=%d height=%d dataLen=%d",
		width, height, len(data)))
	return img, nil
}

// decodeNon32bppDIB constructs a BMP file header and uses the standard bmp.Decode.
func decodeNon32bppDIB(data []byte, headerSize, width, height, bitCount, compression, sizeImage, clrUsed int) (image.Image, error) {
	const fileHeaderLen = 14

	// Calculate image pixel data size if not specified
	if sizeImage == 0 && compression == 0 {
		absHeight := height
		if absHeight < 0 {
			absHeight = -absHeight
		}
		stride := (width*bitCount + 31) / 32 * 4
		sizeImage = stride * absHeight
	}

	// Calculate offset to pixel data
	offset := uint32(fileHeaderLen) + uint32(headerSize)
	if compression == 3 && headerSize == 40 {
		offset += 12
	}
	if bitCount <= 8 {
		colors := uint32(clrUsed)
		if colors == 0 {
			colors = 1 << uint(bitCount)
		}
		offset += colors * 4
	}

	fileSize := offset + uint32(sizeImage)

	// Construct BMP file
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint16('B')|(uint16('M')<<8)) // bfType
	binary.Write(buf, binary.LittleEndian, uint32(fileSize))             // bfSize
	binary.Write(buf, binary.LittleEndian, uint32(0))                    // bfReserved1, bfReserved2
	binary.Write(buf, binary.LittleEndian, uint32(offset))               // bfOffBits

	// Write DIB data from our copy
	dibSize := fileSize - fileHeaderLen
	if int(dibSize) > len(data) {
		dibSize = uint32(len(data))
	}
	buf.Write(data[:dibSize])

	decoded, decodeErr := bmp.Decode(buf)
	if decodeErr != nil {
		return nil, fmt.Errorf("clipboard: failed to decode BMP (headerSize=%d bitCount=%d compression=%d): %w",
			headerSize, bitCount, compression, decodeErr)
	}
	logClipboardDiagnostic(fmt.Sprintf("clipboard: decoded non-32bpp image width=%d height=%d bitCount=%d dataLen=%d",
		decoded.Bounds().Dx(), decoded.Bounds().Dy(), bitCount, len(data)))
	return decoded, nil
}

func writeTextData(text string) error {
	start := time.Now()

	if len(text) == 0 {
		cText := (*C.wchar_t)(unsafe.Pointer(&[]uint16{0}[0]))
		ret := C.clipboardWriteText(cText, 0)
		if ret != 0 {
			return fmt.Errorf("clipboard: writeText(empty) failed (code=%d)", int(ret))
		}
		return nil
	}

	u16 := utf16Encode(text)
	cText := (*C.wchar_t)(unsafe.Pointer(&u16[0]))
	ret := C.clipboardWriteText(cText, C.int(len(u16)))

	if d := time.Since(start); d > 200*time.Millisecond {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("clipboard: writeTextData took %s (chars=%d)", d.String(), len(u16)))
	}

	if ret != 0 {
		return fmt.Errorf("clipboard: writeText failed (code=%d)", int(ret))
	}

	// Update lastSeqNum to avoid triggering watchChange on our own writes
	lastSeqNum = uint32(C.clipboardGetSequenceNumber())
	return nil
}

func writeFilePaths(filePaths []string) error {
	trimmedPaths := make([]string, 0, len(filePaths))
	for _, filePath := range filePaths {
		trimmedPath := strings.TrimSpace(filePath)
		if trimmedPath == "" {
			continue
		}
		trimmedPaths = append(trimmedPaths, trimmedPath)
	}

	if len(trimmedPaths) == 0 {
		return fmt.Errorf("clipboard: file paths are empty")
	}

	buffer := make([]uint16, 0)
	for _, filePath := range trimmedPaths {
		encodedPath := utf16Encode(filePath)
		buffer = append(buffer, encodedPath...)
	}
	buffer = append(buffer, 0)

	ret := C.clipboardWriteFilePaths((*C.wchar_t)(unsafe.Pointer(&buffer[0])), C.int(len(buffer)))
	if ret != 0 {
		return fmt.Errorf("clipboard: writeFilePaths failed (code=%d)", int(ret))
	}

	lastSeqNum = uint32(C.clipboardGetSequenceNumber())
	return nil
}

func writeImageData(img image.Image) error {
	const fileHeaderLen = 14

	// Encode outside clipboard lock
	var pngData []byte
	pngBuf := new(bytes.Buffer)
	if err := png.Encode(pngBuf, img); err == nil {
		pngData = pngBuf.Bytes()
	}

	bmpBuf := new(bytes.Buffer)
	if err := bmp.Encode(bmpBuf, img); err != nil {
		return fmt.Errorf("clipboard: failed to encode image to BMP: %w", err)
	}
	bmpData := bmpBuf.Bytes()
	if len(bmpData) <= fileHeaderLen {
		return fmt.Errorf("clipboard: invalid BMP data: too short")
	}
	dibData := bmpData[fileHeaderLen:]

	return writeImageBytes(pngData, dibData)
}

func writeImageBytes(pngData []byte, dibData []byte) error {
	if len(dibData) == 0 {
		return fmt.Errorf("clipboard: DIB data is empty")
	}

	start := time.Now()

	var cPNG *C.uchar
	cPNGLen := C.int(0)
	if len(pngData) > 0 {
		cPNG = (*C.uchar)(unsafe.Pointer(&pngData[0]))
		cPNGLen = C.int(len(pngData))
	}
	cDIB := (*C.uchar)(unsafe.Pointer(&dibData[0]))
	cDIBLen := C.int(len(dibData))

	ret := C.clipboardWriteImage(cPNG, cPNGLen, cDIB, cDIBLen)

	if d := time.Since(start); d > 200*time.Millisecond {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("clipboard: writeImageBytes took %s (pngBytes=%d dibBytes=%d)", d.String(), len(pngData), len(dibData)))
	}

	if ret != 0 {
		return fmt.Errorf("clipboard: writeImage failed (code=%d)", int(ret))
	}

	// Update lastSeqNum to avoid triggering watchChange on our own writes
	lastSeqNum = uint32(C.clipboardGetSequenceNumber())
	return nil
}

func isClipboardChanged() bool {
	seqNum := uint32(C.clipboardGetSequenceNumber())
	if seqNum == 0 {
		return false
	}
	if seqNum != lastSeqNum {
		lastSeqNum = seqNum
		return true
	}
	return false
}

func buildWatchSnapshot() string {
	buf := make([]byte, 1024)
	n := C.clipboardGetDiagnosticInfo((*C.char)(unsafe.Pointer(&buf[0])), C.int(len(buf)))
	if n <= 0 {
		return ""
	}
	return string(buf[:int(n)])
}

func logClipboardDiagnostic(message string) {
	if !shouldEmitClipboardDiagLog() {
		return
	}
	util.GetLogger().Warn(util.NewTraceContext(), message)
}

func shouldEmitClipboardDiagLog() bool {
	now := time.Now().UnixMilli()
	last := atomic.LoadInt64(&lastClipboardDiagLogTs)
	if now-last < clipboardDiagLogThrottleMs {
		return false
	}
	return atomic.CompareAndSwapInt64(&lastClipboardDiagLogTs, last, now)
}

// utf16Decode converts a UTF-16 slice to a Go string.
func utf16Decode(s []uint16) string {
	runes := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r := rune(s[i])
		if r >= 0xD800 && r <= 0xDBFF && i+1 < len(s) {
			r2 := rune(s[i+1])
			if r2 >= 0xDC00 && r2 <= 0xDFFF {
				r = (r-0xD800)*0x400 + (r2 - 0xDC00) + 0x10000
				i += 2
				runes = append(runes, r)
				continue
			}
		}
		runes = append(runes, r)
		i++
	}
	return string(runes)
}

// utf16Encode converts a Go string to a null-terminated UTF-16 slice.
func utf16Encode(s string) []uint16 {
	result := make([]uint16, 0, len(s)+1)
	for _, r := range s {
		if r >= 0x10000 {
			r -= 0x10000
			result = append(result, uint16(r/0x400+0xD800))
			result = append(result, uint16(r%0x400+0xDC00))
		} else {
			result = append(result, uint16(r))
		}
	}
	result = append(result, 0)
	return result
}
