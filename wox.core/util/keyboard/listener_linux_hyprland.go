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

// hyprlandMu guards the bind file write + hyprctl eval sequence so concurrent
// re-registrations don't race on the file or the compositor. It also tracks
// whether binds have been loaded once already: hl.bind accumulates without
// replacement, so repeated dofile() calls would stack duplicate binds.
var (
	hyprlandMu       sync.Mutex
	hyprlandBindsSet bool
)

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

	// hl.bind accumulates without replacement, so loading the same bind file
	// more than once stacks duplicate handlers that all fire on each key press.
	// Wox registers all hotkeys as one group, so one dofile per process lifetime
	// is sufficient. If hotkey config changes, Wox restarts and gets a fresh load.
	if hyprlandBindsSet {
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
			"[hotkey] Hyprland binds already loaded, skipping duplicate registration (%d specs)", len(specs)))
		return &hyprlandHotkeyRegistration{}, true, nil
	}

	bindings := make([]hyprlandBinding, 0, len(specs))
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
			RegisterHyprlandHotkeyCallback(luaKey, spec.Callback)
		}
	}

	if err := hyprlandWriteAndLoadBinds(bindings); err != nil {
		return nil, true, fmt.Errorf("failed to register Hyprland global hotkeys: %w", err)
	}
	hyprlandBindsSet = true

	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[hotkey] Hyprland registered %d shortcuts via hl.bind", len(specs)))

	return &hyprlandHotkeyRegistration{}, true, nil
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

	var sb strings.Builder
	sb.WriteString("-- Auto-generated by Wox. Do not edit.\n")
	sb.WriteString("-- Hyprland global hotkey bindings for Wox launcher.\n")
	// Guard against duplicate loading: hl.bind accumulates and never replaces,
	// so repeated dofile() calls would stack duplicate binds that all fire on
	// a single key press. Use a module-level flag to skip if already loaded.
	sb.WriteString("if wox_binds_loaded then return end\n")
	sb.WriteString("wox_binds_loaded = true\n")
	for _, b := range bindings {
		cmd := fmt.Sprintf("%s %s", woxExec, b.deeplink)
		// repeating=false prevents key-repeat from spawning multiple processes
		// when the user holds the key down. locked=true keeps the bind active
		// during screen lock / compositor busy states.
		sb.WriteString(fmt.Sprintf("hl.bind(%q, hl.dsp.exec_cmd(%q), { repeating = false })\n", b.luaKey, cmd))
	}

	if err := os.WriteFile(luaPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write Hyprland binds file: %w", err)
	}

	// Load the binds via hyprctl eval dofile(). In Hyprland 0.55+ Lua config
	// mode, hyprctl eval executes Lua code, and dofile() loads and runs a file.
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

type hyprlandHotkeyRegistration struct{}

func (r *hyprlandHotkeyRegistration) Unregister() error {
	hyprlandMu.Lock()
	defer hyprlandMu.Unlock()

	// Write an empty binds file and reload to clear all Wox bindings.
	luaPath, err := hyprlandBindsLuaPath()
	if err != nil {
		return err
	}
	emptyContent := "-- Auto-generated by Wox. All bindings cleared.\n"
	if err := os.WriteFile(luaPath, []byte(emptyContent), 0644); err != nil {
		return fmt.Errorf("failed to clear Hyprland binds file: %w", err)
	}
	evalCmd := exec.Command("hyprctl", "eval", fmt.Sprintf("dofile(%q)", luaPath))
	_, _ = evalCmd.CombinedOutput()
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