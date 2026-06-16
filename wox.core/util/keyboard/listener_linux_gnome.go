//go:build linux && cgo

package keyboard

// GNOME custom keybinding fallback for global hotkeys on Wayland sessions where
// the XDG GlobalShortcuts portal is not available (e.g. xdg-desktop-portal-gnome
// < 47 which does not implement the org.freedesktop.impl.portal.GlobalShortcuts
// interface used by GNOME 47+).
//
// Instead of registering with the portal, we create a GNOME custom keyboard
// shortcut via gsettings (org.gnome.settings-daemon.plugins.media-keys
// custom-keybindings). When the key combination fires, GNOME executes the
// configured shell command, which runs the wox binary with a "wox://gnome-hotkey"
// deeplink argument. The running wox instance detects the secondary process,
// reads the lock file, and forwards the deeplink via HTTP to ProcessDeeplink,
// which calls InvokeGnomeHotkeyCallback to fire the original callback.

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const (
	// gsettings schema and field for the list of active custom keybinding paths.
	gnomeCustomKeybindingsKey   = "org.gnome.settings-daemon.plugins.media-keys"
	gnomeCustomKeybindingsField = "custom-keybindings"

	// Per-path schema used to configure each individual custom keybinding.
	gnomeCustomKeybindingSchema = "org.gnome.settings-daemon.plugins.media-keys.custom-keybinding"

	// Base dconf path under which all wox-managed keybindings are stored.
	gnomeCustomKeybindingBasePath = "/org/gnome/settings-daemon/plugins/media-keys/custom-keybindings/"
)

const (
	// gnomeHotkeyDebounce is the minimum interval between two consecutive
	// invocations of the same GNOME hotkey callback. GNOME's custom keybinding
	// mechanism runs the configured command on every key-repeat event, so without
	// this guard a single long key-press would fire the callback many times.
	gnomeHotkeyDebounce = 500 * time.Millisecond
)

var (
	gnomeMu sync.Mutex
	// gnomeCallbacks maps a GNOME binding string (e.g. "<Primary><Shift>k") to the
	// registered callback. The binding is the natural unique key for a hotkey.
	gnomeCallbacks = map[string]func(){}
	// gnomeLastFired tracks when each binding was last invoked for debouncing.
	gnomeLastFired = map[string]time.Time{}

	// gnomeCleanupOnce ensures stale keybindings from a previous wox session
	// are removed exactly once, before the first new keybinding is registered.
	gnomeCleanupOnce sync.Once
)

// gnomeHotkeyRegistration is returned to callers and provides Unregister().
type gnomeHotkeyRegistration struct {
	binding string // GNOME binding string, e.g. "<Primary><Shift>k"
	path    string // dconf path for this keybinding
	once    sync.Once
}

// registerGlobalHotkeyLinuxGnome registers a global hotkey via GNOME's
// custom-keybindings gsettings schema. The hotkey persists for the lifetime of
// the returned HotkeyRegistration; call Unregister() to remove it.
func registerGlobalHotkeyLinuxGnome(modifiers Modifier, key Key, callback func()) (HotkeyRegistration, error) {
	// Remove leftover wox-* keybindings from previous wox sessions before
	// adding new ones, so gsettings does not accumulate stale entries.
	gnomeCleanupOnce.Do(gnomeClearStaleWoxKeybindings)

	// Convert the Wox key description to GNOME/X11 keybinding notation.
	binding, err := modifiersKeyToGnomeBinding(modifiers, key)
	if err != nil {
		return nil, fmt.Errorf("cannot convert hotkey to GNOME binding format: %w", err)
	}

	// Determine the path to the running wox binary so GNOME can invoke it.
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot determine wox executable path for GNOME shortcut: %w", err)
	}

	// The binding string is the natural unique key for a hotkey (two shortcuts
	// cannot share the same modifier+key combination). Use it as both the dconf
	// path component and the deeplink parameter so no separate ID map is needed.
	pathID := gnomeSanitizeBinding(binding) // e.g. "primary-shift-k"
	path := gnomeCustomKeybindingBasePath + "wox-" + pathID + "/"

	// Build the shell command GNOME will execute when the shortcut fires.
	// The binding is URL-encoded so it survives as a deeplink query parameter;
	// ProcessDeeplink in the running instance will decode and look up the callback.
	shellCmd := fmt.Sprintf(`"%s" "wox://gnome-hotkey?binding=%s"`, exePath, url.QueryEscape(binding))

	if err := gnomeAddCustomKeybinding(path, binding, shellCmd); err != nil {
		return nil, fmt.Errorf("failed to register GNOME custom keybinding %s: %w", binding, err)
	}

	gnomeMu.Lock()
	gnomeCallbacks[binding] = callback
	gnomeMu.Unlock()

	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[GNOME hotkey] registered custom keybinding binding=%s", binding))

	return &gnomeHotkeyRegistration{binding: binding, path: path}, nil
}

