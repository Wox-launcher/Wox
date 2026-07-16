//go:build !windows && !darwin && !linux

package woxui

type platformWindow struct{}

func platformRun(start func() error) error {
	return ErrPlatformUnsupported
}

func platformCall(fn func()) error {
	return ErrPlatformUnsupported
}

func openPlatformWindow(options WindowOptions) (*platformWindow, error) {
	return nil, ErrPlatformUnsupported
}

func (w *platformWindow) show() (FocusEpoch, error) {
	return 0, ErrPlatformUnsupported
}

func (w *platformWindow) hide() error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) setBounds(bounds Rect) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) bounds() (Rect, error) {
	return Rect{}, ErrPlatformUnsupported
}

func (w *platformWindow) center(size Size) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) startDragging() error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) setHideOnBlur(enabled bool) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) setFontFamily(family string) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) pickFile(options FileDialogOptions) (string, error) {
	return "", ErrPlatformUnsupported
}

func (w *platformWindow) openExternalURL(rawURL string) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) showWebView(content WebViewContent, bounds Rect) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) hideWebView() error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) writeClipboardText(text string) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) writeClipboardImage(image *clipboardImage) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) invalidate() error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) setTextInputState(state TextInputState) error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) measureText(text string, style TextStyle) (TextMetrics, error) {
	return TextMetrics{}, ErrPlatformUnsupported
}

func (w *platformWindow) close() error {
	return ErrPlatformUnsupported
}
