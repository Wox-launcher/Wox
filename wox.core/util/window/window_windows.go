//go:build windows

package window

/*
#cgo LDFLAGS: -lpsapi -lgdi32 -luser32 -lshell32 -lole32 -loleaut32 -luiautomationcore
#include <windows.h>
#include <psapi.h>
#include <shellapi.h>
#include <stdint.h>

typedef struct {
	int x;
	int y;
	int width;
	int height;
} WoxWindowRectC;

typedef struct {
	char id[64];
	WoxWindowRectC bounds;
	WoxWindowRectC workArea;
	int isPrimary;
} WoxDisplayInfoC;

typedef struct {
	char id[64];
	int pid;
	char title[1024];
	WoxWindowRectC bounds;
	WoxDisplayInfoC display;
	int isMinimized;
} WoxManagedWindowC;

char* getActiveWindowIcon(unsigned char **iconData, int *iconSize, int *width, int *height);
char* getWindowIconByPid(int pid, unsigned char **iconData, int *iconSize, int *width, int *height);
char* getActiveWindowName();
char* getWindowNameByPid(int pid);
int getActiveWindowPid();
char* getActiveWindowIdForManagement();
int getManagedWindowForManagement(const char* windowId, int pid, WoxManagedWindowC* outWindow);
int listManagedWindowsForManagement(WoxManagedWindowC** outWindows, int* outCount);
void freeManagedWindowsForManagement(WoxManagedWindowC* windows);
int listDisplaysForManagement(WoxDisplayInfoC** outDisplays, int* outCount);
void freeDisplaysForManagement(WoxDisplayInfoC* displays);
int moveResizeWindowForManagement(const char* windowId, int pid, int x, int y, int width, int height);
int maximizeWindowForManagement(const char* windowId, int pid);
int minimizeWindowForManagement(const char* windowId, int pid);
int activateWindowByPid(int pid);
int focusFileExplorerContentByHwnd(uintptr_t hwnd);
int isOpenSaveDialog();
int isOpenSaveDialogByPid(int pid);
int navigateActiveFileDialog(const char* path);
int selectInActiveFileDialog(const char* path);
int highlightInActiveFileDialog(const char* path);
char* getActiveFileDialogPath();
char* getFileDialogPathByWindowId(const char* windowId, int pid);
char* getFileDialogPathByPid(int pid);
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

const windowManagementWin32ErrorOffset = 1000

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

// GetWindowIconByPid resolves the icon from the captured foreground PID instead
// of the current foreground window, which may already be Wox when the snapshot
// detail refresh runs in the background.
func GetWindowIconByPid(pid int) (image.Image, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid")
	}

	var iconData *C.uchar
	var iconSize C.int
	var width, height C.int

	errMsgC := C.getWindowIconByPid(C.int(pid), &iconData, &iconSize, &width, &height)
	if errMsgC != nil {
		errMsg := C.GoString(errMsgC)
		return nil, fmt.Errorf("failed to get window icon by pid: %s", errMsg)
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

// GetWindowNameByPid finds a visible top-level window for the captured process
// so delayed snapshot updates do not depend on the current foreground window.
func GetWindowNameByPid(pid int) string {
	if pid <= 0 {
		return ""
	}

	cStr := C.getWindowNameByPid(C.int(pid))
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

// GetActiveWindowId returns the foreground top-level HWND as a decimal string.
func GetActiveWindowId() string {
	windowId := C.getActiveWindowIdForManagement()
	if windowId == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(windowId))
	return C.GoString(windowId)
}

// GetManagedWindow resolves a captured top-level HWND and returns its current bounds.
func GetManagedWindow(windowId string, pid int, title string) (ManagedWindow, error) {
	cWindowId := C.CString(windowId)
	defer C.free(unsafe.Pointer(cWindowId))

	var out C.WoxManagedWindowC
	result := int(C.getManagedWindowForManagement(cWindowId, C.int(pid), &out))
	if result != 1 {
		return ManagedWindow{}, windowManagementErrorFromCode(result)
	}

	return managedWindowFromWindowsWindow(out, title), nil
}

// ListManagedWindows returns visible top-level windows in native z-order for app-based layouts.
func ListManagedWindows() ([]ManagedWindow, error) {
	var outWindows *C.WoxManagedWindowC
	var outCount C.int
	result := int(C.listManagedWindowsForManagement(&outWindows, &outCount))
	if result != 1 {
		return nil, windowManagementErrorFromCode(result)
	}
	defer C.freeManagedWindowsForManagement(outWindows)

	count := int(outCount)
	if count == 0 {
		return []ManagedWindow{}, nil
	}

	rawWindows := unsafe.Slice(outWindows, count)
	windows := make([]ManagedWindow, 0, count)
	for _, rawWindow := range rawWindows {
		windows = append(windows, managedWindowFromWindowsWindow(rawWindow, ""))
	}
	return windows, nil
}

// ListDisplays returns monitor bounds and work areas in desktop coordinates.
func ListDisplays() ([]DisplayInfo, error) {
	var outDisplays *C.WoxDisplayInfoC
	var outCount C.int
	result := int(C.listDisplaysForManagement(&outDisplays, &outCount))
	if result != 1 {
		return nil, windowManagementErrorFromCode(result)
	}
	defer C.freeDisplaysForManagement(outDisplays)

	count := int(outCount)
	if count == 0 {
		return nil, ErrWindowManagementDisplayNotFound
	}

	rawDisplays := unsafe.Slice(outDisplays, count)
	displays := make([]DisplayInfo, 0, count)
	for _, rawDisplay := range rawDisplays {
		displays = append(displays, displayInfoFromWindowsDisplay(rawDisplay))
	}
	SortDisplays(displays)
	return displays, nil
}

// MoveResizeWindow restores maximized/minimized windows before applying the target frame.
func MoveResizeWindow(managedWindow ManagedWindow, rect WindowRect) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.moveResizeWindowForManagement(cWindowId, C.int(managedWindow.Pid), C.int(rect.X), C.int(rect.Y), C.int(max(1, rect.Width)), C.int(max(1, rect.Height))))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// MaximizeWindow uses the native maximize state so Windows updates caption button behavior.
func MaximizeWindow(managedWindow ManagedWindow) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.maximizeWindowForManagement(cWindowId, C.int(managedWindow.Pid)))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// MinimizeWindow minimizes the captured top-level HWND.
func MinimizeWindow(managedWindow ManagedWindow) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.minimizeWindowForManagement(cWindowId, C.int(managedWindow.Pid)))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// windowManagementErrorFromCode maps native return codes to shared errors.
func windowManagementErrorFromCode(code int) error {
	switch code {
	case 0:
		return ErrWindowManagementWindowNotFound
	case -2:
		return ErrWindowManagementPermissionDenied
	case -3:
		return ErrWindowManagementDisplayNotFound
	default:
		if code < -windowManagementWin32ErrorOffset {
			win32Error := -code - windowManagementWin32ErrorOffset
			if win32Error == 5 {
				return fmt.Errorf("%w: win32 error %d", ErrWindowManagementPermissionDenied, win32Error)
			}
			return fmt.Errorf("window management failed with code %d (win32 error %d)", code, win32Error)
		}
		return fmt.Errorf("window management failed with code %d", code)
	}
}

// windowRectFromWindowsRect converts the CGO rect into the shared Go type.
func windowRectFromWindowsRect(rect C.WoxWindowRectC) WindowRect {
	return WindowRect{
		X:      int(rect.x),
		Y:      int(rect.y),
		Width:  int(rect.width),
		Height: int(rect.height),
	}
}

// displayInfoFromWindowsDisplay converts Win32 monitor metrics into the shared Go type.
func displayInfoFromWindowsDisplay(display C.WoxDisplayInfoC) DisplayInfo {
	return DisplayInfo{
		Id:        C.GoString(&display.id[0]),
		Bounds:    windowRectFromWindowsRect(display.bounds),
		WorkArea:  windowRectFromWindowsRect(display.workArea),
		IsPrimary: int(display.isPrimary) == 1,
	}
}

// managedWindowFromWindowsWindow converts native window data and normalizes process identity.
func managedWindowFromWindowsWindow(rawWindow C.WoxManagedWindowC, fallbackTitle string) ManagedWindow {
	pid := int(rawWindow.pid)
	title := strings.TrimSpace(fallbackTitle)
	if title == "" {
		title = C.GoString(&rawWindow.title[0])
	}

	return ManagedWindow{
		Id:          C.GoString(&rawWindow.id[0]),
		Pid:         pid,
		Title:       title,
		AppIdentity: strings.TrimSpace(GetProcessIdentity(pid)),
		Bounds:      windowRectFromWindowsRect(rawWindow.bounds),
		Display:     displayInfoFromWindowsDisplay(rawWindow.display),
		IsMinimized: int(rawWindow.isMinimized) == 1,
	}
}

func ActivateWindowByPid(pid int) bool {
	result := C.activateWindowByPid(C.int(pid))
	return int(result) == 1
}

func IsOpenSaveDialog() (bool, error) {
	result := C.isOpenSaveDialog()
	return int(result) == 1, nil
}

// IsOpenSaveDialogByPid checks dialog windows owned by the captured process
// because the foreground window may change before the slow detail refresh runs.
func IsOpenSaveDialogByPid(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	result := C.isOpenSaveDialogByPid(C.int(pid))
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

func SelectInActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.selectInActiveFileDialog(cPath)) == 1
}

func HighlightInActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.highlightInActiveFileDialog(cPath)) == 1
}

func GetActiveFileDialogPath() string {
	result := C.getActiveFileDialogPath()
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

func GetFileDialogPathByPid(pid int) string {
	if pid <= 0 {
		return ""
	}
	result := C.getFileDialogPathByPid(C.int(pid))
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

func GetFileDialogPathByWindowId(windowId string, pid int) string {
	windowId = strings.TrimSpace(windowId)
	if windowId == "" {
		return ""
	}
	cWindowId := C.CString(windowId)
	defer C.free(unsafe.Pointer(cWindowId))
	result := C.getFileDialogPathByWindowId(cWindowId, C.int(pid))
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

type explorerShellWindowCandidate struct {
	index        int
	hwnd         uintptr
	path         string
	locationName string
	z            int
}

func getExplorerWindowZOrder() map[uintptr]int {
	zOrder := map[uintptr]int{}
	idx := 0
	for wnd := win.GetWindow(win.GetDesktopWindow(), win.GW_CHILD); wnd != 0; wnd = win.GetWindow(wnd, win.GW_HWNDNEXT) {
		zOrder[uintptr(wnd)] = idx
		idx++
	}
	return zOrder
}

func scoreExplorerShellWindowCandidate(candidate explorerShellWindowCandidate, windowTitle string) int {
	score := 0

	titleLower := strings.ToLower(strings.TrimSpace(windowTitle))
	loc := strings.TrimSpace(candidate.locationName)
	if loc == "" {
		loc = filepath.Base(candidate.path)
	}
	locLower := strings.ToLower(loc)
	if titleLower != "" && locLower != "" {
		if titleLower == locLower {
			score += 100
		} else if strings.Contains(titleLower, locLower) || strings.Contains(locLower, titleLower) {
			score += 50
		}
	}

	if candidate.z < (1 << 30) {
		score += 10
	}

	return score
}

func selectBestExplorerShellWindowCandidate(candidates []explorerShellWindowCandidate, preferredHwnd uintptr, windowTitle string) int {
	if len(candidates) == 0 {
		return -1
	}

	bestIdx := 0
	bestScore := -1
	for i, candidate := range candidates {
		score := scoreExplorerShellWindowCandidate(candidate, windowTitle)
		if preferredHwnd != 0 && candidate.hwnd == preferredHwnd {
			score += 1000
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
			continue
		}

		if score == bestScore && candidate.z < candidates[bestIdx].z {
			bestIdx = i
		}
	}

	return bestIdx
}

// NavigateInFileExplorer navigates the active Explorer tab/window owned by pid to targetPath
// and restores keyboard focus to the file list so Explorer type-to-search can continue.
// Windows 11 Explorer tabs can share one top-level HWND while ShellWindows still exposes
// one automation entry per tab. Navigating by pid/HWND alone can therefore hit the first
// tab entry instead of the focused tab. We rank ShellWindows candidates with the active
// window title and z-order before calling Navigate so type-to-search stays on the current tab.
func NavigateInFileExplorer(pid int, targetPath string, windowTitle string) bool {
	if pid <= 0 || targetPath == "" {
		return false
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

	getShellWindowLocationName := func(wDisp *ole.IDispatch) string {
		v, err := oleutil.GetProperty(wDisp, "LocationName")
		if err != nil {
			return ""
		}
		name := strings.TrimSpace(v.ToString())
		v.Clear()
		return name
	}

	candidates := make([]explorerShellWindowCandidate, 0, 4)
	uniqueHwnds := map[uintptr]struct{}{}
	zOrder := getExplorerWindowZOrder()

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

		z := 1 << 30
		if v, ok := zOrder[wnd]; ok {
			z = v
		}
		candidates = append(candidates, explorerShellWindowCandidate{
			index:        i,
			hwnd:         wnd,
			locationName: getShellWindowLocationName(wDisp),
			z:            z,
		})
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

	bestCandidateIdx := selectBestExplorerShellWindowCandidate(candidates, targetHwnd, windowTitle)
	if bestCandidateIdx < 0 {
		return false
	}
	bestIndex := candidates[bestCandidateIdx].index

	itemVar, err := oleutil.CallMethod(windowsDisp, "Item", bestIndex)
	if err != nil {
		return false
	}
	wDisp := itemVar.ToIDispatch()
	if wDisp == nil {
		itemVar.Clear()
		return false
	}

	_, err = oleutil.CallMethod(wDisp, "Navigate", targetPath)
	if err == nil {
		C.focusFileExplorerContentByHwnd(C.uintptr_t(targetHwnd))
	}
	itemVar.Clear()

	return err == nil
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

	foreground := uintptr(win.GetForegroundWindow())
	zOrder := getExplorerWindowZOrder()
	candidates := make([]explorerShellWindowCandidate, 0, 8)
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
		candidates = append(candidates, explorerShellWindowCandidate{
			index:        i,
			hwnd:         hwnd,
			path:         p,
			locationName: loc,
			z:            z,
		})
		itemVar.Clear()
	}

	if len(candidates) == 0 {
		return ""
	}

	bestIdx := selectBestExplorerShellWindowCandidate(candidates, foreground, windowTitle)
	if bestIdx < 0 {
		return ""
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
