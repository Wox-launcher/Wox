package woxui

// ProcessGPUMemory reports the graphics memory a driver attributes to this process, split by
// where it lives. SystemBytes is the driver's VidMm / shared-system total. On Windows those
// pages are kernel-managed and are not part of the process private working set, so callers
// must list them separately rather than subtracting them from native private memory.
// DedicatedBytes lives on a discrete adapter and is never part of the process footprint; it
// is always zero on a unified memory adapter. Budgets come from the driver and describe how
// much the process may use before it is asked to release resources.
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
