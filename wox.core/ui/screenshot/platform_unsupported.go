//go:build !windows && !darwin && !linux

package screenshot

func captureScreenshotPlatform(options ScreenshotOptions) (ScreenshotResult, error) {
	return ScreenshotResult{}, ErrPlatformUnsupported
}
