//go:build wox_ui_smoke && windows

package smoke

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const windowsKeyEventFlagKeyUp = 0x0002

// windowsVisibleWindowSearch carries one title match through EnumWindows.
type windowsVisibleWindowSearch struct {
	window uintptr
	pid    uint32
	title  [512]uint16
}

var (
	windowsKeybdEvent          = windows.NewLazySystemDLL("user32.dll").NewProc("keybd_event")
	windowsEnumWindows         = windows.NewLazySystemDLL("user32.dll").NewProc("EnumWindows")
	windowsGetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")
	windowsGetWindowProcessID  = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	windowsGetWindowText       = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")
	windowsIsWindowVisible     = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowVisible")
	windowsSetForegroundWindow = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")
	windowsEnumWindowCallback  = windows.NewCallback(func(window uintptr, lParam uintptr) uintptr {
		search := (*windowsVisibleWindowSearch)(unsafe.Pointer(lParam))
		visible, _, _ := windowsIsWindowVisible.Call(window)
		if visible == 0 {
			return 1
		}
		var windowPID uint32
		windowsGetWindowProcessID.Call(window, uintptr(unsafe.Pointer(&windowPID)))
		var windowTitle [512]uint16
		length, _, _ := windowsGetWindowText.Call(window, uintptr(unsafe.Pointer(&windowTitle[0])), uintptr(len(windowTitle)))
		actualTitle := strings.ToLower(windows.UTF16ToString(windowTitle[:length]))
		targetTitle := strings.ToLower(windows.UTF16ToString(search.title[:]))
		if strings.Contains(actualTitle, targetTitle) {
			search.window = window
			search.pid = windowPID
			return 0
		}
		return 1
	})
)

// SendNativeKeyChord posts one chord through the real Windows input path.
func SendNativeKeyChord(keys ...string) error {
	keyCodes := map[string]uintptr{"ctrl": 0x11, "alt": 0x12, "space": 0x20, "a": 0x41, "c": 0x43, "f12": 0x7B}
	resolved := make([]uintptr, 0, len(keys))
	for _, key := range keys {
		code, ok := keyCodes[strings.ToLower(key)]
		if !ok {
			return fmt.Errorf("unsupported Windows smoke key %q", key)
		}
		resolved = append(resolved, code)
	}
	if len(resolved) == 0 {
		return fmt.Errorf("empty Windows smoke chord")
	}
	if err := windowsKeybdEvent.Find(); err != nil {
		return fmt.Errorf("resolve keybd_event: %w", err)
	}
	for _, key := range resolved {
		windowsKeybdEvent.Call(key, 0, 0, 0)
	}
	for index := len(resolved) - 1; index >= 0; index-- {
		windowsKeybdEvent.Call(resolved[index], 0, windowsKeyEventFlagKeyUp, 0)
	}
	return nil
}

// OpenWindowsNotepad launches, focuses, and registers cleanup for one Notepad document.
func OpenWindowsNotepad(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	title, err := windows.UTF16FromString(filepath.Base(path))
	if err != nil {
		t.Fatalf("encode temporary Notepad document title: %v", err)
	}
	command := exec.Command(filepath.Join(systemRoot, "System32", "notepad.exe"), path)
	if err := command.Start(); err != nil {
		t.Fatalf("start Windows Notepad: %v", err)
	}
	var windowPID uint32
	t.Cleanup(func() {
		if windowPID != 0 && windowPID != uint32(command.Process.Pid) {
			if process, findErr := os.FindProcess(int(windowPID)); findErr == nil {
				_ = process.Kill()
			}
		}
		if command.Process != nil {
			_ = command.Process.Kill()
			_, _ = command.Process.Wait()
		}
	})

	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		window, pid := windowsVisibleWindowForTitle(title)
		if window != 0 {
			windowPID = pid
			windowsSetForegroundWindow.Call(window)
			if windowsForegroundWindowProcessID() == pid {
				return
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for temporary Windows Notepad document to become foreground: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// windowsVisibleWindowForTitle finds a visible top-level window whose title contains the target text.
func windowsVisibleWindowForTitle(title []uint16) (uintptr, uint32) {
	search := windowsVisibleWindowSearch{}
	copy(search.title[:], title)
	windowsEnumWindows.Call(windowsEnumWindowCallback, uintptr(unsafe.Pointer(&search)))
	return search.window, search.pid
}

// windowsForegroundWindowProcessID returns the process owning the foreground window.
func windowsForegroundWindowProcessID() uint32 {
	window, _, _ := windowsGetForegroundWindow.Call()
	if window == 0 {
		return 0
	}
	var pid uint32
	windowsGetWindowProcessID.Call(window, uintptr(unsafe.Pointer(&pid)))
	return pid
}
