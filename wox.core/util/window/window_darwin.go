package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework ApplicationServices -framework ScriptingBridge
#include <stdlib.h>

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

int getActiveWindowIcon(unsigned char **iconData);
int getWindowIconByPid(int pid, unsigned char **iconData);
char* getActiveWindowName();
char* getWindowNameByPid(int pid);
char* getProcessBundleIdentifier(int pid);
int isProcessIdentityRunning(const char* identity);
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
int isOpenSaveDialog();
int isOpenSaveDialogByPid(int pid);
int navigateActiveFileDialog(const char* path);
int selectInActiveFileDialog(const char* path);
char* getActiveFileDialogPath();
char* getFileDialogPathByPid(int pid);
int isFinder(int pid);
char* getOpenFinderWindowPaths();
char* getActiveFinderWindowPath();
char* getFinderWindowPathByPid(int pid);
int selectInFinder(const char* path);
int navigateInFinder(const char* path);
*/
import "C"
import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
	"unsafe"
)

func GetActiveWindowIcon() (image.Image, error) {
	var iconData *C.uchar
	length := C.getActiveWindowIcon(&iconData)
	if length == 0 {
		return nil, errors.New("failed to get active window icon")
	}
	defer C.free(unsafe.Pointer(iconData))

	data := C.GoBytes(unsafe.Pointer(iconData), length)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return img, nil
}

// GetWindowIconByPid resolves the icon from the captured foreground PID instead
// of the current foreground app, which may already be Wox by the time the
// asynchronous launcher snapshot detail refresh runs.
func GetWindowIconByPid(pid int) (image.Image, error) {
	if pid <= 0 {
		return nil, errors.New("invalid pid")
	}

	var iconData *C.uchar
	length := C.getWindowIconByPid(C.int(pid), &iconData)
	if length == 0 {
		return nil, errors.New("failed to get window icon by pid")
	}
	defer C.free(unsafe.Pointer(iconData))

	data := C.GoBytes(unsafe.Pointer(iconData), length)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	return img, nil
}

func GetActiveWindowName() string {
	name := C.getActiveWindowName()
	if name == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(name))
	return C.GoString(name)
}

// GetWindowNameByPid mirrors GetActiveWindowName for a captured PID so delayed
// snapshot updates do not accidentally read Wox after the launcher appears.
func GetWindowNameByPid(pid int) string {
	if pid <= 0 {
		return ""
	}

	name := C.getWindowNameByPid(C.int(pid))
	if name == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(name))
	return C.GoString(name)
}

func GetProcessIdentity(pid int) string {
	if pid <= 0 {
		return ""
	}

	identity := C.getProcessBundleIdentifier(C.int(pid))
	if identity == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(identity))
	return strings.TrimSpace(C.GoString(identity))
}

// IsProcessIdentityRunning checks app process identity without touching Accessibility windows.
func IsProcessIdentityRunning(identity string) bool {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return false
	}

	cIdentity := C.CString(identity)
	defer C.free(unsafe.Pointer(cIdentity))
	return int(C.isProcessIdentityRunning(cIdentity)) == 1
}

func GetActiveWindowPid() int {
	pid := C.getActiveWindowPid()
	return int(pid)
}

// GetActiveWindowId returns the CGWindowID from the focused Accessibility window as a decimal string.
func GetActiveWindowId() string {
	windowId := C.getActiveWindowIdForManagement()
	if windowId == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(windowId))
	return C.GoString(windowId)
}

// GetManagedWindow resolves a captured Accessibility window and returns its current bounds.
func GetManagedWindow(windowId string, pid int, title string) (ManagedWindow, error) {
	cWindowId := C.CString(windowId)
	defer C.free(unsafe.Pointer(cWindowId))

	var out C.WoxManagedWindowC
	result := int(C.getManagedWindowForManagement(cWindowId, C.int(pid), &out))
	if result != 1 {
		return ManagedWindow{}, windowManagementErrorFromCode(result)
	}

	return managedWindowFromDarwinWindow(out, title), nil
}

// ListManagedWindows returns visible windows that macOS Accessibility can later move.
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
		windows = append(windows, managedWindowFromDarwinWindow(rawWindow, ""))
	}
	return windows, nil
}

// ListDisplays returns macOS screen bounds and visible frames in top-left desktop coordinates.
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
		displays = append(displays, displayInfoFromDarwinDisplay(rawDisplay))
	}
	SortDisplays(displays)
	return displays, nil
}

// MoveResizeWindow applies an Accessibility position and size to the target window.
func MoveResizeWindow(managedWindow ManagedWindow, rect WindowRect) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.moveResizeWindowForManagement(cWindowId, C.int(managedWindow.Pid), C.int(rect.X), C.int(rect.Y), C.int(max(1, rect.Width)), C.int(max(1, rect.Height))))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// MaximizeWindow triggers the native zoom control so macOS keeps window state in sync.
func MaximizeWindow(managedWindow ManagedWindow) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.maximizeWindowForManagement(cWindowId, C.int(managedWindow.Pid)))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// MinimizeWindow minimizes the captured Accessibility window.
func MinimizeWindow(managedWindow ManagedWindow) error {
	cWindowId := C.CString(managedWindow.Id)
	defer C.free(unsafe.Pointer(cWindowId))

	result := int(C.minimizeWindowForManagement(cWindowId, C.int(managedWindow.Pid)))
	if result != 1 {
		return windowManagementErrorFromCode(result)
	}
	return nil
}

