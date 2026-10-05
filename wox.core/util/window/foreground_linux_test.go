package window

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"wox/util"
)

func TestLinuxForegroundBackend(t *testing.T) {
	if got := linuxForegroundBackend(true, true); got != "hyprland" {
		t.Fatalf("Hyprland backend = %q", got)
	}
	if got := linuxForegroundBackend(true, false); got != "" {
		t.Fatalf("other Wayland backend = %q", got)
	}
	if got := linuxForegroundBackend(false, false); got != "x11" {
		t.Fatalf("X11 backend = %q", got)
	}
	if got := linuxForegroundBackend(false, true); got != "x11" {
		t.Fatalf("X11 backend with stray Hyprland flag = %q", got)
	}
}

func TestParseHyprlandForeground(t *testing.T) {
	app, err := parseHyprlandForeground([]byte(`{"class":"kitty","initialClass":"kitty","title":"shell","pid":920}`))
	if err != nil {
		t.Fatal(err)
	}
	if app.AppID != "kitty" || app.Pid != 920 || app.Title != "shell" {
		t.Fatalf("foreground = %+v", app)
	}
	if _, err := parseHyprlandForeground([]byte(`{"class":"","pid":0}`)); !errors.Is(err, errLinuxForegroundMissing) {
		t.Fatalf("empty window error = %v", err)
	}
	if _, err := parseHyprlandForeground([]byte(`not-json`)); err == nil {
		t.Fatal("expected malformed hyprctl output to fail")
	}
}

func TestParseX11Foreground(t *testing.T) {
	if got := parseX11ActiveWindowID("_NET_ACTIVE_WINDOW(WINDOW): window id # 0x1a2b\n"); got != "0x1a2b" {
		t.Fatalf("window id = %q", got)
	}
	if got := parseX11ActiveWindowID("_NET_ACTIVE_WINDOW(WINDOW): window id # 0x0\n"); got != "" {
		t.Fatalf("empty window id = %q", got)
	}

	app, err := parseX11ForegroundProperties("WM_CLASS(STRING) = \"google-chrome\", \"Google-chrome\"\n_NET_WM_PID(CARDINAL) = 42\n_NET_WM_NAME(UTF8_STRING) = \"hello, \\\"world\\\"\"\n")
	if err != nil {
		t.Fatal(err)
	}
	if app.AppID != "google-chrome" || app.AltID != "Google-chrome" || app.Pid != 42 || app.Title != `hello, "world"` {
		t.Fatalf("foreground = %+v", app)
	}
}

func TestMatchLinuxDesktopApp(t *testing.T) {
	apps := []linuxDesktopApp{
		{ID: "kitty-open", Name: "kitty URL Launcher", ExecBase: "kitty", Icon: "kitty"},
		{ID: "kitty", Name: "kitty", ExecBase: "kitty", Icon: "kitty"},
		{ID: "com.google.Chrome", Name: "Google Chrome", StartupWMClass: "google-chrome", Icon: "com.google.Chrome"},
		{ID: "code-oss", Name: "Code - OSS", StartupWMClass: "code-oss", Icon: "com.visualstudio.code.oss"},
		{ID: "code-url-handler", Name: "Code - OSS - URL Handler", ExecBase: "code-oss", Icon: "com.visualstudio.code.oss", NoDisplay: true},
		{ID: "tool-handler", Name: "Tool Handler", ExecBase: "tool", NoDisplay: true},
		{ID: "tool-app", Name: "Tool", ExecBase: "tool"},
	}

	matched, ok := matchLinuxDesktopApp(apps, "kitty", "")
	if !ok || matched.ID != "kitty" {
		t.Fatalf("kitty match = %+v ok=%v", matched, ok)
	}
	matched, ok = matchLinuxDesktopApp(apps, "google-chrome", "Google-chrome")
	if !ok || matched.ID != "com.google.Chrome" {
		t.Fatalf("chrome match = %+v ok=%v", matched, ok)
	}
	matched, ok = matchLinuxDesktopApp(apps, "code-oss", "")
	if !ok || matched.ID != "code-oss" {
		t.Fatalf("code match = %+v ok=%v", matched, ok)
	}
	matched, ok = matchLinuxDesktopApp(apps, "tool", "")
	if !ok || matched.ID != "tool-app" {
		t.Fatalf("visible exec match = %+v ok=%v", matched, ok)
	}
}

