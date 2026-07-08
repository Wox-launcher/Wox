package hotkey

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
	"wox/util"
	"wox/util/keyboard"
)

const recordingModifierChordDebounce = 120 * time.Millisecond

type recordedHotkey struct {
	Hotkey string
	Kind   hotkeyKind
}

type recordingSessionOptions struct {
	allowedKinds []hotkeyKind
	onRecorded   func(recordedHotkey)
}

type hotkeyRecordingCapability struct {
	RawRecorderAvailable bool
	FallbackAllowedKinds []hotkeyKind
	UnavailableReason    string
}

// RecordingResult is the canonical hotkey candidate emitted by the Go recorder.
type RecordingResult struct {
	Hotkey string
	Kind   string
}

// RecordingCapability tells Flutter whether it should wait for Go raw-key
// events or fall back to local normal-combo parsing.
type RecordingCapability struct {
	RawRecorderAvailable bool
	FallbackAllowedKinds []string
	UnavailableReason    string
}

var defaultRecordingSessionManager = newHotkeyRecordingSessionManager()

// StartRecordingSession starts a single active recorder session for the UI.
// Any previous session is stopped before the new one is installed.
func StartRecordingSession(allowedKinds []string, onRecorded func(RecordingResult)) (RecordingCapability, error) {
	kinds := []hotkeyKind{}
	for _, kind := range allowedKinds {
		parsedKind, ok := parseHotkeyKind(kind)
		if !ok {
			return RecordingCapability{}, fmt.Errorf("unsupported hotkey recording kind: %s", kind)
		}
		kinds = append(kinds, parsedKind)
	}
	if len(kinds) == 0 {
		return RecordingCapability{}, fmt.Errorf("hotkey recording requires at least one allowed kind")
	}

	capability, err := defaultRecordingSessionManager.Start(recordingSessionOptions{
		allowedKinds: kinds,
		onRecorded: func(result recordedHotkey) {
			if onRecorded != nil {
				onRecorded(RecordingResult{Hotkey: result.Hotkey, Kind: string(result.Kind)})
			}
		},
	})
	if err != nil {
		return RecordingCapability{}, err
	}
	return recordingCapabilityToPublic(capability), nil
}

// StopRecordingSession stops the active UI recorder session, if any.
func StopRecordingSession() {
	defaultRecordingSessionManager.Stop()
}

// SubmitRecordingFallbackCandidate accepts Flutter's local normal-combo
// fallback only when the active Go recorder session explicitly allows it.
func SubmitRecordingFallbackCandidate(hotkey string) error {
	return defaultRecordingSessionManager.SubmitFallbackCandidate(hotkey)
}

// RecordingKindForHotkeyString classifies an already-recorded hotkey string so
// legacy Wox-owned hotkey callbacks can still include a kind in the UI event.
func RecordingKindForHotkeyString(hotkey string) string {
	kind, err := classifyRecordedHotkey(hotkey, registerOptions{allowModifierPress: true})
	if err != nil {
		return ""
	}
	return string(kind)
}

func parseHotkeyKind(kind string) (hotkeyKind, bool) {
	switch hotkeyKind(kind) {
	case hotkeyKindNormalCombo:
		return hotkeyKindNormalCombo, true
	case hotkeyKindDoubleModifier:
		return hotkeyKindDoubleModifier, true
	case hotkeyKindCapsLockCombo:
		return hotkeyKindCapsLockCombo, true
	case hotkeyKindHoldModifier:
		return hotkeyKindHoldModifier, true
	case hotkeyKindPressModifier:
		return hotkeyKindPressModifier, true
	default:
		return hotkeyKindUnknown, false
	}
}

func recordingCapabilityToPublic(capability hotkeyRecordingCapability) RecordingCapability {
	fallbackKinds := []string{}
	for _, kind := range capability.FallbackAllowedKinds {
		fallbackKinds = append(fallbackKinds, string(kind))
	}
	return RecordingCapability{
		RawRecorderAvailable: capability.RawRecorderAvailable,
		FallbackAllowedKinds: fallbackKinds,
		UnavailableReason:    capability.UnavailableReason,
	}
}

type hotkeyRecordingSessionManager struct {
	mu       sync.Mutex
	active   bool
	listener keyboard.RawKeySubscription
	state    *recordingRawState
	options  recordingSessionOptions
}

func newHotkeyRecordingSessionManager() *hotkeyRecordingSessionManager {
	return &hotkeyRecordingSessionManager{}
}

