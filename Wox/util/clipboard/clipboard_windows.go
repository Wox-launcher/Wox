package clipboard

import "C"
import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"golang.org/x/image/bmp"
	"image"
	"image/png"
	"syscall"
	"unicode/utf16"
	"unsafe"
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

	kernel32 = syscall.NewLazyDLL("kernel32")
	gLock    = kernel32.NewProc("GlobalLock")
	gUnlock  = kernel32.NewProc("GlobalUnlock")
	gAlloc   = kernel32.NewProc("GlobalAlloc")
	gFree    = kernel32.NewProc("GlobalFree")
	memMove  = kernel32.NewProc("RtlMoveMemory")

	shell32       = syscall.NewLazyDLL("shell32.dll")
	dragQueryFile = shell32.NewProc("DragQueryFileW")
)

// BITMAPV5Header structure, see:
// https://docs.microsoft.com/en-us/windows/win32/api/wingdi/ns-wingdi-bitmapv5header
type bitmapV5Header struct {
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
	RedMask       uint32
	GreenMask     uint32
	BlueMask      uint32
	AlphaMask     uint32
	CSType        uint32
	Endpoints     struct {
		CiexyzRed, CiexyzGreen, CiexyzBlue struct {
			CiexyzX, CiexyzY, CiexyzZ int32 // FXPT2DOT30
		}
	}
	GammaRed    uint32
	GammaGreen  uint32
	GammaBlue   uint32
	Intent      uint32
	ProfileData uint32
	ProfileSize uint32
	Reserved    uint32
}

type bitmapHeader struct {
	Size          uint32
	Width         uint32
	Height        uint32
	PLanes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter uint32
	YPelsPerMeter uint32
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
		infoHeaderLen = 40
		cFmtDIB       = 8
	)

	hClipDat, _, err := getClipboardData.Call(cFmtDIB)
	if err != nil {
		return nil, errors.New("not dib format data: " + err.Error())
	}
	pMemBlk, _, err := gLock.Call(hClipDat)
	if pMemBlk == 0 {
		return nil, errors.New("failed to call global lock: " + err.Error())
	}
	defer gUnlock.Call(hClipDat)

	bmpHeader := (*bitmapHeader)(unsafe.Pointer(pMemBlk))
	dataSize := bmpHeader.SizeImage + fileHeaderLen + infoHeaderLen

	if bmpHeader.SizeImage == 0 && bmpHeader.Compression == 0 {
		iSizeImage := bmpHeader.Height * ((bmpHeader.Width*uint32(bmpHeader.BitCount)/8 + 3) &^ 3)
		dataSize += iSizeImage
	}
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint16('B')|(uint16('M')<<8))
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))
	binary.Write(buf, binary.LittleEndian, uint32(0))
	const sizeofColorbar = 0
	binary.Write(buf, binary.LittleEndian, uint32(fileHeaderLen+infoHeaderLen+sizeofColorbar))
	j := 0
	for i := fileHeaderLen; i < int(dataSize); i++ {
		binary.Write(buf, binary.BigEndian, *(*byte)(unsafe.Pointer(pMemBlk + uintptr(j))))
		j++
	}
	return bmpToPng(buf)
}

func bmpToPng(bmpBuf *bytes.Buffer) (image.Image, error) {
	var f bytes.Buffer
	originalImage, err := bmp.Decode(bmpBuf)
	if err != nil {
		return nil, err
	}
	err = png.Encode(&f, originalImage)
	if err != nil {
		return nil, err
	}
	newImage, err := png.Decode(&f)
	if err != nil {
		return nil, err
	}
	return newImage, nil
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
