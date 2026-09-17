//go:build linux

package shell

import "testing"

func TestSelectLinuxTerminalLauncherForDesktop(t *testing.T) {
	if got := selectLinuxTerminalLauncherFor(true, true, false).name(); got != "kde" {
		t.Fatalf("kde+gnome = %q, want kde", got)
	}
	if got := selectLinuxTerminalLauncherFor(false, true, true).name(); got != "gnome" {
		t.Fatalf("gnome+hyprland = %q, want gnome", got)
	}
	if got := selectLinuxTerminalLauncherFor(false, false, true).name(); got != "hyprland" {
		t.Fatalf("hyprland = %q", got)
	}
	if got := selectLinuxTerminalLauncherFor(false, false, false).name(); got != "x11" {
		t.Fatalf("fallback = %q, want x11", got)
	}
}
