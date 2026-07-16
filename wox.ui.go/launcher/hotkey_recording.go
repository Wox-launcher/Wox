package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

var defaultHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo"}
var dictationHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo", "pressModifier", "holdModifier"}

type hotkeyRecordingCapability struct {
	RawRecorderAvailable bool
	FallbackAllowedKinds []string
	UnavailableReason    string
}

type hotkeyRecordingState struct {
	target     *formFieldsState
	fieldIndex int
	idPrefix   string
	persistKey string
	allowed    map[string]bool
	raw        bool
	fallback   bool
	ready      bool
	checking   bool
	status     string
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
	a.mu.Lock()
	if a.hotkeyRecording != nil && a.hotkeyRecording.target == target && a.hotkeyRecording.fieldIndex == index {
		a.mu.Unlock()
		a.stopHotkeyRecording()
		return
	}
	if target == nil || index < 0 || index >= len(target.definitions) || (target.definitions[index].Type != "hotkey" && target.definitions[index].Type != "dictationHotkey") || !a.hotkeyRecordingTargetCurrentLocked(target) {
		a.mu.Unlock()
		return
	}
	setFormFieldsFocusLocked(target, index)
	state := &hotkeyRecordingState{target: target, fieldIndex: index, idPrefix: idPrefix, persistKey: persistKey, allowed: allowed, status: "Starting recorder…"}
	a.hotkeyRecording = state
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()

	purpose := "normal"
	if target.definitions[index].Type == "dictationHotkey" {
		purpose = "dictation"
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		var capability hotkeyRecordingCapability
		err := a.client.Post(ctx, "/on/hotkey/recording", map[string]any{"isRecording": true, "purpose": purpose, "allowedKinds": allowedKinds}, &capability)
		cancel()
		a.mu.Lock()
		if a.hotkeyRecording == state {
			if err != nil {
				state.status = "Recorder unavailable: " + err.Error()
			} else {
				state.raw = capability.RawRecorderAvailable
				state.fallback = containsString(capability.FallbackAllowedKinds, "normalCombo")
				state.ready = true
				state.status = "Press a hotkey…"
				if !state.raw && !state.fallback {
					state.status = strings.TrimSpace(capability.UnavailableReason)
					if state.status == "" {
						state.status = "Raw hotkey recording is unavailable on this platform."
					}
				}
			}
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
	}()
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
	return target != nil && ((a.mode == viewSettings && a.settingTab == "hotkeys" && target == a.hotkeySettingsForm) ||
		(a.tableEditor != nil && a.tableEditor.rowForm == target) ||
		(a.form != nil && target == &a.form.formFieldsState) ||
		(a.requirementForm != nil && target == &a.requirementForm.formFieldsState) ||
		(a.pluginForm != nil && target == &a.pluginForm.formFieldsState))
}

func (a *App) hotkeyRecordingFieldStatus(idPrefix string, index int) (bool, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	state := a.hotkeyRecording
	if state == nil || state.idPrefix != idPrefix || state.fieldIndex != index {
		return false, ""
	}
	return true, state.status
}

func (a *App) stopHotkeyRecordingForDifferentField(target *formFieldsState, index int) {
	a.mu.RLock()
	state := a.hotkeyRecording
	stop := state != nil && (state.target != target || state.fieldIndex != index)
	a.mu.RUnlock()
	if stop {
		a.stopHotkeyRecording()
	}
}

// stopHotkeyRecording releases both the local field and core's process-wide raw recorder.
func (a *App) stopHotkeyRecording() {
	a.mu.Lock()
	active := a.hotkeyRecording != nil
	a.hotkeyRecording = nil
	a.mu.Unlock()
	if !active {
		return
	}
	a.postHotkeyRecordingStopped()
	_ = a.window.Invalidate()
}

func (a *App) postHotkeyRecordingStopped() {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = a.client.Post(ctx, "/on/hotkey/recording", map[string]any{"isRecording": false}, nil)
		cancel()
	}()
}

// receiveRecordedHotkey validates a core raw-recorder result before mutating a form value.
func (a *App) receiveRecordedHotkey(raw json.RawMessage) error {
	var payload recordedHotkeyPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode recorded hotkey: %w", err)
	}
	payload.Hotkey = strings.TrimSpace(payload.Hotkey)
	if payload.Hotkey == "" {
		return nil
	}
	a.mu.Lock()
	state := a.hotkeyRecording
	if state == nil || state.checking || !state.ready || (payload.Kind != "" && !state.allowed[payload.Kind]) || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
		a.mu.Unlock()
		return nil
	}
	canonical := payload.Hotkey
	if payload.Kind == "holdModifier" && !strings.HasPrefix(canonical, "hold:") {
		canonical = "hold:" + canonical
	}
	current := state.target.values[state.target.definitions[state.fieldIndex].Value.Key]
	if canonical == current {
		a.hotkeyRecording = nil
		a.mu.Unlock()
		a.postHotkeyRecordingStopped()
		_ = a.window.Invalidate()
		return nil
	}
	state.checking = true
	state.status = "Checking availability…"
	a.mu.Unlock()
	_ = a.window.Invalidate()
	go a.checkRecordedHotkey(state, canonical)
	return nil
}

