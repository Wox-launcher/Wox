//go:build !windows && !darwin

package woxui

type platformWindow struct{}

func platformRun(start func() error) error {
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

func (w *platformWindow) invalidate() error {
	return ErrPlatformUnsupported
}

func (w *platformWindow) close() error {
	return ErrPlatformUnsupported
}
