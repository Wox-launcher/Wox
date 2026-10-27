package window

import (
	"context"
	"os"
	"testing"

	"wox/util"
)

func TestLinuxActivationBackend(t *testing.T) {
	if got := linuxActivationBackend(true, true); got != "hyprland" {
		t.Fatalf("Hyprland backend = %q", got)
	}
	if got := linuxActivationBackend(true, false); got != "" {
		t.Fatalf("other Wayland backend = %q", got)
	}
	if got := linuxActivationBackend(false, false); got != "" {
		t.Fatalf("X11 backend = %q", got)
	}
}

func TestHyprlandFocusDispatch(t *testing.T) {
	if got := hyprlandFocusDispatch("0x5556835f2d30"); got != "hl.dsp.focus({ window = 'address:0x5556835f2d30' })" {
		t.Fatalf("dispatch = %q", got)
	}
	if got := hyprlandFocusDispatch("0x12; os.execute('rm')"); got != "" {
		t.Fatalf("unsafe dispatch = %q", got)
	}
	if got := hyprlandFocusDispatch("google-chrome"); got != "" {
		t.Fatalf("class dispatch = %q", got)
	}
}

func TestParseHyprlandActivationWindows(t *testing.T) {
	windows, err := parseHyprlandActivationWindows([]byte(`[
		{"address":"0x10","class":"google-chrome","initialClass":"google-chrome","pid":7,"focusHistoryID":2,"mapped":true,"hidden":false,"workspace":{"id":3}},
		{"address":"not-an-address","class":"google-chrome","mapped":true},
		{"address":"0x11","class":"foot","initialClass":"foot","pid":8,"focusHistoryID":9,"mapped":true,"hidden":false}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 2 || windows[0].Address != "0x10" || windows[0].FocusHistoryID != 2 || windows[1].Class != "foot" {
		t.Fatalf("windows = %+v", windows)
	}
}

func TestSelectLinuxActivationWindowPrefersRecentChrome(t *testing.T) {
	app := linuxDesktopApp{ID: "google-chrome", StartupWMClass: "google-chrome", ExecBase: "google-chrome-stable"}
	windows := []linuxActivationWindow{
		{Address: "0x1", Class: "foot", FocusHistoryID: 9, Mapped: true},
		{Address: "0x2", Class: "google-chrome", FocusHistoryID: 2, Mapped: true},
		{Address: "0x3", Class: "google-chrome", FocusHistoryID: 5, Mapped: true, Hidden: true},
		{Address: "0x4", Class: "Google-chrome", FocusHistoryID: 3, Mapped: true},
		{Address: "0x5", Class: "google-chrome", FocusHistoryID: 8, Mapped: false},
	}
	got, ok := selectLinuxActivationWindow(app, windows, nil)
	if !ok || got.Address != "0x3" {
		t.Fatalf("window = %+v ok=%t", got, ok)
	}
}

func TestSelectLinuxActivationWindowMatchesStartupClass(t *testing.T) {
	app := linuxDesktopApp{ID: "com.google.Chrome", StartupWMClass: "Google-chrome", ExecBase: "chrome"}
	got, ok := selectLinuxActivationWindow(app, []linuxActivationWindow{
		{Address: "0x20", Class: "google-chrome", InitialClass: "Google-chrome", Mapped: true, FocusHistoryID: 1},
	}, nil)
	if !ok || got.Address != "0x20" {
		t.Fatalf("window = %+v ok=%t", got, ok)
	}
}

func TestLiveHyprlandFocusChrome(t *testing.T) {
	if os.Getenv("WOX_HYPRLAND_FOCUS_TEST") != "1" {
		t.Skip("set WOX_HYPRLAND_FOCUS_TEST=1 to move the focused window")
	}
	if !util.IsHyprlandSession() {
		t.Skip("not a Hyprland session")
	}
	ctx := context.Background()
	previous := hyprlandActiveAddress(ctx)
	t.Cleanup(func() {
		if script := hyprlandFocusDispatch(previous); script != "" {
			_ = hyprlandDispatch(context.Background(), script)
		}
	})

	activation, ok := PrepareLinuxAppActivation(ctx, "/usr/share/applications/google-chrome.desktop")
	if !ok {
		t.Fatal("no Chrome window to focus")
	}
	if !activation.Focus(ctx) {
		t.Fatal("focus failed")
	}
	if got := hyprlandActiveAddress(ctx); got != activation.address {
		t.Fatalf("active address = %s, want %s", got, activation.address)
	}
}

func TestSelectLinuxActivationWindowIgnoresOtherApps(t *testing.T) {
	app := linuxDesktopApp{ID: "google-chrome", StartupWMClass: "google-chrome", ExecBase: "google-chrome-stable"}
	if _, ok := selectLinuxActivationWindow(app, []linuxActivationWindow{
		{Address: "0x1", Class: "foot", Mapped: true, FocusHistoryID: 4},
	}, nil); ok {
		t.Fatal("matched an unrelated window")
	}
}
