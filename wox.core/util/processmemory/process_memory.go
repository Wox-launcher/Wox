package processmemory

import (
	"fmt"
	"math"

	"github.com/struCoder/pidusage"
)

// PrivateWorkingSetBreakdown splits a process's private resident pages by allocation type so
// diagnostics can report measured components instead of subtracting incompatible metrics.
type PrivateWorkingSetBreakdown struct {
	PrivateBytes uint64
	MappedBytes  uint64
	ImageBytes   uint64
	Available    bool

	// GoHeapBytes, ThreadStackBytes, NativeHeapBytes and NativeAnonBytes are disjoint and sum to
	// PrivateBytes. They are only filled in when the inspected process is this process, because
	// both Go heap attribution and Win32 heap attribution have to inspect local allocator state.
	//
	// NativeHeapBytes covers pages served by malloc and HeapAlloc, while NativeAnonBytes covers
	// memory a library reserved from the OS directly. Separating them matters because the two
	// have different owners: the former is ordinary C runtime allocation by any loaded component,
	// the latter is dominated by graphics drivers and other subsystems with private allocators.
	GoHeapBytes       uint64
	ThreadStackBytes  uint64
	NativeHeapBytes   uint64
	NativeAnonBytes   uint64
	PrivateAttributed bool
}

func GetProcessRSSBytes(pid int) (uint64, error) {
	return getProcessRSSBytes(pid)
}

func GetProcessMemoryBytes(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid: %d", pid)
	}

	// Keep Wox diagnostics on the native metric shown by the platform monitor:
	// private working set on Windows and physical footprint on macOS.
	return getProcessMemoryBytes(pid)
}

// GetPrivateWorkingSetBreakdown classifies Windows private resident pages by allocation type.
func GetPrivateWorkingSetBreakdown(pid int) (PrivateWorkingSetBreakdown, error) {
	if pid <= 0 {
		return PrivateWorkingSetBreakdown{}, fmt.Errorf("invalid pid: %d", pid)
	}
	return getPrivateWorkingSetBreakdown(pid)
}

// DescendantProcess identifies one process in the subtree below an inspected process. The parent
// id is reported so callers can attribute a helper process to the component that spawned it
// rather than to the root of the subtree.
type DescendantProcess struct {
	ProcessID       int
	ParentProcessID int
	Name            string
}

// ListDescendantProcesses walks the process subtree below pid. Components such as WebView2 run
// out of process, so their cost is invisible in this process's counters and has to be reported
// per child instead.
func ListDescendantProcesses(pid int) ([]DescendantProcess, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid: %d", pid)
	}
	return listDescendantProcesses(pid)
}

func getProcessRSSBytes(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid: %d", pid)
	}

	stat, err := pidusage.GetStat(pid)
	if err != nil {
		return 0, err
	}
	if stat.Memory < 0 || math.IsNaN(stat.Memory) || math.IsInf(stat.Memory, 0) {
		return 0, fmt.Errorf("invalid rss for pid %d: %f", pid, stat.Memory)
	}

	// pidusage reports RSS bytes on macOS/Linux. Windows uses a native PSAPI
	// implementation because pidusage has no working Windows stat backend.
	return uint64(stat.Memory), nil
}
