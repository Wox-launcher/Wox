//go:build windows

package supervisor

import (
	"encoding/binary"
	"fmt"
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
	for _, mode := range []struct{ full, failure bool }{{false, false}, {true, false}, {true, true}} {
		failure := mode.failure
		key, err := registry.OpenKey(registry.CURRENT_USER, windowsCrashHandlerKey, registry.SET_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		folder := m.CrashDumpsDirectory()
		if failure {
			folder = filepath.Join(folder, "missing-directory")
		}
		var full uint32
		if mode.full {
			full = 1
		}
		if err := key.SetDWordValue("DebugMode", full); err != nil {
			t.Fatal(err)
		}
		err = key.SetStringValue("DumpFolder", folder)
		key.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(m.SupervisorLogPath(), nil, 0644); err != nil {
			t.Fatal(err)
		}
		command := exec.Command(executable)
		if output, err := command.CombinedOutput(); err != nil {
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
		if !failure {
			matches, _ := filepath.Glob(filepath.Join(folder, "wox-"+fmt.Sprint(command.Process.Pid)+"-*.dmp"))
			if len(matches) != 1 {
				t.Fatalf("expected one completed dump, got %v", matches)
			}
			assertDumpMemory(t, matches[0], mode.full)
			if mode.full && !strings.Contains(string(data), "stage=full_dump_written") {
				t.Fatalf("missing full dump stage: %s", data)
			}
		}
		if !strings.Contains(string(data), stage) {
			t.Fatalf("missing %s: %s", stage, data)
		}
	}
}

// assertDumpMemory checks actual streams and their backing bytes, not just a valid file signature.
func assertDumpMemory(t *testing.T, path string, full bool) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	header := make([]byte, 32)
	if _, err := file.ReadAt(header, 0); err != nil {
		t.Fatal(err)
	}
	if string(header[:4]) != "MDMP" {
		t.Fatal("invalid dump signature")
	}
	flags := binary.LittleEndian.Uint64(header[24:])
	if (flags&2 != 0) != full {
		t.Fatalf("dump flags=%#x, want full=%t", flags, full)
	}
	count, directory := binary.LittleEndian.Uint32(header[8:]), binary.LittleEndian.Uint32(header[12:])
	streams := make(map[uint32]uint32)
	for i := uint32(0); i < count; i++ {
		entry := make([]byte, 12)
		if _, err := file.ReadAt(entry, int64(directory)+int64(i)*12); err != nil {
			t.Fatal(err)
		}
		streams[binary.LittleEndian.Uint32(entry)] = binary.LittleEndian.Uint32(entry[8:])
	}
	if !full {
		return
	}
	for _, kind := range []uint32{9, 12, 17} {
		if streams[kind] == 0 {
			t.Fatalf("full dump missing stream %d", kind)
		}
	}
	memory := make([]byte, 32)
	if _, err := file.ReadAt(memory, int64(streams[9])); err != nil {
		t.Fatal(err)
	}
	ranges := binary.LittleEndian.Uint64(memory)
	base := binary.LittleEndian.Uint64(memory[8:])
	firstSize := binary.LittleEndian.Uint64(memory[24:])
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if ranges == 0 || firstSize == 0 || base+firstSize > uint64(info.Size()) {
		t.Fatal("full memory stream has no readable backing data")
	}
}
