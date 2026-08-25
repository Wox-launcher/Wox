package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
)

const (
	coInitializeAlreadyInitialized = syscall.Errno(1)
	coInitializeChangedMode        = syscall.Errno(0x80010106)
	seeMaskAsyncOK                 = 0x00100000
	seeMaskInvokeIDList            = 0x0000000C
	seeMaskNoCloseProcess          = 0x00000040
	shellExecuteShowNormal         = 1
	shellExecuteSuccessThreshold   = 32
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW            = shell32.NewProc("ShellExecuteExW")
	procILCreateFromPathW          = shell32.NewProc("ILCreateFromPathW")
	procILFree                     = shell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
)

type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     uintptr
}

type shellExecuteRequest struct {
	File           string
	Verb           string
	Parameters     string
	Directory      string
	Show           int32
	NoCloseProcess bool
}

func Open(path string) error {
	return executeShellVerb(path, "open")
}

// OpenAsAdministrator launches an application through the Windows runas verb.
func OpenAsAdministrator(path string) error {
	return executeShellVerb(path, "runas")
}

// OpenWithParameters launches a file through the Windows open verb with optional arguments.
func OpenWithParameters(file string, parameters string) error {
	_, err := shellExecute(shellExecuteRequest{
		File:       file,
		Verb:       "open",
		Parameters: parameters,
		Show:       shellExecuteShowNormal,
	})
	return err
}

// RunElevated launches a file through the Windows runas verb so UAC can elevate it.
// parameters is the argument string passed to the executable. directory is optional.
func RunElevated(file string, parameters string, directory string) (WaitFunc, error) {
	return shellExecute(shellExecuteRequest{
		File:           file,
		Verb:           "runas",
		Parameters:     parameters,
		Directory:      directory,
		Show:           shellExecuteShowNormal,
		NoCloseProcess: true,
	})
}

// executeShellVerb keeps normal and elevated launches on the same ShellExecute path.
func executeShellVerb(path string, verb string) error {
	_, err := shellExecute(shellExecuteRequest{
		File: path,
		Verb: verb,
		Show: shellExecuteShowNormal,
	})
	return err
}

// shellExecute invokes ShellExecuteExW and optionally returns a wait callback.
func shellExecute(req shellExecuteRequest) (WaitFunc, error) {
	operationPtr, err := windows.UTF16PtrFromString(req.Verb)
	if err != nil {
		return nil, fmt.Errorf("encode ShellExecute verb: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(req.File)
	if err != nil {
		return nil, fmt.Errorf("encode ShellExecute path: %w", err)
	}

	parameterPtr, err := utf16PtrOrNil(req.Parameters)
	if err != nil {
		return nil, fmt.Errorf("encode ShellExecute parameters: %w", err)
	}

	directoryPtr, err := utf16PtrOrNil(req.Directory)
	if err != nil {
		return nil, fmt.Errorf("encode ShellExecute directory: %w", err)
	}

	mask := shellExecuteMask(req.File)
	if req.NoCloseProcess {
		mask |= seeMaskNoCloseProcess
		// Wait for UAC and process creation so hProcess is populated.
		mask &^= seeMaskAsyncOK
	}

	info := shellExecuteInfo{
		cbSize: uint32(unsafe.Sizeof(shellExecuteInfo{})),
		// Keep the direct Shell path handling but let Windows finish DDE/delegate launch work asynchronously.
		fMask:        mask,
		lpVerb:       operationPtr,
		lpFile:       pathPtr,
		lpParameters: parameterPtr,
		lpDirectory:  directoryPtr,
		nShow:        req.Show,
	}

	ret, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			if req.Verb == "runas" && errors.Is(callErr, windows.ERROR_CANCELLED) {
				return nil, ErrElevationCancelled
			}
			return nil, fmt.Errorf("ShellExecute %s failed for %s: %w", req.Verb, req.File, callErr)
		}
		return nil, fmt.Errorf("ShellExecute %s failed for %s", req.Verb, req.File)
	}
	if info.hInstApp <= shellExecuteSuccessThreshold {
		return nil, fmt.Errorf("ShellExecute %s failed for %s with code %d", req.Verb, req.File, info.hInstApp)
	}

	if !req.NoCloseProcess || info.hProcess == 0 {
		return nil, nil
	}

	handle := windows.Handle(info.hProcess)
	return func() (int, error) {
		defer windows.CloseHandle(handle)

		event, err := windows.WaitForSingleObject(handle, windows.INFINITE)
		if err != nil {
			return 1, err
		}
		if event != windows.WAIT_OBJECT_0 {
			return 1, fmt.Errorf("wait for elevated process returned %d", event)
		}

		var exitCode uint32
		if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
			return 1, err
		}
		return int(exitCode), nil
	}, nil
}

