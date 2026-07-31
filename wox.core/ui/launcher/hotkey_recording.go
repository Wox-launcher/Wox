package launcher

import (
	"context"
	"runtime"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	"wox/util"
)

var defaultHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo"}
var dictationHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo", "pressModifier", "holdModifier"}

type hotkeyRecordingState struct {
	target      *formFieldsState
	fieldIndex  int
	idPrefix    string
	persistKey  string
	allowed     map[string]bool
	raw         bool
	fallback    bool
	ready       bool
	checking    bool
	status      string
	hint        string
	display     string
	statusError bool
}

type hotkeyRecordingPresentation struct {
	Active bool
	Status string
	Value  string
	Error  bool
}

type recordedHotkeyPayload struct {
	Hotkey string
	Kind   string
}

// startHotkeyRecording asks core for the strongest recorder available on the current platform.
func (a *App) startHotkeyRecording(idPrefix string, target *formFieldsState, index int, persistKey string, allowedKinds []string) {
	if len(allowedKinds) == 0 {
		allowedKinds = defaultHotkeyRecordingKinds
	}
	allowed := make(map[string]bool, len(allowedKinds))
	for _, kind := range allowedKinds {
		allowed[kind] = true
	}
	if rec := a.hotkeySettings.Recording(); rec != nil && rec.target == target && rec.fieldIndex == index {
		return
	}
	if target == nil || index < 0 || index >= len(target.definitions) || (target.definitions[index].Type != "hotkey" && target.definitions[index].Type != "dictationHotkey") || !a.hotkeyRecordingTargetCurrentLocked(target) {
		return
	}
	setFormFieldsFocusLocked(target, index)
	hint := a.hotkeyRecordingHint(target.definitions[index], allowedKinds)
	key := target.definitions[index].Value.Key
	state := &hotkeyRecordingState{
		target: target, fieldIndex: index, idPrefix: idPrefix, persistKey: persistKey, allowed: allowed,
		status: hint, hint: hint, display: target.values[key],
	}
	a.hotkeySettings.SetRecording(state)
	_ = a.hotkeyRecordingNativeWindow().SetTextInputState(woxui.TextInputState{})
	a.invalidateHotkeyWindows()

	purpose := "normal"
	if target.definitions[index].Type == "dictationHotkey" {
		purpose = "dictation"
	}
	util.Go(a.lifecycleCtx, "start hotkey recording", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		capability, err := a.services.StartHotkeyRecording(ctx, a.sessionID, purpose, allowedKinds)
		cancel()
		_ = a.runOnUI("apply hotkey recording capability", func() {
			if a.hotkeySettings.Recording() == state {
				if err != nil {
					state.status = err.Error()
					state.statusError = true
				} else {
					state.raw = capability.RawRecorderAvailable
					state.fallback = containsString(capability.FallbackAllowedKinds, "normalCombo")
					state.ready = true
					state.status = state.hint
					state.statusError = false
					if !state.raw && !state.fallback {
						state.status = strings.TrimSpace(capability.UnavailableReason)
						if state.status == "" {
							state.status = a.translate("i18n:ui_hotkey_raw_recorder_unavailable")
						}
						state.statusError = true
					}
				}
			}
			a.invalidateHotkeyWindows()
		})
	})
}

func (a *App) hotkeyRecordingHint(definition formDefinition, allowedKinds []string) string {
	if definition.Type == "dictationHotkey" {
		return a.translate("i18n:ui_hotkey_dictation_press_hint")
	}
	if containsString(allowedKinds, "pressModifier") {
		return a.translate("i18n:ui_hotkey_modifier_press_hint")
	}
	return a.translate("i18n:ui_hotkey_press_hint")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (a *App) hotkeyRecordingTargetCurrentLocked(target *formFieldsState) bool {
	pluginForm := a.pluginSettings.Form()
	return target != nil && (((a.onboardingOpen || (a.settingsOpen && a.settingTab == "general")) && target == a.hotkeySettings.Form()) ||
		(a.tableEditor != nil && a.tableEditor.rowForm == target) ||
		(a.form != nil && target == &a.form.formFieldsState) ||
		(a.requirementForm != nil && target == &a.requirementForm.formFieldsState) ||
		(pluginForm != nil && target == &pluginForm.formFieldsState))
}

func (a *App) hotkeyRecordingFieldStatus(idPrefix string, index int) hotkeyRecordingPresentation {
	state := a.hotkeySettings.Recording()
	if state == nil || state.idPrefix != idPrefix || state.fieldIndex != index {
		return hotkeyRecordingPresentation{}
	}
	return hotkeyRecordingPresentation{Active: true, Status: state.status, Value: state.display, Error: state.statusError}
}

func (a *App) stopHotkeyRecordingForDifferentField(target *formFieldsState, index int) {
	state := a.hotkeySettings.Recording()
	stop := state != nil && (state.target != target || state.fieldIndex != index)
	if stop {
		a.stopHotkeyRecording()
	}
}

// stopHotkeyRecording releases both the local field and core's process-wide raw recorder.
func (a *App) stopHotkeyRecording() {
	active := a.hotkeySettings.Recording() != nil
	a.hotkeySettings.ClearRecording()
	if !active {
		return
	}
	a.postHotkeyRecordingStopped()
	a.invalidateHotkeyWindows()
}

func (a *App) postHotkeyRecordingStopped() {
	util.Go(a.lifecycleCtx, "stop hotkey recording", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.services.StopHotkeyRecording(ctx, a.sessionID)
		cancel()
	})
}

