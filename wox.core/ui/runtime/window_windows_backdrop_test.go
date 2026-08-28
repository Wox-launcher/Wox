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

func TestWindows11WindowsShareDesktopAcrylic(t *testing.T) {
	if windowsUsesAccentBackdrop("win11") {
		t.Fatal("Windows 11 windows share Desktop Acrylic")
	}
	if !windowsUsesAccentBackdrop("win10") {
		t.Fatal("Windows 10 windows need the Accent Acrylic fallback")
	}
	for _, options := range []WindowOptions{
		{},
		{Role: WindowRoleApplication},
		{Nonactivating: true},
		{Resizable: true, Topmost: true},
	} {
		if !windowsWindowUsesSystemBackdrop(options) {
			t.Fatalf("window options %#v must keep the process Desktop Acrylic", options)
		}
	}
	if windowsWindowUsesSystemBackdrop(WindowOptions{Role: WindowRoleScreenshot, Nonactivating: true}) {
		t.Fatal("screenshot windows must keep skipping system backdrop")
	}
}

func TestWindowsNCActivateKeepsSystemBackdropActive(t *testing.T) {
	if windowsWMNCActivate != 0x0086 {
		t.Fatalf("WM_NCACTIVATE = %#x, want 0x0086", windowsWMNCActivate)
	}
	if !windowsWindowUsesSystemBackdrop(WindowOptions{Nonactivating: true}) {
		t.Fatal("notifications must keep Desktop Acrylic even though they never take focus")
	}
	if windowsWindowUsesSystemBackdrop(WindowOptions{Role: WindowRoleScreenshot}) {
		t.Fatal("screenshot windows must not force an active system backdrop")
	}
}

func TestWindowsWindowsStayFrameless(t *testing.T) {
	if windowsWindowStyle(WindowOptions{Resizable: true})&windowsWSSizeBox != 0 {
		t.Fatal("Desktop Acrylic stays stable only without WS_THICKFRAME")
	}
	if windowsWindowStyle(WindowOptions{})&windowsWSSizeBox != 0 {
		t.Fatal("fixed windows must stay frameless")
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
