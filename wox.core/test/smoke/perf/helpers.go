//go:build wox_ui_smoke

package perf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	// These bounds assume LazyList/LazyGrid windows plus launcher/settings chrome.
	// Materializing a 500-row tree still fits the old 250k/50k dashboard limits.
	perfMaxLayoutVisits        = 8000
	perfMaxDisplayCommands     = 4000
	perfArtifactEnvironment    = "WOX_PERF_ARTIFACT"
	perfArtifactDirEnvironment = "WOX_PERF_ARTIFACT_DIR"

	// Every scenario collects the same window and drops its warmup head before checking the
	// steady maximum. The ceilings are intentionally loose, so keeping the maximum makes the
	// gate sensitive to a large isolated stall without extending every smoke case to 20+ frames.
	perfSampleCount   = 8
	perfWarmupSamples = 2
)

func waitForPresentedSamples(t *testing.T, ctx context.Context, client *automationdriver.Client) []woxui.FrameMetricsSample {
	t.Helper()
	want := perfSampleCount
	waitForSnapshotQuiet(t, ctx, client, 150*time.Millisecond)
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatalf("reset frame metrics: %v", err)
	}
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read snapshot before collecting frames: %v", err)
	}
	generation := snapshot.Tree.Generation
	lastFrameID := uint64(0)
	observed := make([]woxui.FrameMetricsSample, 0, want)
	waitCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	for waitCtx.Err() == nil && len(observed) < want {
		if requestErr := client.RequestFrame(waitCtx); requestErr != nil {
			t.Fatalf("request performance sample frame: %v", requestErr)
		}
		observedBeforeRequest := len(observed)
		for waitCtx.Err() == nil && len(observed) == observedBeforeRequest {
			next, waitErr := client.WaitForChange(waitCtx, generation)
			if waitErr != nil {
				break
			}
			generation = next.Tree.Generation
			metrics, metricsErr := client.FrameMetrics(ctx)
			if metricsErr != nil {
				t.Fatalf("read frame metrics: %v", metricsErr)
			}
			for _, sample := range metrics.Recent {
				if sample.FrameID <= lastFrameID || !sample.HostCompleted || !sample.Presented {
					continue
				}
				lastFrameID = sample.FrameID
				observed = append(observed, sample)
				if len(observed) >= want {
					writePerfArtifact(t, observed)
					return observed
				}
			}
		}
	}
	t.Fatalf("collected %d presented frames, want %d: %+v", len(observed), want, observed)
	return nil
}

func assertSettledWork(t *testing.T, samples []woxui.FrameMetricsSample) {
	t.Helper()
	if len(samples) == 0 {
		t.Fatal("no presented frames to assert")
	}
	for _, sample := range samples {
		if sample.Dropped {
			t.Fatalf("settled frame %d was dropped: %+v", sample.FrameID, sample)
		}
		if sample.Work.LayoutVisits <= 0 || sample.Work.PaintVisits <= 0 || sample.Work.IdentityVisits <= 0 {
			t.Fatalf("frame %d missing work counters: %+v", sample.FrameID, sample.Work)
		}
		if sample.Work.LayoutVisits > perfMaxLayoutVisits {
			t.Fatalf("frame %d layout visits %d exceeded %d", sample.FrameID, sample.Work.LayoutVisits, perfMaxLayoutVisits)
		}
		if sample.DisplayCommandCount <= 0 || sample.DisplayCommandCount > perfMaxDisplayCommands {
			t.Fatalf("frame %d command count %d outside 1..%d", sample.FrameID, sample.DisplayCommandCount, perfMaxDisplayCommands)
		}
		if sample.Work.TextDraws <= 0 {
			t.Fatalf("frame %d recorded no text draws: %+v", sample.FrameID, sample.Work)
		}
	}
	steady := steadySamples(t, samples)
	layoutMax := maxInt(workValues(steady, func(sample woxui.FrameMetricsSample) int { return sample.Work.LayoutVisits }))
	commandMax := maxInt(workValues(steady, func(sample woxui.FrameMetricsSample) int { return sample.DisplayCommandCount }))
	if layoutMax > perfMaxLayoutVisits {
		t.Fatalf("steady max layout visits %d exceeded %d", layoutMax, perfMaxLayoutVisits)
	}
	if commandMax > perfMaxDisplayCommands {
		t.Fatalf("steady max command count %d exceeded %d", commandMax, perfMaxDisplayCommands)
	}
	assertPhaseBudgets(t, steady)
}

// steadySamples drops the warmup head so percentiles describe repeated frames only.
func steadySamples(t *testing.T, samples []woxui.FrameMetricsSample) []woxui.FrameMetricsSample {
	t.Helper()
	if len(samples) <= perfWarmupSamples {
		t.Fatalf("collected %d frames, want more than the %d warmup frames", len(samples), perfWarmupSamples)
	}
	return samples[perfWarmupSamples:]
}

// waitForSnapshotQuiet waits until streaming updates stop changing the retained semantics tree.
func waitForSnapshotQuiet(t *testing.T, ctx context.Context, client *automationdriver.Client, quiet time.Duration) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read snapshot before quiet wait: %v", err)
	}
	generation := snapshot.Tree.Generation
	quietSince := time.Now()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for snapshot quiet period: %v", ctx.Err())
		case <-ticker.C:
		}
		current, snapshotErr := client.Snapshot(ctx)
		if snapshotErr != nil {
			t.Fatalf("read snapshot during quiet wait: %v", snapshotErr)
		}
		if current.Tree.Generation != generation {
			generation = current.Tree.Generation
			quietSince = time.Now()
			continue
		}
		if time.Since(quietSince) >= quiet {
			return
		}
	}
}

