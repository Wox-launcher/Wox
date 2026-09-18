//go:build linux && cgo

package keyboard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cosmicTestConfig isolates all desktop file mutations from the logged-in user's shortcuts.
func cosmicTestConfig(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "system"))
	t.Setenv("FLATPAK_ID", "")
	t.Setenv("APPIMAGE", "/tmp/Wox user's app.AppImage")
	dir := filepath.Join(root, "system", cosmicShortcutsDirectory)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "defaults"), []byte(`{(modifiers: [Super], key: "q"): Close,}`), 0600); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(root, "config", cosmicShortcutsDirectory)
	if err := os.MkdirAll(userDir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(userDir, "custom")
	if err := os.WriteFile(path, []byte(`{(modifiers: [Alt], key: "u"): Spawn("user-command"),}`), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCosmicRegistrationLifecycle(t *testing.T) {
	path := cosmicTestConfig(t)
	fired := make(chan struct{}, 4)
	registration, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierCtrl | ModifierShift, Key: KeyK, Callback: func() { fired <- struct{}{} }}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registration.Unregister(); err != nil {
			t.Error(err)
		}
	})
	config, _, err := readCosmicConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.entries) != 2 || !strings.Contains(config.entries[1].command, `user'\''s`) {
		t.Fatalf("registration or quoting: %+v", config.entries)
	}
	id := cosmicShortcutID(ModifierCtrl|ModifierShift, "k")
	InvokeCosmicHotkeyCallback(id)
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("callback not dispatched")
	}
	InvokeCosmicHotkeyCallback(id)
	select {
	case <-fired:
		t.Fatal("key repeat was not suppressed")
	case <-time.After(30 * time.Millisecond):
	}
	second, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierAlt, Key: KeyJ, Callback: func() {}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := second.Unregister(); err != nil {
			t.Error(err)
		}
	})
	if err := registration.Unregister(); err != nil {
		t.Fatal(err)
	}
	config, _, err = readCosmicConfig(path)
	if err != nil || len(config.entries) != 2 || config.entries[0].command != "user-command" || config.entries[1].key != "j" {
		t.Fatalf("unregister removed unrelated shortcut: %+v %v", config, err)
	}
	if err := second.Unregister(); err != nil {
		t.Fatal(err)
	}
	config, _, err = readCosmicConfig(path)
	if err != nil || len(config.entries) != 1 || config.entries[0].command != "user-command" {
		t.Fatalf("cleanup: %+v %v", config, err)
	}
	InvokeCosmicHotkeyCallback(id)
	select {
	case <-fired:
		t.Fatal("callback after unregister")
	default:
	}
}

func TestCosmicConflictsAndFailedWritesPreserveConfig(t *testing.T) {
	path := cosmicTestConfig(t)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range []GlobalHotkeySpec{{Modifiers: ModifierSuper, Key: KeyQ, Callback: func() {}}, {Modifiers: ModifierAlt, Key: KeyU, Callback: func() {}}} {
		if _, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{spec}); !errors.Is(err, ErrHotkeyConflict) {
			t.Fatalf("expected conflict: %v", err)
		}
	}
	current, _ := os.ReadFile(path)
	if string(current) != string(original) {
		t.Fatal("conflicting registration changed user config")
	}
	config, _, err := readCosmicConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := append(append([]byte(nil), original...), '\n')
	if err := os.WriteFile(path, updated, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeCosmicConfig(path, original, config); err == nil {
		t.Fatal("overwrote concurrent user edit")
	}
	current, _ = os.ReadFile(path)
	if string(current) != string(updated) {
		t.Fatal("user edit was lost")
	}
	createdPath := filepath.Join(filepath.Dir(path), "created-during-registration")
	if err := os.WriteFile(createdPath, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeCosmicConfig(createdPath, nil, config); err == nil {
		t.Fatal("overwrote a newly created empty file")
	}
	if err := os.WriteFile(path, []byte("malformed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierAlt, Key: KeyK, Callback: func() {}}}); err == nil {
		t.Fatal("overwrote malformed config")
	}
	current, _ = os.ReadFile(path)
	if string(current) != "malformed" {
		t.Fatal("malformed config was lost")
	}
}

func TestCosmicUnregisterPreservesUserReplacement(t *testing.T) {
	path := cosmicTestConfig(t)
	registration, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierAlt, Key: KeyK, Callback: func() {}}})
	if err != nil {
		t.Fatal(err)
	}
	config, original, err := readCosmicConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	config.entries[1].raw = ` (modifiers: [Alt], key: "k"): Spawn("user-replacement"),`
	if err := writeCosmicConfig(path, original, config); err != nil {
		t.Fatal(err)
	}
	if err := registration.Unregister(); err != nil {
		t.Fatal(err)
	}
	config, _, err = readCosmicConfig(path)
	if err != nil || len(config.entries) != 2 || config.entries[1].command != "user-replacement" {
		t.Fatalf("user replacement was lost: %+v %v", config, err)
	}
}

func TestCosmicStaleCleanupAndReadOnlyAvailability(t *testing.T) {
	path := cosmicTestConfig(t)
	old, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierCtrl, Key: KeyK, Callback: func() {}}})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a previous process exiting without unregistering its desktop command.
	delete(cosmicRegistrations, cosmicShortcutID(ModifierCtrl, "k"))
	before, _ := os.ReadFile(path)
	if available, err := isCosmicGlobalHotkeyAvailable(ModifierCtrl, KeyK); !available || err != nil {
		t.Fatalf("stale Wox binding blocked registration: %v %v", available, err)
	}
	if available, err := isCosmicGlobalHotkeyAvailable(ModifierSuper, KeyQ); available || !errors.Is(err, ErrHotkeyConflict) {
		t.Fatalf("missed default conflict: %v %v", available, err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("availability probe wrote configuration")
	}
	current, err := registerGlobalHotkeysLinuxCosmic([]GlobalHotkeySpec{{Modifiers: ModifierAlt, Key: KeyJ, Callback: func() {}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := current.Unregister(); err != nil {
			t.Error(err)
		}
	})
	if err := old.Unregister(); err != nil {
		t.Fatal(err)
	}
	config, _, err := readCosmicConfig(path)
	if err != nil || len(config.entries) != 2 || config.entries[1].key != "j" {
		t.Fatalf("stale cleanup: %+v %v", config, err)
	}
}