func (m *hotkeyRecordingSessionManager) Start(options recordingSessionOptions) (hotkeyRecordingCapability, error) {
	m.Stop()

	allowed := hotkeyKindSet(options.allowedKinds)
	if len(allowed) == 0 {
		return hotkeyRecordingCapability{}, fmt.Errorf("recording session requires at least one allowed kind")
	}

	state := newRecordingRawState(allowed, options.onRecorded)
	listener, err := addRawKeyListener(state.HandleEvent)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.active = true
	m.options = options
	if err != nil {
		return hotkeyRecordingCapability{
			RawRecorderAvailable: false,
			FallbackAllowedKinds: recordingFallbackKinds(allowed),
			UnavailableReason:    err.Error(),
		}, nil
	}

	m.listener = listener
	m.state = state
	return hotkeyRecordingCapability{RawRecorderAvailable: true}, nil
}

func (m *hotkeyRecordingSessionManager) Stop() {
	var listener keyboard.RawKeySubscription
	var state *recordingRawState

	m.mu.Lock()
	listener = m.listener
	state = m.state
	m.listener = nil
	m.state = nil
	m.active = false
	m.options = recordingSessionOptions{}
	m.mu.Unlock()

	if state != nil {
		state.Close()
	}
	if listener != nil {
		_ = listener.Close()
	}
}

func (m *hotkeyRecordingSessionManager) SubmitFallbackCandidate(hotkey string) error {
	m.mu.Lock()
	active := m.active
	options := m.options
	m.mu.Unlock()

	if !active {
		return fmt.Errorf("hotkey recording session is not active")
	}

	kind, err := classifyRecordedHotkey(hotkey, registerOptions{})
	if err != nil {
		return err
	}
	if kind != hotkeyKindNormalCombo {
		return fmt.Errorf("fallback recording only supports %s, got %s", hotkeyKindNormalCombo, kind)
	}
	if !hotkeyKindSet(options.allowedKinds)[kind] {
		return fmt.Errorf("fallback kind %s is not allowed for this recording session", kind)
	}
	if options.onRecorded != nil {
		options.onRecorded(recordedHotkey{Hotkey: hotkey, Kind: kind})
	}
	return nil
}

func classifyRecordedHotkey(hotkey string, options registerOptions) (hotkeyKind, error) {
	spec, err := (&Hotkey{}).parseCombineKey(hotkey)
	if err != nil {
		return hotkeyKindUnknown, err
	}
	return resolveHotkeyKind(spec, false, options)
}

func hotkeyKindSet(kinds []hotkeyKind) map[hotkeyKind]bool {
	set := map[hotkeyKind]bool{}
	for _, kind := range kinds {
		if kind == hotkeyKindUnknown {
			continue
		}
		set[kind] = true
	}
	return set
}

func recordingFallbackKinds(allowed map[hotkeyKind]bool) []hotkeyKind {
	if allowed[hotkeyKindNormalCombo] {
		return []hotkeyKind{hotkeyKindNormalCombo}
	}
	return nil
}

// recordingRawState owns the per-session raw-key recognizers. It intentionally
// keeps recording state separate from registered hotkey runtime state so the
// settings UI cannot perturb active global hotkeys.
type recordingRawState struct {
	mu                sync.Mutex
	allowed           map[hotkeyKind]bool
	onRecorded        func(recordedHotkey)
	pressed           map[keyboard.Key]bool
	doubleTracker     *doublePressTracker
	pressTracker      *modifierPressTracker
	capsTracker       *capsLockComboTracker
	holdPendingCombo  string
	holdPendingKeys   []keyboard.Key
	holdPendingTimer  *time.Timer
	pressPendingTimer *time.Timer
}

func newRecordingRawState(allowed map[hotkeyKind]bool, onRecorded func(recordedHotkey)) *recordingRawState {
	state := &recordingRawState{
		allowed:       allowed,
		onRecorded:    onRecorded,
		pressed:       map[keyboard.Key]bool{},
		doubleTracker: newDoublePressTracker(),
		pressTracker:  newModifierPressTracker(),
		capsTracker:   newCapsLockComboTracker(),
	}
	if allowed[hotkeyKindDoubleModifier] {
		state.doubleTracker.Register(keyboard.KeyCtrl)
		state.doubleTracker.Register(keyboard.KeyShift)
		state.doubleTracker.Register(keyboard.KeyAlt)
		state.doubleTracker.Register(keyboard.KeySuper)
	}
	if allowed[hotkeyKindPressModifier] {
		keys := orderedHoldModifierRecorderKeys()
		for _, key := range keys {
			state.pressTracker.Register([]keyboard.Key{key})
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				state.pressTracker.Register([]keyboard.Key{keys[i], keys[j]})
			}
		}
	}
	return state
}

func (s *recordingRawState) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.holdPendingTimer != nil {
		s.holdPendingTimer.Stop()
		s.holdPendingTimer = nil
	}
	if s.pressPendingTimer != nil {
		s.pressPendingTimer.Stop()
		s.pressPendingTimer = nil
	}
}

