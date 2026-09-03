package processmemory

import (
	"fmt"
	"math"

	"github.com/struCoder/pidusage"
)

type PrivateWorkingSetBreakdown struct {
	PrivateBytes uint64
	MappedBytes  uint64
	ImageBytes   uint64
	Available    bool
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
