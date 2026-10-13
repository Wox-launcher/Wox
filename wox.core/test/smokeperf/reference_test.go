package smokeperf

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestPerformanceQueriesReachDifferentPlugins(t *testing.T) {
	queries := performanceQueries()
	if len(queries) != QueryCount {
		t.Fatalf("queries = %d, want %d", len(queries), QueryCount)
	}
	seen := map[string]struct{}{}
	for _, query := range queries {
		if strings.TrimSpace(query) == "" {
			t.Fatal("empty performance query")
		}
		if _, ok := seen[query]; ok {
			t.Fatalf("duplicate performance query %q", query)
		}
		seen[query] = struct{}{}
	}
	for _, want := range []string{
		"12*34",
		"meters in 10 km",
		"100 usd in eur",
		"#3a7bd5",
		"notepad",
		"volume",
		"设置",
		"readme",
		"https://example.com",
		"emoji smile",
		"cb perf-ref",
		"note perf-ref",
		"timer 5m",
		"theme dark",
		"h perf-ref",
		"> echo wox-perf",
		"window left",
		"hotkeys show",
		"screenshot perf-ref",
	} {
		if _, ok := seen[want]; !ok {
			t.Fatalf("missing %q in %#v", want, queries)
		}
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	samples := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}
	if got := percentile(samples, 0.50); got != 30*time.Millisecond {
		t.Fatalf("p50 = %s, want 30ms", got)
	}
	if got := percentile(samples, 0.95); got != 50*time.Millisecond {
		t.Fatalf("p95 = %s, want 50ms", got)
	}
}

func TestParseLineReadsGoTestPrefix(t *testing.T) {
	ms := 18.4
	bytes := uint64(80 * 1024 * 1024)
	line := "    008_performance_reference_test.go:40: " + Reference{
		When:               "perf/008",
		ShowToVisibleP95Ms: &ms,
		ShowSamples:        20,
		QueryMemoryBytes:   &bytes,
	}.Line()
	parsed, ok := ParseLine(line)
	if !ok {
		t.Fatalf("line %q was not parsed", line)
	}
	if parsed.When != "perf/008" || parsed.ShowToVisibleP95Ms == nil || *parsed.ShowToVisibleP95Ms != ms {
		t.Fatalf("parsed = %+v", parsed)
	}
	if parsed.QueryMemoryBytes == nil || *parsed.QueryMemoryBytes != bytes {
		t.Fatalf("parsed memory = %+v", parsed.QueryMemoryBytes)
	}
}

func TestMergeKeepsEarlierReadings(t *testing.T) {
	ms := 12.5
	base := Reference{ShowToVisibleP95Ms: &ms, ShowSamples: 20}
	hidden := uint64(50 * 1024 * 1024)
	merged := base.Merge(Reference{HiddenMemoryBytes: &hidden})
	if merged.ShowToVisibleP95Ms == nil || *merged.ShowToVisibleP95Ms != ms {
		t.Fatalf("merge dropped the latency reading: %+v", merged)
	}
	if merged.HiddenMemoryBytes == nil || *merged.HiddenMemoryBytes != hidden {
		t.Fatalf("merge dropped the hidden reading: %+v", merged)
	}
}

func TestWriteSectionOmitsAnEmptyReference(t *testing.T) {
	var buffer strings.Builder
	WriteSection(&buffer, Reference{})
	if buffer.Len() != 0 {
		t.Fatalf("empty section = %q", buffer.String())
	}
}

func TestWriteSectionPrintsTheThreeReadings(t *testing.T) {
	ms := 18.4
	queries := uint64(math.Round(82.1 * 1024 * 1024))
	hidden := uint64(math.Round(57.3 * 1024 * 1024))
	collected := 68120.0
	var buffer strings.Builder
	WriteSection(&buffer, Reference{
		When:               "fresh process, before cases",
		CollectedInMs:      &collected,
		ShowToVisibleP95Ms: &ms,
		ShowSamples:        20,
		QueryMemoryBytes:   &queries,
		HiddenMemoryBytes:  &hidden,
	})
	text := buffer.String()
	for _, line := range []string{
		"Performance reference",
		"20 samples after 4 warmups",
		"20 distinct queries across plugins",
		"46s after hide",
		"fresh process, before cases. Collected in 1m08.12s.",
		"show to visible p95",
		"18.4 ms (20 samples)",
		"memory after 20 queries",
		"82.1 MB",
		"memory after 46s hidden",
		"57.3 MB",
	} {
		if !strings.Contains(text, line) {
			t.Fatalf("section = %s, want %q", text, line)
		}
	}
	if strings.Index(text, "show to visible p95") > strings.Index(text, "memory after 46s hidden") {
		t.Fatalf("section = %s, want latency before the hidden reading", text)
	}
}

func TestWriteSectionReportsAnUnavailableReading(t *testing.T) {
	var buffer strings.Builder
	WriteSection(&buffer, Reference{ShowError: "wait for presented show frame: context deadline exceeded"})
	text := buffer.String()
	if !strings.Contains(text, "show to visible p95") || !strings.Contains(text, "unavailable: wait for presented show frame") {
		t.Fatalf("section = %s, want the latency failure", text)
	}
	if !strings.Contains(text, "memory after 20 queries  unavailable") && !strings.Contains(text, "unavailable") {
		t.Fatalf("section = %s, want the missing memory readings", text)
	}
}
