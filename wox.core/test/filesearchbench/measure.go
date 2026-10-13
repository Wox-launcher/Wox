package filesearchbench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"testing"

	"wox/util/processmemory"
)

type QueryMetric struct {
	Query          string  `json:"query"`
	Count          int     `json:"result_count"`
	P50Micros      float64 `json:"p50_us"`
	P95Micros      float64 `json:"p95_us"`
	AllocatedBytes uint64  `json:"allocated_bytes_per_query"`
}

type Report struct {
	Implementation       string        `json:"implementation"`
	Platform             string        `json:"platform"`
	GoVersion            string        `json:"go_version"`
	MemoryMetric         string        `json:"memory_metric"`
	Files                int           `json:"files"`
	BuildMillis          float64       `json:"build_ms"`
	DiskBytes            int64         `json:"disk_bytes"`
	HeapBefore           uint64        `json:"heap_before_bytes"`
	HeapIndexed          uint64        `json:"heap_indexed_bytes"`
	ResidentBefore       *uint64       `json:"resident_before_bytes"`
	ResidentIndexed      *uint64       `json:"resident_indexed_bytes"`
	ResidentAfterQueries *uint64       `json:"resident_after_queries_bytes"`
	Queries              []QueryMetric `json:"queries"`
}

// Memory pairs the Go heap with the same native process metric used by Wox diagnostics.
// On Windows this is private working set, not RSS or total mapped bytes.
func Memory() (heap uint64, resident *uint64) {
	debug.FreeOSMemory()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	if value, err := processmemory.GetProcessMemoryBytes(os.Getpid()); err == nil {
		resident = &value
	}
	return stats.HeapAlloc, resident
}

// Measure reports observations only; query execution errors still indicate a broken harness.
func Measure(t *testing.T, report Report, search func(string) (int, error)) {
	t.Helper()
	report.Platform = runtime.GOOS + "/" + runtime.GOARCH
	report.GoVersion = runtime.Version()
	report.MemoryMetric = "platform monitor metric; Windows private working set, macOS physical footprint, Linux monitor memory"
	for _, query := range Queries {
		for range 3 {
			if _, err := search(query); err != nil {
				t.Fatal(err)
			}
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		samples := make([]float64, 20)
		count := 0
		for i := range samples {
			start := queryClock()
			var err error
			count, err = search(query)
			if err != nil {
				t.Fatal(err)
			}
			samples[i] = float64(queryClock()-start) / 1000
		}
		runtime.ReadMemStats(&after)
		sort.Float64s(samples)
		report.Queries = append(report.Queries, QueryMetric{query, count, samples[9], samples[18], (after.TotalAlloc - before.TotalAlloc) / 20})
	}
	_, report.ResidentAfterQueries = Memory()
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(data))
	if output := os.Getenv("WOX_FILESEARCH_COMPARISON_OUTPUT"); output != "" {
		if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
