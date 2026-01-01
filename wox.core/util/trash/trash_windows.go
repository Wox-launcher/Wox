//go:build windows

package trash

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	foDelete          = 0x0003
	fofAllowUndo      = 0x0040
	fofNoConfirmation = 0x0010
	fofSilent         = 0x0004
)

type shFileOpStruct struct {
	hwnd                  windows.Handle
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	procSHFileOperationW = shell32.NewProc("SHFileOperationW")
)

func MoveToTrash(path string) error {
	if path == "" {
		return fmt.Errorf("trash path is empty")
	}

	pathUTF16, err := windows.UTF16FromString(path)
	if err != nil {
		return fmt.Errorf("trash path encode failed: %w", err)
	}
	// SHFileOperation expects double null-terminated strings.
	pathUTF16 = append(pathUTF16, 0)

	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &pathUTF16[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofSilent,
	}

	ret, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if ret != 0 {
		return fmt.Errorf("trash failed with code %d", ret)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("trash operation was aborted")
	}

	return nil
}
