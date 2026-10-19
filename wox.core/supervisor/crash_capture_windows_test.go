//go:build windows

package supervisor

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"
	"wox/resource"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// TestFindWindowsCrashDumpsRejectsOtherProcesses prevents a previous incident or
// another application's anonymous WER dump from ending the current crash wait.
func TestFindWindowsCrashDumpsRejectsOtherProcesses(t *testing.T) {
	directory := setupSupervisorTest(t)
	t.Setenv("LOCALAPPDATA", filepath.Join(directory, "local"))
	t.Setenv("ProgramData", filepath.Join(directory, "program"))
	m := GetManager()
	if err := m.EnsureDirectories(); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	standard := filepath.Join(os.Getenv("LOCALAPPDATA"), "CrashDumps")
	temporary := filepath.Join(os.Getenv("ProgramData"), "Microsoft", "Windows", "WER", "Temp")
	for _, folder := range []string{standard, temporary} {
		if err := os.MkdirAll(folder, 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, fixture := range []struct {
		path string
		pid  uint32
	}{
		{filepath.Join(m.CrashDumpsDirectory(), "wox-1234-current.dmp"), 1234},
		{filepath.Join(m.CrashDumpsDirectory(), "wox-4321-previous.dmp"), 4321},
		{filepath.Join(standard, filepath.Base(executable)+".1234.dmp"), 1234},
		{filepath.Join(standard, filepath.Base(executable)+".4321.dmp"), 4321},
		{filepath.Join(temporary, "WER-unrelated.mdmp"), 4321},
		{filepath.Join(temporary, "WER-current.mdmp"), 1234},
	} {
		data := make([]byte, 68)
		copy(data, "MDMP")
		binary.LittleEndian.PutUint32(data[8:], 1)
		binary.LittleEndian.PutUint32(data[12:], 32)
		binary.LittleEndian.PutUint32(data[32:], 15)
		binary.LittleEndian.PutUint32(data[36:], 24)
		binary.LittleEndian.PutUint32(data[40:], 44)
		binary.LittleEndian.PutUint32(data[44:], 24)
		binary.LittleEndian.PutUint32(data[48:], 1)
		binary.LittleEndian.PutUint32(data[52:], fixture.pid)
		if err := os.WriteFile(fixture.path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(temporary, "WER-incomplete.mdmp"), []byte("MDMP"), 0644); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	want := []string{filepath.Join(m.CrashDumpsDirectory(), "wox-1234-current.dmp"), filepath.Join(standard, filepath.Base(executable)+".1234.dmp"), filepath.Join(temporary, "WER-current.mdmp")}
	got := m.findWindowsCrashDumps(1234, now, now)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collected unrelated or incomplete dump: got=%v want=%v", got, want)
	}
}

const (
	windowsCrashIntegrationEnv      = "WOX_CRASH_CAPTURE_INTEGRATION"
	windowsCrashIntegrationChildEnv = "WOX_CRASH_CAPTURE_INTEGRATION_CHILD"
)

func TestDumpNameMatchesPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
		want bool
	}{
		{name: "wox-15836-20260808.dmp", pid: 15836, want: true},
		{name: "wox.exe.15836.dmp", pid: 15836, want: true},
		{name: "wox-115836-20260808.dmp", pid: 15836, want: false},
		{name: "wox-158360-20260808.dmp", pid: 15836, want: false},
		{name: "wox-15836-20260808.dmp", pid: 0, want: false},
	}
	for _, test := range tests {
		if got := dumpNameMatchesPID(test.name, test.pid); got != test.want {
			t.Fatalf("dumpNameMatchesPID(%q, %d) = %v, want %v", test.name, test.pid, got, test.want)
		}
	}
}

