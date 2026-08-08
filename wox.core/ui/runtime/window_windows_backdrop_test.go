//go:build windows

package woxui

import "testing"

func TestWindows10AcrylicTintMatchesFlutterRunner(t *testing.T) {
	if got := windows10AcrylicTint(true); got != win10DarkAcrylicTint {
		t.Fatalf("dark tint = %#x, want %#x", got, win10DarkAcrylicTint)
	}
	if got := windows10AcrylicTint(false); got != win10LightAcrylicTint {
		t.Fatalf("light tint = %#x, want %#x", got, win10LightAcrylicTint)
	}
}