func TestHyprlandForegroundUsesDesktopIdentityAndIcon(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	dir := t.TempDir()
	installForegroundCommand(t, dir, "hyprctl", "#!/bin/sh\nprintf '%s' \"$WOX_TEST_FOREGROUND\"\nexit \"${WOX_TEST_EXIT:-0}\"\n")
	setForegroundSession(t, "Hyprland", true)
	t.Setenv("PATH", dir)
	home, dataDir := isolateForegroundDesktop(t)
	writeForegroundDesktop(t, filepath.Join(dataDir, "applications", "code-oss.desktop"), ""+
		"[Desktop Entry]\nType=Application\nName=Code - OSS\nStartupWMClass=code-oss\nIcon=code-oss\nExec=code-oss %F\n")
	iconPath := writeForegroundPNG(t, filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", "code-oss.png"))
	t.Setenv("WOX_TEST_FOREGROUND", `{"class":"code-oss","initialClass":"code-oss","title":"main.go","pid":42}`)

	if got := GetActiveWindowPid(); got != 42 {
		t.Fatalf("pid = %d", got)
	}
	if got := GetProcessIdentity(42); got != "code-oss" {
		t.Fatalf("identity = %q", got)
	}
	if got := GetActiveWindowName(); got != "Code - OSS" {
		t.Fatalf("name = %q", got)
	}
	icon, err := GetActiveWindowIcon()
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds().Dx() != 2 || icon.Bounds().Dy() != 2 {
		t.Fatalf("icon bounds = %v", icon.Bounds())
	}
	if got := GetWindowNameByPid(42); got != "Code - OSS" {
		t.Fatalf("name by pid = %q", got)
	}
	if _, err := GetWindowIconByPid(42); err != nil {
		t.Fatal(err)
	}
	if resolved := resolveLinuxAppIconPath("code-oss"); resolved != iconPath {
		t.Fatalf("icon path = %s, want %s", resolved, iconPath)
	}
}

func TestX11ForegroundReadsActiveWindow(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	dir := t.TempDir()
	installForegroundCommand(t, dir, "xprop", "#!/bin/sh\nif [ \"$1\" = '-root' ]; then printf '%s' \"$WOX_TEST_FOREGROUND\"; else printf '%s' \"$WOX_TEST_PROPS\"; fi\nexit \"${WOX_TEST_EXIT:-0}\"\n")
	setForegroundSession(t, "GNOME", false)
	t.Setenv("PATH", dir)
	_, dataDir := isolateForegroundDesktop(t)
	writeForegroundDesktop(t, filepath.Join(dataDir, "applications", "com.google.Chrome.desktop"), ""+
		"[Desktop Entry]\nType=Application\nName=Google Chrome\nStartupWMClass=google-chrome\nIcon=com.google.Chrome\nExec=/usr/bin/flatpak run --command=/app/bin/chrome com.google.Chrome\n")
	t.Setenv("WOX_TEST_FOREGROUND", "_NET_ACTIVE_WINDOW(WINDOW): window id # 0x55\n")
	t.Setenv("WOX_TEST_PROPS", "WM_CLASS(STRING) = \"google-chrome\", \"Google-chrome\"\n_NET_WM_PID(CARDINAL) = 7\n_NET_WM_NAME(UTF8_STRING) = \"inbox\"\n")

	if got := GetActiveWindowPid(); got != 7 {
		t.Fatalf("pid = %d", got)
	}
	if got := GetProcessIdentity(7); got != "com.google.Chrome" {
		t.Fatalf("identity = %q", got)
	}
	if got := GetActiveWindowName(); got != "Google Chrome" {
		t.Fatalf("name = %q", got)
	}
}

func TestWaylandWithoutForegroundQueryLeavesIdentityEmpty(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	dir := t.TempDir()
	marker := filepath.Join(dir, "called")
	installForegroundCommand(t, dir, "hyprctl", "#!/bin/sh\nprintf '' > \"$WOX_TEST_MARKER\"\nexit 1\n")
	installForegroundCommand(t, dir, "xprop", "#!/bin/sh\nprintf '' > \"$WOX_TEST_MARKER\"\nexit 1\n")
	setForegroundSession(t, "GNOME", true)
	t.Setenv("PATH", dir)
	t.Setenv("WOX_TEST_MARKER", marker)

	if got := GetActiveWindowPid(); got != -1 {
		t.Fatalf("pid = %d", got)
	}
	if got := GetActiveWindowName(); got != "" {
		t.Fatalf("name = %q", got)
	}
	if _, err := GetActiveWindowIcon(); !errors.Is(err, errLinuxForegroundUnavailable) {
		t.Fatalf("icon error = %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("GNOME Wayland queried a foreground command")
	}
}

func TestForegroundCommandFailure(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	dir := t.TempDir()
	installForegroundCommand(t, dir, "hyprctl", "#!/bin/sh\nexit \"${WOX_TEST_EXIT:-0}\"\n")
	setForegroundSession(t, "Hyprland", true)
	t.Setenv("PATH", dir)
	t.Setenv("WOX_TEST_EXIT", "1")
	t.Setenv("WOX_TEST_FOREGROUND", `{"class":"kitty","pid":1}`)

	if got := GetActiveWindowPid(); got != -1 {
		t.Fatalf("pid = %d", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := queryHyprlandForeground(ctx); err == nil {
		t.Fatal("cancelled hyprctl should fail")
	}
}

func TestResolveLinuxAppIconPrefersSharpIconOverTinyThemeIcon(t *testing.T) {
	home, dataDir := isolateForegroundDesktop(t)
	requireForegroundDir(t, filepath.Join(home, ".config", "gtk-3.0"))
	if err := os.WriteFile(filepath.Join(home, ".config", "gtk-3.0", "settings.ini"), []byte("[Settings]\ngtk-icon-theme-name=TinyTheme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tiny := writeForegroundPNG(t, filepath.Join(dataDir, "icons", "TinyTheme", "16x16", "apps", "demo.png"))
	sharp := writeForegroundPNG(t, filepath.Join(dataDir, "icons", "hicolor", "256x256", "apps", "demo.png"))
	if got := resolveLinuxAppIconPath("demo"); got != sharp {
		t.Fatalf("icon = %s, want sharp %s (tiny %s)", got, sharp, tiny)
	}

	themed := writeForegroundPNG(t, filepath.Join(dataDir, "icons", "TinyTheme", "48x48", "apps", "demo.png"))
	if got := resolveLinuxAppIconPath("demo"); got != themed {
		t.Fatalf("theme icon = %s, want %s", got, themed)
	}
}

func TestLoadLinuxAppIconRendersSVG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.svg")
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#00aa00"/></svg>`
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		t.Fatal(err)
	}
	icon, err := loadLinuxAppIcon(path)
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds().Dx() != 128 || icon.Bounds().Dy() != 128 {
		t.Fatalf("svg bounds = %v", icon.Bounds())
	}
	_, _, _, alpha := icon.At(8, 8).RGBA()
	if alpha == 0 {
		t.Fatal("svg icon is fully transparent")
	}
}

func TestProcessIdentityFromProcMatchesDesktopExec(t *testing.T) {
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Base(exe)
	_, dataDir := isolateForegroundDesktop(t)
	writeForegroundDesktop(t, filepath.Join(dataDir, "applications", "wox-test-foreground.desktop"), ""+
		"[Desktop Entry]\nType=Application\nName=Foreground Test\nIcon=missing\nExec="+base+"\n")

	if got := GetProcessIdentity(os.Getpid()); got != "wox-test-foreground" {
		t.Fatalf("proc identity = %q", got)
	}
	if got := GetWindowNameByPid(os.Getpid()); got != "Foreground Test" {
		t.Fatalf("proc name = %q", got)
	}
}

func TestLiveHyprlandForeground(t *testing.T) {
	if !util.IsHyprlandSession() {
		t.Skip("not a Hyprland session")
	}
	if _, err := exec.LookPath("hyprctl"); err != nil {
		t.Skip("hyprctl is not installed")
	}
	resetLinuxForegroundState()
	t.Cleanup(resetLinuxForegroundState)

	pid := GetActiveWindowPid()
	if pid <= 0 {
		t.Skip("Hyprland has no focused window")
	}
	identity := GetProcessIdentity(pid)
	name := GetActiveWindowName()
	t.Logf("focused pid=%d identity=%s name=%s", pid, identity, name)
	if identity == "" || name == "" {
		t.Fatalf("focused application pid=%d identity=%q name=%q", pid, identity, name)
	}
	icon, err := GetActiveWindowIcon()
	if err != nil {
		t.Fatal(err)
	}
	if icon.Bounds().Empty() {
		t.Fatal("focused application icon is empty")
	}
}

func setForegroundSession(t *testing.T, desktop string, wayland bool) {
	t.Helper()
	t.Setenv("XDG_CURRENT_DESKTOP", desktop)
	t.Setenv("XDG_SESSION_DESKTOP", desktop)
	t.Setenv("DESKTOP_SESSION", desktop)
	t.Setenv("GDMSESSION", "")
	t.Setenv("GNOME_DESKTOP_SESSION_ID", "")
	if wayland {
		t.Setenv("XDG_SESSION_TYPE", "wayland")
		t.Setenv("WAYLAND_DISPLAY", "wayland-test")
		return
	}
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "")
}

func isolateForegroundDesktop(t *testing.T) (string, string) {
	t.Helper()
	home := t.TempDir()
	dataDir := filepath.Join(home, "share")
	requireForegroundDir(t, filepath.Join(home, ".local", "share"))
	requireForegroundDir(t, dataDir)
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	t.Setenv("XDG_DATA_DIRS", dataDir)
	t.Setenv("LANG", "C")
	return home, dataDir
}

func installForegroundCommand(t *testing.T, dir string, name string, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeForegroundDesktop(t *testing.T, path string, body string) {
	t.Helper()
	requireForegroundDir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeForegroundPNG(t *testing.T, path string) string {
	t.Helper()
	requireForegroundDir(t, filepath.Dir(path))
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func requireForegroundDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
