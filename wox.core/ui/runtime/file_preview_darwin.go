//go:build darwin

package woxui

func (w *platformWindow) showNativeFilePreview(path string, bounds Rect) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) hideNativeFilePreview() error {
	return ErrPlatformUnsupported
}
