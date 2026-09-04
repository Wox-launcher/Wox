//go:build !windows && !darwin && !linux

package woxui

// setNativeFilePreviewOcclusion succeeds as a no-op instead of reporting ErrPlatformUnsupported.
// The launcher re-sends the overlay whenever its layout changes, so a platform without a native
// preview would otherwise turn ordinary layout changes into a recurring error.
func (w *platformWindow) setNativeFilePreviewOcclusion(bounds Rect) error {
	return nil
}

func (w *platformWindow) showNativeFilePreview(path string, bounds Rect, cornerRadius float32, generation uint64) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) hideNativeFilePreview(generation uint64) error {
	return ErrPlatformUnsupported
}
