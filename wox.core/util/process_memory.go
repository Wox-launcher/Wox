package util

import (
	"fmt"
	"math"
	"sync/atomic"

	"github.com/struCoder/pidusage"
)

var woxUIProcessPid atomic.Int64

func SetWoxUIProcessPid(pid int) {
	if pid <= 0 {
		return
	}

	// Debug Glance needs the Flutter process even when dev mode launches it
	// outside the core process tree, so keep the latest UI-reported PID here.
	woxUIProcessPid.Store(int64(pid))
}

func ClearWoxUIProcessPid(pid int) {
	if pid <= 0 {
		return
	}

	// The UI process can restart while an older wait goroutine is still
	// unwinding. Compare-and-swap prevents that stale exit from erasing the new
	// Flutter PID used by Wox memory diagnostics.
	woxUIProcessPid.CompareAndSwap(int64(pid), 0)
}

func GetWoxUIProcessPid() int {
	return int(woxUIProcessPid.Load())
}

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
