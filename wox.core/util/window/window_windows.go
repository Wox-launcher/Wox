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
int navigateActiveFileDialog(const char* path);
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
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.navigateActiveFileDialog(cPath)) == 1
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

	getShellWindowLocationName := func(wDisp *ole.IDispatch) string {
		v, err := oleutil.GetProperty(wDisp, "LocationName")
		if err != nil {
			return ""
		}
		name := strings.TrimSpace(v.ToString())
		v.Clear()
		return name
	}

	getShellWindowPath := func(wDisp *ole.IDispatch) string {
		docVar, err := oleutil.GetProperty(wDisp, "Document")
		if err != nil {
			return ""
		}
		docDisp := docVar.ToIDispatch()
		if docDisp == nil {
			docVar.Clear()
			return ""
		}

		folderVar, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			docVar.Clear()
			return ""
		}
		folderDisp := folderVar.ToIDispatch()
		if folderDisp == nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		selfVar, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}
		selfDisp := selfVar.ToIDispatch()
		if selfDisp == nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		pathVar, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		p := strings.TrimSpace(pathVar.ToString())
		pathVar.Clear()
		selfVar.Clear()
		folderVar.Clear()
		docVar.Clear()
		return p
	}

	type candidate struct {
		path         string
		locationName string
	}

	candidates := make([]candidate, 0, 4)
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

		p := getShellWindowPath(wDisp)
		if p != "" {
			candidates = append(candidates, candidate{path: p, locationName: getShellWindowLocationName(wDisp)})
		}
		itemVar.Clear()
	}

	if len(candidates) == 0 {
		return ""
	}

	// With Explorer tabs, multiple ShellWindow entries may share the same HWND.
	// Use the current window title to identify the active tab.
	activeTitle := strings.TrimSpace(GetActiveWindowName())
	activeTitleLower := strings.ToLower(activeTitle)
	bestIdx := 0
	bestScore := -1
	for i, c := range candidates {
		score := 0
		loc := strings.TrimSpace(c.locationName)
		if loc == "" {
			loc = filepath.Base(c.path)
		}
		locLower := strings.ToLower(loc)
		if activeTitleLower != "" && locLower != "" {
			if activeTitleLower == locLower {
				score = 100
			} else if strings.Contains(activeTitleLower, locLower) || strings.Contains(locLower, activeTitleLower) {
				score = 50
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return candidates[bestIdx].path
}

// GetFileExplorerPathByPidAndWindowTitle returns the filesystem path of an Explorer tab/window owned by pid.
// On Windows 11, File Explorer tabs can share the same top-level HWND, so we use the window title to pick
// the active tab (best-effort) when multiple candidates exist.
func GetFileExplorerPathByPidAndWindowTitle(pid int, windowTitle string) string {
	if pid <= 0 {
		return ""
	}

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

	getShellWindowLocationName := func(wDisp *ole.IDispatch) string {
		v, err := oleutil.GetProperty(wDisp, "LocationName")
		if err != nil {
			return ""
		}
		name := strings.TrimSpace(v.ToString())
		v.Clear()
		return name
	}

	getShellWindowPath := func(wDisp *ole.IDispatch) string {
		docVar, err := oleutil.GetProperty(wDisp, "Document")
		if err != nil {
			return ""
		}
		docDisp := docVar.ToIDispatch()
		if docDisp == nil {
			docVar.Clear()
			return ""
		}

		folderVar, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			docVar.Clear()
			return ""
		}
		folderDisp := folderVar.ToIDispatch()
		if folderDisp == nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		selfVar, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}
		selfDisp := selfVar.ToIDispatch()
		if selfDisp == nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		pathVar, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		p := strings.TrimSpace(pathVar.ToString())
		pathVar.Clear()
		selfVar.Clear()
		folderVar.Clear()
		docVar.Clear()
		return p
	}

	// Compute z-order preference for visible windows (top-most earlier).
	zOrder := map[uintptr]int{}
	idx := 0
	for wnd := win.GetWindow(win.GetDesktopWindow(), win.GW_CHILD); wnd != 0; wnd = win.GetWindow(wnd, win.GW_HWNDNEXT) {
		zOrder[uintptr(wnd)] = idx
		idx++
	}

	titleLower := strings.ToLower(strings.TrimSpace(windowTitle))
	type candidate struct {
		path         string
		locationName string
		hwnd         uintptr
		z            int
	}

	candidates := make([]candidate, 0, 8)
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
		hwnd := uintptr(hwndVar.Val)
		hwndVar.Clear()

		var wndPid uint32
		win.GetWindowThreadProcessId(win.HWND(hwnd), &wndPid)
		if int(wndPid) != pid {
			itemVar.Clear()
			continue
		}

		p := getShellWindowPath(wDisp)
		if p == "" {
			itemVar.Clear()
			continue
		}

		loc := getShellWindowLocationName(wDisp)
		z := 1 << 30
		if v, ok := zOrder[hwnd]; ok {
			z = v
		}
		candidates = append(candidates, candidate{path: p, locationName: loc, hwnd: hwnd, z: z})
		itemVar.Clear()
	}

	if len(candidates) == 0 {
		return ""
	}

	bestIdx := 0
	bestScore := -1
	for i, c := range candidates {
		score := 0
		loc := strings.TrimSpace(c.locationName)
		if loc == "" {
			loc = filepath.Base(c.path)
		}
		locLower := strings.ToLower(loc)
		if titleLower != "" && locLower != "" {
			if titleLower == locLower {
				score += 100
			} else if strings.Contains(titleLower, locLower) || strings.Contains(locLower, titleLower) {
				score += 50
			}
		}

		// Prefer top-most visible window when ambiguous.
		if c.z < (1 << 30) {
			score += 10
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		} else if score == bestScore {
			// tie-break: closer to top in z-order
			if c.z < candidates[bestIdx].z {
				bestIdx = i
			}
		}
	}

	return candidates[bestIdx].path
}

