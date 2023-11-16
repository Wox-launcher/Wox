package clipboard

import "C"
import (
	"fmt"
	"image"
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
	enumClipboardFormats       = user32.MustFindProc("EnumClipboardFormats")
	getClipboardSequenceNumber = user32.MustFindProc("GetClipboardSequenceNumber")
	registerClipboardFormatA   = user32.MustFindProc("RegisterClipboardFormatA")

	kernel32 = syscall.NewLazyDLL("kernel32")
	gLock    = kernel32.NewProc("GlobalLock")
	gUnlock  = kernel32.NewProc("GlobalUnlock")
	gAlloc   = kernel32.NewProc("GlobalAlloc")
	gFree    = kernel32.NewProc("GlobalFree")
	memMove  = kernel32.NewProc("RtlMoveMemory")
)

const (
	cFmtUnicodeText = 13
	gmemMoveable    = 0x0002
	cFmtHdrop       = 15
)

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
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return nil, fmt.Errorf("failed to open clipboard: %w", err)
	}
	defer closeClipboard.Call()

	hMem, _, err := getClipboardData.Call(uintptr(cFmtHdrop))
	if hMem == 0 {
		return nil, fmt.Errorf("failed to get clipboard data: %w", err)
	}

	p, _, err := gLock.Call(hMem)
	if p == 0 {
		return nil, fmt.Errorf("failed to lock global memory: %w", err)
	}
	defer gUnlock.Call(hMem)

	var paths []string
	offset := 0
	for {
		pathPtr := (*uint16)(unsafe.Pointer(p + uintptr(offset*2)))
		if *pathPtr == 0 {
			break
		}
		var utf16Str []uint16
		for i := 0; ; i++ {
			ch := *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(pathPtr)) + uintptr(i*2)))
			if ch == 0 {
				utf16Str = (*[1 << 20]uint16)(unsafe.Pointer(pathPtr))[:i:i]
				break
			}
		}
		path := string(utf16.Decode(utf16Str))
		paths = append(paths, path)
		offset += len(utf16Str) + 1
	}

	return paths, nil
}

func readImage() (image.Image, error) {
	return nil, notImplement
}

func isClipboardChanged() bool {
	return false
}
