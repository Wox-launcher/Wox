package snippets

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestWindowsNotepadExpansion exercises replacement in a temporary Notepad document.
func TestWindowsNotepadExpansion(t *testing.T) {
	if os.Getenv("WOX_TEST_WINDOWS_EDITOR_ISOLATED") != "1" {
		t.Skip("set WOX_TEST_WINDOWS_EDITOR_ISOLATED=1 to open a temporary Notepad document")
	}
	file := filepath.Join(t.TempDir(), "wox-isolated-snippet.txt")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("notepad.exe", file)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer command.Process.Kill()
	go func() { _ = command.Wait() }()
	user32 := windows.NewLazySystemDLL("user32.dll")
	foreground := user32.NewProc("GetForegroundWindow")
	windowText := user32.NewProc("GetWindowTextW")
	keyEvent := user32.NewProc("keybd_event")
	backend := newExpansionBackend()
	deadline := time.Now().Add(20 * time.Second)
	var target uint64
	for time.Now().Before(deadline) {
		window, _, _ := foreground.Call()
		var title [256]uint16
		windowText.Call(window, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		if strings.Contains(windows.UTF16ToString(title[:]), filepath.Base(file)) {
			target = backend.target()
			if target != 0 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if target == 0 {
		t.Fatal("Notepad did not expose a safe focused target")
	}
	var class [128]uint16
	user32.NewProc("GetClassNameW").Call(uintptr(target), uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
	t.Logf("Notepad focused control class: %s", windows.UTF16ToString(class[:]))
	valid := func() bool { return backend.target() == target }
	checks := replacementChecks{valid: valid, current: valid, applied: func() {}}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := backend.replace(ctx, target, 0, ";wx", checks); err != nil {
		t.Fatalf("insert trigger: %v", err)
	}
	const expected = "Hello 😀\r\nWox line two"
	if err := backend.replace(ctx, target, 3, expected, checks); err != nil {
		window, _, _ := foreground.Call()
		var title [256]uint16
		windowText.Call(window, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		t.Logf("expected window focused=%v, target unchanged=%v", strings.Contains(windows.UTF16ToString(title[:]), filepath.Base(file)), backend.target() == target)
		t.Fatalf("expand text: %v", err)
	}
	if !valid() {
		t.Fatal("Notepad focus changed before saving")
	}
	save := func() {
		keyEvent.Call(0x11, 0, 0, 0) // Ctrl+S saves the temporary file.
		keyEvent.Call('S', 0, 0, 0)
		keyEvent.Call('S', 0, 2, 0)
		keyEvent.Call(0x11, 0, 2, 0)
	}
	waitFor := func(want string) bool {
		for time.Now().Before(deadline) {
			data, err := os.ReadFile(file)
			if err == nil && strings.TrimPrefix(string(data), "\xef\xbb\xbf") == want {
				return true
			}
			time.Sleep(50 * time.Millisecond)
		}
		return false
	}
	save()
	if !waitFor(expected) {
		t.Fatal("Notepad did not save the expected expanded text")
	}
	if err := backend.replace(ctx, target, 0, "👩‍💻", checks); err != nil {
		t.Fatalf("insert joined emoji: %v", err)
	}
	save()
	if !waitFor(expected + "👩‍💻") {
		t.Fatal("Notepad did not save the joined emoji")
	}
	keyEvent.Call(0x08, 0, 0, 0)
	keyEvent.Call(0x08, 0, 2, 0)
	save()
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(file)
		if err == nil {
			switch strings.TrimPrefix(string(data), "\xef\xbb\xbf") {
			case expected:
				t.Log("Notepad removed the joined emoji as one grapheme")
				return
			case expected + "👩‍":
				t.Log("Notepad removed one code point from the joined emoji")
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Notepad did not save the expected result after Backspace")
}