func (s *recordingRawState) HandleEvent(event keyboard.RawKeyEvent) bool {
	now := util.GetSystemTimestamp()
	var recorded []recordedHotkey
	consume := false

	s.mu.Lock()
	if s.allowed[hotkeyKindCapsLockCombo] {
		key, capsConsume := s.capsTracker.HandleEvent(event, runtime.GOOS == "darwin")
		consume = consume || capsConsume
		if key != keyboard.KeyUnknown {
			recorded = append(recorded, recordedHotkey{Hotkey: capsLockComboToHotkeyString(key), Kind: hotkeyKindCapsLockCombo})
		}
	}

	s.updatePressedLocked(event)

	if s.allowed[hotkeyKindDoubleModifier] {
		for _, key := range s.doubleTracker.HandleEvent(event, now) {
			s.pressTracker.SuppressNextPressForRawKey(event.Key, now)
			recorded = append(recorded, recordedHotkey{Hotkey: doubleModifierToHotkeyString(key), Kind: hotkeyKindDoubleModifier})
		}
	}

	if s.allowed[hotkeyKindPressModifier] {
		triggers := s.pressTracker.HandleEvent(event, s.shouldDelayPressLocked, now)
		recorded = append(recorded, s.recordedPressTriggersLocked(triggers)...)
		s.reschedulePressFlushLocked()
	}

	if s.allowed[hotkeyKindHoldModifier] {
		s.handleHoldModifierRecordingLocked(event)
	}

	if s.allowed[hotkeyKindNormalCombo] && event.Type == keyboard.EventTypeKeyDown && !isSpecificModifierKey(event.Key) {
		if hotkeyStr := s.normalComboStringLocked(event.Key); hotkeyStr != "" {
			recorded = append(recorded, recordedHotkey{Hotkey: hotkeyStr, Kind: hotkeyKindNormalCombo})
		}
	}
	s.mu.Unlock()

	s.dispatchRecorded(recorded)
	return consume
}

func (s *recordingRawState) updatePressedLocked(event keyboard.RawKeyEvent) {
	if !isSpecificModifierKey(event.Key) {
		return
	}
	switch event.Type {
	case keyboard.EventTypeKeyDown:
		s.pressed[event.Key] = true
	case keyboard.EventTypeKeyUp:
		s.pressed[event.Key] = false
	}
}

func (s *recordingRawState) shouldDelayPressLocked(key keyboard.Key) bool {
	return s.allowed[hotkeyKindDoubleModifier] && modifierKeyMatchesAnyDoubleKey(key)
}

func modifierKeyMatchesAnyDoubleKey(key keyboard.Key) bool {
	for _, registeredKey := range []keyboard.Key{keyboard.KeyCtrl, keyboard.KeyShift, keyboard.KeyAlt, keyboard.KeySuper} {
		if modifierKeyMatchesRawEvent(registeredKey, key) {
			return true
		}
	}
	return false
}

func (s *recordingRawState) recordedPressTriggersLocked(triggers []modifierPressTrigger) []recordedHotkey {
	recorded := []recordedHotkey{}
	for _, trigger := range triggers {
		recorded = append(recorded, recordedHotkey{Hotkey: trigger.combo, Kind: hotkeyKindPressModifier})
	}
	return recorded
}

func (s *recordingRawState) reschedulePressFlushLocked() {
	if s.pressPendingTimer != nil {
		s.pressPendingTimer.Stop()
		s.pressPendingTimer = nil
	}
	due, ok := s.pressTracker.NextPendingDue()
	if !ok {
		return
	}

	delay := time.Duration(due-util.GetSystemTimestamp()) * time.Millisecond
	if delay < 0 {
		delay = 0
	}
	s.pressPendingTimer = time.AfterFunc(delay, func() {
		triggers := s.pressTracker.FlushDelayed(util.GetSystemTimestamp())
		s.dispatchRecorded(s.recordedPressTriggersLocked(triggers))
		s.mu.Lock()
		s.reschedulePressFlushLocked()
		s.mu.Unlock()
	})
}

func (s *recordingRawState) handleHoldModifierRecordingLocked(event keyboard.RawKeyEvent) {
	if event.Type == keyboard.EventTypeKeyUp {
		if containsHoldModifierKey(s.holdPendingKeys, event.Key) {
			s.cancelHoldPendingLocked()
		}
		return
	}
	if event.Type != keyboard.EventTypeKeyDown {
		return
	}
	if !isSpecificModifierKey(event.Key) {
		s.cancelHoldPendingLocked()
		return
	}

	keys := s.currentPressedModifierKeysLocked()
	if len(keys) == 0 || len(keys) > 2 {
		s.cancelHoldPendingLocked()
		return
	}
	combo := holdModifierComboString(keys)
	if combo == s.holdPendingCombo {
		return
	}
	s.cancelHoldPendingLocked()
	s.holdPendingCombo = combo
	s.holdPendingKeys = keys
	s.holdPendingTimer = time.AfterFunc(recordingModifierChordDebounce, func() {
		s.mu.Lock()
		combo := s.holdPendingCombo
		keys := append([]keyboard.Key(nil), s.holdPendingKeys...)
		s.holdPendingCombo = ""
		s.holdPendingKeys = nil
		s.holdPendingTimer = nil
		stillPressed := s.exactModifierKeysPressedLocked(keys)
		if combo != "" && stillPressed {
			s.pressTracker.CancelActiveForKeys(keys)
		}
		s.mu.Unlock()

		if combo != "" && stillPressed {
			s.dispatchRecorded([]recordedHotkey{{Hotkey: combo, Kind: hotkeyKindHoldModifier}})
		}
	})
}