// phaseBudget caps one frame phase at its steady-state maximum, in microseconds.
type phaseBudget struct {
	name  string
	value func(woxui.FrameMetricsSample) int64
	limit int64
}

// perfPhaseBudgets lists the per-phase ceilings for this platform.
//
// Build/Layout is measured net of Snapshot because the recorded BuildLayout window encloses the
// snapshot call, so charging both separately would count the same microseconds twice. Native
// encode is the only phase whose cost is dominated by the platform renderer, so it carries a
// per-GOOS limit instead of sharing the Go-side one.
func perfPhaseBudgets() []phaseBudget {
	return []phaseBudget{
		{"build/layout excluding snapshot", func(sample woxui.FrameMetricsSample) int64 {
			return sample.BuildLayoutMicroseconds - sample.SnapshotMicroseconds
		}, perfBuildLayoutBudgetMicroseconds},
		{"snapshot", func(sample woxui.FrameMetricsSample) int64 { return sample.SnapshotMicroseconds }, perfSnapshotBudgetMicroseconds},
		{"draw record", func(sample woxui.FrameMetricsSample) int64 { return sample.DrawRecordMicroseconds }, perfDrawRecordBudgetMicroseconds},
		{"accessibility", func(sample woxui.FrameMetricsSample) int64 { return sample.AccessibilityMicroseconds }, perfAccessibilityBudgetMicroseconds},
		{"native encode", func(sample woxui.FrameMetricsSample) int64 { return sample.NativeEncodeMicroseconds }, perfNativeEncodeBudgetMicroseconds()},
	}
}

// assertPhaseBudgets gates the wall-clock cost of each frame phase and their sum.
// The ceilings are order-of-magnitude guards, not regression detectors: they are set far above
// observed steady-state P95 so a slower Pango, CoreGraphics, or snapshot path is caught while
// ordinary CI scheduling noise is not.
func assertPhaseBudgets(t *testing.T, steady []woxui.FrameMetricsSample) {
	t.Helper()
	var totalMax int64
	for _, budget := range perfPhaseBudgets() {
		observed := maxInt64(phaseValues(steady, budget.value))
		totalMax += observed
		if observed > budget.limit {
			t.Fatalf("steady max %s %dus exceeded %dus", budget.name, observed, budget.limit)
		}
	}
	if totalMax > perfFrameBudgetMicroseconds() {
		t.Fatalf("steady max phase total %dus exceeded %dus", totalMax, perfFrameBudgetMicroseconds())
	}
}

func phaseValues(samples []woxui.FrameMetricsSample, value func(woxui.FrameMetricsSample) int64) []int64 {
	values := make([]int64, len(samples))
	for index, sample := range samples {
		values[index] = max(int64(0), value(sample))
	}
	return values
}

func maxInt64(values []int64) int64 {
	var result int64
	for _, value := range values {
		result = max(result, value)
	}
	return result
}

func workValues(samples []woxui.FrameMetricsSample, value func(woxui.FrameMetricsSample) int) []int {
	values := make([]int, len(samples))
	for index, sample := range samples {
		values[index] = value(sample)
	}
	return values
}

func maxInt(values []int) int {
	result := 0
	for _, value := range values {
		result = max(result, value)
	}
	return result
}

func assertNoDroppedFrames(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	assertDroppedFramesAtMost(t, ctx, client, 0)
}

// assertDroppedFramesAtMost allows a small drop budget for fixtures that invalidate faster than vsync.
func assertDroppedFramesAtMost(t *testing.T, ctx context.Context, client *automationdriver.Client, maxDropped uint64) {
	t.Helper()
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatalf("read dropped-frame counters: %v", err)
	}
	if metrics.DroppedFrameCount > maxDropped {
		t.Fatalf("dropped frames = %d, want <= %d", metrics.DroppedFrameCount, maxDropped)
	}
}

// assertUnexpectedDroppedFramesAtMost ignores intentional coalescing and bounded renderer backpressure.
func assertUnexpectedDroppedFramesAtMost(t *testing.T, ctx context.Context, client *automationdriver.Client, maxDropped uint64) {
	t.Helper()
	metrics, err := client.FrameMetrics(ctx)
	if err != nil {
		t.Fatalf("read dropped-frame counters: %v", err)
	}
	expected := metrics.CoalescedFrameCount + metrics.BackpressuredFrameCount
	unexpected := metrics.DroppedFrameCount - min(metrics.DroppedFrameCount, expected)
	if unexpected > maxDropped {
		t.Fatalf("unexpected dropped frames = %d, want <= %d (total=%d coalesced=%d backpressured=%d)", unexpected, maxDropped, metrics.DroppedFrameCount, metrics.CoalescedFrameCount, metrics.BackpressuredFrameCount)
	}
}

func writePerfArtifact(t *testing.T, samples []woxui.FrameMetricsSample) {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(perfArtifactEnvironment))
	if dir := strings.TrimSpace(os.Getenv(perfArtifactDirEnvironment)); dir != "" {
		path = filepath.Join(dir, strings.ReplaceAll(t.Name(), "/", "_")+".json")
	}
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !os.IsExist(err) {
		t.Fatalf("create perf artifact directory: %v", err)
	}
	payload, err := json.MarshalIndent(map[string]any{
		"case":    t.Name(),
		"samples": samples,
	}, "", "  ")
	if err != nil {
		t.Fatalf("encode perf artifact: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write perf artifact: %v", err)
	}
}

func runQueryFixture(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	return smoke.ReplaceLauncherQuery(t, ctx, client, query)
}

// fixtureCommandQuery builds a smoke plugin command. A two-token query without a
// trailing space is parsed as search, so "wox-smoke list-500" never reaches Query.Command.
func fixtureCommandQuery(command string) string {
	return "wox-smoke " + command + " "
}
