//go:build wox_ui_smoke && windows

package smoke

import (
	"context"
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
)

//go:embed native_drag_windows.ps1
var nativeDragPeerScript string

// NativeDragPeer is an external OLE file source/target, owned and cleaned up by one smoke case.
type NativeDragPeer struct {
	Root    string
	Handle  uintptr
	done    <-chan struct{}
	exitErr error
}

var dragUser32 = windows.NewLazySystemDLL("user32.dll")

// OpenNativeDragPeer opens a local file-drop target without activating it over Wox.
func OpenNativeDragPeer(t *testing.T, ctx context.Context, client *automationdriver.Client, source string) *NativeDragPeer {
	t.Helper()
	root := t.TempDir()
	script := filepath.Join(root, "peer.ps1")
	if err := os.WriteFile(script, []byte(nativeDragPeerScript), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := os.Create(filepath.Join(root, "peer.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-File", script, "-Root", root, "-Source", source)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Start(); err != nil {
		output.Close()
		t.Fatal(err)
	}
	done := make(chan struct{})
	peer := &NativeDragPeer{Root: root, done: done}
	go func() { peer.exitErr = cmd.Wait(); close(done) }()
	t.Cleanup(func() { NativeDragMouse(0, 0, 4); _ = cmd.Process.Kill(); <-done; _ = output.Close() })
	peer.Wait(t, ctx, client, "ready")
	data, _ := os.ReadFile(filepath.Join(root, "ready"))
	handle, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	peer.Handle = uintptr(handle)
	return peer
}

// Wait observes the native peer's acknowledged drag state rather than guessing input timing.
func (p *NativeDragPeer) Wait(t *testing.T, ctx context.Context, client *automationdriver.Client, name string) {
	t.Helper()
	// Starting PowerShell includes compiling the WinForms peer on cold CI hosts.
	// Give setup its own budget; actual drag acknowledgements retain ActionTimeout.
	timeout := automationdriver.ActionTimeout
	if name == "ready" {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	LogNativeDragState(t, "waiting for peer "+name, p.Handle)
	for {
		data, err := os.ReadFile(filepath.Join(p.Root, name))
		if err == nil && len(data) > 0 {
			return
		}
		select {
		case <-p.done:
			log, _ := os.ReadFile(filepath.Join(p.Root, "peer.log"))
			t.Fatalf("native drag peer exited before %s: %v; %s", name, p.exitErr, log)
		case <-ctx.Done():
			LogNativeDragState(t, "timed out waiting for peer "+name, p.Handle)
			log, _ := os.ReadFile(filepath.Join(p.Root, "peer.log"))
			t.Fatalf("native drag peer %s: %v; %s", name, ctx.Err(), log)
		case <-ticker.C:
		}
	}
}

// LogNativeDragState reads native state without the automation endpoint, which may be blocked in OLE.
func LogNativeDragState(t *testing.T, stage string, handle uintptr) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	defer dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(old)
	point := struct{ X, Y int32 }{}
	cursorOK, _, cursorErr := dragUser32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	rect := struct{ Left, Top, Right, Bottom int32 }{}
	rectOK, _, rectErr := dragUser32.NewProc("GetWindowRect").Call(handle, uintptr(unsafe.Pointer(&rect)))
	dpi, _, _ := dragUser32.NewProc("GetDpiForWindow").Call(handle)
	leftButton, _, _ := dragUser32.NewProc("GetAsyncKeyState").Call(1)
	t.Logf("native drag %s: hwnd=%x foreground=%x visible=%v captured=%v dpi=%d cursorPhysical=%+v cursorOK=%d cursorErr=%v rectPhysical=%+v rectOK=%d rectErr=%v leftButtonDown=%v", stage, handle, NativeDragForeground(), NativeDragVisible(handle), NativeDragCaptured(handle), dpi, point, cursorOK, cursorErr, rect, rectOK, rectErr, leftButton&0x8000 != 0)
}

// NativeDragPoint maps semantic logical coordinates into physical desktop pixels on the HWND's display.
// Native OLE requires OS pointer input; widget pointer dispatch alone cannot drive its modal loop.
func NativeDragPoint(handle uintptr, point woxui.Point) (int32, int32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	defer dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(old)
	dpi, _, _ := dragUser32.NewProc("GetDpiForWindow").Call(handle)
	position := struct{ X, Y int32 }{int32(point.X * float32(dpi) / 96), int32(point.Y * float32(dpi) / 96)}
	dragUser32.NewProc("ClientToScreen").Call(handle, uintptr(unsafe.Pointer(&position)))
	return position.X, position.Y
}

// NativeDragForeground identifies the actual source HWND after the launcher is shown.
func NativeDragForeground() uintptr {
	h, _, _ := dragUser32.NewProc("GetForegroundWindow").Call()
	return h
}

// NativeDragCaptured waits on native button dispatch before moving into the drag threshold.
func NativeDragCaptured(handle uintptr) bool {
	info := struct {
		Size, Flags                                        uint32
		Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
		Rect                                               [4]int32
	}{}
	info.Size = uint32(unsafe.Sizeof(info))
	thread, _, _ := dragUser32.NewProc("GetWindowThreadProcessId").Call(handle, 0)
	ok, _, _ := dragUser32.NewProc("GetGUIThreadInfo").Call(thread, uintptr(unsafe.Pointer(&info)))
	return ok != 0 && info.Capture == handle
}

// NativeDragVisible checks the real HWND, catching native SW_HIDE calls that bypass managed state.
func NativeDragVisible(handle uintptr) bool {
	value, _, _ := windowsIsWindowVisible.Call(handle)
	return value != 0
}

// NativeDragMouse moves the OS pointer or sends a button transition (2 down, 4 up).
func NativeDragMouse(x, y int32, flags uintptr) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old, _, _ := dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(^uintptr(3))
	defer dragUser32.NewProc("SetThreadDpiAwarenessContext").Call(old)
	if flags == 0 {
		dragUser32.NewProc("SetCursorPos").Call(uintptr(x), uintptr(y))
		return
	}
	dragUser32.NewProc("mouse_event").Call(flags, 0, 0, 0, 0)
}

// NativeDragEscape cancels the running OLE drag through real keyboard input.
func NativeDragEscape() {
	windowsKeybdEvent.Call(0x1b, 0, 0, 0)
	windowsKeybdEvent.Call(0x1b, 0, windowsKeyEventFlagKeyUp, 0)
}

// Center returns a physical point inside the native peer's client area.
func (p *NativeDragPeer) Center() (int32, int32) {
	return NativeDragPoint(p.Handle, woxui.Point{X: 50, Y: 50})
}
