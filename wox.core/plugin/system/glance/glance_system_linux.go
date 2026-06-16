//go:build linux

package glance

import (
	"context"
	"os"
	"strconv"
	"strings"
)

func readCPUSample(ctx context.Context) (cpuSample, bool) {
	_ = ctx
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}

	lines := strings.SplitN(string(data), "\n", 2)
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuSample{}, false
	}

	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	if len(values) < 4 {
		return cpuSample{}, false
	}

	// New feature: CPU Glance reads Linux's cumulative /proc/stat counters
	// directly. This keeps the 3-second refresh lightweight and lets the shared
	// sampler calculate a real percentage from total and idle deltas.
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuSample{idle: idle, total: total, valid: true}, true
}

func readMemoryPercent(ctx context.Context) (float64, bool) {
	_ = ctx
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, false
	}

	memInfo := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		memInfo[key] = value
	}

	total := memInfo["MemTotal"]
	available := memInfo["MemAvailable"]
	if total == 0 || available > total {
		return 0, false
	}

	// New feature: Memory Glance uses MemAvailable rather than MemFree so Linux
	// page cache does not make normal cached memory look like real pressure.
	return 100 * float64(total-available) / float64(total), true
}