// GetFileExplorerPathByPid returns the filesystem path of an Explorer window owned by pid.
func GetFileExplorerPathByPid(pid int) string {
	if pid <= 0 {
		return ""
	}

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

	getPath := func(wDisp *ole.IDispatch) string {
		docVar, err := oleutil.GetProperty(wDisp, "Document")
		if err != nil {
			return ""
		}
		docDisp := docVar.ToIDispatch()
		if docDisp == nil {
			docVar.Clear()
			return ""
		}

		folderVar, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			docVar.Clear()
			return ""
		}
		folderDisp := folderVar.ToIDispatch()
		if folderDisp == nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		selfVar, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}
		selfDisp := selfVar.ToIDispatch()
		if selfDisp == nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		pathVar, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		p := strings.TrimSpace(pathVar.ToString())

		pathVar.Clear()
		selfVar.Clear()
		folderVar.Clear()
		docVar.Clear()

		return p
	}

	paths := map[uintptr]string{}
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

		var wndPid uint32
		win.GetWindowThreadProcessId(win.HWND(wnd), &wndPid)
		if int(wndPid) != pid {
			itemVar.Clear()
			continue
		}

		p := getPath(wDisp)
		itemVar.Clear()
		if p == "" {
			continue
		}
		paths[wnd] = p
	}

	if len(paths) == 0 {
		return ""
	}

	for wnd := win.GetWindow(win.GetDesktopWindow(), win.GW_CHILD); wnd != 0; wnd = win.GetWindow(wnd, win.GW_HWNDNEXT) {
		if p, ok := paths[uintptr(wnd)]; ok {
			if win.IsWindowVisible(wnd) && !win.IsIconic(wnd) {
				return p
			}
		}
	}

	for _, p := range paths {
		return p
	}

	return ""
}

// GetOpenFinderWindowPaths returns a list of paths for all currently open Finder windows.
// Not applicable on Windows.
func GetOpenFinderWindowPaths() []string {
	// Theoretically we could implement this for Explorer windows too,
	// but currently the request is specific to Finder paths.
	return []string{}
}

