package woxui

// ProcessGPUMemory reports the graphics memory a driver attributes to this process, split by
// where it lives. SystemBytes is system memory the driver holds on the GPU's behalf, so it is
// part of the process footprint. DedicatedBytes lives on the adapter and is not, and it is always
// zero on a unified memory adapter because such an adapter has no dedicated video memory.
// Budgets come from the driver and describe how much the process may use before it is asked to
// release resources.
type ProcessGPUMemory struct {
	SystemBytes          uint64
	SystemBudgetBytes    uint64
	DedicatedBytes       uint64
	DedicatedBudgetBytes uint64
	UnifiedMemory        bool
	Available            bool
}

// ProcessGPUMemoryUsage reports driver-attributed graphics memory for this process. It reports
// zeroed counters while no window holds a graphics device, which is the expected state after a
// hidden window has been trimmed.
func ProcessGPUMemoryUsage() ProcessGPUMemory {
	return processGPUMemory()
}