// Unregister removes the GNOME custom keybinding and discards the callback.
func (r *gnomeHotkeyRegistration) Unregister() error {
	if r == nil {
		return nil
	}
	var unregErr error
	r.once.Do(func() {
		gnomeMu.Lock()
		delete(gnomeCallbacks, r.binding)
		delete(gnomeLastFired, r.binding)
		gnomeMu.Unlock()

		unregErr = gnomeRemoveCustomKeybinding(r.path)
		if unregErr != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[GNOME hotkey] failed to remove custom keybinding %s: %v", r.path, unregErr))
		}
	})
	return unregErr
}

// InvokeGnomeHotkeyCallback looks up and fires the callback registered for the
// given GNOME binding string. It is called by ProcessDeeplink when the running
// wox instance receives a "wox://gnome-hotkey?binding=..." deeplink forwarded
// from the GNOME custom shortcut command.
func InvokeGnomeHotkeyCallback(binding string) {
	gnomeMu.Lock()
	cb := gnomeCallbacks[binding]
	// Debounce: GNOME fires the command on every key-repeat event. Drop repeated
	// invocations that arrive faster than gnomeHotkeyDebounce.
	now := time.Now()
	if now.Sub(gnomeLastFired[binding]) < gnomeHotkeyDebounce {
		gnomeMu.Unlock()
		util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
			"[GNOME hotkey] debounced binding=%s", binding))
		return
	}
	gnomeLastFired[binding] = now
	gnomeMu.Unlock()

	if cb == nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
			"[GNOME hotkey] received deeplink for unknown binding=%s (callback may have been unregistered)", binding))
		return
	}

	util.Go(util.NewTraceContext(), "gnome hotkey callback", cb)
}

// gnomeSanitizeBinding converts a GNOME binding string like "<Primary><Shift>k"
// into a dconf-path-safe component like "primary-shift-k".
func gnomeSanitizeBinding(binding string) string {
	r := strings.NewReplacer("<", "", ">", "", " ", "-")
	return strings.ToLower(r.Replace(binding))
}

// gnomeAddCustomKeybinding registers one new GNOME custom keyboard shortcut.
// It reads the current custom-keybindings list, appends the new path, then
// sets the name, command, and binding properties on the keybinding schema.
func gnomeAddCustomKeybinding(path, binding, shellCmd string) error {
	existing, err := gnomeReadKeybindingPaths()
	if err != nil {
		// Non-fatal: start from an empty list if reading fails.
		existing = []string{}
	}

	// Guard against duplicate registration for the same path.
	found := false
	for _, p := range existing {
		if p == path {
			found = true
			break
		}
	}
	if !found {
		existing = append(existing, path)
	}

	// Persist the updated path list first so the schema instance is valid
	// before we attempt to write its individual properties.
	if err := gsettingsSet(gnomeCustomKeybindingsKey, gnomeCustomKeybindingsField,
		gnomeStringSliceToVariant(existing)); err != nil {
		return fmt.Errorf("cannot update GNOME custom-keybindings list: %w", err)
	}

	// Write the three required properties: name (display), command (shell command
	// to run), and binding (the key combination in X11/GDK notation).
	// Values are wrapped in single quotes to form valid GVariant string literals.
	schema := gnomeCustomKeybindingSchema + ":" + path
	if err := gsettingsSet(schema, "name", "'Wox Hotkey'"); err != nil {
		return err
	}
	if err := gsettingsSet(schema, "command", "'"+shellCmd+"'"); err != nil {
		return err
	}
	if err := gsettingsSet(schema, "binding", "'"+binding+"'"); err != nil {
		return err
	}
	return nil
}

