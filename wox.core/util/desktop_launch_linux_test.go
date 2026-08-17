//go:build linux

package util

import "testing"

func TestShouldRelaunchLinuxFromDesktopEntrySkipsDevBuilds(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("WAYLAND_DISPLAY", "wayland-0")
	t.Setenv(linuxDesktopRelaunchAttemptedEnv, "")
	t.Setenv(linuxDesktopLaunchConfirmedEnv, "")
	t.Setenv("GIO_LAUNCHED_DESKTOP_FILE", "")
	t.Setenv("GIO_LAUNCHED_DESKTOP_FILE_PID", "")

	originalProdEnv := ProdEnv
	t.Cleanup(func() { ProdEnv = originalProdEnv })

	ProdEnv = ""
	if ShouldRelaunchLinuxFromDesktopEntry(nil) {
		t.Fatal("dev builds should keep startup in-process on Wayland")
	}

	ProdEnv = "true"
	if ShouldRelaunchLinuxFromDesktopEntry([]string{"--bug-aware-child"}) {
		t.Fatal("supervisor children should not relaunch from the desktop entry")
	}
}

func TestIsEphemeralDebugExecutable(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/home/scott/dev/Wox/wox.core/__debug_bin_wox", want: true},
		{path: "/tmp/__debug_bin", want: true},
		{path: "/home/scott/dev/Wox/release/wox-linux-amd64", want: false},
		{path: "/usr/bin/wox", want: false},
	}
	for _, test := range tests {
		if got := isEphemeralDebugExecutable(test.path); got != test.want {
			t.Fatalf("isEphemeralDebugExecutable(%q) = %v, want %v", test.path, got, test.want)
		}
	}
}
