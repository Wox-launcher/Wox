//go:build windows

package supervisor

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestHiddenWindowHangCapture exercises a real blocked native window in a child,
// then verifies the live dump survives report packaging without killing Wox.
func TestHiddenWindowHangCapture(t *testing.T) {
	if os.Getenv("WOX_TEST_HIDDEN_HANG") == "1" {
		runtime.LockOSThread()
		class, _ := windows.UTF16PtrFromString("STATIC")
		hwnd, _, err := hangUser32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(class)), 0, 0, 0, 0, 100, 100, 0, 0, 0, 0)
		if hwnd == 0 {
			t.Fatalf("create window: %v", err)
		}
		fmt.Fprintln(os.Stdout, "ready")
		// Keep the hidden window's owner thread alive without pumping messages.
		time.Sleep(2 * time.Minute)
		return
	}
	setupSupervisorTest(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHiddenWindowHangCapture$")
	cmd.Env = append(os.Environ(), "WOX_TEST_HIDDEN_HANG=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	ready := make(chan bool, 1)
	go func() { scanner := bufio.NewScanner(stdout); ready <- scanner.Scan() && scanner.Text() == "ready" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("child did not create window")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child startup timed out")
	}
	hwnd, err := unresponsiveWindow(cmd.Process.Pid)
	if err != nil || hwnd == 0 {
		t.Fatalf("missed hidden blocked window: hwnd=%x err=%v", hwnd, err)
	}
	m := GetManager()
	stopWatch := m.watchUI(context.Background(), cmd.Process.Pid)
	defer stopWatch()
	var path string
	deadline := time.Now().Add(55 * time.Second)
	for time.Now().Before(deadline) {
		matches, _ := filepath.Glob(filepath.Join(m.CrashDumpsDirectory(), "*-hang-*.dmp"))
		if len(matches) > 0 {
			path = matches[0]
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if path == "" {
		t.Fatal("watchdog did not automatically capture the blocked child")
	}
	stopWatch()
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 4 || string(data[:4]) != "MDMP" {
		t.Fatalf("invalid dump: %v", err)
	}
	report, err := m.ExportCrash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	z, err := zip.OpenReader(report)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	found := false
	for _, file := range z.File {
		if strings.HasPrefix(strings.ReplaceAll(file.Name, `\`, "/"), "windows-dumps/") && strings.HasSuffix(file.Name, ".dmp") {
			found = true
		}
	}
	if !found {
		t.Fatal("report omitted hang dump")
	}
	if !IsProcessRunning(cmd.Process.Pid) {
		t.Fatal("dump capture terminated child")
	}
	if _, err := m.captureHangDump(0); err == nil || !strings.Contains(err.Error(), "open process") {
		t.Fatalf("missing failure stage: %v", err)
	}
}