// windowManagementErrorFromCode maps Objective-C bridge return codes to shared errors.
func windowManagementErrorFromCode(code int) error {
	switch code {
	case 0:
		return ErrWindowManagementWindowNotFound
	case -2:
		return ErrWindowManagementPermissionDenied
	case -3:
		return ErrWindowManagementDisplayNotFound
	case -4:
		return ErrWindowManagementUnsupported
	default:
		return fmt.Errorf("window management failed with code %d", code)
	}
}

// windowRectFromDarwinRect converts the Objective-C bridge rect into the shared Go type.
func windowRectFromDarwinRect(rect C.WoxWindowRectC) WindowRect {
	return WindowRect{
		X:      int(rect.x),
		Y:      int(rect.y),
		Width:  int(rect.width),
		Height: int(rect.height),
	}
}

// displayInfoFromDarwinDisplay converts NSScreen metrics into the shared Go type.
func displayInfoFromDarwinDisplay(display C.WoxDisplayInfoC) DisplayInfo {
	return DisplayInfo{
		Id:        C.GoString(&display.id[0]),
		Bounds:    windowRectFromDarwinRect(display.bounds),
		WorkArea:  windowRectFromDarwinRect(display.workArea),
		IsPrimary: int(display.isPrimary) == 1,
	}
}

// managedWindowFromDarwinWindow converts Accessibility data and resolves the app identity used by settings.
func managedWindowFromDarwinWindow(rawWindow C.WoxManagedWindowC, fallbackTitle string) ManagedWindow {
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
		Bounds:      windowRectFromDarwinRect(rawWindow.bounds),
		Display:     displayInfoFromDarwinDisplay(rawWindow.display),
		IsMinimized: int(rawWindow.isMinimized) == 1,
	}
}

func ActivateWindowByPid(pid int) bool {
	result := C.activateWindowByPid(C.int(pid))
	return int(result) == 1
}

func IsOpenSaveDialog() (bool, error) {
	return int(C.isOpenSaveDialog()) == 1, nil
}

// IsOpenSaveDialogByPid checks the captured process because the active app can
// change to Wox before the slow Accessibility dialog probe finishes.
func IsOpenSaveDialogByPid(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	return int(C.isOpenSaveDialogByPid(C.int(pid))) == 1, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.navigateActiveFileDialog(cPath)) == 1
}

// SelectInActiveFileDialog selects a file/folder item in the currently active
// open/save dialog list without entering/opening it.
func SelectInActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.selectInActiveFileDialog(cPath)) == 1
}

func HighlightInActiveFileDialog(targetPath string) bool {
	return SelectInActiveFileDialog(targetPath)
}

// GetActiveFileDialogPath returns the currently opened directory path in the
// active open/save dialog on macOS.
func GetActiveFileDialogPath() string {
	result := C.getActiveFileDialogPath()
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

// GetFileDialogPathByPid returns the currently opened directory path in an
// open/save dialog owned by the specified process.
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
	return ""
}

func GetLastFileDialogPathResolveDebug() string {
	return ""
}

// NavigateInFileExplorer navigates the active Finder window to targetPath.
func NavigateInFileExplorer(pid int, targetPath string, windowTitle string) bool {
	if pid <= 0 || targetPath == "" {
		return false
	}

	isFinder, _ := IsFileExplorer(pid)
	if !isFinder {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.navigateInFinder(cPath)) == 1
}

// IsFileExplorer checks if the given PID belongs to Finder.
func IsFileExplorer(pid int) (bool, error) {
	if C.isFinder(C.int(pid)) == 1 {
		return true, nil
	}
	return false, nil
}

// GetActiveFileExplorerPath returns the filesystem path of the currently active
// Finder window on macOS, or an empty string if the foreground window is not
// Finder or the path cannot be determined.
func GetActiveFileExplorerPath() string {
	result := C.getActiveFinderWindowPath()
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

// GetFileExplorerPathByPid returns a Finder window path for the given PID when possible.
func GetFileExplorerPathByPid(pid int) string {
	if pid <= 0 {
		return ""
	}
	result := C.getFinderWindowPathByPid(C.int(pid))
	if result == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(result))
	return strings.TrimSpace(C.GoString(result))
}

// GetFileExplorerPathByPidAndWindowTitle is a Windows-specific helper to handle Explorer tabs.
// On macOS, Finder does not have the same tab/HWND behavior, so we fall back to PID-based resolution.
func GetFileExplorerPathByPidAndWindowTitle(pid int, windowTitle string) string {
	return GetFileExplorerPathByPid(pid)
}

// GetOpenFinderWindowPaths returns a list of paths for all currently open Finder windows.
func GetOpenFinderWindowPaths() []string {
	result := C.getOpenFinderWindowPaths()
	if result == nil {
		return []string{}
	}
	defer C.free(unsafe.Pointer(result))

	raw := strings.TrimSpace(C.GoString(result))
	if raw == "" {
		return []string{}
	}
	return strings.Split(raw, "\n")
}

// SelectInFileExplorerByPid selects a file in a Finder window owned by pid.
func SelectInFileExplorerByPid(pid int, fullPath string) bool {
	if pid <= 0 || fullPath == "" {
		return false
	}

	cPath := C.CString(fullPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.selectInFinder(cPath)) == 1
}