// TestWindowsCrashHandlerIntegration intentionally terminates a subprocess and
// is opt-in because Windows records it as a real application crash.
func TestWindowsCrashHandlerIntegration(t *testing.T) {
	if os.Getenv(windowsCrashIntegrationChildEnv) == "1" {
		runWindowsCrashIntegrationChild(t)
		return
	}
	if os.Getenv(windowsCrashIntegrationEnv) != "1" {
		t.Skip("set WOX_CRASH_CAPTURE_INTEGRATION=1 to run the real WER crash test")
	}

	dataDirectory := t.TempDir()
	previousDumpFolder, hadPreviousDumpFolder := readCrashHandlerDumpFolder()
	defer restoreCaptureLogSetting(t)()
	handlerDirectory := filepath.Join(dataDirectory, "diagnostics", "crash-handler")
	defer cleanupWindowsCrashIntegrationRegistry(handlerDirectory, previousDumpFolder, hadPreviousDumpFolder)
	command := exec.Command(os.Args[0], "-test.run=^TestWindowsCrashHandlerIntegration$")
	command.Env = append(os.Environ(),
		"WOX_SUPERVISOR_DATA_DIR="+dataDirectory,
		windowsCrashIntegrationChildEnv+"=1",
	)
	output, err := command.CombinedOutput()
	if command.ProcessState == nil || uint32(command.ProcessState.ExitCode()) != 0xc0000602 {
		t.Fatalf("expected STATUS_FAIL_FAST_EXCEPTION, got %v; output=%s", err, output)
	}
	t.Logf("crash child: %v output=%s", err, output)

	dumpDirectory := filepath.Join(dataDirectory, "diagnostics", "crashes", "dumps")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(dumpDirectory)
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".dmp" {
				info, statErr := entry.Info()
				if statErr == nil && info.Size() > 0 {
					dumpPath := filepath.Join(dumpDirectory, entry.Name())
					assertDumpMemory(t, dumpPath, os.Getenv("WOX_CRASH_TEST_DEBUG") == "1")
					t.Logf("captured dump %s (%d bytes)", dumpPath, info.Size())
					captureLog, err := os.ReadFile(filepath.Join(dataDirectory, "log", "supervisor.log"))
					if err != nil || !strings.Contains(string(captureLog), "stage=dump_completed") {
						t.Fatalf("missing WER completion log: %s error=%v", captureLog, err)
					}
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	captureLog, _ := os.ReadFile(filepath.Join(dataDirectory, "log", "supervisor.log"))
	t.Fatalf("WER crash handler did not create a dump in %s; capture log: %s", dumpDirectory, captureLog)
}

// TestWindowsDebugCrashHandlerIntegration verifies full memory through the real WER path.
func TestWindowsDebugCrashHandlerIntegration(t *testing.T) {
	t.Setenv("WOX_CRASH_TEST_DEBUG", "1")
	TestWindowsCrashHandlerIntegration(t)
}

// restoreCaptureLogSetting preserves the WER logging and debug settings used by native tests.
func restoreCaptureLogSetting(t *testing.T) func() {
	t.Helper()
	var previous string
	var previousDebug uint64
	hadDebug := false
	hadPrevious := false
	if key, err := registry.OpenKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.QUERY_VALUE); err == nil {
		value, _, err := key.GetStringValue("CaptureLog")
		previous, hadPrevious = value, err == nil
		previousDebug, _, err = key.GetIntegerValue("DebugMode")
		hadDebug = err == nil
		key.Close()
	}
	return func() {
		key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE)
		if err != nil {
			t.Errorf("restore capture log setting: %v", err)
			return
		}
		defer key.Close()
		if hadDebug {
			_ = key.SetDWordValue("DebugMode", uint32(previousDebug))
		} else {
			_ = key.DeleteValue("DebugMode")
		}
		if hadPrevious {
			_ = key.SetStringValue("CaptureLog", previous)
		} else {
			_ = key.DeleteValue("CaptureLog")
		}
	}
}