// gnomeRemoveCustomKeybinding removes the given path from the GNOME
// custom-keybindings list, leaving all other custom shortcuts untouched.
func gnomeRemoveCustomKeybinding(path string) error {
	existing, err := gnomeReadKeybindingPaths()
	if err != nil {
		return err
	}

	updated := existing[:0]
	for _, p := range existing {
		if p != path {
			updated = append(updated, p)
		}
	}

	return gsettingsSet(gnomeCustomKeybindingsKey, gnomeCustomKeybindingsField,
		gnomeStringSliceToVariant(updated))
}

// gnomeClearStaleWoxKeybindings removes all custom keybinding paths that were
// created by a previous wox session (paths that start with our base path prefix
// followed by "wox-"). User-defined custom shortcuts are preserved.
func gnomeClearStaleWoxKeybindings() {
	existing, err := gnomeReadKeybindingPaths()
	if err != nil || len(existing) == 0 {
		return
	}

	woxPrefix := gnomeCustomKeybindingBasePath + "wox-"
	updated := existing[:0]
	for _, p := range existing {
		if strings.HasPrefix(p, woxPrefix) {
			util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
				"[GNOME hotkey] removing stale keybinding from previous session: %s", p))
		} else {
			updated = append(updated, p)
		}
	}

	if len(updated) != len(existing) {
		if err := gsettingsSet(gnomeCustomKeybindingsKey, gnomeCustomKeybindingsField,
			gnomeStringSliceToVariant(updated)); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
				"[GNOME hotkey] failed to clear stale keybindings: %v", err))
		}
	}
}

// gnomeReadKeybindingPaths fetches the current custom-keybindings path list
// from gsettings and returns it as a Go string slice.
func gnomeReadKeybindingPaths() ([]string, error) {
	out, err := exec.Command("gsettings", "get",
		gnomeCustomKeybindingsKey, gnomeCustomKeybindingsField).Output()
	if err != nil {
		return nil, fmt.Errorf("gsettings get custom-keybindings failed: %w", err)
	}
	return gnomeParseStringSlice(strings.TrimSpace(string(out))), nil
}

// gnomeStringSliceToVariant formats a Go string slice as a GLib GVariant
// array-of-strings literal that gsettings can consume.
func gnomeStringSliceToVariant(paths []string) string {
	if len(paths) == 0 {
		// "@as []" is the canonical empty-array form required by gsettings.
		return "@as []"
	}
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = "'" + p + "'"
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

// gnomeParseStringSlice parses the GVariant array-of-strings text produced
// by "gsettings get", e.g. "@as []" or "['/path/a/', '/path/b/']".
func gnomeParseStringSlice(s string) []string {
	s = strings.TrimSpace(s)
	if s == "@as []" || s == "[]" {
		return []string{}
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, "'")
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// gsettingsSet runs "gsettings set schema key value" in a subprocess.
// Each argument is passed directly to execve so no shell quoting of the
// arguments is needed; the value must already be formatted as a GVariant literal.
func gsettingsSet(schema, key, value string) error {
	cmd := exec.Command("gsettings", "set", schema, key, value)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gsettings set %s %s: %s: %w",
			schema, key, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// modifiersKeyToGnomeBinding converts Wox modifier flags and a key constant
// into a GNOME/GDK keybinding string such as "<Primary><Shift>space".
// The key names are the same X11/xkb keysym names used by the Wayland portal,
// so we reuse keyToWaylandTriggerName for consistency.
func modifiersKeyToGnomeBinding(modifiers Modifier, key Key) (string, error) {
	// Reuse the Wayland trigger name function: GNOME uses the identical X11/xkb
	// keysym names (e.g. "space", "Return", "F1") for its binding strings.
	keyName, err := keyToWaylandTriggerName(key)
	if err != nil {
		return "", fmt.Errorf("unsupported key for GNOME binding: %w", err)
	}

	var b strings.Builder
	// GNOME convention: <Primary> before <Alt> before <Shift> before <Super>.
	if modifiers&ModifierCtrl != 0 {
		b.WriteString("<Primary>")
	}
	if modifiers&ModifierAlt != 0 {
		b.WriteString("<Alt>")
	}
	if modifiers&ModifierShift != 0 {
		b.WriteString("<Shift>")
	}
	if modifiers&ModifierSuper != 0 {
		b.WriteString("<Super>")
	}
	b.WriteString(keyName)
	return b.String(), nil
}
