//go:build wox_ui_smoke && darwin

package hotkey

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	darwinKeyCodeCommand = uint16(55)
	darwinKeyCodeSpace   = uint16(49)
)

const darwinEventFlagCommand = uint64(1 << 20)

func requireIgnoredHotkeyAppRuntime(t *testing.T) {
	t.Helper()
}

func ignoredHotkeyAppTarget(t *testing.T) (string, string) {
	t.Helper()
	return "com.apple.TextEdit", "com.apple.TextEdit"
}

func ignoredHotkeyAppHotkey(t *testing.T) string {
	t.Helper()
	return "cmd+space"
}

func sendIgnoredAppNativeHotkey(t *testing.T, hotkey string) {
	t.Helper()
	if hotkey != "cmd+space" {
		t.Fatalf("unsupported native macOS ignored-app hotkey %q", hotkey)
	}
	if !postDarwinKeyboardChord(darwinKeyCodeCommand, darwinEventFlagCommand, darwinKeyCodeSpace) {
		t.Fatalf("post native macOS ignored-app hotkey %q", hotkey)
	}
}

// activateIgnoredHotkeyApp launches and focuses one new TextEdit instance without touching existing instances.
func activateIgnoredHotkeyApp(t *testing.T, ctx context.Context) {
	t.Helper()
	before := textEditProcessIDs(t)
	if err := exec.Command("open", "-n", "-a", "TextEdit").Run(); err != nil {
		t.Fatalf("start macOS TextEdit: %v", err)
	}

	pid := waitForNewTextEditProcess(t, ctx, before)
	t.Cleanup(func() { stopTextEditProcess(t, pid) })
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if activateDarwinApplication(pid) && frontmostDarwinApplicationPID() == pid {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for macOS TextEdit process %d to become foreground: %v", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

// textEditProcessIDs returns the current TextEdit process set before or after launching a new instance.
func textEditProcessIDs(t *testing.T) map[int]bool {
	t.Helper()
	output, err := exec.Command("pgrep", "-x", "TextEdit").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return map[int]bool{}
		}
		t.Fatalf("list macOS TextEdit processes: %v", err)
	}
	pids := map[int]bool{}
	for _, value := range strings.Fields(string(output)) {
		pid, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			t.Fatalf("parse macOS TextEdit process ID %q: %v", value, parseErr)
		}
		pids[pid] = true
	}
	return pids
}

// waitForNewTextEditProcess waits until open -n publishes a process absent from the initial snapshot.
func waitForNewTextEditProcess(t *testing.T, ctx context.Context, before map[int]bool) int {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		for pid := range textEditProcessIDs(t) {
			if !before[pid] {
				return pid
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for a new macOS TextEdit process: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// stopTextEditProcess terminates only the TextEdit instance created by this smoke case.
func stopTextEditProcess(t *testing.T, pid int) {
	t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Errorf("find macOS TextEdit process %d for cleanup: %v", pid, err)
		return
	}
	if terminateDarwinApplication(pid) {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for range 40 {
			if err := process.Signal(syscall.Signal(0)); err != nil {
				return
			}
			<-ticker.C
		}
	}
	if err := process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Errorf("stop macOS TextEdit process %d: %v", pid, err)
	}
}
