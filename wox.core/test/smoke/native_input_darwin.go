//go:build wox_ui_smoke && darwin

package smoke

/*
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdint.h>

int woxSmokeActivateApplication(int pid);
int woxSmokeTerminateApplication(int pid);
int woxSmokeFrontmostApplicationPid(void);
int woxSmokePostKeyboardChord(uint16_t modifierKeyCode, uint64_t flags, uint16_t keyCode);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	darwinKeyCodeCommand   = uint16(55)
	darwinEventFlagCommand = uint64(1 << 20)
)

// SendNativeKeyChord posts one modifier-key chord through the real macOS input path.
func SendNativeKeyChord(keys ...string) error {
	if len(keys) != 2 || !strings.EqualFold(keys[0], "command") {
		return fmt.Errorf("unsupported macOS smoke chord %q", strings.Join(keys, "+"))
	}
	keyCodes := map[string]uint16{"a": 0, "c": 8, "space": 49}
	keyCode, ok := keyCodes[strings.ToLower(keys[1])]
	if !ok {
		return fmt.Errorf("unsupported macOS smoke key %q", keys[1])
	}
	if C.woxSmokePostKeyboardChord(C.uint16_t(darwinKeyCodeCommand), C.uint64_t(darwinEventFlagCommand), C.uint16_t(keyCode)) == 0 {
		return fmt.Errorf("post macOS smoke chord %q", strings.Join(keys, "+"))
	}
	return nil
}

// ActivateDarwinApplication activates one application without changing other instances.
func ActivateDarwinApplication(pid int) bool {
	return C.woxSmokeActivateApplication(C.int(pid)) != 0
}

// TerminateDarwinApplication asks one application instance to terminate cleanly.
func TerminateDarwinApplication(pid int) bool {
	return C.woxSmokeTerminateApplication(C.int(pid)) != 0
}

// FrontmostDarwinApplicationPID returns the current macOS foreground application process.
func FrontmostDarwinApplicationPID() int {
	return int(C.woxSmokeFrontmostApplicationPid())
}

// OpenDarwinTextEdit launches, focuses, and registers cleanup for one isolated TextEdit instance.
func OpenDarwinTextEdit(t *testing.T, ctx context.Context, path string) int {
	t.Helper()
	before := darwinProcessIDs(t, "TextEdit")
	args := []string{"-n", "-a", "TextEdit"}
	if path != "" {
		args = append(args, path)
	}
	if err := exec.Command("open", args...).Run(); err != nil {
		t.Fatalf("start macOS TextEdit: %v", err)
	}
	pid := waitForNewDarwinProcess(t, ctx, "TextEdit", before)
	t.Cleanup(func() { stopDarwinApplication(t, pid, "TextEdit") })
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if ActivateDarwinApplication(pid) && FrontmostDarwinApplicationPID() == pid {
			return pid
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for macOS TextEdit process %d to become foreground: %v", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

// darwinProcessIDs returns the current process set for one exact executable name.
func darwinProcessIDs(t *testing.T, executable string) map[int]bool {
	t.Helper()
	output, err := exec.Command("pgrep", "-x", executable).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return map[int]bool{}
		}
		t.Fatalf("list macOS %s processes: %v", executable, err)
	}
	pids := map[int]bool{}
	for _, value := range strings.Fields(string(output)) {
		pid, parseErr := strconv.Atoi(value)
		if parseErr != nil {
			t.Fatalf("parse macOS %s process ID %q: %v", executable, value, parseErr)
		}
		pids[pid] = true
	}
	return pids
}

// waitForNewDarwinProcess waits for one process absent from the initial snapshot.
func waitForNewDarwinProcess(t *testing.T, ctx context.Context, executable string, before map[int]bool) int {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		for pid := range darwinProcessIDs(t, executable) {
			if !before[pid] {
				return pid
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for a new macOS %s process: %v", executable, ctx.Err())
		case <-ticker.C:
		}
	}
}

// stopDarwinApplication terminates only the process created by a smoke case.
func stopDarwinApplication(t *testing.T, pid int, application string) {
	t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Errorf("find macOS %s process %d for cleanup: %v", application, pid, err)
		return
	}
	if TerminateDarwinApplication(pid) {
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
		t.Errorf("stop macOS %s process %d: %v", application, pid, err)
	}
}
