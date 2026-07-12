//go:build windows

package windowhook

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
)

var dialogCommandMu sync.Mutex

type navigationDiagnostic struct {
	Stage           uint32
	Win32Error      uint32
	HResult         int32
	TargetPid       uint32
	TargetThread    uint32
	ShellViewFound  uint32
	HookInstalled   uint32
	CallbackEntered uint32
	WaitResult      uint32
}

func DLLPath() string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "window_hook", "WoxWindowHook64.dll")
}

// NavigateDialog performs one target-thread Shell browser navigation and unloads its DLL reference afterward.
func NavigateDialog(ctx context.Context, windowID string, pid int, targetPath string) bool {
	return runDialogCommand(ctx, windowID, pid, targetPath, "WoxWindowHookNavigateDialog", "navigation")
}

// SelectDialogItem selects one path in the dialog's active Shell view.
func SelectDialogItem(ctx context.Context, windowID string, pid int, targetPath string) bool {
	return runDialogCommand(ctx, windowID, pid, targetPath, "WoxWindowHookSelectDialogItem", "selection")
}

// runDialogCommand executes one serialized command because the DLL uses process-wide IPC names.
func runDialogCommand(ctx context.Context, windowID string, pid int, targetPath string, procName string, operation string) bool {
	hwnd, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || hwnd == 0 {
		return false
	}

	dialogCommandMu.Lock()
	defer dialogCommandMu.Unlock()

	dll, err := windows.LoadDLL(DLLPath())
	if err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Window hook load failed: %v", err))
		return false
	}
	defer dll.Release()

	command, err := dll.FindProc(procName)
	if err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Window hook %s export missing: %v", operation, err))
		return false
	}

	pathPtr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return false
	}

	startedAt := time.Now()
	diagnostic := navigationDiagnostic{}
	result, _, _ := command.Call(uintptr(hwnd), uintptr(uint32(pid)), uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&diagnostic)))
	util.GetLogger().Debug(ctx, fmt.Sprintf("Explorer dialog hook %s: succeeded=%t stage=%s(%d) win32Error=%d hresult=0x%08X targetPid=%d targetThread=%d shellView=%t hookInstalled=%t callbackEntered=%t waitResult=0x%08X pid=%d hwnd=%d elapsedMs=%d",
		operation,
		result != 0, navigationStageName(diagnostic.Stage), diagnostic.Stage, diagnostic.Win32Error, uint32(diagnostic.HResult), diagnostic.TargetPid, diagnostic.TargetThread,
		diagnostic.ShellViewFound != 0, diagnostic.HookInstalled != 0, diagnostic.CallbackEntered != 0, diagnostic.WaitResult, pid, hwnd, time.Since(startedAt).Milliseconds()))
	return result != 0
}

func navigationStageName(stage uint32) string {
	names := [...]string{"none", "validate_input", "validate_window", "resolve_thread", "create_ipc", "map_ipc", "install_hook", "post_message", "wait", "callback", "callback_validate", "co_initialize", "get_shell_browser", "parse_path", "browse", "completed", "query_active_view", "bind_parent", "get_view_folder", "compare_parent", "select_item"}
	if int(stage) < len(names) {
		return names[stage]
	}
	return "unknown"
}
