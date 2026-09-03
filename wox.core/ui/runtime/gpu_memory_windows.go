//go:build windows

package woxui

/*
#include "renderer_windows.h"
*/
import "C"

// processGPUMemory reads the driver's per-process attribution through the shared Direct3D device,
// which keeps the query on the adapter Wox actually renders with.
func processGPUMemory() ProcessGPUMemory {
	var memory C.WoxRendererGpuMemory
	if C.wox_renderer_process_gpu_memory(&memory) < 0 {
		return ProcessGPUMemory{}
	}
	return ProcessGPUMemory{
		SystemBytes:          uint64(memory.system_bytes),
		SystemBudgetBytes:    uint64(memory.system_budget_bytes),
		DedicatedBytes:       uint64(memory.dedicated_bytes),
		DedicatedBudgetBytes: uint64(memory.dedicated_budget_bytes),
		UnifiedMemory:        memory.unified_memory != 0,
		Available:            true,
	}
}
