//go:build darwin

package woxui

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	return ScreenshotResult{}, ErrPlatformUnsupported
}
