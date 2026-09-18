//go:build linux && cgo

package keyboard

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/util"
)

const cosmicShortcutsDirectory = "cosmic/com.system76.CosmicSettings.Shortcuts/v1"

// cosmicMu serializes Wox registrations; every edit re-reads the shared desktop config.
var cosmicMu sync.Mutex
var cosmicRegistrations = map[string]*cosmicHotkeyRegistration{}

type cosmicHotkeyRegistration struct {
	bindings  map[string]cosmicShortcut
	callbacks map[string]func()
	lastFired map[string]time.Time
	path      string
}

func init() {
	isCosmicGlobalHotkeyAvailablePlatform = isCosmicGlobalHotkeyAvailable
}

// cosmicShortcutID uses the same key identity for conflict checks and callback routing.
func cosmicShortcutID(modifiers Modifier, key string) string {
	return fmt.Sprintf("%d:%s", modifiers, strings.ToLower(key))
}

func cosmicShortcutURL(entry cosmicShortcut) string {
	return "wox://cosmic-hotkey?binding=" + url.QueryEscape(cosmicShortcutID(entry.modifiers, entry.key))
}

func cosmicOwnedShortcut(entry cosmicShortcut) bool {
	return entry.description == cosmicHotkeyDescription && strings.HasSuffix(entry.command, " '"+cosmicShortcutURL(entry)+"'")
}

// cosmicConfigPaths follows cosmic-config's user config and XDG data directory precedence.
func cosmicConfigPaths() (string, string, error) {
	configHome, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	if os.Getenv("FLATPAK_ID") != "" {
		return "", "", fmt.Errorf("%w: COSMIC shortcuts require access to host configuration", ErrGlobalHotkeysUnavailable)
	}
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataDirs := os.Getenv("XDG_DATA_DIRS")
	if dataDirs == "" {
		dataDirs = "/usr/local/share:/usr/share"
	}
	for _, root := range append([]string{dataHome}, filepath.SplitList(dataDirs)...) {
		if !filepath.IsAbs(root) {
			continue
		}
		path := filepath.Join(root, cosmicShortcutsDirectory)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if info.IsDir() {
			return filepath.Join(configHome, cosmicShortcutsDirectory), path, nil
		}
	}
	return "", "", fmt.Errorf("%w: COSMIC system shortcut configuration was not found", ErrGlobalHotkeysUnavailable)
}

// readCosmicConfig preserves malformed/unreadable files as errors, including system defaults.
func readCosmicConfig(path string) (cosmicShortcutConfig, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cosmicShortcutConfig{}, nil, err
	}
	config, err := parseCosmicShortcuts(string(data))
	if err != nil {
		return config, data, fmt.Errorf("read %s: %w", path, err)
	}
	return config, data, nil
}

// loadCosmicConfigs applies the same per-file fallback as cosmic-config.
func loadCosmicConfigs(userDir, systemDir string) (cosmicShortcutConfig, cosmicShortcutConfig, []byte, error) {
	defaults, _, err := readCosmicConfig(filepath.Join(userDir, "defaults"))
	if os.IsNotExist(err) {
		defaults, _, err = readCosmicConfig(filepath.Join(systemDir, "defaults"))
	}
	if err != nil {
		return defaults, cosmicShortcutConfig{}, nil, err
	}
	custom, original, err := readCosmicConfig(filepath.Join(userDir, "custom"))
	if os.IsNotExist(err) {
		custom, _, err = readCosmicConfig(filepath.Join(systemDir, "custom"))
		if os.IsNotExist(err) {
			custom, err = parseCosmicShortcuts("{\n}\n")
		}
	}
	return defaults, custom, original, err
}

// validateCosmicConflicts never replaces user entries, including explicit Disable overrides.
// ponytail: keycode bindings with the same modifiers fail closed; resolve the active XKB map if needed.
func validateCosmicConflicts(defaults, custom cosmicShortcutConfig, requested []cosmicShortcut) error {
	effective := map[string]cosmicShortcut{}
	for _, config := range []cosmicShortcutConfig{defaults, custom} {
		for _, entry := range config.entries {
			if cosmicOwnedShortcut(entry) {
				continue
			}
			effective[cosmicShortcutID(entry.modifiers, entry.key)] = entry
		}
	}
	seen := map[string]bool{}
	for _, entry := range requested {
		id := cosmicShortcutID(entry.modifiers, entry.key)
		if seen[id] {
			return fmt.Errorf("%w: duplicate COSMIC shortcut %s", ErrHotkeyConflict, id)
		}
		seen[id] = true
		if _, exists := effective[id]; exists {
			return fmt.Errorf("%w: COSMIC shortcut %s is already configured", ErrHotkeyConflict, id)
		}
		for _, existing := range effective {
			if existing.keycode && existing.modifiers == entry.modifiers {
				return fmt.Errorf("cannot safely check COSMIC keycode shortcut for %s", id)
			}
		}
	}
	return nil
}

// writeCosmicConfig atomically replaces a validated snapshot and rejects intervening edits.
func writeCosmicConfig(path string, original []byte, config cosmicShortcutConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	mode := os.FileMode(0600)
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to replace non-regular COSMIC config %s", path)
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".wox-shortcuts-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if _, err := file.WriteString(config.render()); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, original) || (original == nil) != os.IsNotExist(err) {
		return fmt.Errorf("COSMIC shortcuts changed during registration; retry")
	}
	return os.Rename(file.Name(), path)
}

