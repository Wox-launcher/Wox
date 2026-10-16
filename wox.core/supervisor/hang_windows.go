//go:build windows

package supervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	hangUser32      = windows.NewLazySystemDLL("user32.dll")
	hangEnumWindows = hangUser32.NewProc("EnumWindows")
	hangWindowPID   = hangUser32.NewProc("GetWindowThreadProcessId")
	hangSendMessage = hangUser32.NewProc("SendMessageTimeoutW")
	hangMiniDump    = windows.NewLazySystemDLL("dbghelp.dll").NewProc("MiniDumpWriteDump")
)

// watchUI probes hidden as well as visible top-level windows from outside Wox.
// WM_NULL must be dispatched by the owning thread, so background Go activity
// cannot disguise a blocked UI loop. It never terminates the child.
func (m *Manager) watchUI(ctx context.Context, pid int) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var observation hangObservation
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			hwnd, err := unresponsiveWindow(pid)
			if err != nil {
				m.logf("UI responsiveness probe failed: pid=%d error=%v", pid, err)
				observation = hangObservation{}
				continue
			}
			if !observation.observe(time.Now(), hwnd != 0) || ctx.Err() != nil {
				continue
			}
			m.logf("UI unresponsive for at least 30s: pid=%d hwnd=%#x; capturing live dump", pid, hwnd)
			path, err := m.captureHangDump(pid)
			if err != nil {
				m.logf("UI hang dump failed: pid=%d hwnd=%#x error=%v", pid, hwnd, err)
			} else {
				m.logf("UI hang dump captured: pid=%d path=%s", pid, path)
			}
			// Preserve evidence even if the user later exits cleanly. This is a
			// diagnostic report, not a crash incident or a restart request.
			if report, err := m.ExportCrash(ctx); err != nil {
				m.logf("UI hang report failed: pid=%d error=%v", pid, err)
			} else {
				m.logf("UI hang report exported: pid=%d path=%s", pid, report)
			}
		}
	}()
	return func() { cancel(); <-done }
}

// unresponsiveWindow stops at the first timed-out window owned by this child.
func unresponsiveWindow(pid int) (uintptr, error) {
	probe := windowProbe{pid: pid}

	ok, _, err := hangEnumWindows.Call(hangWindowCallback, uintptr(unsafe.Pointer(&probe)))
	if ok == 0 && probe.blocked == 0 {
		return 0, fmt.Errorf("EnumWindows: %w", err)
	}
	return probe.blocked, nil
}

// captureHangDump writes from the supervisor while the target threads still
// exist. Publish only completed dumps so report collection cannot copy a partial file.
func (m *Manager) captureHangDump(pid int) (string, error) {
	if err := m.EnsureDirectories(); err != nil {
		return "", fmt.Errorf("prepare dump directory: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return "", fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(process)
	path := filepath.Join(m.CrashDumpsDirectory(), fmt.Sprintf("wox-%d-hang-%d.dmp", pid, time.Now().UnixMilli()))
	file, err := os.Create(path + ".tmp")
	if err != nil {
		return "", fmt.Errorf("create dump: %w", err)
	}
	defer os.Remove(path + ".tmp")
	// Thread, unloaded-module and memory-region information without a full heap.
	ok, _, dumpErr := hangMiniDump.Call(uintptr(process), uintptr(pid), file.Fd(), 0x1820, 0, 0, 0)
	if ok == 0 {
		file.Close()
		return "", fmt.Errorf("MiniDumpWriteDump: %w", dumpErr)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	if syncErr != nil {
		return "", fmt.Errorf("flush dump: %w", syncErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("close dump: %w", closeErr)
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return "", fmt.Errorf("publish dump: %w", err)
	}
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".dmp", retainedCrashArtifacts)
	return path, nil
}

// windowProbe is scoped to one synchronous EnumWindows call; the callback itself
// is allocated once because Windows callbacks cannot be freed by the Go runtime.
type windowProbe struct {
	pid     int
	blocked uintptr
}

var hangWindowCallback = windows.NewCallback(func(hwnd, parameter uintptr) uintptr {
	probe := (*windowProbe)(unsafe.Pointer(parameter))
	var owner uint32
	hangWindowPID.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
	if owner != uint32(probe.pid) {
		return 1
	}
	var result uintptr
	// SMTO_BLOCK | SMTO_ABORTIFHUNG | SMTO_ERRORONEXIT, timeout in ms.
	ok, _, err := hangSendMessage.Call(hwnd, 0, 0, 0, 0x23, 1000, uintptr(unsafe.Pointer(&result)))
	if ok == 0 && (err == windows.ERROR_TIMEOUT || err == windows.ERROR_SUCCESS) {
		probe.blocked = hwnd
		return 0
	}
	return 1
})
