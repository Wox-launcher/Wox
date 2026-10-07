package screenshot

import (
	"errors"
	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	"wox/util/screen"
)

var ErrScreenshotDisplayLayoutChanged = errors.New("screenshot display layout is missing or differs from the current desktop")

// ScreenshotOptions configures one interactive desktop-region capture.
type ScreenshotOptions struct {
	capturedDisplays      []screenshotDisplay
	SaveEditableScene     bool
	EditScreenshotPath    string
	ExportFilePath        string
	CopyToClipboard       bool
	HideAnnotationToolbar bool
	AutoConfirm           bool
	AllowVideoRecording   bool
	ExtraActions          []common.ScreenshotExtraAction
	RecordingDefaults     RecordingDefaults
	WindowManager         *woxui.WindowManager
	AnnotationTooltips    ScreenshotAnnotationTooltips
	ActionTooltips        ScreenshotActionTooltips
	RecordingTooltips     RecordingTooltips
	RecordingRuntime      RecordingRuntimeLabels
	SizeLabels            ScreenshotSizeLabels
	Theme                 woxcomponent.ControlTheme
	FontFamily            string
}

// ScreenshotSizeLabels carries localized copy for editing capture dimensions in pixels.
type ScreenshotSizeLabels struct {
	Title, Width, Height, Apply, Cancel, InvalidSize string
	LockAspectRatio, Swap                            string
}

// RecordingRuntimeLabels carries the consent, transfer, and recovery copy for optional FFmpeg installation.
type RecordingRuntimeLabels struct {
	Title, Description, Install, Cancel, Retry   string
	Downloading, Installing, Failed, Unsupported string
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
	Format         string
	FormatMP4      string
	FormatGIF      string
	FormatWebP     string
	ExportFailed   string
}

// ScreenshotActionTooltips carries localized labels for screenshot-wide actions.
type ScreenshotActionTooltips struct {
	Undo              string
	ScrollingCapture  string
	Cursor            string
	Background        string
	BackgroundLoading string
	BackgroundFailed  string
	Pin               string
	Record            string
	Cancel            string
	// Save labels the download control. SaveTitle is the native Save As dialog title.
	Save      string
	SaveTitle string
	Confirm   string
}

// ScreenshotAnnotationTooltips carries localized labels for the annotation creation tools.
type ScreenshotAnnotationTooltips struct {
	FontSize  string
	Rectangle string
	Ellipse   string
	Text      string
	Arrow     string
	Number    string
	Mosaic    string
	Brush     string
	Eraser    string
}

const ScreenshotWindowID WindowID = "wox.screenshot"

// ScreenshotResult reports the exported image and its logical desktop selection.
type ScreenshotResult struct {
	SaveEditableScene    func() error
	EditableSceneWarning string
	Cancelled            bool
	ArtifactKind         string
	ArtifactPath         string
	CopiedColor          string
	PinToScreen          bool
	// PinOverlayShown is true when the editor already opened the pinned window
	// from in-memory pixels. The plugin then skips a second file-backed overlay.
	PinOverlayShown         bool
	ScreenshotPath          string
	LogicalSelection        woxui.Rect
	ClipboardWriteSucceeded bool
	ClipboardWarningMessage string
	// ExtraActionID is set when a caller-owned toolbar button completed the capture.
	ExtraActionID string
}

// CaptureScreenshot runs the native desktop capture and Go-rendered selection surface.
func CaptureScreenshot(options ScreenshotOptions) (ScreenshotResult, error) {
	if options.EditScreenshotPath != "" {
		return editSavedScreenshot(options)
	}
	// Freeze monitor geometry for both selection fallback and editable scenes before the capture overlays appear.
	displays, err := screen.ListDisplays()
	if err == nil {
		options.capturedDisplays = screenshotDisplayLayout(displays)
	}
	return captureScreenshotPlatform(options)
}
