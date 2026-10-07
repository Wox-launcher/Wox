//go:build windows

package woxui

import (
	window "wox/ui/runtime/internal/window"
)

// WindowsLogicalRectFromPhysical converts a pixel rectangle using the DPI of its dominant monitor.
func WindowsLogicalRectFromPhysical(bounds Rect) Rect {
	return window.WindowsLogicalRectFromPhysical(bounds)
}

// WindowsPhysicalRectScale returns the effective DPI scale of a physical rectangle's dominant monitor.
func WindowsPhysicalRectScale(bounds Rect) float32 { return window.WindowsPhysicalRectScale(bounds) }

// SetNativeFileDialogListener observes Windows picker lifetime. The listener must
// return promptly: notifications run inside the native modal UI callback.
func SetNativeFileDialogListener(listener func(windowID uintptr, opened bool)) {
	window.SetNativeFileDialogListener(listener)
}

// NavigateNativeFileDialog changes the folder on the owning COM/UI thread.
// A stale HWND fails without falling back to a confirmation keystroke.
func NavigateNativeFileDialog(windowID uintptr, path string) error {
	return window.NavigateNativeFileDialog(windowID, path)
}
