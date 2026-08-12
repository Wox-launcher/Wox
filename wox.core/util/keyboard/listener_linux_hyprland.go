//go:build linux && cgo

package keyboard

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"
)

// Hyprland's xdg-desktop-portal-hyprland implements the GlobalShortcuts portal
// but does not auto-bind keys from preferred_trigger. Its own
// hyprland_global_shortcuts_manager_v1 Wayland protocol also requires the
// compositor config to bind keys manually. On Hyprland with the Lua config
// (0.55+), traditional "bind = ..." syntax is not available, so we use the
// Lua API hl.bind + hl.dsp.exec_cmd to register keybindings that invoke
// wox:// deeplinks. The secondary wox process forwards the deeplink to the
// running instance via HTTP, just like the GNOME custom-keybinding fallback.
//
// This backend is chosen when IsHyprlandSession() is true. It takes priority
// over the portal backend because the portal alone cannot deliver key events
// on Hyprland without manual compositor-side bind configuration.

const (
	hyprlandBindsLuaName = "wox-binds.lua"
)

// hyprlandMu guards bind file updates and compositor evaluations so a new
// registration cannot race with teardown of the previous registration.
var hyprlandMu sync.Mutex

type hyprlandBinding struct {
	luaKey   string
	deeplink string
}

func isHyprlandSession() bool {
	return util.IsHyprlandSession()
}

// hyprlandKeyToLuaKey converts a Wox modifier+key combination to the Hyprland
// Lua config key string used by hl.bind. E.g. ModifierAlt+KeySpace -> "ALT + SPACE".
func hyprlandKeyToLuaKey(modifiers Modifier, key Key) string {
	var parts []string
	if modifiers&ModifierCtrl != 0 {
		parts = append(parts, "CTRL")
	}
	if modifiers&ModifierAlt != 0 {
		parts = append(parts, "ALT")
	}
	if modifiers&ModifierShift != 0 {
		parts = append(parts, "SHIFT")
	}
	if modifiers&ModifierSuper != 0 {
		parts = append(parts, "SUPER")
	}
	parts = append(parts, hyprlandKeyName(key))
	return strings.Join(parts, " + ")
}

func hyprlandKeyName(key Key) string {
	switch key {
	case KeySpace:
		return "SPACE"
	case KeyReturn:
		return "RETURN"
	case KeyEscape:
		return "ESCAPE"
	case KeyTab:
		return "TAB"
	case KeyDelete:
		return "DELETE"
	case KeyLeft:
		return "LEFT"
	case KeyRight:
		return "RIGHT"
	case KeyUp:
		return "UP"
	case KeyDown:
		return "DOWN"
	case KeyCapsLock:
		return "CAPSLOCK"
	case KeyBackquote:
		return "GRAVE"
	default:
		if key >= KeyA && key <= KeyZ {
			return string(rune('A' + (key - KeyA)))
		}
		if key >= Key0 && key <= Key9 {
			return string(rune('0' + (key - Key0)))
		}
		if key >= KeyF1 && key <= KeyF12 {
			return fmt.Sprintf("F%d", int(key-KeyF1)+1)
		}
		return "UNKNOWN"
	}
}

// registerGlobalHotkeysLinuxHyprland binds all Wox shortcuts via Hyprland's Lua
// config API (hl.bind + hl.dsp.exec_cmd) by generating a Lua file and loading
// it with hyprctl eval dofile().
func registerGlobalHotkeysLinuxHyprland(specs []GlobalHotkeySpec) (HotkeyRegistration, bool, error) {
	if !isHyprlandSession() {
		return nil, false, nil
	}
	if len(specs) == 0 {
		return &hyprlandHotkeyRegistration{}, true, nil
	}

	hyprlandMu.Lock()
	defer hyprlandMu.Unlock()

	bindings := make([]hyprlandBinding, 0, len(specs))
	callbackKeys := make([]string, 0, len(specs)-1)
	for i, spec := range specs {
		luaKey := hyprlandKeyToLuaKey(spec.Modifiers, spec.Key)
		var deeplink string
		if i == 0 {
			deeplink = "wox://toggle"
		} else {
			deeplink = fmt.Sprintf("wox://hyprland-hotkey?key=%s", url.QueryEscape(luaKey))
		}
		bindings = append(bindings, hyprlandBinding{
			luaKey:   luaKey,
			deeplink: deeplink,
		})
		if i > 0 {
			callbackKeys = append(callbackKeys, luaKey)
		}
	}

	if err := hyprlandWriteAndLoadBinds(bindings); err != nil {
		return nil, true, fmt.Errorf("failed to register Hyprland global hotkeys: %w", err)
	}
	for i, callbackKey := range callbackKeys {
		RegisterHyprlandHotkeyCallback(callbackKey, specs[i+1].Callback)
	}

	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] Hyprland registered %d shortcuts via hl.bind", len(specs)))

	return &hyprlandHotkeyRegistration{
		luaKeys:      hyprlandBindingKeys(bindings),
		callbackKeys: callbackKeys,
	}, true, nil
}

func hyprlandWriteAndLoadBinds(bindings []hyprlandBinding) error {
	luaPath, err := hyprlandBindsLuaPath()
	if err != nil {
		return err
	}

	// Determine the wox executable path for the bind command. We prefer the
	// APPIMAGE env var (set when running as AppImage) so the bind targets the
	// same binary the user launched. The secondary wox process detects the
	// running instance via the lock file and forwards the deeplink via HTTP,
	// then exits immediately.
	woxExec := os.Getenv("APPIMAGE")
	if woxExec == "" {
		woxExec, err = os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get wox executable path: %w", err)
		}
	}

	if err := os.WriteFile(luaPath, []byte(hyprlandBindScript(bindings, woxExec)), 0644); err != nil {
		return fmt.Errorf("failed to write Hyprland binds file: %w", err)
	}
	return hyprlandEvalFile(luaPath)
}

