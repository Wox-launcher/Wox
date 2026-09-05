//go:build windows

// Command window_hook_helper runs WoxWindowHook operations in a process whose bitness
// differs from Wox's.
//
// SetWindowsHookEx refuses to inject across bitness, and a 64-bit process cannot even
// load the 32-bit DLL to obtain the module handle the call requires. Windows therefore
// leaves exactly one option: a helper built for the target's bitness that owns the hook
// on Wox's behalf.
//
// Two shapes of work go through here. The sticky subclass outlives the call, so that mode
// keeps the helper alive; navigation and selection leave nothing behind in the target, so
// those run once and exit.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// diagnostic mirrors WoxWindowHookDiagnostic. Both this process and the DLL it loads are
// built for the same bitness, so the layout matches by construction.
type diagnostic struct {
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

func (d diagnostic) String() string {
	return fmt.Sprintf("stage=%d win32Error=%d hresult=0x%08X targetPid=%d targetThread=%d shellView=%d hookInstalled=%d callbackEntered=%d waitResult=%d",
		d.Stage, d.Win32Error, uint32(d.HResult), d.TargetPid, d.TargetThread, d.ShellViewFound, d.HookInstalled, d.CallbackEntered, d.WaitResult)
}

func main() {
	dllPath := flag.String("dll", "", "path to the matching-bitness WoxWindowHook DLL")
	command := flag.String("command", "sticky", "sticky, navigate or select")
	target := flag.Uint64("target", 0, "target window handle")
	pid := flag.Int("pid", 0, "target process id")
	overlay := flag.Uint64("overlay", 0, "overlay window handle owned by Wox")
	flag.Parse()

	if *dllPath == "" || *target == 0 || *pid <= 0 {
		refuse(2, "invalid-arguments")
	}

	// Every export installs a hook owned by the calling thread, so the runtime must never
	// migrate the goroutine that makes the call.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dll, err := windows.LoadDLL(*dllPath)
	if err != nil {
		refuse(3, fmt.Sprintf("load-dll err=%v", err))
	}

	switch *command {
	case "sticky":
		holdSticky(dll, uintptr(*target), *pid, uintptr(*overlay))
	case "navigate":
		runDialogCommand(dll, "WoxWindowHookNavigateDialog", uintptr(*target), *pid)
	case "select":
		runDialogCommand(dll, "WoxWindowHookSelectDialogItem", uintptr(*target), *pid)
	default:
		refuse(2, fmt.Sprintf("unknown-command command=%q", *command))
	}
}

// refuse reports why the helper could not do its job and exits. Wox reads this first line.
func refuse(code int, reason string) {
	fmt.Printf("fail reason=%s\n", reason)
	os.Exit(code)
}

// holdSticky installs the subclass and then stays alive, because a hook dies with the
// process that owns it.
//
// Nothing is relayed back through here afterwards: once the subclass is in the target it
// reads the overlay offset straight off the target's window properties and repositions
// the overlay itself.
func holdSticky(dll *windows.DLL, target uintptr, pid int, overlay uintptr) {
	if overlay == 0 {
		refuse(2, "invalid-arguments")
	}

	attach, err := dll.FindProc("WoxWindowHookAttachSticky")
	if err != nil {
		refuse(4, fmt.Sprintf("missing-attach err=%v", err))
	}
	detach, err := dll.FindProc("WoxWindowHookDetachSticky")
	if err != nil {
		refuse(4, fmt.Sprintf("missing-detach err=%v", err))
	}

	var diag diagnostic
	handle, _, callErr := attach.Call(target, uintptr(uint32(pid)), overlay, uintptr(unsafe.Pointer(&diag)))
	if handle == 0 {
		refuse(5, fmt.Sprintf("attach %s callErr=%v", diag, callErr))
	}

	// os.Stdout is unbuffered, so Wox sees this as soon as the attach returns.
	fmt.Println("ok")

	// Blocking on stdin ties the hook's lifetime to Wox's: a normal detach closes the
	// pipe, and so does a Wox crash. Neither can leave a subclass stranded in the target.
	_, _ = io.Copy(io.Discard, os.Stdin)

	detach.Call(handle)
}

// runDialogCommand performs one navigation or selection and exits.
func runDialogCommand(dll *windows.DLL, export string, target uintptr, pid int) {
	// The path arrives on stdin rather than argv so user file paths never appear in a
	// command line, which any other process on the machine can read.
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		refuse(2, fmt.Sprintf("read-path err=%v", err))
	}
	path := strings.TrimRight(string(raw), "\r\n")
	if path == "" {
		refuse(2, "empty-path")
	}
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		refuse(2, fmt.Sprintf("bad-path err=%v", err))
	}

	proc, err := dll.FindProc(export)
	if err != nil {
		refuse(4, fmt.Sprintf("missing-export export=%s err=%v", export, err))
	}

	var diag diagnostic
	result, _, callErr := proc.Call(target, uintptr(uint32(pid)), uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&diag)))
	if result == 0 {
		refuse(5, fmt.Sprintf("command %s callErr=%v", diag, callErr))
	}

	fmt.Println("ok")
}
