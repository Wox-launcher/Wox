//go:build linux && cgo

package keyboard

import (
	"fmt"
	"strings"
	"wox/util"
	"wox/util/screen"

	"github.com/google/uuid"
)

func init() {
	standaloneHoldListenerPlatform = addStandaloneHoldListenerHyprland
}

type hyprlandStandaloneHold struct {
	*hyprlandRawSubscription
}

// addStandaloneHoldListenerHyprland consumes only the unmodified binding; all
// other keyboard input is observed solely to interrupt a pending gesture.
func addStandaloneHoldListenerHyprland(key Key, handler RawKeyHandler) (StandaloneHoldSubscription, bool, error) {
	if !util.IsHyprlandSession() {
		return nil, false, nil
	}
	if !SupportsStandaloneHold(key) {
		return nil, true, fmt.Errorf("unsupported Hyprland standalone hold key")
	}
	available, err := isHyprlandGlobalHotkeyAvailableLinux(0, key)
	if err != nil {
		return nil, true, fmt.Errorf("check Hyprland key %s: %w", key.Character(), err)
	}
	if !available {
		return nil, true, fmt.Errorf("Hyprland key %s is already configured", key.Character())
	}
	code := -1
	for i := 0; i < 768; i++ {
		if evdevCodeToKey(uint16(i)) == key {
			code = i + 8
			break
		}
	}
	if code < 0 {
		return nil, true, fmt.Errorf("no Hyprland key code for %s", key.Character())
	}
	s := &hyprlandRawSubscription{
		name: "wox_hold_key_" + uuid.NewString(),
		eval: func(script string) (string, error) {
			response, err := screen.ReadHyprlandIPC("/repl "+script, 16384)
			return strings.TrimSpace(string(response)), err
		},
		stop: make(chan struct{}),
	}
	s.install = hyprlandStandaloneHoldInstallScript(s.name, hyprlandKeyName(key), code)
	if err := s.start(handler); err != nil {
		return nil, true, err
	}
	return &hyprlandStandaloneHold{s}, true, nil
}

// ReplayTap runs in the compositor so focus comparison and injection are atomic.
func (s *hyprlandStandaloneHold) ReplayTap(_ uint32) error {
	s.ipcMu.Lock()
	defer s.ipcMu.Unlock()
	response, err := s.eval(fmt.Sprintf(`local s = _G[%q]; if not s then return "missing" end; return s.replay()`, s.name))
	if err != nil {
		return err
	}
	if response != "replayed" && response != "focus_changed" {
		return fmt.Errorf("Hyprland tap replay: %s", response)
	}
	return nil
}

// hyprlandStandaloneHoldInstallScript leases the consuming bindings so a Wox
// crash cannot permanently steal the key. Only the original focused window can
// receive a replayed short tap.
func hyprlandStandaloneHoldInstallScript(name, key string, code int) string {
	return fmt.Sprintf(`local name, key, code = %q, %q, %d
if _G[name] then return "ready" end
if type(hl.on) ~= "function" or type(hl.timer) ~= "function" or type(hl.bind) ~= "function" or not hl.dsp or not hl.dsp.send_shortcut then return "unsupported" end
local s = {queue = {}, down = false}
function s.enqueue(eventCode, state)
  if #s.queue >= 256 then s.queue = {"reset"} end
  s.queue[#s.queue + 1] = eventCode .. "," .. state
end
function s.close()
  if s.hook then s.hook:remove(); s.hook = nil end
  if s.press then s.press:set_enabled(false); s.press = nil end
  if s.release then s.release:set_enabled(false); s.release = nil end
  s.queue = {}
  if _G[name] == s then _G[name] = nil end
end
function s.replay()
  local target, active = s.target, hl.get_active_window()
  s.target = nil
  if not target or not active or target ~= active then return "focus_changed" end
  local result = hl.dispatch(hl.dsp.send_shortcut({key = key, mods = "", window = active}))
  if result and result.ok == false then return "failed" end
  return "replayed"
end
s.timer = hl.timer(s.close, {timeout = 2000, type = "oneshot"})
s.hook = hl.on("input.keyboard.key", function(eventCode, timestamp, state)
  if state ~= 0 and state ~= 1 then return end
  if eventCode == code then
    if state == 0 and s.down then s.down = false; s.enqueue(code, 0) end
  elseif s.down then
    s.enqueue(eventCode, state)
  end
end)
s.press = hl.bind(key, function()
  if not s.down then
    s.down = true
    s.target = hl.get_active_window()
    s.enqueue(code, 1)
  end
end, {description = %q})
s.release = hl.bind(key, function() end, {release = true, description = %q})
if not s.hook or not s.press or not s.release then s.close(); return "unsupported" end
_G[name] = s
return "ready"`, name, key, code, hyprlandBindDescription, hyprlandBindDescription)
}
