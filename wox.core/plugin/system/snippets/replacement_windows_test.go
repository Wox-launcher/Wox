package snippets

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"wox/util/keyboard"
)

// TestWindowsReplacementBatch verifies the native events without sending them to the desktop.
func TestWindowsReplacementBatch(t *testing.T) {
	compiler, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("GCC unavailable")
	}
	binary := filepath.Join(t.TempDir(), "replacement-test.exe")
	command := exec.Command(compiler, "testdata/replacement_windows.c", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile replacement test: %v\n%s", err, output)
	}
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("replacement events: %v\n%s", err, output)
	}
}

// TestWindowsReplacementInNativeEdit exercises SendInput against a dedicated edit control.
func TestWindowsReplacementInNativeEdit(t *testing.T) {
	if os.Getenv("WOX_TEST_WINDOWS_INPUT_ISOLATED") != "1" {
		t.Skip("set WOX_TEST_WINDOWS_INPUT_ISOLATED=1 to allow a temporary foreground test window")
	}
	compiler, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("GCC unavailable")
	}
	binary := filepath.Join(t.TempDir(), "replacement-native-test.exe")
	command := exec.Command(compiler, "testdata/replacement_windows_native.c", "keyboard_windows.c", "-o", binary, "-luser32", "-lole32", "-loleaut32", "-luuid", "-luiautomationcore")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile native replacement test: %v\n%s", err, output)
	}
	var observer windowsInputObserver
	type observedInput struct{ own, invalidated bool }
	observed := make(chan observedInput, 128)
	subscription, err := keyboard.AddRawKeyListener(func(raw keyboard.RawKeyEvent) bool {
		if raw.NativeFlags&0x10 == 0 {
			return false
		}
		event, handled := observer.translate(raw, 0, func(uint32, uint32) uint32 { return 0 })
		select {
		case observed <- observedInput{own: raw.NativeExtraInfo == windowsSnippetInputTag && !handled, invalidated: raw.NativeExtraInfo != windowsSnippetInputTag && handled && event.kind == inputInvalidate}:
		default:
		}
		return false
	})
	if err != nil {
		t.Fatalf("install native keyboard listener: %v", err)
	}
	defer subscription.Close()
	if output, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("native replacement: %v\n%s", err, output)
	} else if len(output) != 0 {
		t.Logf("native replacement: %s", output)
	}
	deadline := time.After(2 * time.Second)
	ownSeen, otherSeen := false, false
	for !ownSeen || !otherSeen {
		select {
		case event := <-observed:
			ownSeen, otherSeen = ownSeen || event.own, otherSeen || event.invalidated
		case <-deadline:
			t.Fatalf("native hook events: own=%v other=%v", ownSeen, otherSeen)
		}
	}
}
