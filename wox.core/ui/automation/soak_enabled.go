//go:build wox_automation

package automation

import (
	"os"
	"runtime"
	woxui "wox/ui/runtime"
	"wox/util/processmemory"
)

// readSoakStats avoids forcing GC or requesting a frame during hidden measurements.
func readSoakStats(controller Controller) (SoakStats, error) {
	frames, err := controller.AutomationFrameMetrics()
	if err != nil {
		return SoakStats{}, err
	}
	process, err := processmemory.GetProcessMemoryBytes(os.Getpid())
	if err != nil {
		return SoakStats{}, err
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	cpu, handles, err := processSoakCounters()
	if err != nil {
		return SoakStats{}, err
	}
	return SoakStats{
		ProcessMemoryBytes: process, GoRetainedBytes: stats.Sys - stats.HeapReleased,
		GoHeapAllocBytes: stats.HeapAlloc, Goroutines: runtime.NumGoroutine(),
		HandlesOrFDs: handles, CPUTimeNS: cpu, Frames: frames.FrameCount, GPU: woxui.ProcessGPUMemoryUsage(),
	}, nil
}
