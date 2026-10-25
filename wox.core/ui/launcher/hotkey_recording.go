package launcher

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	corehotkey "wox/hotkey"
	woxui "wox/ui/runtime"
	"wox/util"
	utilhotkey "wox/util/hotkey"
)

var defaultHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo"}
var dictationHotkeyRecordingKinds = []string{"normalCombo", "doubleModifier", "capsLockCombo", "pressModifier", "holdModifier"}

type hotkeyRecordingState struct {
	diagnosticCtx   context.Context
	localEventCount int
	target          *formFieldsState
	fieldIndex      int
	idPrefix        string
	persistKey      string
	allowed         map[string]bool
	raw             bool
	fallback        bool
	ready           bool
	checking        bool
	status          string
	hint            string
	display         string
	statusError     bool
	// onKey is an optional parent hook. If it handles the event, the combo is not recorded.
	onKey func(woxui.KeyEvent) bool
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
// onKey, when set, is asked before a captured combo is stored.
func (c *hotkeyRecordingController) startHotkeyRecording(idPrefix string, target *formFieldsState, index int, persistKey string, allowedKinds []string, onKey func(woxui.KeyEvent) bool) {
	// Action panel shortcuts are local key events, so global special triggers cannot fire them.
	if persistKey == "ActionPanelHotkey" {
		allowedKinds = []string{"normalCombo"}
	}
	if len(allowedKinds) == 0 {
		allowedKinds = defaultHotkeyRecordingKinds
	}
	if runtime.GOOS == "windows" && persistKey == "MainHotkey" {
		allowedKinds = append(append([]string{}, allowedKinds...), "pressModifier")
	}
	allowed := make(map[string]bool, len(allowedKinds))
	for _, kind := range allowedKinds {
		allowed[kind] = true
	}
	if rec := c.Recording(); rec != nil && rec.target == target && rec.fieldIndex == index && !rec.statusError {
		return
	}
	if target == nil || index < 0 || index >= len(target.definitions) || (target.definitions[index].Type != "hotkey" && target.definitions[index].Type != "dictationHotkey") || !c.targetCurrent(target) {
		return
	}
	setFormFieldsFocusLocked(target, index)
	hint := c.hotkeyRecordingHint(target.definitions[index], allowedKinds)
	if runtime.GOOS == "windows" && persistKey == "MainHotkey" {
		hint = c.language.translate("i18n:ui_hotkey_windows_key_hint")
	}
	key := target.definitions[index].Value.Key
	state := &hotkeyRecordingState{
		diagnosticCtx: util.NewTraceContext(),
		target:        target, fieldIndex: index, idPrefix: idPrefix, persistKey: persistKey, allowed: allowed,
		status: hint, hint: hint, display: target.values[key], onKey: onKey,
	}
	c.SetRecording(state)
	util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI start: session=%s target=%s field=%d persistKey=%s allowed=%v", c.sessionID, idPrefix, index, persistKey, allowedKinds))
	_ = c.nativeWindow().SetTextInputState(woxui.TextInputState{})
	c.invalidate()

	purpose := "normal"
	if target.definitions[index].Type == "dictationHotkey" {
		purpose = "dictation"
	}
	util.Go(c.lifecycleCtx, "start hotkey recording", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		capability, err := c.services.StartHotkeyRecording(ctx, c.sessionID, purpose, allowedKinds)
		cancel()
		_ = c.runOnUI("apply hotkey recording capability", func() {
			util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI capability: current=%t raw=%t fallback=%v unavailable=%s error=%v", c.Recording() == state, capability.RawRecorderAvailable, capability.FallbackAllowedKinds, capability.UnavailableReason, err))
			if c.Recording() == state {
				if err != nil {
					state.status = c.language.translate(err.Error())
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
							state.status = c.language.translate("i18n:ui_hotkey_raw_recorder_unavailable")
						}
						state.statusError = true
					}
				}
			}
			c.invalidate()
		})
	})
}