// SelectInFileExplorerByPid selects a file in an Explorer window owned by pid.
func SelectInFileExplorerByPid(pid int, fullPath string) bool {
	if pid <= 0 || fullPath == "" {
		return false
	}

	// In Windows 11 File Explorer, multiple tabs may share the same top-level HWND.
	// When that happens, selecting a ShellWindow only by HWND can pick the wrong tab.
	// Prefer the tab whose current folder path matches the target file's directory.
	targetDir := filepath.Clean(filepath.Dir(fullPath))

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

	getShellWindowPath := func(wDisp *ole.IDispatch) string {
		docVar, err := oleutil.GetProperty(wDisp, "Document")
		if err != nil {
			return ""
		}
		docDisp := docVar.ToIDispatch()
		if docDisp == nil {
			docVar.Clear()
			return ""
		}

		folderVar, err := oleutil.GetProperty(docDisp, "Folder")
		if err != nil {
			docVar.Clear()
			return ""
		}
		folderDisp := folderVar.ToIDispatch()
		if folderDisp == nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		selfVar, err := oleutil.GetProperty(folderDisp, "Self")
		if err != nil {
			folderVar.Clear()
			docVar.Clear()
			return ""
		}
		selfDisp := selfVar.ToIDispatch()
		if selfDisp == nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		pathVar, err := oleutil.GetProperty(selfDisp, "Path")
		if err != nil {
			selfVar.Clear()
			folderVar.Clear()
			docVar.Clear()
			return ""
		}

		p := strings.TrimSpace(pathVar.ToString())
		pathVar.Clear()
		selfVar.Clear()
		folderVar.Clear()
		docVar.Clear()
		return p
	}

	type shellWindowCandidate struct {
		index int
		hwnd  uintptr
		path  string
	}

	candidates := make([]shellWindowCandidate, 0, 4)
	uniqueHwnds := map[uintptr]struct{}{}
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

		var wndPid uint32
		win.GetWindowThreadProcessId(win.HWND(wnd), &wndPid)
		if int(wndPid) != pid {
			itemVar.Clear()
			continue
		}

		p := getShellWindowPath(wDisp)
		candidates = append(candidates, shellWindowCandidate{index: i, hwnd: wnd, path: p})
		uniqueHwnds[wnd] = struct{}{}
		itemVar.Clear()
	}

	if len(candidates) == 0 {
		return false
	}

	// Prefer the foreground window if it belongs to our target PID.
	foreground := uintptr(win.GetForegroundWindow())
	var targetHwnd uintptr
	if foreground != 0 {
		if _, ok := uniqueHwnds[foreground]; ok {
			targetHwnd = foreground
		}
	}

	// Otherwise pick a visible, non-minimized window handle.
	if targetHwnd == 0 {
		for wnd := win.GetWindow(win.GetDesktopWindow(), win.GW_CHILD); wnd != 0; wnd = win.GetWindow(wnd, win.GW_HWNDNEXT) {
			if _, ok := uniqueHwnds[uintptr(wnd)]; ok {
				if win.IsWindowVisible(wnd) && !win.IsIconic(wnd) {
					targetHwnd = uintptr(wnd)
					break
				}
			}
		}
	}

	if targetHwnd == 0 {
		// Fallback to any candidate.
		targetHwnd = candidates[0].hwnd
	}

	cleanEqualFold := func(a, b string) bool {
		if a == "" || b == "" {
			return false
		}
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}

	bestIndex := -1
	// 1) Best: same hwnd + same folder path.
	for _, c := range candidates {
		if c.hwnd == targetHwnd && cleanEqualFold(c.path, targetDir) {
			bestIndex = c.index
			break
		}
	}
	// 2) Next: any tab with matching folder path.
	if bestIndex == -1 {
		for _, c := range candidates {
			if cleanEqualFold(c.path, targetDir) {
				bestIndex = c.index
				break
			}
		}
	}
	// 3) Next: same hwnd.
	if bestIndex == -1 {
		for _, c := range candidates {
			if c.hwnd == targetHwnd {
				bestIndex = c.index
				break
			}
		}
	}
	// 4) Final: first candidate.
	if bestIndex == -1 {
		bestIndex = candidates[0].index
	}

	itemVar, err := oleutil.CallMethod(windowsDisp, "Item", bestIndex)
	if err != nil {
		return false
	}
	wDisp := itemVar.ToIDispatch()
	if wDisp == nil {
		itemVar.Clear()
		return false
	}

	// Found the window/tab. Now select the file.
	documentVar, err := oleutil.GetProperty(wDisp, "Document")
	if err != nil {
		itemVar.Clear()
		return false
	}
	documentDisp := documentVar.ToIDispatch()
	if documentDisp == nil {
		documentVar.Clear()
		itemVar.Clear()
		return false
	}

	folderVar, err := oleutil.GetProperty(documentDisp, "Folder")
	if err != nil {
		documentVar.Clear()
		itemVar.Clear()
		return false
	}
	folderDisp := folderVar.ToIDispatch()
	if folderDisp == nil {
		folderVar.Clear()
		documentVar.Clear()
		itemVar.Clear()
		return false
	}

	fileName := filepath.Base(fullPath)
	parsedItemVar, err := oleutil.CallMethod(folderDisp, "ParseName", fileName)
	if err != nil {
		folderVar.Clear()
		documentVar.Clear()
		itemVar.Clear()
		return false
	}

	// SelectItem (1=Select, 4=Deselect others, 8=Ensure visible, 16=Focus)
	// We must pass the IDispatch of the Item, specifically.
	// parsedItemVar is a *VARIANT.
	// oleutil.CallMethod(documentDisp, "SelectItem", parsedItemVar.ToIDispatch(), 1|4|8|16)
	// However, ToIDispatch might not be enough if the variant type is not strictly dispatch, but usually it is.
	itemDisp := parsedItemVar.ToIDispatch()
	if itemDisp != nil {
		_, err = oleutil.CallMethod(documentDisp, "SelectItem", itemDisp, 1|4|8|16)
	} else {
		// fallback: try passing valid directly if it happens to be something else?
		// But for ParseName it should return FolderItem object.
		err = fmt.Errorf("ParseName returned null dispatch")
	}

	parsedItemVar.Clear()
	folderVar.Clear()
	documentVar.Clear()
	itemVar.Clear()

	return err == nil

}
