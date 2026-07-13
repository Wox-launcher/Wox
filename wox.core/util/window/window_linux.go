//go:build !windows && !darwin

package window

import (
	"errors"
	"image"
)

func GetActiveWindowIcon() (image.Image, error) {
	return nil, errors.New("not implemented")
}

// GetWindowIconByPid is a PID-based companion for asynchronous snapshot detail
// refreshes; Linux keeps the existing unsupported behavior.
func GetWindowIconByPid(pid int) (image.Image, error) {
	return nil, errors.New("not implemented")
}

func GetActiveWindowName() string {
	return ""
}

// GetWindowNameByPid is a PID-based companion for asynchronous snapshot detail
// refreshes; Linux keeps the existing unsupported behavior.
func GetWindowNameByPid(pid int) string {
	return ""
}

func GetActiveWindowPid() int {
	return -1
}

func GetActiveWindowId() string {
	return ""
}

// GetManagedWindow is not implemented on Linux yet.
func GetManagedWindow(windowId string, pid int, title string) (ManagedWindow, error) {
	return ManagedWindow{}, ErrWindowManagementUnsupported
}

// ListManagedWindows is not implemented on Linux yet.
func ListManagedWindows() ([]ManagedWindow, error) {
	return nil, ErrWindowManagementUnsupported
}

// ListDisplays is not implemented on Linux yet.
func ListDisplays() ([]DisplayInfo, error) {
	return nil, ErrWindowManagementUnsupported
}

// MoveResizeWindow is not implemented on Linux yet.
func MoveResizeWindow(managedWindow ManagedWindow, rect WindowRect) error {
	return ErrWindowManagementUnsupported
}

// MaximizeWindow is not implemented on Linux yet.
func MaximizeWindow(managedWindow ManagedWindow) error {
	return ErrWindowManagementUnsupported
}

// MinimizeWindow is not implemented on Linux yet.
func MinimizeWindow(managedWindow ManagedWindow) error {
	return ErrWindowManagementUnsupported
}

func GetProcessIdentity(pid int) string {
	return ""
}

// IsProcessIdentityRunning is not implemented on Linux yet.
func IsProcessIdentityRunning(identity string) bool {
	return false
}

func ActivateWindowByPid(pid int) bool {
	return false
}

// ActivateWindow is not implemented on Linux yet.
func ActivateWindow(managedWindow ManagedWindow) bool {
	return false
}

func IsOpenSaveDialog() (bool, error) {
	return false, nil
}

// IsOpenSaveDialogByPid is a PID-based companion for asynchronous snapshot
// detail refreshes; Linux keeps the existing unsupported behavior.
func IsOpenSaveDialogByPid(pid int) (bool, error) {
	return false, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	return false
}

// NavigateFileDialog is not supported on Linux yet.
func NavigateFileDialog(windowId string, pid int, targetPath string) bool {
	return false
}

func SelectInActiveFileDialog(targetPath string) bool {
	return false
}

// SelectInFileDialog is not supported on Linux yet.
func SelectInFileDialog(windowId string, pid int, targetPath string) bool {
	return false
}

func HighlightInActiveFileDialog(targetPath string) bool {
	return false
}

// HighlightInFileDialog is not supported on Linux yet.
func HighlightInFileDialog(windowId string, pid int, targetPath string) bool {
	return false
}

func GetActiveFileDialogPath() string {
	return ""
}

func GetFileDialogPathByPid(pid int) string {
	return ""
}

func GetFileDialogPathByWindowId(windowId string, pid int) string {
	return ""
}

func GetLastFileDialogPathResolveDebug() string {
	return ""
}

func NavigateInFileExplorer(pid int, targetPath string, windowTitle string, windowId string) bool {
	return false
}

// GetActiveFileExplorerPath returns empty string on platforms other than Windows and macOS.
func GetActiveFileExplorerPath() string {
	return ""
}

// GetFileExplorerPathByPid returns empty string on platforms other than Windows and macOS.
func GetFileExplorerPathByPid(pid int) string {
	return ""
}

// GetFileExplorerPathByPidAndWindowTitle is a Windows-specific helper to handle Explorer tabs.
// Not supported on this platform.
func GetFileExplorerPathByPidAndWindowTitle(pid int, windowTitle string) string {
	return ""
}

// IsFileExplorer returns false on platforms other than Windows and macOS.
func IsFileExplorer(pid int) (bool, error) {
	return false, nil
}

// SelectInFileExplorer is not supported on this platform.
func SelectInFileExplorer(pid int, fullPath string, windowTitle string, windowId string) bool {
	return false
}
