package common

import (
	"context"
	"wox/util/selection"
)

type ShowSource string

const (
	ShowSourceDefault     ShowSource = "default"
	ShowSourceQueryHotkey ShowSource = "query_hotkey"
	ShowSourceSelection   ShowSource = "selection"
	ShowSourceTrayQuery   ShowSource = "tray_query"
	ShowSourceExplorer    ShowSource = "explorer"
)

type PlainQuery struct {
	QueryId        string
	QueryType      string // see plugin.QueryType
	QueryText      string
	QuerySelection selection.Selection
	// QueryRefinements carries selected secondary controls from the UI. The
	// public transport stays a simple string map; multi-select refinements use a
	// comma-separated value that plugins can decode with shared helpers.
	QueryRefinements map[string]string
}

var DefaultSettingWindowContext = SettingWindowContext{Path: "/"}

type SettingWindowSource string

const (
	// SettingWindowSourceDefault keeps the setting page tied to the launcher flow.
	// It is the safe default for query-result actions and plugin-initiated opens.
	SettingWindowSourceDefault SettingWindowSource = "default"
	// SettingWindowSourceTray marks settings opened from the tray/menu-bar menu,
	// where Escape should close the management window instead of restoring query UI.
	SettingWindowSourceTray SettingWindowSource = "tray"
)

// SettingWindowContext carries both the target settings page and the opener source.
// The source is explicit because native visibility is not a reliable proxy for
// whether settings came from the launcher or from the tray/menu-bar entry.
type SettingWindowContext struct {
	Path   string
	Param  string
	Source SettingWindowSource
}

func (c PlainQuery) IsEmpty() bool {
	return c.QueryText == "" && c.QuerySelection.String() == ""
}

func (c PlainQuery) String() string {
	if c.QueryText != "" {
		return c.QueryText
	}

	return c.QuerySelection.String()
}

// ui methods that can be invoked by plugins
// because the golang recycle dependency issue, we can't use UI interface directly from plugin, so we need to define a new interface here
type UI interface {
	ChangeQuery(ctx context.Context, query PlainQuery)
	RefreshQuery(ctx context.Context, preserveSelectedIndex bool)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context, showContext ShowContext)
	ToggleApp(ctx context.Context, showContext ShowContext)
	OpenSettingWindow(ctx context.Context, windowContext SettingWindowContext)
	OpenOnboardingWindow(ctx context.Context)
	PickFiles(ctx context.Context, params PickFilesParams) []string
	CaptureScreenshot(ctx context.Context, request CaptureScreenshotRequest) (CaptureScreenshotResult, error)
	GetActiveWindowSnapshot(ctx context.Context) ActiveWindowSnapshot
	GetServerPort(ctx context.Context) int
	GetAllThemes(ctx context.Context) []Theme
	ChangeTheme(ctx context.Context, theme Theme)
	InstallTheme(ctx context.Context, theme Theme)
	UninstallTheme(ctx context.Context, theme Theme)
	RestoreTheme(ctx context.Context)
	Notify(ctx context.Context, msg NotifyMsg)
	ShowToolbarMsg(ctx context.Context, msg interface{})
	ClearToolbarMsg(ctx context.Context, toolbarMsgId string)
	UpdateDiagnosticStatus(ctx context.Context, enabled bool)
	// UpdateResult updates a result that is currently displayed in the UI.
	// Returns true if the result was successfully updated (still visible in UI).
	// Returns false if the result is no longer visible (caller should stop updating).
	// The result parameter should be plugin.UpdatableResult, but we use interface{} to avoid circular dependency.
	UpdateResult(ctx context.Context, result interface{}) bool
	// PushResults pushes additional results for the current query.
	// Returns true if results were accepted by UI, false if query is no longer active.
	// The payload should be plugin.PushResultsPayload, but we use interface{} to avoid circular dependency.
	PushResults(ctx context.Context, payload interface{}) bool
	// IsVisible returns true if the Wox window is currently visible
	IsVisible(ctx context.Context) bool

	// AI chat plugin related methods
	FocusToChatInput(ctx context.Context)
	SendChatResponse(ctx context.Context, chatData AIChatData)
	ReloadChatResources(ctx context.Context, resouceName string)

	// ReloadSettingPlugins asks the UI to refresh plugin lists.
	ReloadSettingPlugins(ctx context.Context)

	// ReloadSetting asks the UI to reload Wox settings from backend.
	ReloadSetting(ctx context.Context)

	// RefreshGlance asks the UI to pull the latest Global Glance items. The
	// backend sends ids only; UI still applies user slot settings before rendering.
	RefreshGlance(ctx context.Context, pluginId string, ids []string)
}

type ActiveWindowSnapshot struct {
	Name             string   // active window name before wox is activated
	Pid              int      // active window pid before wox is activated
	Icon             WoxImage // active window icon before wox is activated
	IsOpenSaveDialog bool     // is active window open/save dialog before wox is activated
}