func (s *recordingRawState) cancelHoldPendingLocked() {
	if s.holdPendingTimer != nil {
		s.holdPendingTimer.Stop()
		s.holdPendingTimer = nil
	}
	s.holdPendingCombo = ""
	s.holdPendingKeys = nil
}

func (s *recordingRawState) currentPressedModifierKeysLocked() []keyboard.Key {
	keys := []keyboard.Key{}
	for key, pressed := range s.pressed {
		if pressed {
			keys = append(keys, key)
		}
	}
	return canonicalHoldModifierKeys(keys)
}

func (s *recordingRawState) exactModifierKeysPressedLocked(keys []keyboard.Key) bool {
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if !s.pressed[key] {
			return false
		}
	}
	for key, pressed := range s.pressed {
		if pressed && !containsHoldModifierKey(keys, key) {
			return false
		}
	}
	return true
}

func (s *recordingRawState) normalComboStringLocked(key keyboard.Key) string {
	keyStr := normalKeyString(key)
	if keyStr == "" {
		return ""
	}

	modifiers := s.currentGenericModifiersLocked()
	if len(modifiers) == 0 {
		return ""
	}

	parts := []string{}
	for _, modifier := range modifiers {
		parts = append(parts, genericModifierString(modifier))
	}
	parts = append(parts, keyStr)
	return joinHotkeyParts(parts)
}

func (s *recordingRawState) currentGenericModifiersLocked() []keyboard.Key {
	modifierSet := map[keyboard.Key]bool{}
	for key, pressed := range s.pressed {
		if !pressed {
			continue
		}
		switch key {
		case keyboard.KeyLeftCtrl, keyboard.KeyRightCtrl:
			modifierSet[keyboard.KeyCtrl] = true
		case keyboard.KeyLeftShift, keyboard.KeyRightShift:
			modifierSet[keyboard.KeyShift] = true
		case keyboard.KeyLeftAlt, keyboard.KeyRightAlt:
			modifierSet[keyboard.KeyAlt] = true
		case keyboard.KeyLeftSuper, keyboard.KeyRightSuper:
			modifierSet[keyboard.KeySuper] = true
		}
	}
	modifiers := []keyboard.Key{}
	for modifier := range modifierSet {
		modifiers = append(modifiers, modifier)
	}
	sort.Slice(modifiers, func(i, j int) bool {
		return modifiers[i] < modifiers[j]
	})
	return modifiers
}

func (s *recordingRawState) dispatchRecorded(recorded []recordedHotkey) {
	for _, result := range recorded {
		if result.Hotkey == "" || result.Kind == hotkeyKindUnknown || s.onRecorded == nil {
			continue
		}
		s.onRecorded(result)
	}
}

func normalKeyString(key keyboard.Key) string {
	if character := key.Character(); character != "" {
		return character
	}
	switch key {
	case keyboard.KeySpace:
		return "space"
	case keyboard.KeyReturn:
		return "enter"
	case keyboard.KeyEscape:
		return "escape"
	case keyboard.KeyTab:
		return "tab"
	case keyboard.KeyDelete:
		return "delete"
	case keyboard.KeyLeft:
		return "left"
	case keyboard.KeyRight:
		return "right"
	case keyboard.KeyUp:
		return "up"
	case keyboard.KeyDown:
		return "down"
	default:
		return ""
	}
}

func genericModifierString(key keyboard.Key) string {
	switch key {
	case keyboard.KeyCtrl:
		return "ctrl"
	case keyboard.KeyShift:
		return "shift"
	case keyboard.KeyAlt:
		if runtime.GOOS == "darwin" {
			return "option"
		}
		return "alt"
	case keyboard.KeySuper:
		if runtime.GOOS == "darwin" {
			return "cmd"
		}
		return "win"
	default:
		return ""
	}
}

func doubleModifierToHotkeyString(key keyboard.Key) string {
	part := genericModifierString(key)
	if part == "" {
		return ""
	}
	return part + "+" + part
}

func joinHotkeyParts(parts []string) string {
	result := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		if result != "" {
			result += "+"
		}
		result += part
	}
	return result
}
