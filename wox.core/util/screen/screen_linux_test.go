//go:build linux

package screen

import "testing"

func TestIsHyprlandSessionFromInstanceSignature(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-instance")
	t.Setenv("XDG_CURRENT_DESKTOP", "")
	if !isHyprlandSession() {
		t.Fatal("expected Hyprland instance signature to identify the session")
	}
}

func TestIsHyprlandSessionFromDesktop(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "Hyprland")
	if !isHyprlandSession() {
		t.Fatal("expected XDG_CURRENT_DESKTOP to identify the session")
	}
}

func TestIsHyprlandSessionRejectsOtherDesktop(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	if isHyprlandSession() {
		t.Fatal("did not expect GNOME to identify as Hyprland")
	}
}

func TestGetHyprlandCursorPositionRequiresSignature(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	if _, _, err := getHyprlandCursorPosition(); err == nil {
		t.Fatal("expected missing instance signature to fail")
	}
}

func TestParseHyprlandCursorPosition(t *testing.T) {
	x, y, err := parseHyprlandCursorPosition([]byte(`{"x":2160,"y":114}`))
	if err != nil {
		t.Fatalf("parse cursor position: %v", err)
	}
	if x != 2160 || y != 114 {
		t.Fatalf("unexpected cursor position: %d,%d", x, y)
	}
}

func TestParseHyprlandCursorPositionRejectsInvalidJSON(t *testing.T) {
	if _, _, err := parseHyprlandCursorPosition([]byte(`not-json`)); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestParseHyprlandCursorPositionRejectsMissingCoordinate(t *testing.T) {
	if _, _, err := parseHyprlandCursorPosition([]byte(`{"x":2160}`)); err == nil {
		t.Fatal("expected a missing coordinate to fail")
	}
}
