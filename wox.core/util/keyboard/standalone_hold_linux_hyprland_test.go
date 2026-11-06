//go:build linux && cgo

package keyboard

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// TestHyprlandStandaloneHoldScript exercises interception, replay, and cleanup in Lua.
func TestHyprlandStandaloneHoldScript(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("Lua interpreter is needed for the compositor script fixture")
	}
	script := `
local hooks, binds, timers, sent = {}, {}, {}, {}
local active = {address = "one"}
hl = {dsp = {}}
function hl.get_active_window() return active end
function hl.dsp.send_shortcut(args) return args end
function hl.dispatch(args) sent[#sent + 1] = args end
function hl.on(name, callback)
  local h = {callback = callback, enabled = true}
  function h:remove() self.enabled = false end
  hooks[#hooks + 1] = h
  return h
end
function hl.timer(callback, opts)
  local t = {callback = callback}
  function t:set_timeout(ms) assert(ms == 2000) end
  timers[#timers + 1] = t
  return t
end
function hl.bind(key, callback, opts)
  local b = {key = key, callback = callback, opts = opts, enabled = true}
  function b:set_enabled(value) self.enabled = value end
  binds[#binds + 1] = b
  return b
end
`
	script += fmt.Sprintf("local install = assert(load(%q))\nlocal poll = assert(load(%q))\nlocal close = assert(load(%q))\n", hyprlandStandaloneHoldInstallScript("test_hold", "DELETE", 119), hyprlandRawKeyPollScript("test_hold"), hyprlandRawKeyCloseScript("test_hold"))
	script += `
assert(install() == "ready" and #binds == 2)
assert(binds[1].key == "DELETE" and binds[2].opts.release)
-- Physical key events precede the matching bind callback.
hooks[1].callback(119, 1, 1); binds[1].callback()
hooks[1].callback(119, 2, 2); hooks[1].callback(119, 3, 0)
assert(poll() == "keys:119,1;119,0")
assert(test_hold.replay() == "replayed" and #sent == 1)
assert(sent[1].key == "DELETE" and sent[1].window == active)
assert(test_hold.replay() == "focus_changed" and #sent == 1)
binds[1].callback()
hooks[1].callback(38, 4, 1)
assert(poll() == "keys:119,1;38,1")
active = {address = "two"}
assert(test_hold.replay() == "focus_changed" and #sent == 1)
hooks[1].callback(119, 5, 0)
assert(poll() == "keys:119,0")
assert(close() == "closed" and not hooks[1].enabled)
assert(not binds[1].enabled and not binds[2].enabled)
assert(install() == "ready")
timers[2].callback()
assert(test_hold == nil and not binds[3].enabled and not binds[4].enabled)
`
	cmd := exec.Command(lua, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Lua standalone hold fixture: %v\n%s", err, output)
	}
}
