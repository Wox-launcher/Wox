package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLinuxNewWindowDesktopActions(t *testing.T) {
	root := t.TempDir()
	// An existing executable lets GIO validate the fixture without depending on installed browsers.
	for _, name := range []string{"firefox", "unknown"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name, command, actions string
		want                   bool
	}{
		{"declared action", "unknown", "Actions=new-window;\n\n[Desktop Action new-window]\nName=New Window\nExec=/bin/true\n", true},
		{"known executable", "firefox --profile /tmp/profile", "", true},
		{"unknown executable", "unknown", "", false},
		{"conflicting mode", "firefox --no-remote", "", false},
		{"unlisted action", "unknown", "\n[Desktop Action new-window]\nName=New Window\nExec=/bin/true\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, "test.desktop")
			content := "[Desktop Entry]\nType=Application\nName=Test\nExec=" + root + "/" + tc.command + "\n" + tc.actions
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := supportsAppNewWindow(appInfo{Path: path}); got != tc.want {
				t.Fatalf("supported=%t, want %t", got, tc.want)
			}
		})
	}
}
