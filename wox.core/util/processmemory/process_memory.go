package processmemory

import (
	"fmt"
	"math"

	"github.com/struCoder/pidusage"
)

func GetProcessRSSBytes(pid int) (uint64, error) {
	return getProcessRSSBytes(pid)
}

func GetProcessMemoryBytes(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid: %d", pid)
	}

	// Feature change: Wox's debug memory Glance should track the process memory
	// number users compare in platform tools. Platform-specific implementations
	// can use the closest native signal instead of forcing every OS through RSS.
	return getProcessMemoryBytes(pid)
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
