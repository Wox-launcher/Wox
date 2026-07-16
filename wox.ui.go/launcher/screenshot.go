package launcher

import (
	"encoding/json"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

type captureScreenshotRequest struct {
	Output                string `json:"output"`
	ExportFilePath        string `json:"exportFilePath"`
	HideAnnotationToolbar bool   `json:"hideAnnotationToolbar"`
	AutoConfirm           bool   `json:"autoConfirm"`
}

type captureScreenshotResult struct {
	Status                  string      `json:"status"`
	ScreenshotPath          string      `json:"screenshotPath,omitempty"`
	LogicalSelectionRect    *woxui.Rect `json:"logicalSelectionRect,omitempty"`
	ClipboardWriteSucceeded bool        `json:"clipboardWriteSucceeded"`
	ClipboardWarningMessage string      `json:"clipboardWarningMessage,omitempty"`
	ErrorCode               string      `json:"errorCode,omitempty"`
	ErrorMessage            string      `json:"errorMessage,omitempty"`
}

// captureScreenshot hides the launcher before starting the native desktop capture session.
func (a *App) captureScreenshot(raw json.RawMessage) captureScreenshotResult {
	var request captureScreenshotRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return captureScreenshotResult{Status: "failed", ErrorCode: "invalid_request", ErrorMessage: err.Error()}
	}
	if err := a.hideWindow(true); err != nil {
		return captureScreenshotResult{Status: "failed", ErrorCode: "hide_launcher_failed", ErrorMessage: err.Error()}
	}
	result, err := woxui.CaptureScreenshot(woxui.ScreenshotOptions{
		ExportFilePath:        request.ExportFilePath,
		CopyToClipboard:       request.Output == "" || strings.EqualFold(request.Output, "clipboard"),
		HideAnnotationToolbar: request.HideAnnotationToolbar,
		AutoConfirm:           request.AutoConfirm,
	})
	if err != nil {
		return captureScreenshotResult{Status: "failed", ErrorCode: "capture_failed", ErrorMessage: err.Error()}
	}
	if result.Cancelled {
		return captureScreenshotResult{Status: "cancelled"}
	}
	selection := result.LogicalSelection
	return captureScreenshotResult{
		Status:                  "completed",
		ScreenshotPath:          result.ScreenshotPath,
		LogicalSelectionRect:    &selection,
		ClipboardWriteSucceeded: result.ClipboardWriteSucceeded,
		ClipboardWarningMessage: result.ClipboardWarningMessage,
	}
}
