package launcher

import (
	"context"

	woxui "wox/ui/runtime"
)

type formActionServices interface {
	SubmitFormAction(context.Context, string, string, string, string, map[string]string) error
}

// formActionController owns action-form editing.
// Shared settings forms and native hotkey recording remain separate capabilities.
type formActionController struct {
	services                   formActionServices
	sessionID                  string
	window                     func() *woxui.Window
	actions                    *actionPanelController
	applyWindowBounds          func() error
	updateFormTextInput        func(bool)
	restoreQueryTextInput      func()
	openActionFormTable        func(int)
	startHotkeyRecording       func(string, *formFieldsState, int, string, []string, func(woxui.KeyEvent) bool)
	stopHotkeyRecording        func()
	commitHotkeyRecordingValue func()
	formFieldNativeWindow      func(string) *woxui.Window
	form                       *formState
}

// bindForms connects form editing to shared input and table-editor capabilities.
func (a *App) bindForms() {
	c := &a.forms
	c.services = a.services
	c.sessionID = a.sessionID
	c.window = func() *woxui.Window { return a.window }
	c.actions = &a.actions
	c.applyWindowBounds = a.applyWindowBounds
	c.updateFormTextInput = a.updateFormTextInput
	c.restoreQueryTextInput = a.restoreQueryTextInput
	c.openActionFormTable = a.openActionFormTable
	c.startHotkeyRecording = a.startHotkeyRecording
	c.stopHotkeyRecording = a.stopHotkeyRecording
	c.commitHotkeyRecordingValue = a.commitHotkeyRecordingValue
	c.formFieldNativeWindow = a.formFieldNativeWindow
}
