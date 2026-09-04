//go:build windows && cgo

package window

import (
	"os"
	"syscall"
	"testing"
	"unsafe"
)

const wsOverlappedVisible = 0x00CF0000 | 0x10000000

func TestOrdinaryDialogWithoutTreeIsNotAFileDialog(t *testing.T) {
	user32 := syscall.NewLazyDLL("user32.dll")
	createWindowExW := user32.NewProc("CreateWindowExW")
	destroyWindow := user32.NewProc("DestroyWindow")

	className, err := syscall.UTF16PtrFromString("#32770")
	if err != nil {
		t.Fatal(err)
	}
	title, err := syscall.UTF16PtrFromString("About Wox")
	if err != nil {
		t.Fatal(err)
	}
	hwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedVisible,
		80, 80, 320, 200,
		0, 0, 0, 0,
	)
	if hwnd == 0 {
		t.Skip("could not create a dummy #32770 window")
	}
	defer destroyWindow.Call(hwnd)

	isDialog, err := IsOpenSaveDialogByPid(os.Getpid())
	if err != nil {
		t.Fatalf("IsOpenSaveDialogByPid: %v", err)
	}
	if isDialog {
		t.Fatal("a generic #32770 window was treated as a file dialog")
	}
}
