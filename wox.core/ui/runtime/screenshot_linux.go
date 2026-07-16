//go:build linux

package woxui

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	return ScreenshotResult{}, ErrPlatformUnsupported
}
