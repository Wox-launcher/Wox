//go:build linux

package ime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwitchInputMethodABCHyprland(t *testing.T) {
	dir := t.TempDir()
	command := filepath.Join(dir, "fcitx5-remote")
	if err := os.WriteFile(command, []byte("#!/bin/sh\n[ \"$1\" = --check ] && [ \"$2\" = -c ]\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if err := switchInputMethodABCHyprland(); err != nil {
		t.Fatalf("switch input method: %v", err)
	}

	if err := os.WriteFile(command, []byte("#!/bin/sh\necho 'Fcitx unavailable' >&2\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := switchInputMethodABCHyprland(); err == nil || !strings.Contains(err.Error(), "Fcitx unavailable") {
		t.Fatalf("expected Fcitx error, got %v", err)
	}
}
