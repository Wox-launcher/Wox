package window

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa -framework ApplicationServices -framework ScriptingBridge
#include <stdlib.h>

int getActiveWindowIcon(unsigned char **iconData);
char* getActiveWindowName();
char* getProcessBundleIdentifier(int pid);
int getActiveWindowPid();
int activateWindowByPid(int pid);
int isOpenSaveDialog();
int navigateActiveFileDialog(const char* path);
int isFinder(int pid);
char* getOpenFinderWindowPaths();
char* getActiveFinderWindowPath();
char* getFinderWindowPathByPid(int pid);
int selectInFinder(const char* path);
*/
import "C"
import (
	"bytes"
	"errors"
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

func GetActiveWindowName() string {
	name := C.getActiveWindowName()
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

func GetActiveWindowPid() int {
	pid := C.getActiveWindowPid()
	return int(pid)
}

func ActivateWindowByPid(pid int) bool {
	result := C.activateWindowByPid(C.int(pid))
	return int(result) == 1
}

func IsOpenSaveDialog() (bool, error) {
	return int(C.isOpenSaveDialog()) == 1, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	if targetPath == "" {
		return false
	}

	cPath := C.CString(targetPath)
	defer C.free(unsafe.Pointer(cPath))
	return int(C.navigateActiveFileDialog(cPath)) == 1
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
