//go:build windows && cgo

package window

import (
	"os"
	"strconv"
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

	windowId := strconv.FormatUint(uint64(hwnd), 10)
	if IsBrowseForFolderDialog(windowId, os.Getpid()) {
		t.Fatal("a generic #32770 window was treated as SHBrowseForFolder")
	}
}

func TestOpenFolderDialogIsNotBrowseForFolder(t *testing.T) {
	hwnd := createDummyDialog(t, "Open Folder")
	defer destroyDummyDialog(hwnd)

	if IsBrowseForFolderDialog(strconv.FormatUint(uint64(hwnd), 10), os.Getpid()) {
		t.Fatal("IFileDialog Open Folder must not use the SHBrowseForFolder path reader")
	}
}

func TestSelectProjectRootDialogIsNotBrowseForFolder(t *testing.T) {
	hwnd := createDummyDialog(t, "Select Project Root")
	defer destroyDummyDialog(hwnd)

	if IsBrowseForFolderDialog(strconv.FormatUint(uint64(hwnd), 10), os.Getpid()) {
		t.Fatal("Select Project Root must not use the SHBrowseForFolder path reader")
	}
}

func TestMoveItemsDialogIsBrowseForFolder(t *testing.T) {
	hwnd := createDummyDialog(t, "Move Items")
	defer destroyDummyDialog(hwnd)

	if !IsBrowseForFolderDialog(strconv.FormatUint(uint64(hwnd), 10), os.Getpid()) {
		t.Fatal("Move Items should stay on the SHBrowseForFolder path")
	}
}

func createDummyDialog(t *testing.T, titleText string) uintptr {
	t.Helper()
	user32 := syscall.NewLazyDLL("user32.dll")
	createWindowExW := user32.NewProc("CreateWindowExW")

	className, err := syscall.UTF16PtrFromString("#32770")
	if err != nil {
		t.Fatal(err)
	}
	title, err := syscall.UTF16PtrFromString(titleText)
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
	return hwnd
}

func destroyDummyDialog(hwnd uintptr) {
	user32 := syscall.NewLazyDLL("user32.dll")
	destroyWindow := user32.NewProc("DestroyWindow")
	destroyWindow.Call(hwnd)
}
