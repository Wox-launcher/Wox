//go:build !windows

package woxui

// processGPUMemory has no portable equivalent. macOS and Linux drivers do not expose a
// per-process graphics allocation counter comparable to the DXGI query used on Windows.
func processGPUMemory() ProcessGPUMemory {
	return ProcessGPUMemory{}
}
