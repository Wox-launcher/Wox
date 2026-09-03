//go:build windows

package processmemory

import (
	"os"
	"testing"
)

func TestTaskManagerPrivateWorkingSet(t *testing.T) {
	handle, err := openProcessForMemory(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	defer processMemoryCloseHandle.Call(handle)

	bytes, err := getTaskManagerPrivateWorkingSetBytes(handle)
	if err != nil || bytes == 0 {
		t.Fatalf("private working set = %d, err=%v", bytes, err)
	}
}

func TestPrivateWorkingSetBreakdown(t *testing.T) {
	breakdown, err := GetPrivateWorkingSetBreakdown(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if !breakdown.Available || breakdown.PrivateBytes+breakdown.MappedBytes+breakdown.ImageBytes == 0 {
		t.Fatalf("unexpected breakdown: %#v", breakdown)
	}
}
