package screenshot

import woxui "wox/ui/runtime"

// ScreenshotOptions configures one interactive desktop-region capture.
type ScreenshotOptions struct {
	ExportFilePath        string
	CopyToClipboard       bool
	HideAnnotationToolbar bool
	AutoConfirm           bool
	WindowManager         *woxui.WindowManager
}

const ScreenshotWindowID WindowID = "wox.screenshot"

// ScreenshotResult reports the exported image and its logical desktop selection.
type ScreenshotResult struct {
	Cancelled               bool
	PinToScreen             bool
	ScreenshotPath          string
	LogicalSelection        woxui.Rect
	ClipboardWriteSucceeded bool
	ClipboardWarningMessage string
}

// CaptureScreenshot runs the native desktop capture and Go-rendered selection surface.
func CaptureScreenshot(options ScreenshotOptions) (ScreenshotResult, error) {
	return captureScreenshotPlatform(options)
}
