//go:build wox_ui_smoke && darwin

package smoke

/*
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdint.h>
#include <stdlib.h>

int woxSmokeActivateApplication(int pid);
int woxSmokeTerminateApplication(int pid);
int woxSmokeForceTerminateApplication(int pid);
int woxSmokeFrontmostApplicationPid(void);
char *woxSmokeFrontmostApplicationBundleID(void);
int woxSmokeSessionAllowsForegroundActivation(void);
int woxSmokeCanPostKeyboardEvents(void);
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
	"unsafe"
)

const (
	darwinKeyCodeCommand   = uint16(55)
	darwinEventFlagCommand = uint64(1 << 20)
)

// CanPostDarwinKeyboardEvents checks permission without prompting on CI hosts.
func CanPostDarwinKeyboardEvents() bool {
	return C.woxSmokeCanPostKeyboardEvents() != 0
}

// SendNativeKeyChord posts one modifier-key chord through the real macOS input path.
func SendNativeKeyChord(keys ...string) error {
	if !CanPostDarwinKeyboardEvents() {
		return fmt.Errorf("macOS does not allow this test process to post keyboard events")
	}
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

// ForceTerminateDarwinApplication ends one application instance without save dialogs.
func ForceTerminateDarwinApplication(pid int) bool {
	return C.woxSmokeForceTerminateApplication(C.int(pid)) != 0
}

// FrontmostDarwinApplicationPID returns the current macOS foreground application process.
func FrontmostDarwinApplicationPID() int {
	return int(C.woxSmokeFrontmostApplicationPid())
}

// frontmostDarwinBundleID copies the native bundle identifier for failure diagnostics.
func frontmostDarwinBundleID() string {
	value := C.woxSmokeFrontmostApplicationBundleID()
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func darwinForegroundActivationAvailable() bool {
	return C.woxSmokeSessionAllowsForegroundActivation() != 0
}

// activateDarwinTextEdit asks only the tracked TextEdit instance to come forward.
// `open -a TextEdit` without `-n` would hand the document to a different instance
// that smoke cleanup never owns, leaving an unsaved window after TempDir deletion.
func activateDarwinTextEdit(pid int) {
	_ = ActivateDarwinApplication(pid)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	script := fmt.Sprintf(`tell application "System Events" to set frontmost of (first process whose unix id is %d) to true`, pid)
	_ = exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

// OpenDarwinTextEdit launches, focuses, and registers cleanup for one isolated TextEdit instance.
func OpenDarwinTextEdit(t *testing.T, ctx context.Context, path string) int {
	t.Helper()
	// A locked session keeps loginwindow frontmost, so an isolated TextEdit
	// instance can never become the source app for clipboard or hotkey ignores.
	if !darwinForegroundActivationAvailable() {
		t.Skipf("macOS session cannot activate TextEdit (frontmost pid=%d bundle=%s)", FrontmostDarwinApplicationPID(), frontmostDarwinBundleID())
	}
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
	activateCtx, cancelActivate := context.WithTimeout(ctx, 10*time.Second)
	defer cancelActivate()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	nudge := time.NewTicker(250 * time.Millisecond)
	defer nudge.Stop()
	activateDarwinTextEdit(pid)
	for {
		// Only the isolated instance owns the document that subsequent chords target.
		if FrontmostDarwinApplicationPID() == pid {
			return pid
		}
		select {
		case <-activateCtx.Done():
			// The session may have locked during activation; other timeouts are failures.
			if !darwinForegroundActivationAvailable() {
				t.Skipf("macOS session cannot activate TextEdit (frontmost pid=%d bundle=%s)", FrontmostDarwinApplicationPID(), frontmostDarwinBundleID())
			}
			t.Fatalf("wait for macOS TextEdit process %d to become foreground (frontmost pid=%d bundle=%s): %v", pid, FrontmostDarwinApplicationPID(), frontmostDarwinBundleID(), activateCtx.Err())
		case <-nudge.C:
			activateDarwinTextEdit(pid)
		case <-ticker.C:
			_ = ActivateDarwinApplication(pid)
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
	// TextEdit keeps unsaved smoke documents open across terminate:, so force
	// the tracked instance down instead of leaving a save sheet behind.
	if ForceTerminateDarwinApplication(pid) || TerminateDarwinApplication(pid) {
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
