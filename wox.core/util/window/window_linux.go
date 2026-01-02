//go:build !windows && !darwin

package window

import (
	"errors"
	"image"
)

func GetActiveWindowIcon() (image.Image, error) {
	return nil, errors.New("not implemented")
}

func GetActiveWindowName() string {
	return ""
}

func GetActiveWindowPid() int {
	return -1
}

func GetProcessIdentity(pid int) string {
	return ""
}

func ActivateWindowByPid(pid int) bool {
	return false
}

func IsOpenSaveDialog() (bool, error) {
	return false, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	return false
}

// GetActiveFileExplorerPath returns empty string on platforms other than Windows and macOS.
func GetActiveFileExplorerPath() string {
	return ""
}

// IsFileExplorer returns false on platforms other than Windows and macOS.
func IsFileExplorer(pid int) (bool, error) {
	return false, nil
}

// NavigateActiveFileExplorer is not supported on this platform.
func NavigateActiveFileExplorer(targetPath string) bool {
	return false
}

func GetOpenFinderWindowPaths() []string {
	return []string{}
}
