//go:build windows

package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// TestNativeCaptureStages exercises actual native dump success and failure,
// separately from whether Windows chooses to invoke a registered WER module.
func TestNativeCaptureStages(t *testing.T) {
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc is required for native callback tests")
	}
	directory := setupSupervisorTest(t)
	m := GetManager()
	if err := m.EnsureDirectories(); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(directory, "capture-test.exe")
	if output, err := exec.Command(gcc, "-O2", "-o", executable, "testdata/crash_capture.c", "-ldbghelp", "-ladvapi32").CombinedOutput(); err != nil {
		t.Fatalf("build callback harness: %v %s", err, output)
	}
	previous, existed := readCrashHandlerDumpFolder()
	defer restoreCaptureLogSetting(t)()
	defer cleanupWindowsCrashIntegrationRegistry(directory, previous, existed)
	if err := m.configureWindowsCrashHandlerRegistry(filepath.Join(directory, "unused.dll")); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []bool{false, true} {
		key, err := registry.OpenKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		folder := m.CrashDumpsDirectory()
		if failure {
			folder = filepath.Join(folder, "missing-directory")
		}
		err = key.SetStringValue("DumpFolder", folder)
		key.Close()
		if err != nil {
			t.Fatal(err)
		}
		if output, err := exec.Command(executable).CombinedOutput(); err != nil {
			t.Fatalf("run callback: %v %s", err, output)
		}
		data, err := os.ReadFile(m.SupervisorLogPath())
		if err != nil {
			t.Fatal(err)
		}
		stage := "stage=dump_completed win32Error=0"
		if failure {
			stage = "stage=create_dump_failed win32Error=3"
		}
		if !strings.Contains(string(data), stage) {
			t.Fatalf("missing %s: %s", stage, data)
		}
	}
}
