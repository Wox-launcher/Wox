//go:build windows

package window

/*
#cgo LDFLAGS: -lpsapi -lgdi32 -luser32 -lshell32
#include <windows.h>
#include <psapi.h>
#include <shellapi.h>

char* getActiveWindowIcon(unsigned char **iconData, int *iconSize, int *width, int *height);
char* getActiveWindowName();
int getActiveWindowPid();
int activateWindowByPid(int pid);
int isOpenSaveDialog();
*/
import "C"
import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/lxn/win"
)

var (
	modkernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                = modkernel32.NewProc("OpenProcess")
	procQueryFullProcessImageNameW = modkernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle                = modkernel32.NewProc("CloseHandle")
)

const (
	oleSFalse                         = 0x00000001
	rpcEChangedMode                   = 0x80010106
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

func GetActiveWindowIcon() (image.Image, error) {
	var iconData *C.uchar
	var iconSize C.int
	var width, height C.int

	errMsgC := C.getActiveWindowIcon(&iconData, &iconSize, &width, &height)
	if errMsgC != nil {
		errMsg := C.GoString(errMsgC)
		return nil, fmt.Errorf("failed to get active window icon: %s", errMsg)
	}
	defer C.free(unsafe.Pointer(iconData))

	data := C.GoBytes(unsafe.Pointer(iconData), iconSize)
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	idx := 0
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: data[idx+2],
				G: data[idx+1],
				B: data[idx],
				A: data[idx+3],
			})
			idx += 4
		}
	}

	return img, nil
}

func GetActiveWindowName() string {
	cStr := C.getActiveWindowName()
	if cStr == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(cStr))
	length := C.int(C.strlen(cStr))
	bytes := C.GoBytes(unsafe.Pointer(cStr), length)
	return string(bytes)
}

func GetActiveWindowPid() int {
	pid := C.getActiveWindowPid()
	return int(pid)
}

func ActivateWindowByPid(pid int) bool {
	result := C.activateWindowByPid(C.int(pid))
	return int(result) == 1
}

func IsOpenSaveDialog() (bool, error) {
	result := C.isOpenSaveDialog()
	return int(result) == 1, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	return NavigateActiveFileExplorer(targetPath)
}

// IsFileExplorer checks if the given PID belongs to Explorer by checking the process image name.
func IsFileExplorer(pid int) (bool, error) {
	if pid == 0 {
		return false, nil
	}

	name, err := getProcessImageName(uint32(pid))
	if err != nil {
		return false, err
	}

	// Check if the executable name is explorer.exe
	baseName := filepath.Base(name)
	return strings.EqualFold(baseName, "explorer.exe"), nil
}

func getProcessImageName(pid uint32) (string, error) {
	hProcess, _, _ := procOpenProcess.Call(
		uintptr(PROCESS_QUERY_LIMITED_INFORMATION),
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return "", fmt.Errorf("OpenProcess failed")
	}
	defer procCloseHandle.Call(hProcess)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	// QueryFullProcessImageNameW
	ret, _, _ := procQueryFullProcessImageNameW.Call(
		hProcess,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return "", fmt.Errorf("QueryFullProcessImageNameW failed")
	}

	return syscall.UTF16ToString(buf[:size]), nil
}

func GetProcessIdentity(pid int) string {
	if pid <= 0 {
		return ""
	}

	name, err := getProcessImageName(uint32(pid))
	if err != nil {
		return ""
	}

	baseName := filepath.Base(name)
	if baseName == "" {
		return ""
	}
	return strings.ToLower(baseName)
}