func (a *App) applyRecordedHotkey(payload recordedHotkeyPayload) error {
	payload.Hotkey = strings.TrimSpace(payload.Hotkey)
	if payload.Hotkey == "" {
		return nil
	}
	state := a.hotkeySettings.Recording()
	if state == nil || state.checking || (payload.Kind != "" && !state.allowed[payload.Kind]) || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
		return nil
	}
	canonical := canonicalRecordedHotkey(payload)
	if canonical == state.display {
		return nil
	}
	if hotkeyKindSkipsAvailability(payload.Kind) {
		a.acceptRecordedHotkey(state, canonical)
		return nil
	}
	state.checking = true
	util.Go(a.lifecycleCtx, "check recorded hotkey", func() {
		a.checkRecordedHotkey(state, canonical)
	})
	return nil
}

func canonicalRecordedHotkey(payload recordedHotkeyPayload) string {
	canonical := strings.TrimSpace(payload.Hotkey)
	if payload.Kind == "holdModifier" && !strings.HasPrefix(canonical, "hold:") {
		return "hold:" + canonical
	}
	return canonical
}

func hotkeyKindSkipsAvailability(kind string) bool {
	return kind == "holdModifier" || kind == "pressModifier"
}

func (a *App) checkRecordedHotkey(state *hotkeyRecordingState, hotkey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	availability, err := a.services.CheckHotkeyAvailability(ctx, a.sessionID, hotkey)
	cancel()
	_ = a.runOnUI("apply recorded hotkey availability", func() {
		if a.hotkeySettings.Recording() != state || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
			return
		}
		state.checking = false
		if err != nil {
			state.status = state.hint
			state.statusError = false
			a.invalidateHotkeyWindows()
			return
		}
		if !availability.Available {
			state.display = hotkey
			state.status = a.hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
			state.statusError = true
			a.invalidateHotkeyWindows()
			return
		}
		a.acceptRecordedHotkey(state, hotkey)
	})
}

func (a *App) hotkeyConflictMessage(kind, value string) string {
	switch kind {
	case "main":
		return a.translate("i18n:ui_hotkey_conflict_main")
	case "selection":
		return a.translate("i18n:ui_hotkey_conflict_selection")
	case "query":
		return strings.ReplaceAll(a.translate("i18n:ui_hotkey_conflict_query"), "{query}", value)
	case "system":
		return a.translate("i18n:ui_hotkey_conflict_system")
	default:
		return a.translate("i18n:ui_hotkey_unavailable")
	}
}

func (a *App) acceptRecordedHotkey(state *hotkeyRecordingState, value string) {
	if a.hotkeySettings.Recording() != state || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
		return
	}
	key := state.target.definitions[state.fieldIndex].Value.Key
	previous := state.target.values[key]
	state.target.values[key] = value
	state.display = value
	state.checking = false
	state.status = state.hint
	state.statusError = false
	if a.tableEditor != nil && a.tableEditor.rowForm == state.target {
		a.tableEditor.status = ""
	}
	a.invalidateHotkeyWindows()
	if state.idPrefix == "plugin-settings" {
		a.submitPluginSettings()
	}
	if state.persistKey != "" {
		util.Go(a.lifecycleCtx, "save recorded hotkey setting", func() {
			a.saveRecordedHotkeySetting(state, key, value, previous)
		})
	}
}

