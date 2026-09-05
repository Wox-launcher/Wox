//go:build windows

package windowhook

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
)

var dialogCommandMu sync.Mutex

// StickyHook owns the loaded hook DLL and injected target subclass.
type StickyHook struct {
	dll    *windows.DLL
	handle uintptr
	target uintptr
}

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
	return HelperDLLPath(false)
}

// AttachSticky injects the existing target-thread hook and sends move notifications to observerHWND.
// Every failure is logged with the native stage, because a silent nil here degrades the overlay to
// polling and looks like a lag bug rather than a broken injection.
func AttachSticky(windowID string, pid int, observerHWND uintptr) *StickyHook {
	target, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || target == 0 || pid <= 0 || observerHWND == 0 {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("sticky attach rejected input: windowId=%q pid=%d overlay=0x%X err=%v", windowID, pid, observerHWND, err))
		return nil
	}
	dll, err := windows.LoadDLL(DLLPath())
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("sticky attach cannot load hook dll: path=%q err=%v", DLLPath(), err))
		return nil
	}
	attach, err := dll.FindProc("WoxWindowHookAttachSticky")
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("sticky attach missing export: err=%v", err))
		_ = dll.Release()
		return nil
	}
	var diagnostic navigationDiagnostic
	handle, _, callErr := attach.Call(uintptr(target), uintptr(uint32(pid)), observerHWND, uintptr(unsafe.Pointer(&diagnostic)))
	if handle == 0 {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("sticky attach failed: target=0x%X pid=%d overlay=0x%X stage=%d win32Error=%d targetPid=%d targetThread=%d hookInstalled=%d callbackEntered=%d waitResult=%d callErr=%v",
			target, pid, observerHWND, diagnostic.Stage, diagnostic.Win32Error, diagnostic.TargetPid, diagnostic.TargetThread,
			diagnostic.HookInstalled, diagnostic.CallbackEntered, diagnostic.WaitResult, callErr))
		_ = dll.Release()
		return nil
	}
	return &StickyHook{dll: dll, handle: handle, target: uintptr(target)}
}

// Property names must match the injected hook, which reads them on the target thread.
const (
	stickyOffsetActiveProp = "Wox.WindowHook.StickyOffset.Active.v1"
	stickyOffsetXProp      = "Wox.WindowHook.StickyOffset.X.v1"
	stickyOffsetYProp      = "Wox.WindowHook.StickyOffset.Y.v1"
)

var (
	user32          = windows.NewLazySystemDLL("user32.dll")
	procGetWindowRc = user32.NewProc("GetWindowRect")
	procSetPropW    = user32.NewProc("SetPropW")
	procRemovePropW = user32.NewProc("RemovePropW")
)

type rect struct {
	Left, Top, Right, Bottom int32
}

func windowRect(hwnd uintptr) (rect, bool) {
	var out rect
	result, _, _ := procGetWindowRc.Call(hwnd, uintptr(unsafe.Pointer(&out)))
	return out, result != 0
}

func setProp(hwnd uintptr, name string, value uintptr) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return
	}
	procSetPropW.Call(hwnd, uintptr(unsafe.Pointer(namePtr)), value)
}

func removeProp(hwnd uintptr, name string) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return
	}
	procRemovePropW.Call(hwnd, uintptr(unsafe.Pointer(namePtr)))
}

// publishStickyOffset records where the overlay currently sits relative to the target's
// outer frame so the injected subclass can translate it without asking Wox for geometry.
// Wox stays the authority: it republishes after every authoritative layout.
//
// A 32-bit target truncates these values to the low word when it reads them back, which
// is exactly the right answer for a negative offset, so the same props serve both the
// in-process hook and the 32-bit helper.
func publishStickyOffset(target, overlayHWND uintptr) {
	if target == 0 || overlayHWND == 0 {
		return
	}
	targetRect, ok := windowRect(target)
	if !ok {
		return
	}
	overlayRect, ok := windowRect(overlayHWND)
	if !ok {
		return
	}
	setProp(target, stickyOffsetXProp, uintptr(int64(overlayRect.Left-targetRect.Left)))
	setProp(target, stickyOffsetYProp, uintptr(int64(overlayRect.Top-targetRect.Top)))
	setProp(target, stickyOffsetActiveProp, 1)
}

// clearStickyOffsetProps drops the published offset so a reattached overlay cannot inherit it.
func clearStickyOffsetProps(target uintptr) {
	if target == 0 {
		return
	}
	removeProp(target, stickyOffsetActiveProp)
	removeProp(target, stickyOffsetXProp)
	removeProp(target, stickyOffsetYProp)
}

// PublishStickyOffset republishes the offset for an in-process injected hook.
func (hook *StickyHook) PublishStickyOffset(overlayHWND uintptr) {
	if hook == nil {
		return
	}
	publishStickyOffset(hook.target, overlayHWND)
}

// Detach removes the target subclass before releasing Wox's DLL reference.
func (hook *StickyHook) Detach() bool {
	if hook == nil || hook.dll == nil || hook.handle == 0 {
		return true
	}
	detach, err := hook.dll.FindProc("WoxWindowHookDetachSticky")
	if err != nil {
		return false
	}
	detached, _, _ := detach.Call(hook.handle)
	if detached == 0 {
		return false
	}
	clearStickyOffsetProps(hook.target)
	hook.target = 0
	hook.handle = 0
	_ = hook.dll.Release()
	hook.dll = nil
	return true
}

// NavigateDialog performs one target-thread Shell browser navigation and unloads its DLL reference afterward.
func NavigateDialog(ctx context.Context, windowID string, pid int, targetPath string) bool {
	return runDialogCommand(ctx, windowID, pid, targetPath, "WoxWindowHookNavigateDialog", "navigate", "navigation")
}

// SelectDialogItem selects one path in the dialog's active Shell view.
func SelectDialogItem(ctx context.Context, windowID string, pid int, targetPath string) bool {
	return runDialogCommand(ctx, windowID, pid, targetPath, "WoxWindowHookSelectDialogItem", "select", "selection")
}

// runDialogCommand executes one serialized command because the DLL uses process-wide IPC names.
func runDialogCommand(ctx context.Context, windowID string, pid int, targetPath string, procName string, helperCommand string, operation string) bool {
	hwnd, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || hwnd == 0 {
		return false
	}

	// A target of a different bitness can never load this DLL, so the in-process path
	// below would only wait out its timeout before failing. The helper owns no shared IPC
	// name with Wox either, which is why it needs none of the serialization below.
	if NeedsBitnessHelper(pid) {
		return RunDialogCommandViaHelper(ctx, helperCommand, uintptr(hwnd), pid, targetPath)
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