type ShowContext struct {
	SelectAll        bool
	IsQueryFocus     bool // auto focus chat input on next ui update
	HideQueryBox     bool
	HideToolbar      bool
	QueryBoxAtBottom bool
	HideOnBlur       bool
	ShowSource       ShowSource
	// ActivationStartedAt carries the original hotkey callback timestamp to the
	// UI. Go receives the websocket acknowledgement before Flutter actually
	// shows the native window, so visibility diagnostics must finish in Flutter.
	ActivationStartedAt int64

	WindowPosition *WindowPosition
	TrayAnchor     *TrayAnchor
	WindowWidth    int
	MaxResultCount int
}

type WindowPosition struct {
	X int
	Y int
}

type TrayAnchor struct {
	WindowX    int
	Bottom     int
	ScreenRect WindowRect
}

type WindowRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type PickFilesParams struct {
	IsDirectory bool
}

type CaptureScreenshotStatus string

const (
	CaptureScreenshotStatusCompleted CaptureScreenshotStatus = "completed"
	CaptureScreenshotStatusCancelled CaptureScreenshotStatus = "cancelled"
	CaptureScreenshotStatusFailed    CaptureScreenshotStatus = "failed"
)

// ScreenshotRect keeps screenshot geometry in logical desktop coordinates.
// The Flutter workspace uses this rect shape for selection, display bounds, and export metadata
// so Go can forward one stable contract without depending on platform-specific geometry types.
type ScreenshotRect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// CaptureScreenshotRequest defines the single v1 screenshot workflow that the system plugin supports.
// We keep the request explicit instead of inferring defaults in Flutter so both layers stay aligned
// when tests trigger the flow directly through the UI bridge.
type CaptureScreenshotRequest struct {
	SessionId      string   `json:"sessionId"`
	Trigger        string   `json:"trigger"`
	Scope          string   `json:"scope"`
	Output         string   `json:"output"`
	Tools          []string `json:"tools"`
	ExportFilePath string   `json:"exportFilePath"`
	// HideAnnotationToolbar is an API-facing simplification for plugins that only need a selected
	// image region. The previous toolbar always exposed markup controls, which slowed down raw OCR
	// style workflows and implied annotation support the caller would ignore.
	HideAnnotationToolbar bool `json:"hideAnnotationToolbar"`
	// AutoConfirm lets capture-only plugins finish on mouse-up after a valid rectangle is selected.
	// It reuses the normal confirm/export path so file output, clipboard behavior, and cleanup stay
	// identical to an explicit confirm button press.
	AutoConfirm bool `json:"autoConfirm"`
	// CallerIcon is set only by plugin-originated screenshot API calls. The previous request did not
	// carry caller identity, so Flutter could not visually distinguish a third-party capture from the
	// built-in Wox screenshot flow; passing the already-resolved icon keeps that decision in Go.
	CallerIcon *WoxImage `json:"callerIcon,omitempty"`
}

// DisplaySnapshot describes one native capture surface that Flutter can render and crop from.
// The platform bridge must provide both image bytes and geometry from the same native source
// so mixed-DPI export does not drift because of mismatched coordinate systems.
type DisplaySnapshot struct {
	DisplayId        string         `json:"displayId"`
	LogicalBounds    ScreenshotRect `json:"logicalBounds"`
	PixelBounds      ScreenshotRect `json:"pixelBounds"`
	Scale            float64        `json:"scale"`
	Rotation         int            `json:"rotation"`
	ImageBytesBase64 string         `json:"imageBytesBase64"`
}

// CaptureScreenshotResult carries the exported screenshot file back to Go.
// The previous websocket contract base64-wrapped full PNG payloads, which added avoidable transport
// cost on every completed screenshot. Returning the exported file path keeps annotation state in the
// UI while still giving Go the saved artifact path and clipboard warning state without re-sending
// the PNG bytes over the websocket.
type CaptureScreenshotResult struct {
	Status               CaptureScreenshotStatus `json:"status"`
	ScreenshotPath       string                  `json:"screenshotPath,omitempty"`
	LogicalSelectionRect *ScreenshotRect         `json:"logicalSelectionRect,omitempty"`
	// PinToScreen tells the Go screenshot plugin that Flutter exported the image for a pinned
	// desktop overlay. The previous completed result only described file/clipboard output, so Go had
	// no way to distinguish a normal confirmation from a toolbar pin action.
	PinToScreen bool `json:"pinToScreen,omitempty"`
	// ClipboardWriteSucceeded stays explicit instead of overloading Status so export-success plus
	// clipboard-failure can still return a completed screenshot together with a warning.
	ClipboardWriteSucceeded bool   `json:"clipboardWriteSucceeded"`
	ClipboardWarningMessage string `json:"clipboardWarningMessage,omitempty"`
	ErrorCode               string `json:"errorCode,omitempty"`
	ErrorMessage            string `json:"errorMessage,omitempty"`
}

func DefaultCaptureScreenshotRequest() CaptureScreenshotRequest {
	return CaptureScreenshotRequest{
		Trigger: "plugin",
		Scope:   "all_displays",
		Output:  "clipboard",
		Tools:   []string{"rect", "ellipse", "arrow", "text"},
	}
}

type NotifyMsg struct {
	PluginId       string // can be empty
	Icon           string // WoxImage.String(), can be empty
	Text           string // can be empty
	DisplaySeconds int    // 0 means display forever
}
