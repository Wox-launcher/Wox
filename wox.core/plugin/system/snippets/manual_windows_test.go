package snippets

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"wox/util/clipboard"
)

// TestWindowsManualPhysicalInput verifies real physical typing through the hook and worker.
func TestWindowsManualPhysicalInput(t *testing.T) {
	if os.Getenv("WOX_TEST_WINDOWS_MANUAL") != "1" {
		t.Skip("requires physical typing in an isolated foreground window")
	}
	compiler, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("GCC unavailable")
	}
	binary := filepath.Join(t.TempDir(), "snippet-manual-test.exe")
	if output, err := exec.Command(compiler, "testdata/manual_windows.c", "-o", binary, "-luser32").CombinedOutput(); err != nil {
		t.Fatalf("compile manual test: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 105*time.Second)
	defer cancel()
	multiline := os.Getenv("WOX_TEST_WINDOWS_MANUAL_MULTILINE") != ""
	text := "Wox test expanded"
	var previous clipboard.Data
	if multiline {
		text = "Wox line one\r\nWox line two"
		previous, err = clipboard.Read()
		if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
			t.Fatalf("read clipboard before test: %v", err)
		}
	}
	session, err := startExpansion(ctx, newExpansionBackend(), []Record{{ID: "manual", Keyword: ";wx", Text: text}})
	if err != nil {
		t.Fatal(err)
	}
	defer session.close()
	command := exec.CommandContext(ctx, binary)
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(output)
	var states []string
	for scanner.Scan() {
		state := scanner.Text()
		states = append(states, state)
		t.Log(state)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("manual input test: %v; states=%v", err, states)
	}
	if multiline {
		if len(states) != 3 || states[0] != "READY" || states[1] != "EXPANDED" || states[2] != "UNDONE" {
			t.Fatalf("unexpected states: %v", states)
		}
		time.Sleep(250 * time.Millisecond)
		current, err := clipboard.Read()
		if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
			t.Fatalf("read clipboard after test: %v", err)
		}
		if (previous == nil) != (current == nil) {
			t.Fatal("clipboard type changed")
		}
		if previous != nil {
			before, beforeErr := previous.MarshalJSON()
			after, afterErr := current.MarshalJSON()
			if beforeErr != nil || afterErr != nil || previous.GetType() != current.GetType() || !bytes.Equal(before, after) {
				t.Fatal("clipboard content was not restored")
			}
		}
		return
	}
	if len(states) != 3 || states[0] != "READY" || states[1] != "EXPANDED" || states[2] != "UNDONE" {
		t.Fatalf("unexpected states: %v", states)
	}
}
