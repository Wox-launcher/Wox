//go:build wox_ui_smoke

package perf

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	"wox/test/smokeperf"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test009KeystrokeToResultsLatency measures a real query edit through the first presented result.
// Flow: show the launcher -> prepare 1+19 -> press Backspace -> wait for 1+1 and its result.
// Evidence: native presentation timestamps for result frames yield steady P50, P95 and maximum latency.
func Test009KeystrokeToResultsLatency(t *testing.T) {
	requireNativePerf(t)
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		const warmupCount, steadyCount = 4, 20
		const scenario = smokeperf.ScenarioKeystroke
		smoke.ShowLauncher(t, ctx, client)
		samples := make([]time.Duration, 0, warmupCount+steadyCount)
		for range cap(samples) {
			smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "1+19")
			if err := client.PressKey(ctx, woxui.KeyEnd, 0); err != nil {
				t.Fatalf("move caret to the end of the query: %v", err)
			}
			samples = append(samples, measureKeystrokeToResults(t, ctx, client))
		}
		budget := latencyBudget(scenario, runtime.GOOS)
		header := perfArtifactHeader(t, scenario, smokeperf.LatencyMetricKeystroke, samples[warmupCount:])
		writePerfJSON(t, scenario, struct {
			smokeperf.PerformanceArtifact
			BudgetP95Ms float64   `json:"budget_p95_ms"`
			Warmup      int       `json:"warmup_samples"`
			All         []float64 `json:"all_samples_ms"`
		}{header, durationMilliseconds(budget), warmupCount, durationMillisList(samples)})
		t.Logf("keystroke-to-results samples=%d p50=%.2fms p95=%.2fms max=%.2fms budget=%.0fms", steadyCount, header.P50Ms, header.P95Ms, header.MaxMs, durationMilliseconds(budget))
		if header.P95Ms > durationMilliseconds(budget) {
			t.Fatalf("keystroke-to-results p95 %.2fms exceeded %s", header.P95Ms, budget)
		}
	})
}

// measureKeystrokeToResults matches the edited query's result tree to a native frame.
func measureKeystrokeToResults(t *testing.T, ctx context.Context, client *automationdriver.Client) time.Duration {
	t.Helper()
	started := time.Now()
	if err := client.PressKey(ctx, woxui.KeyBackspace, 0); err != nil {
		t.Fatalf("edit calculator query: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		if !found || input.Value != "1+1" || snapshot.FrameID == 0 {
			return false
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for edited calculator result: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)

	wait, cancel := context.WithTimeout(ctx, frameSampleBudget)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	var latency time.Duration
	for latency == 0 {
		metrics, err := client.FrameMetrics(wait)
		if err != nil {
			t.Fatalf("read result frame metrics: %v", err)
		}
		for _, frame := range metrics.Recent {
			// A coalesced result frame can be replaced by a later frame with the same query.
			if frame.FrameID >= snapshot.FrameID && frame.HostCompleted && frame.Presented &&
				frame.PresentedAtUnixMicroseconds > started.UnixMicro() {
				elapsed := time.Duration(frame.PresentedAtUnixMicroseconds-started.UnixMicro()) * time.Microsecond
				if latency == 0 || elapsed < latency {
					latency = elapsed
				}
			}
		}
		if latency == 0 {
			select {
			case <-wait.Done():
				t.Fatalf("result frame %d did not present: %s", snapshot.FrameID, describeFrameMetrics(metrics))
			case <-ticker.C:
			}
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, hasInput := automationdriver.Find(snapshot, "launcher.query.input")
		results, hasResults := automationdriver.Find(snapshot, "launcher.results")
		return hasInput && input.Value == "1+1" && hasResults && results.Value == "complete"
	}); err != nil {
		t.Fatalf("wait for calculator query completion: %v", err)
	}
	return latency
}
