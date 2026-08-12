package util

import (
	"strings"
	"testing"
)

func TestBuildLinuxDesktopEntryDeclaresKWinScreenshotInterface(t *testing.T) {
	t.Setenv("APPIMAGE", "/tmp/Wox.AppImage")
	entry, err := BuildLinuxDesktopEntry(true, false)
	if err != nil {
		t.Fatalf("build Linux desktop entry: %v", err)
	}
	if !strings.Contains(entry, "X-KDE-DBUS-Restricted-Interfaces=org.kde.KWin.ScreenShot2\n") {
		t.Fatalf("desktop entry does not declare KWin screenshot interface:\n%s", entry)
	}
}