// GetActiveFileExplorerPath returns the filesystem path of the currently active
// File Explorer window, or an empty string if the foreground window is not an
// Explorer folder or the path cannot be determined.
func GetActiveFileExplorerPath() string {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok {
			switch oleErr.Code() {
			case ole.S_OK, oleSFalse:
				initialized = true
			case rpcEChangedMode:
				// COM already initialized with different concurrency model; proceed.
			default:
				return ""
			}
		} else {
			return ""
		}
	} else {
		initialized = true
	}
	if initialized {
		defer ole.CoUninitialize()
	}

	fg := win.GetForegroundWindow()
	if fg == 0 {
		return ""
	}

	// Shell.Application automation to enumerate shell windows
	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return ""
	}
	defer unknown.Release()

	shellDisp, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return ""
	}
	defer shellDisp.Release()

	windowsVar, err := oleutil.CallMethod(shellDisp, "Windows")
	if err != nil {
		return ""
	}
	defer windowsVar.Clear()
	windowsDisp := windowsVar.ToIDispatch()
	if windowsDisp == nil {
		return ""
	}

	countVar, err := oleutil.GetProperty(windowsDisp, "Count")
	if err != nil {
		return ""
	}
	count := int(countVar.Val)
	countVar.Clear()

	for i := 0; i < count; i++ {
		itemVar, err := oleutil.CallMethod(windowsDisp, "Item", i)
		if err != nil {
			continue
		}
		wDisp := itemVar.ToIDispatch()
		if wDisp == nil {
			itemVar.Clear()
			continue
		}

		hwndVar, err := oleutil.GetProperty(wDisp, "HWND")
		if err != nil {
			itemVar.Clear()
			continue
		}
		wnd := uintptr(hwndVar.Val)
		hwndVar.Clear()

		if wnd != uintptr(fg) {
			itemVar.Clear()
			continue
		}

		// Matched the foreground window: get Document -> Folder -> Self -> Path
		docVar, err := oleutil.GetProperty(wDisp, "Document")
		if err != nil {
			itemVar.Clear()
			break
		}
		docDisp := docVar.ToIDispatch()
		if docDisp == nil {
			docVar.Clear()
			itemVar.Clear()
			break
		}

		folderVar, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			docVar.Clear()
			itemVar.Clear()
			break
		}
		folderDisp := folderVar.ToIDispatch()
		if folderDisp == nil {
			folderVar.Clear()
			docVar.Clear()
			itemVar.Clear()
			break
		}

		selfVar, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			folderVar.Clear()
			docVar.Clear()
			itemVar.Clear()
			break
		}
		selfDisp := selfVar.ToIDispatch()
		if selfDisp == nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			itemVar.Clear()
			break
		}

		pathVar, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			itemVar.Clear()
			break
		}

		p := strings.TrimSpace(pathVar.ToString())

		pathVar.Clear()
		selfVar.Clear()
		folderVar.Clear()
		docVar.Clear()
		itemVar.Clear()

		return p
	}

	return ""
}

// NavigateActiveFileExplorer navigates the currently active Explorer window to the specified path.
// Returns true if successful, false otherwise.
func NavigateActiveFileExplorer(targetPath string) bool {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok {
			switch oleErr.Code() {
			case ole.S_OK, oleSFalse:
				initialized = true
			case rpcEChangedMode:
				// COM already initialized with different concurrency model; proceed.
			default:
				return false
			}
		} else {
			return false
		}
	} else {
		initialized = true
	}
	if initialized {
		defer ole.CoUninitialize()
	}

	fg := win.GetForegroundWindow()
	if fg == 0 {
		return false
	}

	// Shell.Application automation to enumerate shell windows
	unknown, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return false
	}
	defer unknown.Release()

	shellDisp, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return false
	}
	defer shellDisp.Release()

	windowsVar, err := oleutil.CallMethod(shellDisp, "Windows")
	if err != nil {
		return false
	}
	defer windowsVar.Clear()
	windowsDisp := windowsVar.ToIDispatch()
	if windowsDisp == nil {
		return false
	}

	countVar, err := oleutil.GetProperty(windowsDisp, "Count")
	if err != nil {
		return false
	}
	count := int(countVar.Val)
	countVar.Clear()

	for i := 0; i < count; i++ {
		itemVar, err := oleutil.CallMethod(windowsDisp, "Item", i)
		if err != nil {
			continue
		}
		wDisp := itemVar.ToIDispatch()
		if wDisp == nil {
			itemVar.Clear()
			continue
		}

		hwndVar, err := oleutil.GetProperty(wDisp, "HWND")
		if err != nil {
			itemVar.Clear()
			continue
		}
		wnd := uintptr(hwndVar.Val)
		hwndVar.Clear()

		if wnd != uintptr(fg) {
			itemVar.Clear()
			continue
		}

		// Found the foreground Explorer window, navigate to the target path
		_, err = oleutil.CallMethod(wDisp, "Navigate", targetPath)
		itemVar.Clear()
		return err == nil
	}

	return false
}

// GetOpenFinderWindowPaths returns a list of paths for all currently open Finder windows.
// Not applicable on Windows.
func GetOpenFinderWindowPaths() []string {
	// Theoretically we could implement this for Explorer windows too,
	// but currently the request is specific to Finder paths.
	return []string{}
}
