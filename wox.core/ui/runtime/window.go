package woxui

import (
	window "wox/ui/runtime/internal/window"
)

// ReleaseIdleTextMetricsCache drops all cached MeasureText results while every
// window is hidden. The hot set is re-measured lazily on the first frame after
// show, which costs far less than keeping the strings resident while idle.
func ReleaseIdleTextMetricsCache() { window.ReleaseIdleTextMetricsCache() }

// FocusEpoch identifies one show/focus lifetime of a window.
type FocusEpoch = window.FocusEpoch

// FileDialogOptions configures a single-selection native file dialog.
type FileDialogOptions = window.FileDialogOptions

// SaveFileOptions configures a platform-native save dialog with overwrite confirmation.
type SaveFileOptions = window.SaveFileOptions

// FrameInfo describes both the logical layout space and its backing surface.
type FrameInfo = window.FrameInfo

// FocusEvent reports whether this window's focus domain owns keyboard input.
// Moving focus between child or owned native surfaces in the same domain does not emit a blur.
type FocusEvent = window.FocusEvent

// WindowRole selects taskbar, coordinate, and focus behavior. It does not
// select window material; Open always applies the process default except for
// WindowRoleScreenshot, which shows the desktop.
type WindowRole = window.WindowRole

const (
	WindowRoleUtility     = window.WindowRoleUtility
	WindowRoleApplication = window.WindowRoleApplication
	WindowRoleScreenshot  = window.WindowRoleScreenshot
)

// FileDragStatus reports how a native file drag ended.
type FileDragStatus = window.FileDragStatus

const (
	FileDragStatusSuccess        = window.FileDragStatusSuccess
	FileDragStatusCancel         = window.FileDragStatusCancel
	FileDragStatusCancelInSource = window.FileDragStatusCancelInSource
	FileDragStatusPending        = window.FileDragStatusPending
)

// WindowOptions configures a launcher window using platform-neutral units and behavior.
// Size is the preferred initial logical client size; FrameInfo reports the actual drawable size.
type WindowOptions = window.WindowOptions

// TitleBarControls describes native caption visibility in logical units.
// Minimize also shows a disabled zoom button when Maximize is false, as in Settings.
// The zero value keeps launcher and overlay windows free of caption controls.
type TitleBarControls = window.TitleBarControls

// Window wraps the native implementation selected for the current platform.
type Window = window.Window

// Open creates a hidden window. It must be called from Run's start callback or a UI callback.
func Open(options WindowOptions) (*Window, error) { return window.Open(options) }

const DefaultWindowCornerRadius = window.DefaultWindowCornerRadius

// WindowID identifies one logical top-level surface across its native lifetime.
type WindowID = window.WindowID

// WindowLifecycle describes the state owned by one managed window instance.
type WindowLifecycle = window.WindowLifecycle

const (
	WindowLifecycleCreated    = window.WindowLifecycleCreated
	WindowLifecyclePresenting = window.WindowLifecyclePresenting
	WindowLifecycleVisible    = window.WindowLifecycleVisible
	WindowLifecycleHidden     = window.WindowLifecycleHidden
	WindowLifecycleClosing    = window.WindowLifecycleClosing
	WindowLifecycleClosed     = window.WindowLifecycleClosed
)

// WindowLifecycleEvent reports one state transition from the managed registry.
type WindowLifecycleEvent = window.WindowLifecycleEvent

// WindowMessage carries application state changes between independently hosted windows.
type WindowMessage = window.WindowMessage

// WindowManager owns named native windows, lifecycle notifications, and in-process messages.
type WindowManager = window.WindowManager

// ManagedWindow wraps one named native window and serializes its lifecycle transitions.
type ManagedWindow = window.ManagedWindow

// NewWindowManager creates an empty process-local top-level window registry.
func NewWindowManager() *WindowManager { return window.NewWindowManager() }

// SetDefaultAppearance is the light/dark inherited by windows created after this call.
func SetDefaultAppearance(isDark bool) { window.SetDefaultAppearance(isDark) }

// DefaultAppearanceIsDark reports the light/dark Open will apply.
func DefaultAppearanceIsDark() bool { return window.DefaultAppearanceIsDark() }

// HasNativeWindowMaterial reports whether windows can show compositor backdrop
// through a translucent theme wash.
func HasNativeWindowMaterial() bool { return window.HasNativeWindowMaterial() }

// NativeWindowCornerRadius returns the painted window-outline radius.
// Linux default material uses a square surface. Authored chrome bypasses this
// helper and supplies its own radius to both the present clip and the blur region.
func NativeWindowCornerRadius(requested float32) float32 {
	return window.NativeWindowCornerRadius(requested)
}

// ThemeCapabilities exposes available materials for nested theme variants.
// A theme can tune an available capability but cannot enable an unsupported protocol.
func ThemeCapabilities() []string { return window.ThemeCapabilities() }

// IsWindowShortcut identifies reserved settings and close commands with exact platform modifiers.
func IsWindowShortcut(key Key, modifiers KeyModifiers) bool {
	return window.IsWindowShortcut(key, modifiers)
}