func (c *hotkeyRecordingController) hotkeyRecordingHint(definition formDefinition, allowedKinds []string) string {
	if definition.Type == "dictationHotkey" {
		return c.language.translate("i18n:ui_hotkey_dictation_press_hint")
	}
	if containsString(allowedKinds, "pressModifier") {
		return c.language.translate("i18n:ui_hotkey_modifier_press_hint")
	}
	return c.language.translate("i18n:ui_hotkey_press_hint")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (c *hotkeyRecordingController) hotkeyRecordingFieldStatus(idPrefix string, index int) hotkeyRecordingPresentation {
	state := c.Recording()
	if state == nil || state.idPrefix != idPrefix || state.fieldIndex != index {
		return hotkeyRecordingPresentation{}
	}
	return hotkeyRecordingPresentation{Active: true, Status: state.status, Value: state.display, Error: state.statusError}
}

// hotkeyRecordingSectionSignature is part of launcher form/table Boundary keys.
// Recording lives outside the form snapshot, so omitting it keeps the cached
// "Recording..." tree after Escape/Enter have already stopped capture.
func (c *hotkeyRecordingController) hotkeyRecordingSectionSignature() any {
	state := c.Recording()
	if state == nil {
		return ""
	}
	return struct {
		Prefix   string
		Field    int
		Value    string
		Status   string
		Error    bool
		Ready    bool
		Checking bool
	}{state.idPrefix, state.fieldIndex, state.display, state.status, state.statusError, state.ready, state.checking}
}

func (c *hotkeyRecordingController) stopHotkeyRecordingForDifferentField(target *formFieldsState, index int) {
	state := c.Recording()
	stop := state != nil && (state.target != target || state.fieldIndex != index)
	if stop {
		c.stopHotkeyRecording()
	}
}

// commitHotkeyRecordingValue copies the last shown combo into the field before Save.
// Availability still lives on recording.display until accept, so submitting without
// this flush would persist the previous value.
func (c *hotkeyRecordingController) commitHotkeyRecordingValue() {
	state := c.Recording()
	if state == nil || state.target == nil || state.fieldIndex < 0 || state.fieldIndex >= len(state.target.definitions) {
		return
	}
	value := strings.TrimSpace(state.display)
	if value == "" {
		return
	}
	key := state.target.definitions[state.fieldIndex].Value.Key
	state.target.values[key] = value
}

// stopHotkeyRecording releases both the local field and core's process-wide raw recorder.
func (c *hotkeyRecordingController) stopHotkeyRecording() {
	if state := c.Recording(); state != nil {
		util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI stop: localEvents=%d checking=%t statusError=%t", state.localEventCount, state.checking, state.statusError))
	}
	active := c.Recording() != nil
	c.ClearRecording()
	if !active {
		return
	}
	c.postHotkeyRecordingStopped()
	c.invalidate()
}

func (c *hotkeyRecordingController) postHotkeyRecordingStopped() {
	util.Go(c.lifecycleCtx, "stop hotkey recording", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = c.services.StopHotkeyRecording(ctx, c.sessionID)
		cancel()
	})
}

