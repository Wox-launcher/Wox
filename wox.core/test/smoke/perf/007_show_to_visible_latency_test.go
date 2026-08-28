//go:build wox_ui_smoke

package perf

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const (
	showToVisibleWarmupSamples = 4
	showToVisibleSampleCount   = 20
	// Product target for hotkey-to-visible / activation latency. This is the
	// "press the shortcut and it is already there" SLA, not the smoke gate.
	showToVisibleTargetP95 = 120 * time.Millisecond
	// Order-of-magnitude guard so a stalled show path fails CI without treating
	// a loaded runner as a 120ms regression detector.
	showToVisibleBudgetP95 = 300 * time.Millisecond
)

// Test007ShowToVisibleLatency measures in-process activation latency after a hidden launcher is shown.
// Flow: hide the primary launcher -> show through the product window path -> wait for the query input and a presented frame.
// Evidence: twenty steady hide/show cycles report p50/p95/max, and p95 stays under the catastrophic 500ms smoke gate.
func Test007ShowToVisibleLatency(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		samples := make([]time.Duration, 0, showToVisibleWarmupSamples+showToVisibleSampleCount)
		for range showToVisibleWarmupSamples + showToVisibleSampleCount {
			samples = append(samples, measureShowToVisible(t, ctx, client))
		}
		steady := samples[showToVisibleWarmupSamples:]
		p50 := percentileDuration(steady, 0.50)
		p95 := percentileDuration(steady, 0.95)
		maxLatency := slices.Max(steady)
		writeShowToVisibleArtifact(t, samples, steady, p50, p95, maxLatency)
		t.Logf(
			"show-to-visible (activation latency) warmup=%d samples=%d p50=%.1fms p95=%.1fms max=%.1fms target_p95=%.0fms met=%t values_ms=%s",
			showToVisibleWarmupSamples,
			len(steady),
			durationMilliseconds(p50),
			durationMilliseconds(p95),
			durationMilliseconds(maxLatency),
			durationMilliseconds(showToVisibleTargetP95),
			p95 <= showToVisibleTargetP95,
			formatDurationMillis(steady),
		)
		if p95 > showToVisibleBudgetP95 {
			t.Fatalf("show-to-visible p95 %.1fms exceeded %dms smoke budget", durationMilliseconds(p95), showToVisibleBudgetP95.Milliseconds())
		}
	})
}

// measureShowToVisible times one hidden-to-query-ready activation through the product show path.
func measureShowToVisible(t *testing.T, ctx context.Context, client *automationdriver.Client) time.Duration {
	t.Helper()
	hideLauncher(t, ctx, client)
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatalf("reset frame metrics before show: %v", err)
	}
	started := time.Now()
	if err := client.Show(ctx); err != nil {
		t.Fatalf("show launcher: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "launcher.query.input")
		return found
	}); err != nil {
		t.Fatalf("wait for query input: %v", err)
	}
	elapsed := time.Since(started)
	state, err := client.WindowState(ctx, "primary")
	if err != nil {
		t.Fatalf("read launcher state after show: %v", err)
	}
	if !state.Visible {
		t.Fatalf("launcher was not visible after show; last state: %+v", state)
	}
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatalf("read frame metrics after show: %v", err)
	}
	if metrics.PresentedFrameCount < 1 {
		t.Fatalf("show produced no presented frame: %+v", metrics)
	}
	return elapsed
}

// hideLauncher dismisses the primary launcher and waits until it is no longer visible.
func hideLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && !state.Visible
	}); err != nil {
		t.Fatalf("wait for launcher to hide: %v", err)
	}
}

// percentileDuration returns the nearest-rank percentile of a non-empty sample set.
func percentileDuration(samples []time.Duration, quantile float64) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	ordered := slices.Clone(samples)
	slices.Sort(ordered)
	index := int(math.Round(quantile * float64(len(ordered)-1)))
	index = max(0, min(index, len(ordered)-1))
	return ordered[index]
}

func durationMilliseconds(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func formatDurationMillis(samples []time.Duration) string {
	parts := make([]string, 0, len(samples))
	for _, sample := range samples {
		parts = append(parts, strings.TrimRight(strings.TrimRight(formatMillis(durationMilliseconds(sample)), "0"), "."))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func formatMillis(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func writeShowToVisibleArtifact(t *testing.T, all, steady []time.Duration, p50, p95, maxLatency time.Duration) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(perfArtifactEnvironment))
	if dir := strings.TrimSpace(os.Getenv(perfArtifactDirEnvironment)); dir != "" {
		path = filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+".json")
	}
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsExist(err) {
		t.Fatalf("create show-to-visible artifact directory: %v", err)
	}
	payload, err := json.MarshalIndent(map[string]any{
		"case":              t.Name(),
		"metric":            "show_to_visible",
		"target_p95_ms":     durationMilliseconds(showToVisibleTargetP95),
		"budget_p95_ms":     durationMilliseconds(showToVisibleBudgetP95),
		"warmup_samples":    showToVisibleWarmupSamples,
		"steady_samples":    len(steady),
		"p50_ms":            durationMilliseconds(p50),
		"p95_ms":            durationMilliseconds(p95),
		"max_ms":            durationMilliseconds(maxLatency),
		"target_met":        p95 <= showToVisibleTargetP95,
		"all_samples_ms":    durationMillisList(all),
		"steady_samples_ms": durationMillisList(steady),
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode show-to-visible artifact: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write show-to-visible artifact: %v", err)
	}
}

func durationMillisList(samples []time.Duration) []float64 {
	values := make([]float64, 0, len(samples))
	for _, sample := range samples {
		values = append(values, durationMilliseconds(sample))
	}
	return values
}
