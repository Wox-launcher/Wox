//go:build windows

package window

import (
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/lxn/win"
)

const (
	oleSFalse       = 0x00000001
	rpcEChangedMode = 0x80010106
)

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