func (c *hotkeyRecordingController) applyRecordedHotkey(payload recordedHotkeyPayload) error {
	payload.Hotkey = strings.TrimSpace(payload.Hotkey)
	if payload.Hotkey == "" {
		return nil
	}
	state := c.Recording()
	if state == nil {
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("hotkey UI candidate ignored: session=%s reason=no_active_recorder kind=%s", c.sessionID, payload.Kind))
		return nil
	}
	util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI candidate: hotkey=%s kind=%s checking=%t kindAllowed=%t targetCurrent=%t", payload.Hotkey, payload.Kind, state.checking, payload.Kind == "" || state.allowed[payload.Kind], c.targetCurrent(state.target)))
	if !c.targetCurrent(state.target) {
		return nil
	}
	canonical := canonicalRecordedHotkey(payload)
	if state.persistKey == "MainHotkey" && payload.Kind == "pressModifier" && !utilhotkey.IsWindowsKeyHotkeyString(canonical) {
		return nil
	}
	if event, ok := recordedHotkeyEvent(canonical); ok && state.onKey != nil && state.onKey(event) {
		util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI candidate handled by parent: hotkey=%s", canonical))
		return nil
	}
	if state.checking || (payload.Kind != "" && !state.allowed[payload.Kind]) {
		return nil
	}
	if canonical == state.display && !state.statusError {
		util.GetLogger().Info(state.diagnosticCtx, "hotkey UI candidate ignored: reason=unchanged")
		return nil
	}
	if hotkeyKindSkipsAvailability(payload.Kind) && state.persistKey != "MainHotkey" {
		c.acceptRecordedHotkey(state, canonical)
		return nil
	}
	// Show the combination immediately. Availability probes a real OS
	// registration and a taken combo (for example Ctrl+Shift+P) can take
	// hundreds of milliseconds; waiting hid the keycaps until the error arrived.
	state.display = canonical
	state.status = state.hint
	state.statusError = false
	state.checking = true
	c.invalidate()
	util.Go(c.lifecycleCtx, "check recorded hotkey", func() {
		c.checkRecordedHotkey(state, canonical)
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

func (c *hotkeyRecordingController) checkRecordedHotkey(state *hotkeyRecordingState, hotkey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	availability, err := c.services.CheckHotkeyAvailability(ctx, c.sessionID, hotkey)
	cancel()
	util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI availability: hotkey=%s available=%t conflictType=%s error=%v", hotkey, availability.Available, availability.ConflictType, err))
	_ = c.runOnUI("apply recorded hotkey availability", func() {
		if c.Recording() != state || !c.targetCurrent(state.target) {
			util.GetLogger().Info(state.diagnosticCtx, "hotkey UI availability ignored: reason=stale_target")
			return
		}
		state.checking = false
		if err != nil {
			state.status = state.hint
			state.statusError = false
			c.invalidate()
			return
		}
		if !availability.Available {
			// Re-selecting the shortcut Wox already registered is not a conflict with itself.
			// The availability check reports that chord as the current main hotkey.
			if state.persistKey == "MainHotkey" && availability.ConflictType == "main" {
				c.acceptRecordedHotkey(state, hotkey)
				return
			}
			state.display = hotkey
			state.status = c.hotkeyConflictMessage(availability.ConflictType, availability.ConflictValue)
			state.statusError = true
			c.invalidate()
			return
		}
		c.acceptRecordedHotkey(state, hotkey)
	})
}

func (c *hotkeyRecordingController) hotkeyConflictMessage(kind, value string) string {
	switch kind {
	case "unsupported":
		return c.language.translate("i18n:ui_hotkey_registration_unsupported")
	case "registration_failed":
		return c.language.translate("i18n:ui_hotkey_registration_failed")
	case "main":
		return c.language.translate("i18n:ui_hotkey_conflict_main")
	case "selection":
		return c.language.translate("i18n:ui_hotkey_conflict_selection")
	case "query":
		return strings.ReplaceAll(c.language.translate("i18n:ui_hotkey_conflict_query"), "{query}", value)
	case "system":
		return c.language.translate("i18n:ui_hotkey_conflict_system")
	default:
		return c.language.translate("i18n:ui_hotkey_unavailable")
	}
}

func (c *hotkeyRecordingController) acceptRecordedHotkey(state *hotkeyRecordingState, value string) {
	if c.Recording() != state || !c.targetCurrent(state.target) {
		return
	}
	key := state.target.definitions[state.fieldIndex].Value.Key
	util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI accepted: key=%s hotkey=%s persistKey=%s", key, value, state.persistKey))
	previous := state.target.values[key]
	state.target.values[key] = value
	state.display = value
	state.checking = false
	state.status = state.hint
	state.statusError = false
	if editor := c.formTables.activeFormTableEditor(); editor != nil && editor.rowForm == state.target {
		editor.status = ""
	}
	c.invalidate()
	if c.onboarding.isOpen() && state.idPrefix == "onboarding-query-hotkey" {
		c.onboarding.saveOnboardingQueryHotkey(value)
		c.stopHotkeyRecording()
		return
	}
	if state.idPrefix == "plugin-settings" {
		c.pluginSettings().submitPluginSettings()
	}
	if state.persistKey != "" {
		util.Go(c.lifecycleCtx, "save recorded hotkey setting", func() {
			c.saveRecordedHotkeySetting(state, key, value, previous)
		})
	}
	// Onboarding treats one valid combination as completion; Settings keeps recording for rapid edits.
	if c.onboarding.isOpen() && state.idPrefix == "hotkey-settings" {
		c.stopHotkeyRecording()
	}
}

func (c *hotkeyRecordingController) saveRecordedHotkeySetting(state *hotkeyRecordingState, key, value, previous string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	err := c.services.UpdateGeneralSetting(ctx, c.sessionID, state.persistKey, value)
	cancel()
	util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI save: key=%s hotkey=%s error=%v", state.persistKey, value, err))
	_ = c.runOnUI("apply recorded hotkey setting", func() {
		if err != nil {
			if state.target != nil {
				state.target.values[key] = previous
			}
			state.display = previous
			state.status = err.Error()
			state.statusError = true
			if state.persistKey == "MainHotkey" || state.persistKey == "SelectionHotkey" {
				state.status = c.language.translate(corehotkey.RegistrationErrorKey(err))
			}
			// Onboarding closes the recorder before the asynchronous save finishes.
			// Keep its error on the page instead of losing it with the recorder state.
			if state.persistKey == "MainHotkey" {
				c.onboarding.showPageError(state.status)
			}
		} else {
			if state.persistKey == "MainHotkey" {
				c.onboarding.showPageError("")
			}
			switch state.persistKey {
			case "MainHotkey":
				c.generalSettings().Update(func(d *settingsData) {
					d.MainHotkey = value
					d.MainHotkeyRegistrationFailed = false
					d.MainHotkeyRegistrationError = ""
				})
				c.onboarding.clearStagedMainHotkey()
			case "SelectionHotkey":
				c.generalSettings().Update(func(d *settingsData) { d.SelectionHotkey = value })
			case "ActionPanelHotkey":
				c.generalSettings().Update(func(d *settingsData) { d.ActionPanelHotkey = value })
				c.syncWebViewActionHotkey()
			}
		}
		c.invalidate()
	})
}

// recordedHotkeyEvent rebuilds the key event for a raw recorder candidate so OnKey can decide.
func recordedHotkeyEvent(hotkey string) (woxui.KeyEvent, bool) {
	parsed, ok := woxui.ParseHotkey(hotkey)
	if !ok {
		return woxui.KeyEvent{}, false
	}
	return woxui.KeyEvent{Key: parsed.Key, Down: true, Modifiers: parsed.Modifiers}, true
}

// onHotkeyRecordingKey supplements raw recording with local normal-combo candidates.
func (c *hotkeyRecordingController) onHotkeyRecordingKey(event woxui.KeyEvent) bool {
	state := c.Recording()
	if state == nil {
		return false
	}
	state.localEventCount++
	if state.localEventCount <= 40 {
		util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI local event: index=%d down=%t repeat=%t modifiers=%d unknownKey=%t ready=%t raw=%t fallback=%t checking=%t stop=%t clear=%t", state.localEventCount, event.Down, event.Repeat, event.Modifiers, event.Key == woxui.KeyUnknown, state.ready, state.raw, state.fallback, state.checking, hotkeyRecordingStops(event), event.Key == woxui.KeyBackspace && event.Modifiers == 0))
	}
	if event.Key == woxui.KeyBackspace && event.Modifiers == 0 {
		c.acceptRecordedHotkey(state, "")
		return true
	}
	if hotkeyRecordingStops(event) {
		c.stopHotkeyRecording()
		return true
	}
	if event.Down && !event.Repeat && state.onKey != nil && state.onKey(event) {
		return true
	}
	if !state.ready || !state.fallback || state.checking {
		return true
	}
	hotkey := fallbackHotkeyString(event)
	if hotkey == "" {
		return true
	}
	util.Go(c.lifecycleCtx, "submit fallback hotkey candidate", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := c.services.SubmitHotkeyRecordingCandidate(ctx, c.sessionID, hotkey)
		cancel()
		util.GetLogger().Info(state.diagnosticCtx, fmt.Sprintf("hotkey UI fallback submitted: hotkey=%s error=%v", hotkey, err))
		if err != nil {
			_ = c.runOnUI("apply fallback hotkey candidate error", func() {
				if c.Recording() == state {
					state.status = state.hint
					state.statusError = false
				}
				c.invalidate()
			})
		}
	})
	return true
}

func hotkeyRecordingStops(event woxui.KeyEvent) bool {
	if event.Key == woxui.KeyEscape {
		return true
	}
	if event.Key == woxui.KeyTab {
		return event.Modifiers & ^woxui.KeyModifierShift == 0
	}
	return event.Key == woxui.KeyEnter && event.Modifiers == 0
}

func fallbackHotkeyString(event woxui.KeyEvent) string {
	if !event.Down || event.Repeat || event.Key == woxui.KeyUnknown || (event.Modifiers == 0 && !isFunctionKeyEvent(event.Key)) {
		return ""
	}
	// Modifier presses belong to the raw dictation/double-modifier recorder.
	// They carry their own modifier flag and would otherwise become cmd+meta.
	switch event.Key {
	case woxui.KeyAlt, woxui.KeyMeta, woxui.Key("shift"), woxui.Key("ctrl"), woxui.Key("control"):
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

// isFunctionKeyEvent limits modifier-free fallback candidates to function keys.
func isFunctionKeyEvent(key woxui.Key) bool {
	value := strings.TrimPrefix(string(key), "f")
	if len(value) == len(key) {
		return false
	}
	number, err := strconv.Atoi(value)
	return err == nil && number >= 1 && number <= 24
}

// recordFormTableRowHotkey starts recording for a specialized hotkey column in the shared table row editor.
func (c *hotkeyRecordingController) recordFormTableRowHotkey(index int) {
	state := c.formTables.activeFormTableEditor()
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
	c.startHotkeyRecording("form-table-row", target, index, "", allowed, c.formTables.onFormTableKey)
}
