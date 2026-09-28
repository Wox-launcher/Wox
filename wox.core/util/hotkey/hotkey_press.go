package hotkey

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"wox/util"
	"wox/util/keyboard"
)

const doubleModifierPressWindowMs int64 = 500

type modifierPressTrigger struct {
	combo string
	keys  []keyboard.Key
}

type modifierPressRegistration struct {
	keys       []keyboard.Key
	combo      string
	active     bool
	canceled   bool
	exactSeen  bool
	pending    bool
	pendingDue int64
}

// modifierPressTracker recognizes pure left/right modifier press chords. Single
// modifier presses can be delayed so double-modifier hotkeys get first refusal.
type modifierPressTracker struct {
	mu              sync.Mutex
	registrations   map[string]*modifierPressRegistration
	pressed         map[keyboard.Key]bool
	unknownPressed  map[uint32]bool
	suppressedUntil map[keyboard.Key]int64
}

func newModifierPressTracker() *modifierPressTracker {
	return &modifierPressTracker{
		registrations:   map[string]*modifierPressRegistration{},
		pressed:         map[keyboard.Key]bool{},
		unknownPressed:  map[uint32]bool{},
		suppressedUntil: map[keyboard.Key]int64{},
	}
}

func (t *modifierPressTracker) Register(keys []keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	canonicalKeys := canonicalHoldModifierKeys(keys)
	if len(canonicalKeys) == 0 {
		return
	}
	if len(t.registrations) == 0 {
		// Releases can be missed while no press binding owns the listener.
		clear(t.pressed)
		clear(t.unknownPressed)
		clear(t.suppressedUntil)
	}
	combo := holdModifierComboString(canonicalKeys)
	t.registrations[combo] = &modifierPressRegistration{keys: canonicalKeys, combo: combo}
}

func (t *modifierPressTracker) Unregister(keys []keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.registrations, holdModifierComboString(keys))
}

func (t *modifierPressTracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.registrations)
}

// HasKey identifies single modifiers or chords matched during this gesture.
func (t *modifierPressTracker) HasKey(key keyboard.Key) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, registration := range t.registrations {
		if containsHoldModifierKey(registration.keys, key) && (len(registration.keys) == 1 || registration.exactSeen) {
			return true
		}
	}
	return false
}

func (t *modifierPressTracker) HandleEvent(event keyboard.RawKeyEvent, shouldDelaySinglePress func(keyboard.Key) bool, now int64) []modifierPressTrigger {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch event.Type {
	case keyboard.EventTypeKeyDown:
		// Win+L can lose releases while the desktop is locked. Native Windows
		// events can reconcile the old physical state before applying this down.
		if runtime.GOOS == "windows" && event.NativeKeyCode != 0 {
			for code := range t.unknownPressed {
				if !isPressNativeKeyPressed(code) {
					delete(t.unknownPressed, code)
				}
			}
			for key := range t.pressed {
				if key != keyboard.KeyUnknown && !isPressModifierPhysicallyPressed(key) {
					delete(t.pressed, key)
				}
			}
			for _, registration := range t.registrations {
				if !t.anyChordKeyPressedLocked(registration.keys) {
					registration.active = false
				}
			}
		}
		t.cancelPendingLocked(event.Key)
		if event.Key == keyboard.KeyUnknown {
			t.unknownPressed[event.NativeKeyCode] = true
		} else {
			t.pressed[event.Key] = true
		}
		for _, registration := range t.registrations {
			if !containsHoldModifierKey(registration.keys, event.Key) {
				if registration.active {
					registration.canceled = true
				}
				continue
			}
			if !registration.active {
				registration.active = true
				registration.canceled = false
				registration.exactSeen = false
			}
			if len(t.unknownPressed) > 0 {
				registration.canceled = true
			}
			// A key held before the chord started also makes this a combination.
			for key := range t.pressed {
				if !containsHoldModifierKey(registration.keys, key) {
					registration.canceled = true
				}
			}
			if t.exactKeysPressedLocked(registration.keys) {
				registration.exactSeen = true
			}
		}
		return nil
	case keyboard.EventTypeKeyUp:
		delete(t.pressed, event.Key)
		if event.Key == keyboard.KeyUnknown {
			delete(t.unknownPressed, event.NativeKeyCode)
		}

		triggers := []modifierPressTrigger{}
		for _, registration := range t.registrations {
			if !registration.active || !containsHoldModifierKey(registration.keys, event.Key) {
				continue
			}
			if t.anyChordKeyPressedLocked(registration.keys) {
				continue
			}
			if !registration.canceled && registration.exactSeen && !t.pressSuppressedLocked(registration.keys, now) {
				if len(registration.keys) == 1 && shouldDelaySinglePress != nil && shouldDelaySinglePress(registration.keys[0]) {
					registration.pending = true
					registration.pendingDue = now + doubleModifierPressWindowMs
				} else {
					triggers = append(triggers, modifierPressTrigger{combo: registration.combo, keys: registration.keys})
				}
			}
			registration.active = false
			registration.canceled = false
			registration.exactSeen = false
		}
		return triggers
	default:
		return nil
	}
}

func (t *modifierPressTracker) FlushDelayed(now int64) []modifierPressTrigger {
	t.mu.Lock()
	defer t.mu.Unlock()

	triggers := []modifierPressTrigger{}
	for _, registration := range t.registrations {
		if !registration.pending || now < registration.pendingDue {
			continue
		}
		registration.pending = false
		registration.pendingDue = 0
		triggers = append(triggers, modifierPressTrigger{combo: registration.combo, keys: registration.keys})
	}
	return triggers
}

