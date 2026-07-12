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

var navigateMu sync.Mutex

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
	hwnd, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || hwnd == 0 {
		return false
	}

	navigateMu.Lock()
	defer navigateMu.Unlock()

	dll, err := windows.LoadDLL(DLLPath())
	if err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Window hook load failed: %v", err))
		return false
	}
	defer dll.Release()

	navigate, err := dll.FindProc("WoxWindowHookNavigateDialog")
	if err != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Window hook navigation export missing: %v", err))
		return false
	}

	pathPtr, err := windows.UTF16PtrFromString(targetPath)
	if err != nil {
		return false
	}

	startedAt := time.Now()
	diagnostic := navigationDiagnostic{}
	result, _, _ := navigate.Call(uintptr(hwnd), uintptr(uint32(pid)), uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&diagnostic)))
	util.GetLogger().Debug(ctx, fmt.Sprintf("Explorer dialog hook navigation: succeeded=%t stage=%s(%d) win32Error=%d hresult=0x%08X targetPid=%d targetThread=%d shellView=%t hookInstalled=%t callbackEntered=%t waitResult=0x%08X pid=%d hwnd=%d elapsedMs=%d",
		result != 0, navigationStageName(diagnostic.Stage), diagnostic.Stage, diagnostic.Win32Error, uint32(diagnostic.HResult), diagnostic.TargetPid, diagnostic.TargetThread,
		diagnostic.ShellViewFound != 0, diagnostic.HookInstalled != 0, diagnostic.CallbackEntered != 0, diagnostic.WaitResult, pid, hwnd, time.Since(startedAt).Milliseconds()))
	return result != 0
}

func navigationStageName(stage uint32) string {
	names := [...]string{"none", "validate_input", "validate_window", "resolve_thread", "create_ipc", "map_ipc", "install_hook", "post_message", "wait", "callback", "callback_validate", "co_initialize", "get_shell_browser", "parse_path", "browse", "completed"}
	if int(stage) < len(names) {
		return names[stage]
	}
	return "unknown"
}
