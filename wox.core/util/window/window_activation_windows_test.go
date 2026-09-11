//go:build windows && cgo

package window

import (
	"os"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"unsafe"
)

// TestActivateWindowPreservesEditorFocus reproduces paste losing its target child control.
func TestActivateWindowPreservesEditorFocus(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	user32 := syscall.NewLazyDLL("user32.dll")
	parentClass, _ := syscall.UTF16PtrFromString("STATIC")
	parent, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(parentClass)), 0,
		wsOverlappedVisible, 80, 80, 320, 200, 0, 0, 0, 0)
	if parent == 0 {
		t.Fatal("could not create test window")
	}
	defer destroyDummyDialog(parent)
	class, _ := syscall.UTF16PtrFromString("EDIT")
	child, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0,
		0x50000000, 0, 0, 100, 30, parent, 0, 0, 0)
	if child == 0 {
		t.Fatal("could not create editor")
	}
	user32.NewProc("SetForegroundWindow").Call(parent)
	user32.NewProc("SetFocus").Call(child)
	if !ActivateWindow(ManagedWindow{Id: strconv.FormatUint(uint64(parent), 10), Pid: os.Getpid()}) {
		t.Fatal("could not activate test window")
	}
	if focused, _, _ := user32.NewProc("GetFocus").Call(); focused != child {
		t.Fatalf("activation moved focus from editor %#x to %#x", child, focused)
	}
}
