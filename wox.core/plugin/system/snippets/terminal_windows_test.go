package snippets

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestWindowsTerminalExpansion runs in a new terminal window and leaves existing sessions alone.
func TestWindowsTerminalExpansion(t *testing.T) {
	if os.Getenv("WOX_TEST_WINDOWS_TERMINAL_ISOLATED") != "1" {
		t.Skip("set WOX_TEST_WINDOWS_TERMINAL_ISOLATED=1 to open a temporary terminal")
	}
	compiler, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("GCC unavailable")
	}
	terminal, err := exec.LookPath("wt.exe")
	if err != nil {
		t.Skip("Windows Terminal unavailable")
	}
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "terminal-input.exe")
	if output, err := exec.Command(compiler, "testdata/terminal_windows.c", "-municode", "-o", binaryPath).CombinedOutput(); err != nil {
		t.Fatalf("compile terminal input: %v\n%s", err, output)
	}
	ready, result := filepath.Join(dir, "ready"), filepath.Join(dir, "result")
	title := "Wox isolated snippets test " + strconv.FormatInt(time.Now().UnixNano(), 10)
	command := exec.Command(terminal, "-w", "new", "new-tab", "--title", title, binaryPath, ready, result)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = command.Wait() }()
	deadline := time.Now().Add(15 * time.Second)
	var child *os.Process
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(ready); err == nil {
			pid, err := strconv.Atoi(string(data))
			if err == nil {
				child, _ = os.FindProcess(pid)
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if child == nil {
		t.Fatal("terminal helper did not start")
	}
	defer child.Kill()
	user32 := windows.NewLazySystemDLL("user32.dll")
	foreground := user32.NewProc("GetForegroundWindow")
	windowText := user32.NewProc("GetWindowTextW")
	backend := newExpansionBackend()
	var target uint64
	for time.Now().Before(deadline) {
		window, _, _ := foreground.Call()
		var text [256]uint16
		windowText.Call(window, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
		if strings.Contains(windows.UTF16ToString(text[:]), title) {
			target = backend.target()
			if target != 0 {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if target == 0 {
		t.Fatal("terminal did not expose a safe focused target")
	}
	valid := func() bool { return backend.target() == target }
	checks := replacementChecks{valid: valid, current: valid, applied: func() {}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := backend.replace(ctx, target, 0, ";wx", checks); err != nil {
		t.Fatalf("insert trigger: %v", err)
	}
	if err := backend.replace(ctx, target, 3, "Wox terminal line one\r\nWox terminal line two\r\n", checks); err != nil {
		t.Fatalf("expand multiline text: %v", err)
	}
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(result)
		if err == nil && len(data) >= len("Wox terminal line one\r\nWox terminal line two\r\n")*2 {
			if len(data)%2 != 0 {
				t.Fatal("invalid UTF-16 output")
			}
			units := make([]uint16, len(data)/2)
			for i := range units {
				units[i] = binary.LittleEndian.Uint16(data[i*2:])
			}
			if line := string(utf16.Decode(units)); line != "Wox terminal line one\r\nWox terminal line two\r\n" {
				t.Fatal("terminal received unexpected text")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("terminal did not receive the expanded line")
}
