//go:build wox_ui_smoke

package perf

import "runtime"

// Frame phase ceilings in microseconds, applied to the steady-state maximum of every scenario.
//
// These are order-of-magnitude guards, not regression detectors. Each value is about five times
// the highest steady-state P95 measured across the perf scenarios on a developer machine, with a
// floor so a phase that normally costs microseconds still has workable headroom on a loaded CI
// runner. Because one set of limits covers every scenario, the heaviest case sets the bar and the
// lighter ones are gated loosely; per-scenario budgets would be the next step if these need teeth.
// Turning them into real regression detection needs a per-runner stored baseline, which the perf
// artifacts feed.
//
// Reference P95 on an Apple silicon developer machine, worst scenario per phase:
// build/layout net of snapshot 3.6ms (chat stream), snapshot 6us, draw record 2.5ms (list-500),
// accessibility 0.6ms (list-500), native encode 34.3ms (catalog-500).
const (
	perfBuildLayoutBudgetMicroseconds   = 20000
	perfSnapshotBudgetMicroseconds      = 4000
	perfDrawRecordBudgetMicroseconds    = 15000
	perfAccessibilityBudgetMicroseconds = 4000

	// perfGoPhaseTotalBudgetMicroseconds covers the four Go-side phases inside the frame total.
	perfGoPhaseTotalBudgetMicroseconds = 35000
)

// perfNativeEncodeBudgetMicroseconds caps the platform renderer's encode phase.
// This is the one phase dominated by native drawing rather than Go work, so it cannot share a
// cross-platform limit: CoreGraphics, Cairo/Pango, and Direct2D differ by more than the margin
// above, and a macOS measurement says nothing about a Linux CI runner.
func perfNativeEncodeBudgetMicroseconds() int64 {
	switch runtime.GOOS {
	case "darwin":
		return 175000
	default:
		// Linux and Windows renderers have not been measured on their own CI runners. This starts
		// deliberately loose and should be recalibrated from the first CI perf artifacts.
		return 400000
	}
}

// perfFrameBudgetMicroseconds caps the sum of the five measured phases. It stays below the sum of
// the individual ceilings so a frame cannot pass by sitting just under every single limit.
func perfFrameBudgetMicroseconds() int64 {
	return perfNativeEncodeBudgetMicroseconds() + perfGoPhaseTotalBudgetMicroseconds
}
