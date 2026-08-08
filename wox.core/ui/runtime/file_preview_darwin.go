//go:build darwin

package woxui

func (w *platformWindow) showNativeFilePreview(path string, bounds Rect, generation uint64) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) hideNativeFilePreview(generation uint64) error {
	return ErrPlatformUnsupported
}
