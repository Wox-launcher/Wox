//go:build linux && cgo

package keyboard

import (
	"encoding/json"
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
	hyprlandBindsLuaName    = "wox-binds.lua"
	hyprlandBindDescription = "Wox global hotkey"
)

// hyprlandMu guards bind file updates and compositor evaluations so a new
// registration cannot race with teardown of the previous registration.
var hyprlandMu sync.Mutex

type hyprlandBinding struct {
	luaKey        string
	deeplink      string
	nativeModMask uint32
	nativeKey     string
}

// hyprlandConfiguredBind is the conflict-relevant subset of hyprctl bind data.
type hyprlandConfiguredBind struct {
	ModMask     uint32 `json:"modmask"`
	Key         string `json:"key"`
	Submap      string `json:"submap"`
	Description string `json:"description"`
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
		return &hyprlandHotkeyRegistration{unregistered: true}, true, nil
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
			luaKey:        luaKey,
			deeplink:      deeplink,
			nativeModMask: hyprlandKeyToModMask(spec.Modifiers),
			nativeKey:     hyprlandKeyName(spec.Key),
		})
		if i > 0 {
			callbackKeys = append(callbackKeys, luaKey)
		}
	}
	if err := validateHyprlandBindingConflicts(bindings); err != nil {
		return nil, true, err
	}

	if err := hyprlandWriteAndLoadBinds(bindings); err != nil {
		return nil, true, fmt.Errorf("failed to register Hyprland global hotkeys: %w", err)
	}
	for i, callbackKey := range callbackKeys {
		RegisterHyprlandHotkeyCallback(callbackKey, specs[i+1].Callback)
	}

	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] Hyprland registered %d shortcuts via hl.bind", len(specs)))

	return &hyprlandHotkeyRegistration{callbackKeys: callbackKeys}, true, nil
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
// evaluation. Bind handles let Wox disable only its own handlers without
// removing a user's Hyprland binding for the same key.
func hyprlandBindScript(bindings []hyprlandBinding, woxExec string) string {
	var sb strings.Builder
	sb.WriteString("-- Auto-generated by Wox. Do not edit.\n")
	sb.WriteString("-- Hyprland global hotkey bindings for Wox launcher.\n")
	sb.WriteString("if wox_bind_handles then\n")
	sb.WriteString("  for _, bind in ipairs(wox_bind_handles) do bind:set_enabled(false) end\n")
	sb.WriteString("end\n")
	sb.WriteString("wox_bind_handles = {}\n")
	for _, b := range bindings {
		cmd := fmt.Sprintf("%s %s", woxExec, b.deeplink)
		sb.WriteString(fmt.Sprintf("table.insert(wox_bind_handles, hl.bind(%q, hl.dsp.exec_cmd(%q), { repeating = false, description = %q }))\n", b.luaKey, cmd, hyprlandBindDescription))
	}
	return sb.String()
}

// hyprlandDisableScript disables only the handles created by Wox.
func hyprlandDisableScript() string {
	return "-- Auto-generated by Wox. All bindings disabled.\n" +
		"if wox_bind_handles then\n" +
		"  for _, bind in ipairs(wox_bind_handles) do bind:set_enabled(false) end\n" +
		"end\n" +
		"wox_bind_handles = nil\n"
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
	if err := os.WriteFile(luaPath, []byte(hyprlandDisableScript()), 0644); err != nil {
		return fmt.Errorf("failed to clear Hyprland binds file: %w", err)
	}
	if err := hyprlandEvalFile(luaPath); err != nil {
		return fmt.Errorf("failed to unregister Hyprland global hotkeys: %w", err)
	}
	unregisterHyprlandHotkeyCallbacks(r.callbackKeys)
	r.unregistered = true
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] Hyprland disabled %d owned shortcuts", len(r.callbackKeys)+1))
	return nil
}

// isHyprlandGlobalHotkeyAvailableLinux checks active default-submap bindings
// while ignoring Wox-owned handles from the current or a previous registration.
func isHyprlandGlobalHotkeyAvailableLinux(modifiers Modifier, key Key) (bool, error) {
	configuredBinds, err := readHyprlandConfiguredBinds()
	if err != nil {
		return false, err
	}
	return !hyprlandBindingConflicts(configuredBinds, hyprlandKeyToModMask(modifiers), hyprlandKeyName(key)), nil
}

// validateHyprlandBindingConflicts rejects user-owned default-submap conflicts.
func validateHyprlandBindingConflicts(bindings []hyprlandBinding) error {
	configuredBinds, err := readHyprlandConfiguredBinds()
	if err != nil {
		return fmt.Errorf("failed to inspect Hyprland hotkeys: %w", err)
	}
	for _, binding := range bindings {
		if hyprlandBindingConflicts(configuredBinds, binding.nativeModMask, binding.nativeKey) {
			return fmt.Errorf("Hyprland hotkey is already configured: %s", binding.luaKey)
		}
	}
	return nil
}

// readHyprlandConfiguredBinds snapshots the compositor's active bind registry.
func readHyprlandConfiguredBinds() ([]hyprlandConfiguredBind, error) {
	output, err := exec.Command("hyprctl", "-j", "binds").Output()
	if err != nil {
		return nil, fmt.Errorf("hyprctl -j binds failed: %w", err)
	}
	var binds []hyprlandConfiguredBind
	if err := json.Unmarshal(output, &binds); err != nil {
		return nil, fmt.Errorf("failed to parse hyprctl binds: %w", err)
	}
	return binds, nil
}

// hyprlandBindingConflicts ignores Wox-owned and non-default-submap bindings.
func hyprlandBindingConflicts(binds []hyprlandConfiguredBind, modMask uint32, key string) bool {
	for _, bind := range binds {
		if bind.Description == hyprlandBindDescription || bind.Submap != "" {
			continue
		}
		if bind.ModMask == modMask && strings.EqualFold(bind.Key, key) {
			return true
		}
	}
	return false
}

// hyprlandKeyToModMask converts Wox modifiers to Hyprland's xkb modifier mask.
func hyprlandKeyToModMask(modifiers Modifier) uint32 {
	var modMask uint32
	if modifiers&ModifierShift != 0 {
		modMask |= 1
	}
	if modifiers&ModifierCtrl != 0 {
		modMask |= 4
	}
	if modifiers&ModifierAlt != 0 {
		modMask |= 8
	}
	if modifiers&ModifierSuper != 0 {
		modMask |= 64
	}
	return modMask
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
