package shell

import (
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// All Tasks is the Control Panel namespace behind God Mode. Its children expose
// parsing names, but SHParseDisplayName rejects those names. Icons and launches
// have to bind the folder and use the enumerated item ID list.
const controlPanelAllTasksNamespace = `shell:::{ED7BA470-8E54-465E-825C-99712043E01C}`

const (
	shcontfFolders            = 0x20
	shcontfNonFolders         = 0x40
	shcontfIncludeHidden      = 0x80
	shgdnForParsing           = 0x8000
	strretWSTR                = 0
	shellFolderBindToObject   = 5
	shellFolderEnumObjects    = 4
	shellFolderGetDisplayName = 11
	enumIDListNext            = 3
	unknownRelease            = 2
)

var (
	procSHParseDisplayName = shell32.NewProc("SHParseDisplayName")
	procSHGetDesktopFolder = shell32.NewProc("SHGetDesktopFolder")
	procILCombine          = shell32.NewProc("ILCombine")
)

var shellFolderIID = windows.GUID{
	Data1: 0x000214E6,
	Data2: 0x0000,
	Data3: 0x0000,
	Data4: [8]byte{0xC0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46},
}

type shellStrRet struct {
	uType   uint32
	_       uint32
	pOleStr *uint16
	rest    [256]byte
}

// VisitNamespaceChildren calls visit with each child's parsing name and absolute item ID list.
// The item ID list is released when visit returns.
func VisitNamespaceChildren(parent string, visit func(parsingName string, absolutePIDL uintptr) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	cleanupCOM, err := initializeCOMForShell()
	if err != nil {
		return err
	}
	defer cleanupCOM()

	parentPIDL, err := parseDisplayName(parent)
	if err != nil {
		return err
	}
	defer procILFree.Call(parentPIDL)

	desktop, err := desktopFolder()
	if err != nil {
		return err
	}
	defer releaseUnknown(desktop)

	folder, err := bindToFolder(desktop, parentPIDL)
	if err != nil {
		return err
	}
	defer releaseUnknown(folder)

	enum, err := enumFolderChildren(folder)
	if err != nil {
		return err
	}
	defer releaseUnknown(enum)

	for {
		child, ok := nextEnumID(enum)
		if !ok {
			return nil
		}
		parsingName := folderDisplayName(folder, child, shgdnForParsing)
		absolutePIDL, _, _ := procILCombine.Call(parentPIDL, child)
		procILFree.Call(child)
		if absolutePIDL == 0 || parsingName == "" {
			continue
		}
		visitErr := visit(parsingName, absolutePIDL)
		procILFree.Call(absolutePIDL)
		if visitErr != nil {
			return visitErr
		}
	}
}

// executeControlPanelTask launches a Control Panel task through its enumerated item ID list.
func executeControlPanelTask(req shellExecuteRequest) error {
	executed := false
	err := VisitNamespaceChildren(controlPanelAllTasksNamespace, func(parsingName string, absolutePIDL uintptr) error {
		if !strings.EqualFold(parsingName, req.File) {
			return nil
		}
		executed = true
		if execErr := executeIDList(req, absolutePIDL); execErr != nil {
			return execErr
		}
		return errStopNamespaceVisit
	})
	if err != nil && err != errStopNamespaceVisit {
		return err
	}
	if !executed {
		return fmt.Errorf("Control Panel task not found: %s", req.File)
	}
	return nil
}

var errStopNamespaceVisit = fmt.Errorf("stop namespace visit")

func isControlPanelTaskParsingName(path string) bool {
	// Child parsing names keep the All Tasks GUID and then another GUID.
	marker := strings.ToLower(strings.TrimPrefix(controlPanelAllTasksNamespace, "shell:::") + `\`)
	return strings.Contains(strings.ToLower(path), marker)
}

func parseDisplayName(path string) (uintptr, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("encode shell path: %w", err)
	}
	var pidl uintptr
	var attrs uint32
	hr, _, _ := procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		uintptr(unsafe.Pointer(&attrs)),
	)
	if int32(hr) < 0 || pidl == 0 {
		return 0, fmt.Errorf("parse shell path %s: 0x%08x", path, uint32(hr))
	}
	return pidl, nil
}

func desktopFolder() (uintptr, error) {
	var desktop uintptr
	hr, _, _ := procSHGetDesktopFolder.Call(uintptr(unsafe.Pointer(&desktop)))
	if int32(hr) < 0 || desktop == 0 {
		return 0, fmt.Errorf("desktop folder: 0x%08x", uint32(hr))
	}
	return desktop, nil
}

func bindToFolder(desktop uintptr, parentPIDL uintptr) (uintptr, error) {
	var folder uintptr
	hr := callShellMethod(desktop, shellFolderBindToObject, parentPIDL, 0, uintptr(unsafe.Pointer(&shellFolderIID)), uintptr(unsafe.Pointer(&folder)))
	if int32(hr) < 0 || folder == 0 {
		return 0, fmt.Errorf("bind shell folder: 0x%08x", uint32(hr))
	}
	return folder, nil
}

func enumFolderChildren(folder uintptr) (uintptr, error) {
	var enum uintptr
	hr := callShellMethod(folder, shellFolderEnumObjects, 0, uintptr(shcontfFolders|shcontfNonFolders|shcontfIncludeHidden), uintptr(unsafe.Pointer(&enum)))
	if int32(hr) < 0 || enum == 0 {
		return 0, fmt.Errorf("enumerate shell folder: 0x%08x", uint32(hr))
	}
	return enum, nil
}

func nextEnumID(enum uintptr) (uintptr, bool) {
	var child uintptr
	var fetched uint32
	hr := callShellMethod(enum, enumIDListNext, 1, uintptr(unsafe.Pointer(&child)), uintptr(unsafe.Pointer(&fetched)))
	if fetched == 0 || child == 0 || int32(hr) < 0 {
		return 0, false
	}
	return child, true
}

func folderDisplayName(folder uintptr, child uintptr, flags uintptr) string {
	var name shellStrRet
	hr := callShellMethod(folder, shellFolderGetDisplayName, child, flags, uintptr(unsafe.Pointer(&name)))
	if int32(hr) < 0 || name.uType != strretWSTR || name.pOleStr == nil {
		return ""
	}
	value := windows.UTF16PtrToString(name.pOleStr)
	windows.CoTaskMemFree(unsafe.Pointer(name.pOleStr))
	return value
}

func releaseUnknown(obj uintptr) {
	callShellMethod(obj, unknownRelease)
}

func callShellMethod(obj uintptr, index int, args ...uintptr) uintptr {
	vtbl := *(**[20]uintptr)(unsafe.Pointer(obj))
	callArgs := make([]uintptr, 0, len(args)+1)
	callArgs = append(callArgs, obj)
	callArgs = append(callArgs, args...)
	hr, _, _ := syscall.SyscallN(vtbl[index], callArgs...)
	return hr
}

func executeIDList(req shellExecuteRequest, pidl uintptr) error {
	operationPtr, err := windows.UTF16PtrFromString(req.Verb)
	if err != nil {
		return fmt.Errorf("encode ShellExecute verb: %w", err)
	}
	info := shellExecuteInfo{
		cbSize:   uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:    seeMaskInvokeIDList,
		hwnd:     req.Owner,
		lpVerb:   operationPtr,
		lpIDList: pidl,
		nShow:    req.Show,
	}
	if info.nShow == 0 {
		info.nShow = shellExecuteShowNormal
	}
	ret, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		if callErr != syscall.Errno(0) {
			return fmt.Errorf("ShellExecute task failed for %s: %w", req.File, callErr)
		}
		return fmt.Errorf("ShellExecute task failed for %s", req.File)
	}
	if info.hInstApp <= shellExecuteSuccessThreshold {
		return fmt.Errorf("ShellExecute task failed for %s with code %d", req.File, info.hInstApp)
	}
	return nil
}
