package launcher

import (
	"sync/atomic"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// settingsWindowController owns Settings host and transient editing state.
type settingsWindowController struct {
	windows                           *woxui.WindowManager
	runOnUI                           func(string, func()) error
	fontFamily                        func() string
	primaryOwner                      func() *settingsWindowController
	window                            func() *woxui.Window
	buildSettings                     func(woxui.FrameInfo) woxwidget.Widget
	settingsWindowTitle               func() string
	finishSettingsPointer             func(woxui.PointerEvent)
	hotkeyRecordingUsesSettingsWindow func() bool
	onSettingsWindowKey               func(woxui.KeyEvent) bool
	dispatchWindowKey                 func(woxui.KeyEvent, *woxwidget.Host, string, func(), func(woxui.KeyEvent) bool) bool
	requestSettingsClose              func()
	onSettingsWindowTextInput         func(woxui.TextInputEvent)
	onSettingsWindowClosed            func()
	openingSettingsShortcut           atomic.Bool
	settingsView                      *woxui.ManagedWindow
	settingsHost                      *woxwidget.Host
	settingsOpen                      bool
	settingsCtx                       settingWindowContext
	settingTab                        string
	settingRow                        int
	settingSaving                     bool
	settingRestarting                 bool
	settingRestartError               string
	// pendingBuiltInText is a text value whose field already lost focus. It is held
	// until the pointer click that caused the blur either saves another setting or releases.
	pendingBuiltInText            *pendingBuiltInText
	settingFlash                  string
	settingFlashTimer             *time.Timer
	settingsInlineTooltip         *settingsInlineTooltipState
	settingsInlineTooltipRevision atomic.Uint64
	cloudPlanTooltip              *cloudPlanTooltipState
	cloudPlanTooltipRevision      atomic.Uint64
	settingsDemo                  *settingsDemoState
	settingsDemoRevision          atomic.Uint64
	choiceTooltipRevision         atomic.Uint64
	settingsTableEditor           *formTableEditorState
	nativeSettingsPluginID        string
}

// bindSettingsWindow supplies rendering and feature handoffs to the native window owner.
func (a *App) bindSettingsWindow() {
	c := &a.settingsWindow
	c.windows = a.windows
	c.runOnUI = a.runOnUI
	c.fontFamily = func() string { return a.generalSettings.Data().AppFontFamily }
	c.primaryOwner = func() *settingsWindowController {
		if !a.isPrimary && a.primary != nil {
			return &a.primary.settingsWindow
		}
		return nil
	}
	c.window = func() *woxui.Window { return a.window }
	c.buildSettings = a.buildSettings
	c.settingsWindowTitle = a.settingsWindowTitle
	c.finishSettingsPointer = a.finishSettingsPointer
	c.hotkeyRecordingUsesSettingsWindow = a.hotkeyRecordingUsesSettingsWindow
	c.onSettingsWindowKey = a.onSettingsWindowKey
	c.dispatchWindowKey = a.dispatchWindowKey
	c.requestSettingsClose = a.requestSettingsClose
	c.onSettingsWindowTextInput = a.onSettingsWindowTextInput
	c.onSettingsWindowClosed = a.onSettingsWindowClosed
}
