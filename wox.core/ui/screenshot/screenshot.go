package screenshot

import woxui "wox/ui/runtime"

// ScreenshotOptions configures one interactive desktop-region capture.
type ScreenshotOptions struct {
	ExportFilePath        string
	CopyToClipboard       bool
	HideAnnotationToolbar bool
	AutoConfirm           bool
	AllowVideoRecording   bool
	RecordingDefaults     RecordingDefaults
	WindowManager         *woxui.WindowManager
	AnnotationTooltips    ScreenshotAnnotationTooltips
	ActionTooltips        ScreenshotActionTooltips
	RecordingTooltips     RecordingTooltips
}

// RecordingDefaults configures the options shown before the countdown begins.
type RecordingDefaults struct {
	FPS          int
	ShowPointer  bool
	ShowKeypress bool
}

// RecordingTooltips carries localized labels for recording controls and privacy guidance.
type RecordingTooltips struct {
	Enter          string
	Start          string
	Pause          string
	Resume         string
	Restart        string
	ShowPointer    string
	ShowKeypress   string
	Finish         string
	Save           string
	Play           string
	Cancel         string
	PrivacyWarning string
}

// ScreenshotActionTooltips carries localized labels for screenshot-wide actions.
type ScreenshotActionTooltips struct {
	Undo             string
	ScrollingCapture string
	Cursor           string
	Pin              string
	Record           string
	Cancel           string
	// Save labels the download control. SaveTitle is the native Save As dialog title.
	Save      string
	SaveTitle string
	Confirm   string
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
	ArtifactKind            string
	ArtifactPath            string
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
