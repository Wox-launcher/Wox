//go:build !windows

package main

import (
	"net"
	"os"
	"testing"
)

// TestSupervisorRelaunchPreservesPreviousSocket reproduces a socket left by a killed supervisor.
func TestSupervisorRelaunchPreservesPreviousSocket(t *testing.T) {
	// macOS's default temporary path can exceed the Unix socket address limit.
	directory, err := os.MkdirTemp("/tmp", "wox-smoke-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	previous := suiteControlAddress(directory, 1)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: previous, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	next, err := net.Listen("unix", suiteControlAddress(directory, 2))
	if err != nil {
		t.Fatalf("relaunch with previous socket still present: %v", err)
	}
	defer next.Close()
	if _, err := os.Stat(previous); err != nil {
		t.Fatalf("relaunch removed the previous supervisor's socket: %v", err)
	}
}