// registerGlobalHotkeysLinuxCosmic installs one batch so conflicts cannot leave a partial registration.
func registerGlobalHotkeysLinuxCosmic(specs []GlobalHotkeySpec) (HotkeyRegistration, error) {
	cosmicMu.Lock()
	defer cosmicMu.Unlock()
	userDir, systemDir, err := cosmicConfigPaths()
	if err != nil {
		return nil, err
	}
	defaults, config, original, err := loadCosmicConfigs(userDir, systemDir)
	if err != nil {
		return nil, err
	}
	executable := os.Getenv("APPIMAGE")
	if executable == "" {
		executable, err = os.Executable()
	}
	if err != nil {
		return nil, err
	}
	registration := &cosmicHotkeyRegistration{path: filepath.Join(userDir, "custom"), bindings: map[string]cosmicShortcut{}, callbacks: map[string]func(){}, lastFired: map[string]time.Time{}}
	var requested []cosmicShortcut
	for _, spec := range specs {
		key, err := keyToWaylandTriggerName(spec.Key)
		if err != nil {
			return nil, err
		}
		entry := cosmicShortcut{modifiers: spec.Modifiers, key: key, description: cosmicHotkeyDescription}
		id := cosmicShortcutID(spec.Modifiers, key)
		if spec.Callback == nil {
			return nil, fmt.Errorf("COSMIC shortcut callback is required")
		}
		if cosmicRegistrations[id] != nil {
			return nil, fmt.Errorf("%w: COSMIC shortcut %s is already registered", ErrHotkeyConflict, id)
		}
		var modifiers []string
		for _, modifier := range []struct {
			flag Modifier
			name string
		}{{ModifierSuper, "Super"}, {ModifierCtrl, "Ctrl"}, {ModifierAlt, "Alt"}, {ModifierShift, "Shift"}} {
			if spec.Modifiers&modifier.flag != 0 {
				modifiers = append(modifiers, modifier.name)
			}
		}
		entry.command = "'" + strings.ReplaceAll(executable, "'", "'\\''") + "' '" + cosmicShortcutURL(entry) + "'"
		entry.raw = fmt.Sprintf("\n    (modifiers: [%s], key: %s, description: Some(%s)): Spawn(%s),", strings.Join(modifiers, ", "), strconv.Quote(key), strconv.Quote(entry.description), strconv.Quote(entry.command))
		requested = append(requested, entry)
		registration.bindings[id], registration.callbacks[id] = entry, spec.Callback
	}
	if err := validateCosmicConflicts(defaults, config, requested); err != nil {
		return nil, err
	}
	// Remove only stale entries carrying both our marker and our exact callback URL.
	kept := config.entries[:0]
	for _, entry := range config.entries {
		if cosmicOwnedShortcut(entry) && cosmicRegistrations[cosmicShortcutID(entry.modifiers, entry.key)] == nil {
			continue
		}
		kept = append(kept, entry)
	}
	config.entries = append(kept, requested...)
	if err := writeCosmicConfig(registration.path, original, config); err != nil {
		return nil, err
	}
	for id := range registration.bindings {
		cosmicRegistrations[id] = registration
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[hotkey] COSMIC registered %d custom shortcuts", len(specs)))
	return registration, nil
}

// Unregister removes only unchanged Wox entries, preserving edits made in COSMIC Settings.
func (r *cosmicHotkeyRegistration) Unregister() error {
	cosmicMu.Lock()
	defer cosmicMu.Unlock()
	// A failed disk cleanup must still stop dispatching callbacks for retired bindings.
	defer func() {
		for id := range r.bindings {
			if cosmicRegistrations[id] == r {
				delete(cosmicRegistrations, id)
			}
		}
	}()
	config, original, err := readCosmicConfig(r.path)
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return err
	}
	kept := config.entries[:0]
	changed := false
	for _, entry := range config.entries {
		id := cosmicShortcutID(entry.modifiers, entry.key)
		owned, exists := r.bindings[id]
		if exists && (cosmicRegistrations[id] == nil || cosmicRegistrations[id] == r) && entry.command == owned.command && cosmicOwnedShortcut(entry) {
			changed = true
			continue
		}
		kept = append(kept, entry)
	}
	if changed {
		config.entries = kept
		if err := writeCosmicConfig(r.path, original, config); err != nil {
			return err
		}
	}
	return nil
}

// isCosmicGlobalHotkeyAvailable checks configuration without creating desktop shortcuts.
func isCosmicGlobalHotkeyAvailable(modifiers Modifier, key Key) (bool, error) {
	cosmicMu.Lock()
	defer cosmicMu.Unlock()
	userDir, systemDir, err := cosmicConfigPaths()
	if err != nil {
		return false, err
	}
	defaults, custom, _, err := loadCosmicConfigs(userDir, systemDir)
	if err != nil {
		return false, err
	}
	name, err := keyToWaylandTriggerName(key)
	if err != nil {
		return false, err
	}
	err = validateCosmicConflicts(defaults, custom, []cosmicShortcut{{modifiers: modifiers, key: name}})
	return err == nil, err
}

// InvokeCosmicHotkeyCallback dispatches only active registrations and suppresses key repeat.
func InvokeCosmicHotkeyCallback(binding string) {
	cosmicMu.Lock()
	registration := cosmicRegistrations[binding]
	if registration == nil {
		cosmicMu.Unlock()
		return
	}
	callback := registration.callbacks[binding]
	now := time.Now()
	if now.Sub(registration.lastFired[binding]) < 500*time.Millisecond {
		cosmicMu.Unlock()
		return
	}
	registration.lastFired[binding] = now
	cosmicMu.Unlock()
	util.Go(util.NewTraceContext(), "cosmic hotkey callback", callback)
}
