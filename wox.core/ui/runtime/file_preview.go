package woxui

import (
	"errors"
	"strings"
)

// ShowNativeFilePreview attaches a platform file preview handler to the given client rectangle.
func (w *Window) ShowNativeFilePreview(path string, bounds Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("native file preview path is empty")
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("native file preview bounds must have a positive size")
	}
	return w.native.showNativeFilePreview(path, bounds)
}

// HideNativeFilePreview removes the platform file preview handler from the visible client area.
func (w *Window) HideNativeFilePreview() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hideNativeFilePreview()
}
