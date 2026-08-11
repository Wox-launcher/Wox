//go:build wox_ui_smoke && windows

package hotkey

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// visibleWindowSearch carries the process target and matching window through EnumWindows.
type visibleWindowSearch struct {
	window uintptr
	pid    uint32
	title  [512]uint16
}

var (
	enumWindows               = windows.NewLazySystemDLL("user32.dll").NewProc("EnumWindows")
	getForegroundWindow       = windows.NewLazySystemDLL("user32.dll").NewProc("GetForegroundWindow")
	getWindowThreadProcessID  = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowThreadProcessId")
	getWindowText             = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowTextW")
	isWindowVisible           = windows.NewLazySystemDLL("user32.dll").NewProc("IsWindowVisible")
	setForegroundWindow       = windows.NewLazySystemDLL("user32.dll").NewProc("SetForegroundWindow")
	enumVisibleWindowCallback = windows.NewCallback(func(window uintptr, lParam uintptr) uintptr {
		search := (*visibleWindowSearch)(unsafe.Pointer(lParam))
		visible, _, _ := isWindowVisible.Call(window)
		if visible == 0 {
			return 1
		}
		var windowPID uint32
		getWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&windowPID)))
		var windowTitle [512]uint16
		length, _, _ := getWindowText.Call(window, uintptr(unsafe.Pointer(&windowTitle[0])), uintptr(len(windowTitle)))
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

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "notepad.exe", "notepad.exe"
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	return "alt+space"
}

func sendIgnoredAppNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	sendNativeHotkey(t, hotkey)
}

// activateIgnoredHotkeyApp starts one isolated Notepad process and waits until its window owns foreground focus.
func activateIgnoredHotkeyApp(t *testing.T, ctx context.Context) {
	t.Helper()
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	tempFile, err := os.CreateTemp("", "wox-smoke-notepad-*.txt")
	if err != nil {
		t.Fatalf("create temporary Notepad document: %v", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		t.Fatalf("close temporary Notepad document: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tempPath) })

	title, err := windows.UTF16FromString(filepath.Base(tempPath))
	if err != nil {
		t.Fatalf("encode temporary Notepad document title: %v", err)
	}
	command := exec.Command(filepath.Join(systemRoot, "System32", "notepad.exe"), tempPath)
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
		window, pid := visibleWindowForTitle(title)
		if window != 0 {
			windowPID = pid
			setForegroundWindow.Call(window)
			if foregroundWindowProcessID() == pid {
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

// visibleWindowForTitle finds a visible top-level window whose title contains the target text.
func visibleWindowForTitle(title []uint16) (uintptr, uint32) {
	search := visibleWindowSearch{}
	copy(search.title[:], title)
	enumWindows.Call(enumVisibleWindowCallback, uintptr(unsafe.Pointer(&search)))
	return search.window, search.pid
}

// foregroundWindowProcessID returns the process currently owning the Windows foreground window.
func foregroundWindowProcessID() uint32 {
	window, _, _ := getForegroundWindow.Call()
	if window == 0 {
		return 0
	}
	var pid uint32
	getWindowThreadProcessID.Call(window, uintptr(unsafe.Pointer(&pid)))
	return pid
}
