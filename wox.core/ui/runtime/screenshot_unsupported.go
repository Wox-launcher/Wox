//go:build !windows && !darwin && !linux

package woxui

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	return ScreenshotResult{}, ErrPlatformUnsupported
}
