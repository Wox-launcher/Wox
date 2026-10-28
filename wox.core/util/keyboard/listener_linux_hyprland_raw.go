//go:build linux && cgo

package keyboard

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/util/screen"

	"github.com/google/uuid"
)

type hyprlandRawSubscription struct {
	name     string
	eval     func(string) (string, error)
	stop     chan struct{}
	ipcMu    sync.Mutex
	once     sync.Once
	closeErr error
}

// addRawKeyListenerLinuxHyprland observes compositor events without grabbing the
// keyboard or requiring /dev/input access. Like evdev, it cannot consume events.
func addRawKeyListenerLinuxHyprland(handler RawKeyHandler) (RawKeySubscription, error) {
	subscription, err := startHyprlandRawKeyListener(handler, func(script string) (string, error) {
		response, err := screen.ReadHyprlandIPC("/repl "+script, 16384)
		return strings.TrimSpace(string(response)), err
	})
	if err != nil {
		return nil, err
	}
	return subscription, nil
}

// startHyprlandRawKeyListener keeps each subscription independent, including
// subscriptions belonging to another Wox process or another recording session.
func startHyprlandRawKeyListener(handler RawKeyHandler, eval func(string) (string, error)) (*hyprlandRawSubscription, error) {
	if handler == nil {
		return nil, fmt.Errorf("raw key handler is required")
	}
	s := &hyprlandRawSubscription{
		name: "wox_raw_keys_" + uuid.NewString(), eval: eval,
		stop: make(chan struct{}),
	}
	response, err := eval(hyprlandRawKeyInstallScript(s.name))
	if err != nil || response != "ready" {
		// A timed-out request may still have installed the hook.
		_, _ = eval(hyprlandRawKeyCloseScript(s.name))
		if err != nil {
			return nil, fmt.Errorf("Hyprland keyboard listener: %w", err)
		}
		return nil, fmt.Errorf("Hyprland keyboard listener requires Lua input.keyboard.key events")
	}
	go s.listen(handler)
	return s, nil
}

// Close removes the compositor hook without waiting on an active callback, which
// may itself close this subscription. The lease covers failed IPC cleanup.
func (s *hyprlandRawSubscription) Close() error {
	s.once.Do(func() {
		close(s.stop)
		s.ipcMu.Lock()
		defer s.ipcMu.Unlock()
		response, err := s.eval(hyprlandRawKeyCloseScript(s.name))
		s.closeErr = err
		if err == nil && response != "closed" {
			s.closeErr = fmt.Errorf("could not remove Hyprland keyboard listener")
		}
	})
	return s.closeErr
}

// listen drains a bounded in-memory queue through IPC, avoiding per-key processes,
// files, and broadcasting typed keys on Hyprland's public event socket.
func (s *hyprlandRawSubscription) listen(handler RawKeyHandler) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var pressed [768]bool
	release := func() {
		// A reload, overflow, or lost IPC reply can lose key-up events. Release
		// held keys so consumers do not retain a stuck modifier afterwards.
		for code, down := range pressed {
			select {
			case <-s.stop:
				return
			default:
			}
			if down {
				pressed[code] = false
				key := evdevCodeToKey(uint16(code))
				handler(RawKeyEvent{Type: EventTypeKeyUp, Key: key, Character: key.Character(), NativeKeyCode: uint32(code)})
			}
		}
	}
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
		// Serialize IPC with Close so an in-flight reload cannot reinstall a
		// hook after cleanup. Callbacks never run under this lock.
		s.ipcMu.Lock()
		select {
		case <-s.stop:
			s.ipcMu.Unlock()
			return
		default:
		}
		response, err := s.eval(hyprlandRawKeyPollScript(s.name))
		if err == nil && response == "missing" {
			_, _ = s.eval(hyprlandRawKeyInstallScript(s.name))
		}
		s.ipcMu.Unlock()
		select {
		case <-s.stop:
			return
		default:
		}
		if err != nil {
			release()
			continue
		}
		if response == "missing" {
			release()
			continue
		}
		events, reset, err := parseHyprlandRawKeys(response)
		if reset || err != nil {
			release()
		}
		if err != nil {
			continue
		}
		for _, event := range events {
			select {
			case <-s.stop:
				return
			default:
			}
			down := event.Type == EventTypeKeyDown
			if down && pressed[event.NativeKeyCode] {
				continue
			}
			pressed[event.NativeKeyCode] = down
			handler(event)
		}
	}
}

// hyprlandRawKeyInstallScript leases the hook for two seconds. Every successful
// poll renews it; after a crash the timer clears both the hook and buffered keys.
func hyprlandRawKeyInstallScript(name string) string {
	return fmt.Sprintf(`local name = %q
if _G[name] then return "ready" end
if type(hl.on) ~= "function" or type(hl.timer) ~= "function" then return "unsupported" end
local s = {queue = {}}
function s.close()
  if s.hook then s.hook:remove(); s.hook = nil end
  s.queue = {}
  if _G[name] == s then _G[name] = nil end
end
s.timer = hl.timer(s.close, {timeout = 2000, type = "oneshot"})
s.hook = hl.on("input.keyboard.key", function(code, timestamp, state)
  if state ~= 0 and state ~= 1 then return end
  if #s.queue >= 256 then s.queue = {"reset"} end
  s.queue[#s.queue + 1] = code .. "," .. state
end)
if not s.hook then s.close(); return "unsupported" end
_G[name] = s
return "ready"`, name)
}

// hyprlandRawKeyPollScript renews the lease and atomically drains queued events.
func hyprlandRawKeyPollScript(name string) string {
	return fmt.Sprintf(`local s = _G[%q]
if not s then return "missing" end
s.timer:set_timeout(2000)
local keys = table.concat(s.queue, ";")
s.queue = {}
return "keys:" .. keys`, name)
}

func hyprlandRawKeyCloseScript(name string) string {
	return fmt.Sprintf(`local s = _G[%q]; if s then s.close() end; return "closed"`, name)
}

// parseHyprlandRawKeys converts XKB codes (evdev + 8) to the same event identity
// as the Linux evdev backend. Invalid batches are discarded as a whole.
func parseHyprlandRawKeys(response string) ([]RawKeyEvent, bool, error) {
	keys, ok := strings.CutPrefix(response, "keys:")
	if !ok {
		return nil, false, fmt.Errorf("invalid Hyprland keyboard response")
	}
	var events []RawKeyEvent
	reset := false
	for _, record := range strings.Split(keys, ";") {
		if record == "" {
			continue
		}
		if record == "reset" {
			reset = true
			events = nil
			continue
		}
		codeText, state, found := strings.Cut(record, ",")
		code, err := strconv.Atoi(codeText)
		if !found || err != nil || code < 8 || code >= 776 || (state != "0" && state != "1") {
			return nil, false, fmt.Errorf("invalid Hyprland keyboard event")
		}
		key := evdevCodeToKey(uint16(code - 8))
		event := RawKeyEvent{Type: EventTypeKeyDown, Key: key, Character: key.Character(), NativeKeyCode: uint32(code - 8)}
		if state == "0" {
			event.Type = EventTypeKeyUp
		}
		events = append(events, event)
	}
	return events, reset, nil
}
