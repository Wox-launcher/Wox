//go:build linux && cgo

package keyboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"wox/util/screen"
)

func TestParseHyprlandRawKeys(t *testing.T) {
	events, reset, err := parseHyprlandRawKeys("keys:37,1;38,1;38,0;37,0")
	if err != nil || reset || len(events) != 4 {
		t.Fatalf("parse: events=%v reset=%v err=%v", events, reset, err)
	}
	want := []RawKeyEvent{
		{Type: EventTypeKeyDown, Key: KeyLeftCtrl, Character: "left_ctrl", NativeKeyCode: 29},
		{Type: EventTypeKeyDown, Key: KeyA, Character: "a", NativeKeyCode: 30},
		{Type: EventTypeKeyUp, Key: KeyA, Character: "a", NativeKeyCode: 30},
		{Type: EventTypeKeyUp, Key: KeyLeftCtrl, Character: "left_ctrl", NativeKeyCode: 29},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %#v, want %#v", events, want)
	}
	for _, response := range []string{"ok", "error: unsupported", "keys:7,1", "keys:776,1", "keys:38,2", "keys:38,1;broken"} {
		if events, _, err := parseHyprlandRawKeys(response); err == nil || len(events) != 0 {
			t.Errorf("accepted malformed batch %q", response)
		}
	}
	events, reset, err = parseHyprlandRawKeys("keys:37,1;reset;38,1")
	if err != nil || !reset || len(events) != 1 || events[0].Key != KeyA {
		t.Fatalf("overflow: events=%v reset=%v err=%v", events, reset, err)
	}
}

