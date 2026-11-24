package util

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxDesktopResourcesUseXDGDataHome(t *testing.T) {
	dataHome := filepath.Join(t.TempDir(), "Desktop Data")
	t.Setenv("XDG_DATA_HOME", dataHome)

	desktop, err := defaultLinuxDesktopEntryPath()
	if want := filepath.Join(dataHome, "applications", linuxDesktopFile); err != nil || desktop != want {
		t.Fatalf("desktop entry = %q, %v; want %q", desktop, err, want)
	}
	icon, err := LinuxDesktopIconPath()
	if want := filepath.Join(dataHome, "icons", "hicolor", "256x256", "apps", linuxDesktopIconFile); err != nil || icon != want {
		t.Fatalf("desktop icon = %q, %v; want %q", icon, err, want)
	}
	mime, err := linuxMimeDirectory()
	if want := filepath.Join(dataHome, "mime"); err != nil || mime != want {
		t.Fatalf("MIME directory = %q, %v; want %q", mime, err, want)
	}
}

func TestBuildLinuxDesktopEntryDeclaresKWinScreenshotInterface(t *testing.T) {
	t.Setenv("APPIMAGE", "/tmp/Wox.AppImage")
	entry, err := BuildLinuxDesktopEntry(true, false)
	if err != nil {
		t.Fatalf("build Linux desktop entry: %v", err)
	}
	if !strings.Contains(entry, "X-KDE-DBUS-Restricted-Interfaces=org.kde.KWin.ScreenShot2\n") {
		t.Fatalf("desktop entry does not declare KWin screenshot interface:\n%s", entry)
	}
	if !strings.Contains(entry, "StartupWMClass="+LinuxDesktopWMClass+"\n") {
		t.Fatalf("desktop entry does not declare WM class %q:\n%s", LinuxDesktopWMClass, entry)
	}
	if !strings.Contains(entry, "Icon="+LinuxDesktopAppID+"\n") {
		t.Fatalf("desktop entry does not declare icon %q:\n%s", LinuxDesktopAppID, entry)
	}
	if !strings.Contains(entry, "Exec=\"/tmp/Wox.AppImage\" %U\n") {
		t.Fatalf("desktop entry does not accept URL and file arguments:\n%s", entry)
	}
	if !strings.Contains(entry, "MimeType="+pluginPackageURLMIME+";"+PluginPackageMIMEType+";"+ThemePackageMIMEType+";\n") {
		t.Fatalf("desktop entry does not declare plugin package MIME type:\n%s", entry)
	}
}

func TestBuildLinuxPluginPackageMimeTypeDeclaresGlob(t *testing.T) {
	mime := buildLinuxPluginPackageMimeType()
	if !strings.Contains(mime, `type="`+PluginPackageMIMEType+`"`) {
		t.Fatalf("MIME type missing %s:\n%s", PluginPackageMIMEType, mime)
	}
	if !strings.Contains(mime, `<glob pattern="*`+PluginPackageExtension+`"/>`) {
		t.Fatalf("MIME type missing .wox glob:\n%s", mime)
	}
}
