//go:build linux && cgo

package snippets

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestHyprlandFocusTracksWindows exercises the compositor event and snapshot boundary.
func TestHyprlandFocusTracksWindows(t *testing.T) {
	dir := t.TempDir()
	socketDir := filepath.Join(dir, "hypr", "test")
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", filepath.Join(socketDir, ".socket2.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	state := filepath.Join(dir, "active.json")
	writeWindow := func(value string) {
		t.Helper()
		if err := os.WriteFile(state, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeWindow(`{"address":"0x123","class":"foot"}`)
	command := filepath.Join(dir, "hyprctl")
	if err := os.WriteFile(command, []byte("#!/bin/sh\ncat \"$WOX_TEST_HYPR_WINDOW\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("WOX_TEST_HYPR_WINDOW", state)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test")
	changes := make(chan struct{}, 8)
	focus, err := startHyprlandFocus(func() { changes <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	defer focus.close()
	conn, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	first := focus.verifiedTarget()
	if first == 0 || !focus.isTerminal(first) {
		t.Fatal("initial terminal window was not tracked")
	}
	writeWindow(`{"address":"0x456","class":"code"}`)
	if focus.verifiedTarget() != 0 {
		t.Fatal("changed window accepted before its event")
	}
	<-changes // initial snapshot
	if _, err := conn.Write([]byte("activewindowv2>>456\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("window change was not observed")
	}
	deadline := time.Now().Add(time.Second)
	for focus.fast() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	second := focus.verifiedTarget()
	if second == 0 || second == first || focus.isTerminal(second) {
		t.Fatal("new editor window was not tracked")
	}
	_ = conn.Close()
	select {
	case <-focus.done:
	case <-time.After(time.Second):
		t.Fatal("disconnected listener did not stop")
	}
	if focus.fast() != 0 {
		t.Fatal("disconnected compositor left a valid target")
	}
}

func TestHyprlandFallbackHonorsAccessibleRejection(t *testing.T) {
	window := &hyprlandFocus{token: 1}
	focus := &accessibilityFocus{}
	backend := &waylandExpansionBackend{window: window, focus: focus}
	if backend.fastTarget() != 1 {
		t.Fatal("window fallback should work before a control is reported")
	}
	focus.blocked.Store(true)
	if backend.fastTarget() != 0 {
		t.Fatal("rejected or unresolved accessible control must block fallback")
	}
	focus.reset()
	if backend.fastTarget() != 1 {
		t.Fatal("new window should discard the previous control")
	}
}

// TestLiveHyprlandFocus checks the real compositor without observing or injecting input.
func TestLiveHyprlandFocus(t *testing.T) {
	if os.Getenv("WOX_TEST_HYPRLAND_FOCUS") != "1" {
		t.Skip("live Hyprland focus test is not enabled")
	}
	focus, err := startHyprlandFocus(func() {})
	if err != nil {
		t.Fatal(err)
	}
	defer focus.close()
	if focus.verifiedTarget() == 0 {
		t.Fatal("could not verify the active Hyprland window")
	}
}