func (a *App) saveRecordedHotkeySetting(state *hotkeyRecordingState, key, value, previous string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	err := a.services.UpdateGeneralSetting(ctx, a.sessionID, state.persistKey, value)
	cancel()
	_ = a.runOnUI("apply recorded hotkey setting", func() {
		if err != nil {
			if state.target != nil {
				state.target.values[key] = previous
			}
			a.settingNote = "Could not save " + state.persistKey + ": " + err.Error()
		} else {
			switch state.persistKey {
			case "MainHotkey":
				a.generalSettings.Update(func(d *settingsData) { d.MainHotkey = value })
			case "SelectionHotkey":
				a.generalSettings.Update(func(d *settingsData) { d.SelectionHotkey = value })
			}
			a.settingNote = state.persistKey + " saved"
		}
		a.invalidateHotkeyWindows()
	})
}

// onHotkeyRecordingKey provides the normal-combo fallback when a raw recorder is unavailable.
func (a *App) onHotkeyRecordingKey(event woxui.KeyEvent) bool {
	state := a.hotkeySettings.Recording()
	if state == nil {
		return false
	}
	if event.Key == woxui.KeyEscape {
		return true
	}
	if event.Key == woxui.KeyBackspace && event.Modifiers == 0 {
		a.acceptRecordedHotkey(state, "")
		return true
	}
	if hotkeyRecordingMovesFocus(event) {
		a.stopHotkeyRecording()
		return false
	}
	if !state.ready || state.raw || !state.fallback || state.checking {
		return true
	}
	hotkey := fallbackHotkeyString(event)
	if hotkey == "" {
		return true
	}
	util.Go(a.lifecycleCtx, "submit fallback hotkey candidate", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := a.services.SubmitHotkeyRecordingCandidate(ctx, a.sessionID, hotkey)
		cancel()
		if err != nil {
			_ = a.runOnUI("apply fallback hotkey candidate error", func() {
				if a.hotkeySettings.Recording() == state {
					state.status = state.hint
					state.statusError = false
				}
				a.invalidateHotkeyWindows()
			})
		}
	})
	return true
}

func hotkeyRecordingMovesFocus(event woxui.KeyEvent) bool {
	if event.Key == woxui.KeyTab {
		return event.Modifiers & ^woxui.KeyModifierShift == 0
	}
	return event.Key == woxui.KeyEnter && event.Modifiers == 0
}

func fallbackHotkeyString(event woxui.KeyEvent) string {
	if !event.Down || event.Repeat || event.Key == woxui.KeyUnknown || event.Modifiers == 0 {
		return ""
	}
	parts := make([]string, 0, 5)
	if event.Modifiers&woxui.KeyModifierControl != 0 {
		parts = append(parts, "ctrl")
	}
	if event.Modifiers&woxui.KeyModifierShift != 0 {
		parts = append(parts, "shift")
	}
	if event.Modifiers&woxui.KeyModifierAlt != 0 {
		if runtime.GOOS == "darwin" {
			parts = append(parts, "option")
		} else {
			parts = append(parts, "alt")
		}
	}
	if event.Modifiers&woxui.KeyModifierMeta != 0 {
		if runtime.GOOS == "darwin" {
			parts = append(parts, "cmd")
		} else {
			parts = append(parts, "win")
		}
	}
	key := string(event.Key)
	switch event.Key {
	case woxui.KeyArrowLeft:
		key = "left"
	case woxui.KeyArrowRight:
		key = "right"
	case woxui.KeyArrowUp:
		key = "up"
	case woxui.KeyArrowDown:
		key = "down"
	case woxui.KeyPageUp:
		key = "pageup"
	case woxui.KeyPageDown:
		key = "pagedown"
	case woxui.Key("`"):
		key = "~"
	}
	return strings.Join(append(parts, key), "+")
}

// recordFormTableRowHotkey starts recording for a specialized hotkey column in the shared table row editor.
func (a *App) recordFormTableRowHotkey(index int) {
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		return
	}
	target := state.rowForm
	key := target.definitions[index].Value.Key
	allowed := []string(nil)
	for _, column := range state.definition.Value.Columns {
		if column.Key == key {
			allowed = append([]string(nil), column.AllowedHotkeyKinds...)
			break
		}
	}
	a.startHotkeyRecording("form-table-row", target, index, "", allowed)
}
