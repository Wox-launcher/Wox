//go:build windows

package processmemory

import (
	"os"
	"testing"
	"unsafe"
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
	if !breakdown.PrivateAttributed {
		t.Fatal("private pages of this process must be attributed to an owner")
	}
	if got := breakdown.GoHeapBytes + breakdown.ThreadStackBytes + breakdown.NativeHeapBytes + breakdown.NativeAnonBytes; got != breakdown.PrivateBytes {
		t.Fatalf("attributed private pages sum to %d, want the measured private total %d", got, breakdown.PrivateBytes)
	}
	// The C runtime heap always holds the loader's and the runtime's own allocations, so a zero
	// share means the heap walk stopped finding the segments behind malloc and HeapAlloc.
	if breakdown.NativeHeapBytes == 0 {
		t.Fatal("Win32 heap pages must be recognized in this process's private pages")
	}
	// The arena probe derives the Go heap address block from a live heap pointer, so a zero Go
	// share means the probe no longer matches where this runtime places its arenas.
	if breakdown.GoHeapBytes == 0 {
		t.Fatal("Go heap arenas must be recognized in this process's private pages")
	}
	if breakdown.GoHeapBytes > breakdown.PrivateBytes {
		t.Fatalf("Go heap share %d exceeds the private total %d", breakdown.GoHeapBytes, breakdown.PrivateBytes)
	}
}

// TestPrivateWorkingSetBreakdownAttributesHeapAllocations covers the allocation shape that native
// components such as the SQLite page cache produce: a block large enough that the heap manager
// serves it from its own virtual allocation rather than from a segment. Such a block must still
// count as heap memory, otherwise it looks like a library reserved it from the OS directly.
func TestPrivateWorkingSetBreakdownAttributesHeapAllocations(t *testing.T) {
	before, err := GetPrivateWorkingSetBreakdown(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}

	const allocationBytes = 16 << 20
	heap, _, _ := processMemoryKernel32.NewProc("GetProcessHeap").Call()
	if heap == 0 {
		t.Skip("no process heap")
	}
	block, _, _ := processMemoryKernel32.NewProc("HeapAlloc").Call(heap, 0, allocationBytes)
	if block == 0 {
		t.Skip("heap allocation failed")
	}
	defer processMemoryKernel32.NewProc("HeapFree").Call(heap, 0, block)

	// Reserved-but-untouched pages never enter the working set, so the block has to be written.
	touched := unsafe.Slice((*byte)(unsafe.Pointer(block)), allocationBytes)
	for index := range touched {
		touched[index] = 1
	}

	after, err := GetPrivateWorkingSetBreakdown(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if growth := after.NativeHeapBytes - before.NativeHeapBytes; growth < allocationBytes/2 {
		t.Fatalf("heap bucket grew by %d after a %d byte heap allocation, want most of it attributed to the heap", growth, allocationBytes)
	}
}

func TestPrivateWorkingSetBreakdownSkipsAttributionForOtherProcesses(t *testing.T) {
	// Attribution probes this runtime's own heap, so it must stay off for any other process
	// rather than labelling unrelated pages as Go memory.
	breakdown, err := GetPrivateWorkingSetBreakdown(os.Getppid())
	if err != nil {
		t.Skipf("parent process is not inspectable: %v", err)
	}
	if breakdown.PrivateAttributed || breakdown.GoHeapBytes != 0 || breakdown.ThreadStackBytes != 0 || breakdown.NativeHeapBytes != 0 || breakdown.NativeAnonBytes != 0 {
		t.Fatalf("unexpected attribution for another process: %#v", breakdown)
	}
}

func TestListDescendantProcessesExcludesTheRoot(t *testing.T) {
	descendants, err := ListDescendantProcesses(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, descendant := range descendants {
		if descendant.ProcessID == os.Getpid() {
			t.Fatalf("the inspected process must not appear in its own subtree: %#v", descendant)
		}
	}
}
