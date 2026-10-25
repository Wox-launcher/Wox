package launcher

import (
	"sync/atomic"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// settingsWindowController owns the Settings window and the session state shared by
// every Settings page: the active route, saving and restart status, hover overlays and
// the search highlight. Page content lives in the settings domain controllers.
type settingsWindowController struct {
	windows                           *woxui.WindowManager
	runOnUI                           func(string, func()) error
	isDestroyed                       func() bool
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
	tooltips                          *tooltipController

	openingShortcut atomic.Bool
	view            *woxui.ManagedWindow
	host            *woxwidget.Host
	open            bool
	tab             string
	row             int
	saving          bool
	restarting      bool
	restartError    string
	// pendingBuiltInText is a text value whose field already lost focus. It is held
	// until the pointer click that caused the blur either saves another setting or releases.
	pendingBuiltInText *pendingBuiltInText
	// flash names the row highlighted after search navigation until flashTimer clears it.
	flash                    string
	flashTimer               *time.Timer
	inlineTooltip            *settingsInlineTooltipState
	inlineTooltipRevision    atomic.Uint64
	cloudPlanTooltip         *cloudPlanTooltipState
	cloudPlanTooltipRevision atomic.Uint64
	demo                     *settingsDemoState
	demoRevision             atomic.Uint64
	choiceTooltipRevision    atomic.Uint64
}

// bindSettingsWindow supplies rendering and feature handoffs to the native window owner.
func (a *App) bindSettingsWindow() {
	c := &a.settingsWindow
	c.windows = a.windows
	c.runOnUI = a.runOnUI
	c.isDestroyed = a.isDestroyed
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
	c.tooltips = &a.tooltips
}

func (c *settingsWindowController) setSettingChoiceTooltip(inside bool, text string, anchor woxui.Rect) {
	c.setSettingsHoverTooltip(inside, text, anchor, "top")
}

// setSettingsHoverTooltip shows one settings-window tooltip on the requested side.
// Linux draws it inside the window; other platforms use a native tooltip window.
func (c *settingsWindowController) setSettingsHoverTooltip(inside bool, text string, anchor woxui.Rect, side string) {
	if util.IsLinux() {
		c.tooltips.scheduleLinuxInlineTooltip(linuxInlineTooltipTarget{
			revision: &c.inlineTooltipRevision, state: &c.inlineTooltip,
			open: c.open, invalidate: c.invalidateSettingsWindow, job: "show settings inline tooltip",
		}, inside, text, anchor, side)
		return
	}

	c.tooltips.setNativeHoverTooltip(&c.choiceTooltipRevision, "go-ui-setting-choice", "update setting choice tooltip", inside, text, anchor, side, c.settingsNativeWindow)
}

// beginSessionLocked opens Settings on a tab. Reopening an already open window behaves
// like a tab switch, so hover overlays anchored to the previous page are dropped.
func (c *settingsWindowController) beginSessionLocked(tab string) {
	c.open = true
	c.tab = tab
	c.row = 0
	c.saving = false
	c.clearOverlaysLocked()
}

// selectTabLocked moves the session to another tab; overlays anchored to the old page go away.
func (c *settingsWindowController) selectTabLocked(tab string) {
	c.tab = tab
	c.row = 0
	c.clearOverlaysLocked()
}

// endSessionLocked releases window-owned session state after the native window closed and
// reports whether Settings was open.
func (c *settingsWindowController) endSessionLocked() bool {
	wasOpen := c.open
	c.open = false
	c.view = nil
	c.host = nil
	c.saving = false
	c.clearFlashLocked()
	c.clearOverlaysLocked()
	return wasOpen
}

// clearOverlaysLocked hides hover overlays and bumps their revisions so pending delayed
// shows are discarded.
func (c *settingsWindowController) clearOverlaysLocked() {
	c.cloudPlanTooltip = nil
	c.cloudPlanTooltipRevision.Add(1)
	c.inlineTooltip = nil
	c.inlineTooltipRevision.Add(1)
	c.demo = nil
	c.demoRevision.Add(1)
}

// flashLocked highlights a search destination and replaces any older highlight.
func (c *settingsWindowController) flashLocked(target string) {
	if c.isDestroyed() || target == "" {
		return
	}
	if c.flashTimer != nil {
		c.flashTimer.Stop()
	}
	c.flash = target
	c.flashTimer = time.AfterFunc(settingsSearchHighlightDuration, func() {
		_ = c.runOnUI("clear settings search highlight", func() {
			if c.isDestroyed() || c.flash != target {
				return
			}
			c.flash = ""
			c.flashTimer = nil
			c.invalidateSettingsWindow()
		})
	})
	c.invalidateSettingsWindow()
}

func (c *settingsWindowController) clearFlashLocked() {
	if c.flashTimer != nil {
		c.flashTimer.Stop()
		c.flashTimer = nil
	}
	c.flash = ""
}