func (t *modifierPressTracker) NextPendingDue() (int64, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var due int64
	for _, registration := range t.registrations {
		if !registration.pending {
			continue
		}
		if due == 0 || registration.pendingDue < due {
			due = registration.pendingDue
		}
	}
	return due, due > 0
}

func (t *modifierPressTracker) CancelPendingForRawKey(key keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cancelPendingLocked(key)
}

// CancelActiveForKeys prevents a modifier chord that has already been consumed
// by another recognizer, such as hold recording, from also completing as a press.
func (t *modifierPressTracker) CancelActiveForKeys(keys []keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, registration := range t.registrations {
		for _, key := range keys {
			if containsHoldModifierKey(registration.keys, key) {
				registration.canceled = true
				registration.pending = false
				registration.pendingDue = 0
				break
			}
		}
	}
}

func (t *modifierPressTracker) SuppressNextPressForRawKey(key keyboard.Key, now int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cancelPendingLocked(key)
	t.suppressedUntil[key] = now + 100
}

func (t *modifierPressTracker) exactKeysPressedLocked(keys []keyboard.Key) bool {
	if len(t.unknownPressed) > 0 {
		return false
	}
	for _, key := range keys {
		if !t.pressed[key] {
			return false
		}
	}
	for key, pressed := range t.pressed {
		if !pressed || containsHoldModifierKey(keys, key) {
			continue
		}
		return false
	}
	return true
}

func (t *modifierPressTracker) anyChordKeyPressedLocked(keys []keyboard.Key) bool {
	for _, key := range keys {
		if t.pressed[key] {
			return true
		}
	}
	return false
}

func (t *modifierPressTracker) cancelPendingLocked(key keyboard.Key) {
	for _, registration := range t.registrations {
		registration.pending = false
		registration.pendingDue = 0
	}
}

func (t *modifierPressTracker) pressSuppressedLocked(keys []keyboard.Key, now int64) bool {
	suppressed := false
	for _, key := range keys {
		until := t.suppressedUntil[key]
		if until == 0 {
			continue
		}
		if now <= until {
			suppressed = true
		}
		delete(t.suppressedUntil, key)
	}
	return suppressed
}

var (
	isPressNativeKeyPressed          = keyboard.IsNativeKeyPressed
	isPressModifierPhysicallyPressed = keyboard.IsKeyPressed
	pressModifierCallbacks           = util.NewHashMap[string, func()]()
	pressModifierTracker             = newModifierPressTracker()
	pressModifierTimerMu             sync.Mutex
	pressModifierFlushTimer          *time.Timer
)

func startPressModifierTracking(keys []keyboard.Key, onPress func()) error {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	canonicalKeys := canonicalHoldModifierKeys(keys)
	combo := holdModifierComboString(canonicalKeys)
	pressModifierCallbacks.Store(combo, onPress)
	pressModifierTracker.Register(canonicalKeys)

	if err := ensureHoldKeyListener(); err != nil {
		pressModifierCallbacks.Delete(combo)
		pressModifierTracker.Unregister(canonicalKeys)
		return err
	}
	return nil
}

func stopPressModifierTracking(keys []keyboard.Key) {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	canonicalKeys := canonicalHoldModifierKeys(keys)
	pressModifierCallbacks.Delete(holdModifierComboString(canonicalKeys))
	pressModifierTracker.Unregister(canonicalKeys)
	closeHoldKeyListenerIfIdle()
	reschedulePressModifierFlush()
}

func handlePressModifierRawEvent(event keyboard.RawKeyEvent) {
	triggers := pressModifierTracker.HandleEvent(event, hasDoubleModifierRegistrationForRawKey, util.GetSystemTimestamp())
	dispatchPressModifierTriggers(triggers)
	reschedulePressModifierFlush()
}

func cancelPressModifierPendingForDouble(rawKey keyboard.Key) {
	pressModifierTracker.SuppressNextPressForRawKey(rawKey, util.GetSystemTimestamp())
	reschedulePressModifierFlush()
}

func dispatchPressModifierTriggers(triggers []modifierPressTrigger) {
	for _, trigger := range triggers {
		callback, ok := pressModifierCallbacks.Load(trigger.combo)
		if !ok || callback == nil {
			continue
		}
		util.Go(util.NewTraceContext(), fmt.Sprintf("press-modifier hotkey: %s", trigger.combo), func() {
			callback()
		})
	}
}

func reschedulePressModifierFlush() {
	pressModifierTimerMu.Lock()
	defer pressModifierTimerMu.Unlock()

	if pressModifierFlushTimer != nil {
		pressModifierFlushTimer.Stop()
		pressModifierFlushTimer = nil
	}

	due, ok := pressModifierTracker.NextPendingDue()
	if !ok {
		return
	}

	now := util.GetSystemTimestamp()
	delay := time.Duration(due-now) * time.Millisecond
	if delay < 0 {
		delay = 0
	}
	pressModifierFlushTimer = time.AfterFunc(delay, func() {
		triggers := pressModifierTracker.FlushDelayed(util.GetSystemTimestamp())
		dispatchPressModifierTriggers(triggers)
		reschedulePressModifierFlush()
	})
}
