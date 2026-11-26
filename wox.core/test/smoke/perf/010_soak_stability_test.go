//go:build wox_ui_smoke

package perf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/ui/automation"
)

type soakSample struct {
	Cycle int `json:"cycle"`
	automation.SoakStats
}

type soakQuietPeriod struct {
	Cycle     int     `json:"cycle"`
	Settled   bool    `json:"settled"`
	ElapsedMS float64 `json:"elapsed_ms"`
	CPUMS     float64 `json:"cpu_ms"`
	Frames    uint64  `json:"frames"`
}

type soakDrift struct {
	Peak     float64 `json:"peak_growth"`
	Trend    float64 `json:"projected_growth"`
	Late     float64 `json:"late_projected_growth"`
	PeakMax  float64 `json:"peak_budget"`
	TrendMax float64 `json:"trend_budget"`
}

// Test010SoakStability measures resources after a warmup and checks that hidden windows stop scheduling frames.
func Test010SoakStability(t *testing.T) {
	if os.Getenv("WOX_SOAK_TEST") != "1" {
		t.Skip("set WOX_SOAK_TEST=1 to run the native soak")
	}
	cycles := 200
	if raw := os.Getenv("WOX_SOAK_CYCLES"); raw != "" {
		count, err := strconv.Atoi(raw)
		if err != nil || count < 10 {
			t.Fatalf("WOX_SOAK_CYCLES must be an integer of at least 10, got %q", raw)
		}
		cycles = count
	}
	smoke.CaseWithTimeout(t, 9*time.Minute, func(ctx context.Context, client *automationdriver.Client) {
		warmup := min(20, cycles/4)
		report := struct {
			OS      string               `json:"os"`
			Cycles  int                  `json:"cycles"`
			Warmup  int                  `json:"warmup_cycles"`
			Samples []soakSample         `json:"samples"`
			Quiet   []soakQuietPeriod    `json:"hidden_intervals"`
			Settled automation.SoakStats `json:"settled_resources"`
			Drift   map[string]soakDrift `json:"drift"`
		}{OS: runtime.GOOS, Cycles: cycles, Warmup: warmup}
		if path := os.Getenv(perfArtifactDirEnvironment); path != "" {
			t.Cleanup(func() { writeSoakReport(t, filepath.Join(path, "soak", "stability.json"), report) })
		}
		for cycle := 1; cycle <= cycles; cycle++ {
			smoke.ShowLauncher(t, ctx, client)
			smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+1")
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, fixtureCommandQuery("preview-scope"))
			preview := false
			for _, node := range snapshot.Tree.Nodes {
				preview = preview || (strings.HasPrefix(node.AutomationID, "preview-") && node.Value == "Preview scope initial body")
			}
			if !preview {
				t.Fatalf("cycle %d did not present the plugin preview: %s", cycle, automationdriver.DescribeSnapshot(snapshot))
			}
			if err := client.Hide(ctx); err != nil {
				t.Fatalf("hide cycle %d: %v", cycle, err)
			}
			if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool { return state.Exists && !state.Visible }); err != nil {
				t.Fatalf("wait for hide at cycle %d: %v", cycle, err)
			}
			if cycle == warmup || cycle == cycles || cycle > warmup && cycle%max(1, min(20, cycles/5)) == 0 {
				report.Samples = append(report.Samples, soakSample{cycle, readSoakSample(t, ctx, client)})
			}
			if cycle == cycles/2 || cycle == cycles {
				report.Quiet = append(report.Quiet, checkSoakQuiet(t, ctx, client, cycle, false))
			}
		}
		report.Quiet = append(report.Quiet, checkSoakQuiet(t, ctx, client, cycles, true))
		report.Settled = readSoakSample(t, ctx, client)
		if report.Samples[len(report.Samples)-1].Frames <= report.Samples[0].Frames {
			t.Error("frame metrics did not record the visible workload")
		}
		report.Drift = checkSoakDrift(t, report.Samples)
	})
}

func readSoakSample(t *testing.T, ctx context.Context, client *automationdriver.Client) automation.SoakStats {
	t.Helper()
	stats, err := client.SoakStats(ctx)
	if err != nil {
		t.Fatalf("sample Wox process: %v", err)
	}
	return stats
}

