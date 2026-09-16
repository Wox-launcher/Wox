//go:build linux

package processmemory

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLinuxProcessMemoryMatchesSystemMonitorColumn(t *testing.T) {
	pid := os.Getpid()
	got, err := GetProcessMemoryBytes(pid)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		t.Fatalf("invalid statm: %q", strings.TrimSpace(string(data)))
	}
	resident, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if shared > resident {
		shared = resident
	}
	want := (resident - shared) * uint64(os.Getpagesize())
	if got != want {
		t.Fatalf("GetProcessMemoryBytes(%d) = %d, want resident-shared %d", pid, got, want)
	}
	if got == 0 {
		t.Fatal("private process memory must be non-zero")
	}
}

func TestLinuxProcessMemoryRejectsMissingProcess(t *testing.T) {
	_, err := GetProcessMemoryBytes(1 << 30)
	if err == nil {
		t.Fatal("expected error for a missing pid")
	}
}
