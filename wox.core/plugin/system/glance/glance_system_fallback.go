//go:build !windows && !linux && !darwin

package glance

import "context"

func readCPUSample(ctx context.Context) (cpuSample, bool) {
	_ = ctx
	return cpuSample{}, false
}

func readMemoryPercent(ctx context.Context) (float64, bool) {
	_ = ctx
	return 0, false
}
