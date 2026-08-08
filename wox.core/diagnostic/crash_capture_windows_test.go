//go:build windows

package diagnostic

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
	"wox/util"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

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
	handlerDirectory := filepath.Join(dataDirectory, "diagnostics", "crash-handler")
	defer cleanupWindowsCrashIntegrationRegistry(handlerDirectory, previousDumpFolder, hadPreviousDumpFolder)
	command := exec.Command(os.Args[0], "-test.run=^TestWindowsCrashHandlerIntegration$")
	command.Env = append(os.Environ(),
		util.TestWoxDataDirEnv+"="+dataDirectory,
		util.TestUserDataDirEnv+"="+filepath.Join(dataDirectory, "user"),
		windowsCrashIntegrationChildEnv+"=1",
	)
	err := command.Run()
	if err == nil {
		t.Fatal("crash integration child exited successfully")
	}

	dumpDirectory := filepath.Join(dataDirectory, "diagnostics", "crashes", "dumps")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(dumpDirectory)
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".dmp" {
				info, statErr := entry.Info()
				if statErr == nil && info.Size() > 0 {
					dumpPath := filepath.Join(dumpDirectory, entry.Name())
					dumpFile, openErr := os.Open(dumpPath)
					if openErr != nil {
						t.Fatalf("failed to open captured dump %s: %v", dumpPath, openErr)
					}
					header := make([]byte, 4)
					_, readErr := io.ReadFull(dumpFile, header)
					dumpFile.Close()
					if readErr != nil || string(header) != "MDMP" {
						t.Fatalf("captured file is not a valid minidump: %s", dumpPath)
					}
					t.Logf("captured dump %s (%d bytes)", dumpPath, info.Size())
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("WER crash handler did not create a dump in %s", dumpDirectory)
}

// runWindowsCrashIntegrationChild registers the module before terminating this subprocess.
func runWindowsCrashIntegrationChild(t *testing.T) {
	debug.SetTraceback("wer")
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	manager := GetManager()
	if err := manager.EnsureDirectories(); err != nil {
		t.Fatal(err)
	}
	handlerPath, err := manager.extractWindowsCrashHandler()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.configureWindowsCrashHandlerRegistry(handlerPath); err != nil {
		t.Fatal(err)
	}
	if err := registerWindowsCrashHandler(handlerPath); err != nil {
		t.Fatal(err)
	}
	windows.NewLazySystemDLL("kernel32.dll").NewProc("RaiseFailFastException").Call(0, 0, 0)
	os.Exit(2)
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