// utf16PtrOrNil encodes a ShellExecute string argument, leaving empty values unset.
func utf16PtrOrNil(value string) (*uint16, error) {
	if value == "" {
		return nil, nil
	}
	return windows.UTF16PtrFromString(value)
}

// shellExecuteMask routes namespace objects through their context-menu handlers so registered verbs are available.
func shellExecuteMask(path string) uint32 {
	mask := uint32(seeMaskAsyncOK)
	if strings.HasPrefix(strings.ToLower(path), "shell:") {
		mask |= seeMaskInvokeIDList
	}
	return mask
}

func Run(name string, arg ...string) (*exec.Cmd, error) {
	return RunWithEnv(name, []string{"PYTHONIOENCODING=utf-8"}, arg...)
}

func RunWithEnv(name string, envs []string, arg ...string) (*exec.Cmd, error) {
	cmd := BuildCommand(name, envs, arg...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory(name)
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return nil, cmdErr
	}

	return cmd, nil
}

func RunOutput(name string, arg ...string) ([]byte, error) {
	cmd := BuildCommand(name, nil, arg...)
	return cmd.Output()
}

func OpenFileInFolder(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	return openFileInFolder(absPath)
}

// openFileInFolder asks Windows Shell to reveal the item directly instead of
// relying on explorer.exe command-line parsing.
func openFileInFolder(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cleanupCOM, err := initializeCOMForShell()
	if err != nil {
		return err
	}
	defer cleanupCOM()

	itemIDList, err := createShellItemIDList(path)
	if err != nil {
		return err
	}
	defer procILFree.Call(itemIDList)

	ret, _, _ := procSHOpenFolderAndSelectItems.Call(itemIDList, 0, 0, 0)
	if ret != 0 {
		return fmt.Errorf("open folder and select item failed with HRESULT 0x%08x", uint32(ret))
	}

	return nil
}

// initializeCOMForShell prepares COM for Shell API calls when this goroutine
// has not already entered a COM apartment.
func initializeCOMForShell() (func(), error) {
	err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE)
	if err == nil || errors.Is(err, coInitializeAlreadyInitialized) {
		return windows.CoUninitialize, nil
	}
	if errors.Is(err, coInitializeChangedMode) {
		return func() {}, nil
	}
	return nil, fmt.Errorf("initialize COM for Shell API: %w", err)
}

// createShellItemIDList converts a filesystem path to the Shell item ID list
// required by SHOpenFolderAndSelectItems.
func createShellItemIDList(path string) (uintptr, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("encode Shell path: %w", err)
	}

	itemIDList, _, callErr := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(pathPtr)))
	if itemIDList == 0 {
		if callErr != syscall.Errno(0) {
			return 0, fmt.Errorf("create Shell item ID list: %w", callErr)
		}
		return 0, fmt.Errorf("create Shell item ID list failed")
	}

	return itemIDList, nil
}

// HideWindowCmd sets the SysProcAttr to hide the console window on Windows
func HideWindowCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
