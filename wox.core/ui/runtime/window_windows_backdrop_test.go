//go:build windows

package woxui

import "testing"

func TestWindows11SystemBackdropValuesMatchWindowsSDK(t *testing.T) {
	if dwmSystemBackdropNone != 1 {
		t.Fatalf("disabled backdrop = %d, want DWMSBT_NONE (1)", dwmSystemBackdropNone)
	}
	if dwmSystemBackdropMica != 2 {
		t.Fatalf("Mica backdrop = %d, want DWMSBT_MAINWINDOW (2)", dwmSystemBackdropMica)
	}
	if dwmSystemBackdropAcrylic != 3 {
		t.Fatalf("Acrylic backdrop = %d, want DWMSBT_TRANSIENTWINDOW (3)", dwmSystemBackdropAcrylic)
	}
	if dwmSystemBackdropMicaAlt != 4 {
		t.Fatalf("Mica Alt backdrop = %d, want DWMSBT_TABBEDWINDOW (4)", dwmSystemBackdropMicaAlt)
	}
	if dwmSystemBackdropWox != dwmSystemBackdropAcrylic {
		t.Fatalf("Wox backdrop = %d, want Desktop Acrylic (%d)", dwmSystemBackdropWox, dwmSystemBackdropAcrylic)
	}
}

func TestWindowsOverlayWindowsUseAccentBackdrop(t *testing.T) {
	if !windowsUsesAccentBackdrop("win11", true) {
		t.Fatal("Windows 11 overlays need Accent Acrylic")
	}
	if windowsUsesAccentBackdrop("win11", false) {
		t.Fatal("the Windows 11 launcher should keep the supported system backdrop")
	}
	if !windowsUsesAccentBackdrop("win10", false) {
		t.Fatal("Windows 10 windows need the Accent Acrylic fallback")
	}
	if windowsWindowUsesSystemBackdrop(WindowOptions{Role: WindowRoleScreenshot, Nonactivating: true}) {
		t.Fatal("screenshot windows must keep skipping system backdrop")
	}
}

func TestWindowsTransientOverlaySkipsThickFrame(t *testing.T) {
	if windowsWindowStyle(WindowOptions{Resizable: true})&windowsWSSizeBox == 0 {
		t.Fatal("regular resizable windows need WS_THICKFRAME")
	}
	if windowsWindowStyle(WindowOptions{Resizable: true, TransientOverlay: true})&windowsWSSizeBox != 0 {
		t.Fatal("Accent overlays must resize through hit testing without WS_THICKFRAME")
	}
}

func TestWindows10AcrylicTintMatchesFlutterRunner(t *testing.T) {
	if got := windows10AcrylicTint(true); got != win10DarkAcrylicTint {
		t.Fatalf("dark tint = %#x, want %#x", got, win10DarkAcrylicTint)
	}
	if got := windows10AcrylicTint(false); got != win10LightAcrylicTint {
		t.Fatalf("light tint = %#x, want %#x", got, win10LightAcrylicTint)
	}
}