// checkSoakQuiet gates frames in every hidden interval and CPU once staged idle cleanup has finished.
func checkSoakQuiet(t *testing.T, ctx context.Context, client *automationdriver.Client, cycle int, settled bool) soakQuietPeriod {
	t.Helper()
	if settled {
		// The last cycle already stayed hidden for the short sample. Allow the
		// existing 35-second native and core memory cleanup to finish as well.
		hidden := readSoakSample(t, ctx, client)
		time.Sleep(35 * time.Second)
		if current := readSoakSample(t, ctx, client); current.Frames != hidden.Frames {
			t.Errorf("long hide scheduled %d frames before the settled sample", current.Frames-hidden.Frames)
		}
	} else {
		time.Sleep(500 * time.Millisecond)
	}
	before := readSoakSample(t, ctx, client)
	start := time.Now()
	time.Sleep(5 * time.Second)
	after := readSoakSample(t, ctx, client)
	period := soakQuietPeriod{cycle, settled, durationMilliseconds(time.Since(start)), float64(after.CPUTimeNS-before.CPUTimeNS) / 1e6, after.Frames - before.Frames}
	if period.Frames != 0 || settled && period.CPUMS > 50+0.05*period.ElapsedMS {
		t.Errorf("hidden cycle %d (settled=%t) scheduled %d frames and used %.1fms CPU over %.1fms", cycle, settled, period.Frames, period.CPUMS, period.ElapsedMS)
	}
	return period
}

// checkSoakDrift compares peak growth and the fitted growth over the complete post-warmup interval.
func checkSoakDrift(t *testing.T, samples []soakSample) map[string]soakDrift {
	t.Helper()
	const mib = 1024 * 1024
	metrics := []struct {
		name        string
		peak, trend float64
		read        func(automation.SoakStats) float64
	}{
		{"process_bytes", 96 * mib, 48 * mib, func(s automation.SoakStats) float64 { return float64(s.ProcessMemoryBytes) }},
		{"go_retained_bytes", 64 * mib, 32 * mib, func(s automation.SoakStats) float64 { return float64(s.GoRetainedBytes) }},
		{"gpu_bytes", 64 * mib, 32 * mib, func(s automation.SoakStats) float64 { return float64(s.GPU.SystemBytes + s.GPU.DedicatedBytes) }},
		{"goroutines", 20, 10, func(s automation.SoakStats) float64 { return float64(s.Goroutines) }},
		{"handles_or_fds", 40, 20, func(s automation.SoakStats) float64 { return float64(s.HandlesOrFDs) }},
	}
	result := make(map[string]soakDrift, len(metrics))
	span := float64(samples[len(samples)-1].Cycle - samples[0].Cycle)
	late := samples[min(len(samples)-2, len(samples)*2/3):]
	for _, metric := range metrics {
		base := metric.read(samples[0].SoakStats)
		var peak float64
		for _, sample := range samples {
			peak = max(peak, metric.read(sample.SoakStats)-base)
		}
		trend := projectedSoakGrowth(samples, span, metric.read)
		lateTrend := projectedSoakGrowth(late, span, metric.read)
		result[metric.name] = soakDrift{peak, trend, lateTrend, metric.peak, metric.trend}
		// A resource can step up and then plateau as a late cache warms. A
		// trend breach must also persist near the end, while peaks always count.
		if peak > metric.peak || (trend > metric.trend && lateTrend > metric.trend) {
			t.Errorf("%s grew above budget (peak %.1f/%.1f, trend %.1f/%.1f, late trend %.1f)", metric.name, peak, metric.peak, trend, metric.trend, lateTrend)
		}
	}
	return result
}

// projectedSoakGrowth fits a line whose slope is scaled to the complete workload.
func projectedSoakGrowth(samples []soakSample, span float64, read func(automation.SoakStats) float64) float64 {
	base := read(samples[0].SoakStats)
	var sumX, sumY, sumXX, sumXY float64
	for _, sample := range samples {
		x := float64(sample.Cycle-samples[0].Cycle) / span
		y := read(sample.SoakStats) - base
		sumX, sumY, sumXX, sumXY = sumX+x, sumY+y, sumXX+x*x, sumXY+x*y
	}
	n := float64(len(samples))
	return (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
}

// writeSoakReport preserves partial samples if an interaction or resource assertion fails.
func writeSoakReport(t *testing.T, path string, report any) {
	t.Helper()
	payload, err := json.MarshalIndent(report, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0755)
	}
	if err == nil {
		err = os.WriteFile(path, append(payload, '\n'), 0644)
	}
	if err != nil {
		t.Errorf("write soak report: %v", err)
		return
	}
	t.Logf("soak report %s", path)
}