func TestHyprlandRawKeyLifecycle(t *testing.T) {
	var installs, polls, closes atomic.Int32
	events := make(chan RawKeyEvent, 16)
	sub, err := startHyprlandRawKeyListener(func(event RawKeyEvent) bool {
		events <- event
		return false
	}, func(script string) (string, error) {
		switch {
		case strings.Contains(script, `return "ready"`):
			installs.Add(1)
			return "ready", nil
		case strings.Contains(script, `return "closed"`):
			closes.Add(1)
			return "closed", nil
		default:
			switch polls.Add(1) {
			case 1:
				return "keys:37,1;37,1", nil // Duplicate presses must not repeat.
			case 2:
				return "missing", nil // Reload loses the held Control release.
			case 3:
				return "keys:38,1", nil
			case 4:
				return "", errors.New("IPC disconnected")
			default:
				return "keys:", nil
			}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	for _, want := range []struct {
		key       Key
		eventType EventType
	}{{KeyLeftCtrl, EventTypeKeyDown}, {KeyLeftCtrl, EventTypeKeyUp}, {KeyA, EventTypeKeyDown}, {KeyA, EventTypeKeyUp}} {
		select {
		case got := <-events:
			if got.Key != want.key || got.Type != want.eventType {
				t.Fatalf("event = %+v, want %+v", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("no keyboard event delivered")
		}
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if err := sub.Close(); err != nil || closes.Load() != 1 || installs.Load() != 2 {
		t.Fatalf("close/reload lifecycle: installs=%d closes=%d err=%v", installs.Load(), closes.Load(), err)
	}
	if len(events) != 0 {
		t.Fatal("unexpected extra keyboard events")
	}
}

func TestHyprlandRawKeyFailedInstallCleansUp(t *testing.T) {
	for _, installErr := range []error{nil, errors.New("IPC timeout")} {
		calls := 0
		sub, err := startHyprlandRawKeyListener(func(RawKeyEvent) bool { return false }, func(script string) (string, error) {
			calls++
			if calls == 1 {
				return "unsupported", installErr
			}
			if !strings.Contains(script, `return "closed"`) {
				t.Fatal("failed installation did not request cleanup")
			}
			return "closed", nil
		})
		if err == nil || sub != nil || calls != 2 {
			t.Fatalf("failed install: sub=%v err=%v calls=%d", sub, err, calls)
		}
	}
}

// TestHyprlandRawKeyLuaQueue executes the actual scripts with a minimal compositor
// API, covering queue bounds, draining, independent handles, and crash expiry.
func TestHyprlandRawKeyLuaQueue(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("Lua interpreter is needed for the compositor script fixture")
	}
	script := `
local hooks, timers = {}, {}
hl = {}
function hl.on(name, callback)
  assert(name == "input.keyboard.key")
  local hook = {callback = callback, active = true}
  function hook:remove() self.active = false end
  hooks[#hooks + 1] = hook
  return hook
end
function hl.timer(callback, opts)
  assert(opts.timeout == 2000 and opts.type == "oneshot")
  local timer = {callback = callback}
  function timer:set_timeout(ms) assert(ms == 2000); self.renewed = true end
  timers[#timers + 1] = timer
  return timer
end
`
	script += fmt.Sprintf("local install = assert(load(%q))\nlocal poll = assert(load(%q))\nlocal close = assert(load(%q))\n", hyprlandRawKeyInstallScript("test_a"), hyprlandRawKeyPollScript("test_a"), hyprlandRawKeyCloseScript("test_a"))
	script += `
assert(install() == "ready")
assert(install() == "ready" and #hooks == 1)
hooks[1].callback(37, 100, 1)
hooks[1].callback(38, 101, 2)
hooks[1].callback(37, 102, 0)
assert(poll() == "keys:37,1;37,0")
assert(timers[1].renewed and poll() == "keys:")
for i = 1, 300 do hooks[1].callback(38, i, 1) end
assert(#test_a.queue <= 256 and poll():match("^keys:reset;"))
`
	script += fmt.Sprintf("assert(assert(load(%q))() == 'ready')\n", hyprlandRawKeyInstallScript("test_b"))
	script += `
assert(close() == "closed" and close() == "closed")
assert(not hooks[1].active and hooks[2].active and test_a == nil)
assert(poll() == "missing")
hooks[2].callback(38, 100, 1)
timers[2].callback()
assert(not hooks[2].active and test_b == nil)
assert(install() == "ready")
timers[1].callback() -- An old timer must not clear a new subscription.
assert(test_a ~= nil and hooks[3].active)
timers[3].callback()
assert(test_a == nil and not hooks[3].active)
`
	cmd := exec.Command(lua, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Lua queue fixture: %v\n%s", err, output)
	}
}

// TestHyprlandRawKeyLive checks native delivery and cleanup in an opted-in desktop
// session. It injects only one Shift press/release, without typing text.
func TestHyprlandRawKeyLive(t *testing.T) {
	if os.Getenv("WOX_TEST_HYPRLAND_RAW_KEYS") != "1" {
		t.Skip("set WOX_TEST_HYPRLAND_RAW_KEYS=1 inside Hyprland for native verification")
	}
	eval := func(script string) (string, error) {
		response, err := screen.ReadHyprlandIPC("/repl "+script, 16384)
		return strings.TrimSpace(string(response)), err
	}
	if response, err := eval("return hl.is_key_down(50)"); err != nil || response != "false" {
		t.Skip("Shift is already held or the desktop is unavailable")
	}
	events := make(chan EventType, 8)
	sub, err := startHyprlandRawKeyListener(func(event RawKeyEvent) bool {
		// wtype assigns its first keysym XKB code 9 (evdev 1), even for Shift.
		if event.NativeKeyCode == 1 {
			select {
			case events <- event.Type:
			default:
			}
		}
		return false
	}, eval)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sub.Close() })
	cmd := exec.Command("wtype", "-P", "Shift_L", "-s", "40", "-p", "Shift_L")
	if output, err := cmd.CombinedOutput(); err != nil {
		_ = exec.Command("wtype", "-p", "Shift_L").Run()
		t.Fatalf("virtual keyboard fixture: %v %s", err, output)
	}
	for _, want := range []EventType{EventTypeKeyDown, EventTypeKeyUp} {
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("Shift event type = %v, want %v", got, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Hyprland did not deliver the Shift event")
		}
	}
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	if response, err := eval(hyprlandRawKeyPollScript(sub.name)); err != nil || response != "missing" {
		t.Fatalf("native listener was not removed: %q, %v", response, err)
	}
	// Install without a polling client to simulate abrupt process exit.
	if response, err := eval(hyprlandRawKeyInstallScript(sub.name)); err != nil || response != "ready" {
		t.Fatalf("lease setup: %q, %v", response, err)
	}
	t.Cleanup(func() { _, _ = eval(hyprlandRawKeyCloseScript(sub.name)) })
	time.Sleep(2200 * time.Millisecond)
	if response, err := eval(hyprlandRawKeyPollScript(sub.name)); err != nil || response != "missing" {
		t.Fatalf("native listener did not expire: %q, %v", response, err)
	}
}

func TestHyprlandRawKeyCallbackCanCloseSubscription(t *testing.T) {
	ready := make(chan struct{})
	closed := make(chan error, 1)
	var sub *hyprlandRawSubscription
	var err error
	sub, err = startHyprlandRawKeyListener(func(RawKeyEvent) bool {
		<-ready
		closed <- sub.Close()
		return false
	}, func(script string) (string, error) {
		switch {
		case strings.Contains(script, `return "ready"`):
			return "ready", nil
		case strings.Contains(script, `return "closed"`):
			return "closed", nil
		default:
			return "keys:38,1;38,0", nil
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	close(ready)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("closing from a callback deadlocked")
	}
}
