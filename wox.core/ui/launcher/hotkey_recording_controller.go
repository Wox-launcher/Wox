package launcher

import (
	"context"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

// hotkeyRecordingServices is the core surface used while a hotkey field is recording:
// the native recorder lifecycle plus the general-setting write for persisted hotkeys.
type hotkeyRecordingServices interface {
	contract.HotkeyInteractionSettingsServices
	UpdateGeneralSetting(ctx context.Context, sessionID string, key string, value string) error
}

// hotkeyRecordingController owns the single active hotkey recording shared by every hotkey
// field: Settings, onboarding, plugin settings, action forms, requirement forms and table rows.
// It drives core's native recorder, local fallback candidates, availability checks and
// persistence. Which forms are still on screen and which window receives input are App
// policies injected as callbacks.
type hotkeyRecordingController struct {
	recording *hotkeyRecordingState

	services     hotkeyRecordingServices
	sessionID    string
	lifecycleCtx context.Context
	runOnUI      func(string, func()) error
	language     *languageCatalog
	formTables   *formTableController
	onboarding   *onboardingController

	generalSettings func() *generalSettingsController
	pluginSettings  func() *pluginSettingsController

	// targetCurrent reports whether the recording target form is still on screen.
	targetCurrent           func(target *formFieldsState) bool
	nativeWindow            func() *woxui.Window
	invalidate              func()
	syncWebViewActionHotkey func()
}

// bindHotkeyRecording wires the recorder to core services, its host forms and window routing.
func (a *App) bindHotkeyRecording() {
	c := &a.hotkeyRecording
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.language = &a.language
	c.formTables = &a.formTables
	c.onboarding = &a.onboarding
	c.generalSettings = func() *generalSettingsController { return a.generalSettings }
	c.pluginSettings = func() *pluginSettingsController { return a.pluginSettings }
	c.targetCurrent = a.hotkeyRecordingTargetCurrentLocked
	c.nativeWindow = a.hotkeyRecordingNativeWindow
	c.invalidate = a.invalidateHotkeyWindows
	c.syncWebViewActionHotkey = a.webView.syncWebViewActionHotkey
}

// Recording returns the active hotkey recording state, or nil when no recording is in flight.
func (c *hotkeyRecordingController) Recording() *hotkeyRecordingState {
	return c.recording
}

func (c *hotkeyRecordingController) SetRecording(state *hotkeyRecordingState) {
	c.recording = state
}

func (c *hotkeyRecordingController) ClearRecording() {
	c.recording = nil
}
