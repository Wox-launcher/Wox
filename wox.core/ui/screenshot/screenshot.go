package screenshot

import woxui "wox/ui/runtime"

// ScreenshotOptions configures one interactive desktop-region capture.
type ScreenshotOptions struct {
	ExportFilePath        string
	CopyToClipboard       bool
	HideAnnotationToolbar bool
	AutoConfirm           bool
	WindowManager         *woxui.WindowManager
	AnnotationTooltips    ScreenshotAnnotationTooltips
	ActionTooltips        ScreenshotActionTooltips
}

// ScreenshotActionTooltips carries localized labels for screenshot-wide actions.
type ScreenshotActionTooltips struct {
	Undo             string
	ScrollingCapture string
	Cursor           string
	Pin              string
	Cancel           string
	Confirm          string
}

// ScreenshotAnnotationTooltips carries localized labels for the annotation creation tools.
type ScreenshotAnnotationTooltips struct {
	Rectangle string
	Ellipse   string
	Text      string
	Arrow     string
	Number    string
	Mosaic    string
}

const ScreenshotWindowID WindowID = "wox.screenshot"

// ScreenshotResult reports the exported image and its logical desktop selection.
type ScreenshotResult struct {
	Cancelled               bool
	CopiedColor             string
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