// hyprlandBindScript replaces the compositor-side Wox bind group in one Lua
// evaluation. hl.bind accumulates handlers, so the previous group must be
// explicitly removed before a changed setting can take effect.
func hyprlandBindScript(bindings []hyprlandBinding, woxExec string) string {
	var sb strings.Builder
	sb.WriteString("-- Auto-generated by Wox. Do not edit.\n")
	sb.WriteString("-- Hyprland global hotkey bindings for Wox launcher.\n")
	sb.WriteString("if wox_bound_keys then\n")
	sb.WriteString("  for _, key in ipairs(wox_bound_keys) do hl.unbind(key) end\n")
	sb.WriteString("end\n")
	sb.WriteString("wox_bound_keys = {}\n")
	for _, b := range bindings {
		cmd := fmt.Sprintf("%s %s", woxExec, b.deeplink)
		sb.WriteString(fmt.Sprintf("hl.bind(%q, hl.dsp.exec_cmd(%q), { repeating = false })\n", b.luaKey, cmd))
		sb.WriteString(fmt.Sprintf("table.insert(wox_bound_keys, %q)\n", b.luaKey))
	}
	return sb.String()
}

func hyprlandBindingKeys(bindings []hyprlandBinding) []string {
	keys := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		keys = append(keys, binding.luaKey)
	}
	return keys
}

// hyprlandUnbindScript removes the compositor-tracked group, falling back to
// the registration snapshot if compositor state was lost or predates tracking.
func hyprlandUnbindScript(luaKeys []string) string {
	var sb strings.Builder
	sb.WriteString("-- Auto-generated by Wox. All bindings cleared.\n")
	sb.WriteString("local keys = wox_bound_keys or {")
	for i, luaKey := range luaKeys {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%q", luaKey))
	}
	sb.WriteString("}\n")
	sb.WriteString("for _, key in ipairs(keys) do hl.unbind(key) end\n")
	sb.WriteString("wox_bound_keys = nil\n")
	return sb.String()
}

func hyprlandEvalFile(luaPath string) error {
	// Hyprland 0.55+ evaluates Lua passed to hyprctl eval, so dofile loads the
	// generated bind transaction into the running compositor.
	evalCmd := exec.Command("hyprctl", "eval", fmt.Sprintf("dofile(%q)", luaPath))
	output, err := evalCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hyprctl eval dofile failed: %w, output: %s", err, string(output))
	}
	if !strings.Contains(string(output), "ok") {
		return fmt.Errorf("hyprctl eval dofile returned unexpected output: %s", string(output))
	}
	return nil
}

func hyprlandBindsLuaPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".config", "hypr", hyprlandBindsLuaName), nil
}

type hyprlandHotkeyRegistration struct {
	luaKeys      []string
	callbackKeys []string
	unregistered bool
}

func (r *hyprlandHotkeyRegistration) Unregister() error {
	hyprlandMu.Lock()
	defer hyprlandMu.Unlock()
	if r == nil || r.unregistered {
		return nil
	}

	luaPath, err := hyprlandBindsLuaPath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(luaPath, []byte(hyprlandUnbindScript(r.luaKeys)), 0644); err != nil {
		return fmt.Errorf("failed to clear Hyprland binds file: %w", err)
	}
	if err := hyprlandEvalFile(luaPath); err != nil {
		return fmt.Errorf("failed to unregister Hyprland global hotkeys: %w", err)
	}
	unregisterHyprlandHotkeyCallbacks(r.callbackKeys)
	r.unregistered = true
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] Hyprland unregistered %d shortcuts via hl.unbind", len(r.luaKeys)))
	return nil
}

// InvokeHyprlandHotkeyCallback dispatches a Hyprland hotkey deeplink to the
// registered callback. Called by ProcessDeeplink when a wox://hyprland-hotkey
// deeplink is received. The key is the Hyprland Lua key string (e.g. "CTRL + K").
var (
	hyprlandCallbacksMu sync.Mutex
	hyprlandCallbacks   = map[string]func(){}
	// hyprlandLastFired debounces rapid repeat invocations of the same hotkey.
	// Hyprland key-repeat or fast double-press can fire the bind callback
	// multiple times in quick succession, causing the main instance to receive
	// multiple toggle deeplinks and shut itself down.
	hyprlandLastFired = map[string]time.Time{}
)

const hyprlandHotkeyDebounce = 300 * time.Millisecond

func RegisterHyprlandHotkeyCallback(key string, callback func()) {
	hyprlandCallbacksMu.Lock()
	hyprlandCallbacks[key] = callback
	hyprlandCallbacksMu.Unlock()
}

// unregisterHyprlandHotkeyCallbacks removes callbacks owned by a native bind group.
func unregisterHyprlandHotkeyCallbacks(keys []string) {
	hyprlandCallbacksMu.Lock()
	defer hyprlandCallbacksMu.Unlock()
	for _, key := range keys {
		delete(hyprlandCallbacks, key)
		delete(hyprlandLastFired, key)
	}
}

func InvokeHyprlandHotkeyCallback(key string) {
	hyprlandCallbacksMu.Lock()
	last := hyprlandLastFired[key]
	now := time.Now()
	if now.Sub(last) < hyprlandHotkeyDebounce {
		hyprlandCallbacksMu.Unlock()
		return
	}
	hyprlandLastFired[key] = now
	cb := hyprlandCallbacks[key]
	hyprlandCallbacksMu.Unlock()
	if cb != nil {
		cb()
	}
}
