package woxui

import (
	"errors"
	"strings"
)

// ShowNativeFilePreview attaches a platform file preview handler for the latest lifecycle generation.
// Bounds and cornerRadius are logical client units matching the widget that reserved the rectangle.
func (w *Window) ShowNativeFilePreview(path string, bounds Rect, cornerRadius float32, generation uint64) error {
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
	return w.native.showNativeFilePreview(path, bounds, cornerRadius, generation)
}

// HideNativeFilePreview removes the platform file preview handler if this generation is still current.
func (w *Window) HideNativeFilePreview(generation uint64) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hideNativeFilePreview(generation)
}

// SetNativeFilePreviewOcclusion cuts a Wox-drawn overlay out of the preview, in logical client units.
// The rectangle is retained, so a preview created later is clipped without a second call, and an
// empty rectangle restores the full preview while keeping the rounded corners from ShowNativeFilePreview.
func (w *Window) SetNativeFilePreviewOcclusion(bounds Rect) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.setNativeFilePreviewOcclusion(bounds)
}