// runWindowsCrashIntegrationChild registers the module before terminating this subprocess.
func runWindowsCrashIntegrationChild(t *testing.T) {
	debug.SetTraceback("wer")
	if os.Getenv("WOX_CRASH_TEST_DEBUG") == "1" {
		os.Args = append(os.Args, ArgDebugMode)
	}
	SetDataDirectory(os.Getenv("WOX_SUPERVISOR_DATA_DIR"))
	manager := GetManager()
	if err := manager.EnsureDirectories(); err != nil {
		t.Fatal(err)
	}
	handler, err := resource.OthersFS.ReadFile("others/crash_handler/WoxCrashHandler64.dll")
	if err != nil {
		t.Fatal(err)
	}
	handlerPath, err := manager.extractWindowsCrashHandler(handler)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.configureWindowsCrashHandlerRegistry(handlerPath); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WOX_CRASH_TEST_FAIL_DUMP") == "1" {
		key, err := registry.OpenKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		err = key.SetStringValue("DumpFolder", filepath.Join(manager.CrashDumpsDirectory(), "missing-directory"))
		key.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := registerWindowsCrashHandler(handlerPath); err != nil {
		t.Fatal(err)
	}
	windows.NewLazySystemDLL("kernel32.dll").NewProc("RaiseFailFastException").Call(0, 0, 0)
	os.Exit(2)
}

// TestWindowsCrashHandlerFailureIntegration forces CreateFile to fail and checks
// that the external WER callback records the failing stage and Windows error.
func TestWindowsCrashHandlerFailureIntegration(t *testing.T) {
	if os.Getenv(windowsCrashIntegrationEnv) != "1" {
		t.Skip("set WOX_CRASH_CAPTURE_INTEGRATION=1 to run the real WER crash test")
	}
	directory := t.TempDir()
	previous, existed := readCrashHandlerDumpFolder()
	defer restoreCaptureLogSetting(t)()
	defer cleanupWindowsCrashIntegrationRegistry(filepath.Join(directory, "diagnostics", "crash-handler"), previous, existed)
	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsCrashHandlerIntegration$")
	cmd.Env = append(os.Environ(), "WOX_SUPERVISOR_DATA_DIR="+directory, windowsCrashIntegrationChildEnv+"=1", "WOX_CRASH_TEST_FAIL_DUMP=1")
	output, err := cmd.CombinedOutput()
	if cmd.ProcessState == nil || uint32(cmd.ProcessState.ExitCode()) != 0xc0000602 {
		t.Fatalf("expected STATUS_FAIL_FAST_EXCEPTION, got %v; output=%s", err, output)
	}
	deadline := time.Now().Add(10 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, _ = os.ReadFile(filepath.Join(directory, "log", "supervisor.log"))
		if strings.Contains(string(data), "stage=create_dump_failed win32Error=3") {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("missing dump failure stage and ERROR_PATH_NOT_FOUND: %s", data)
}

// readCrashHandlerDumpFolder snapshots the user setting changed by the integration test.
func readCrashHandlerDumpFolder() (string, bool) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.QUERY_VALUE)
	if err != nil {
		return "", false
	}
	defer key.Close()
	value, _, err := key.GetStringValue("DumpFolder")
	return value, err == nil
}

// cleanupWindowsCrashIntegrationRegistry restores user state changed by the crashing subprocess.
func cleanupWindowsCrashIntegrationRegistry(handlerDirectory, previousDumpFolder string, hadPreviousDumpFolder bool) {
	if moduleKey, err := registry.OpenKey(registry.CURRENT_USER, windowsWERModuleKey, registry.SET_VALUE|registry.QUERY_VALUE); err == nil {
		valueNames, _ := moduleKey.ReadValueNames(-1)
		for _, valueName := range valueNames {
			if strings.EqualFold(filepath.Dir(valueName), handlerDirectory) {
				_ = moduleKey.DeleteValue(valueName)
			}
		}
		moduleKey.Close()
	}
	if hadPreviousDumpFolder {
		if configKey, _, err := registry.CreateKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE); err == nil {
			_ = configKey.SetStringValue("DumpFolder", previousDumpFolder)
			configKey.Close()
		}
	} else {
		_ = registry.DeleteKey(registry.CURRENT_USER, windowsCrashHandlerKey)
	}
}
