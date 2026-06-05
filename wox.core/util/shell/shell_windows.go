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
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procILCreateFromPathW          = shell32.NewProc("ILCreateFromPathW")
	procILFree                     = shell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
)

func Open(path string) error {
	cmd := BuildCommand("cmd", []string{"PYTHONIOENCODING=utf-8"})
	// Set CmdLine directly to bypass Go's automatic argument escaping.
	// This ensures our quoting is preserved so cmd.exe won't treat & as a command separator.
	cmd.SysProcAttr.CmdLine = `cmd /C start "" ` + QuoteCmdArg(path)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = getWorkingDirectory("cmd")
	return cmd.Start()
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

// QuoteCmdArg wraps a value in double quotes for use as a cmd.exe argument,
// escaping any embedded double quotes. This prevents cmd.exe from treating
// characters like & as command separators in URLs.
func QuoteCmdArg(value string) string {
	if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