func (a *App) checkRecordedHotkey(state *hotkeyRecordingState, hotkey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	var availability struct {
		Available     bool
		ConflictType  string
		ConflictValue string
	}
	err := a.client.Post(ctx, "/hotkey/availability", map[string]string{"hotkey": hotkey}, &availability)
	cancel()
	a.mu.Lock()
	if a.hotkeyRecording != state || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
		a.mu.Unlock()
		return
	}
	state.checking = false
	if err != nil {
		state.status = "Could not check hotkey: " + err.Error()
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	if !availability.Available {
		state.status = hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
		a.mu.Unlock()
		_ = a.window.Invalidate()
		return
	}
	a.mu.Unlock()
	a.acceptRecordedHotkey(state, hotkey)
}

func hotkeyConflictMessage(kind, value string) string {
	switch kind {
	case "main":
		return "Already used by the main Wox hotkey."
	case "selection":
		return "Already used by the selection hotkey."
	case "query":
		return "Already used by query hotkey " + value + "."
	case "system":
		return "The operating system has reserved this hotkey."
	default:
		return "This hotkey is unavailable."
	}
}

func (a *App) acceptRecordedHotkey(state *hotkeyRecordingState, value string) {
	a.mu.Lock()
	if a.hotkeyRecording != state || !a.hotkeyRecordingTargetCurrentLocked(state.target) {
		a.mu.Unlock()
		return
	}
	key := state.target.definitions[state.fieldIndex].Value.Key
	previous := state.target.values[key]
	state.target.values[key] = value
	a.hotkeyRecording = nil
	if a.tableEditor != nil && a.tableEditor.rowForm == state.target {
		a.tableEditor.status = ""
	}
	a.mu.Unlock()
	a.postHotkeyRecordingStopped()
	_ = a.window.Invalidate()
	if state.persistKey != "" {
		go a.saveRecordedHotkeySetting(state, key, value, previous)
	}
}

func (a *App) saveRecordedHotkeySetting(state *hotkeyRecordingState, key, value, previous string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	err := a.client.Post(ctx, "/setting/wox/update", map[string]string{"Key": state.persistKey, "Value": value}, nil)
	cancel()
	a.mu.Lock()
	if err != nil {
		if state.target != nil {
			state.target.values[key] = previous
		}
		a.settingNote = "Could not save " + state.persistKey + ": " + err.Error()
	} else {
		switch state.persistKey {
		case "MainHotkey":
			a.settings.MainHotkey = value
		case "SelectionHotkey":
			a.settings.SelectionHotkey = value
		}
		a.settingNote = state.persistKey + " saved"
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// onHotkeyRecordingKey provides the normal-combo fallback when a raw recorder is unavailable.
func (a *App) onHotkeyRecordingKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	state := a.hotkeyRecording
	a.mu.RUnlock()
	if state == nil {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.stopHotkeyRecording()
		return true
	}
	if event.Key == woxui.KeyBackspace && event.Modifiers == 0 {
		a.acceptRecordedHotkey(state, "")
		return true
	}
	if (event.Key == woxui.KeyTab || event.Key == woxui.KeyEnter) && event.Modifiers == 0 {
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
	a.mu.Lock()
	if a.hotkeyRecording == state {
		state.status = "Recording…"
	}
	a.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := a.client.Post(ctx, "/on/hotkey/recording/candidate", map[string]string{"hotkey": hotkey}, nil)
		cancel()
		if err != nil {
			a.mu.Lock()
			if a.hotkeyRecording == state {
				state.status = "Could not record: " + err.Error()
			}
			a.mu.Unlock()
			_ = a.window.Invalidate()
		}
	}()
	return true
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
	a.mu.RLock()
	state := a.tableEditor
	if state == nil || state.rowForm == nil || index < 0 || index >= len(state.rowForm.definitions) {
		a.mu.RUnlock()
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
	a.mu.RUnlock()
	a.startHotkeyRecording("form-table-row", target, index, "", allowed)
}
