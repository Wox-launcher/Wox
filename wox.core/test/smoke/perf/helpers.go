//go:build wox_ui_smoke

package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

	// quietFrameRequestInterval re-arms a frame request that produced nothing. A single
	// invalidate can be coalesced into an in-flight frame or dropped before it presents,
	// and the sampler used to fire exactly once and then wait out its entire budget, so
	// one lost request failed the case indistinguishably from a stalled renderer.
	quietFrameRequestInterval = 500 * time.Millisecond

	// frameSampleBudget bounds one sampling wait. It is deliberately shorter than
	// CaseTimeout so exhausting it reports the renderer counters rather than letting
	// the case deadline surface somewhere less informative.
	frameSampleBudget = 12 * time.Second
)

func waitForPresentedSamples(t *testing.T, ctx context.Context, client *automationdriver.Client) []woxui.FrameMetricsSample {
	t.Helper()
	want := perfSampleCount
	waitForSnapshotQuiet(t, ctx, client, 150*time.Millisecond)
	if err := client.ResetFrameMetrics(ctx); err != nil {
		t.Fatalf("reset frame metrics: %v", err)
	}
	lastFrameID := uint64(0)
	observed := make([]woxui.FrameMetricsSample, 0, want)
	waitCtx, cancel := context.WithTimeout(ctx, frameSampleBudget)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var metrics woxui.FrameMetricsSnapshot
	requestFrame := true
	requestedAt := time.Now()
	startedAt := time.Now()
	frameRequests := 0
	for waitCtx.Err() == nil && len(observed) < want {
		if requestFrame {
			if requestErr := client.RequestFrame(waitCtx); requestErr != nil {
				if waitCtx.Err() != nil {
					break
				}
				t.Fatalf("request performance sample frame: %v", requestErr)
			}
			frameRequests++
			requestFrame = false
			requestedAt = time.Now()
		}
		observedBeforePoll := len(observed)
		// The poll result goes to a local first: a failed call reports a zero
		// snapshot, and overwriting the counters with it would leave the failure
		// message describing a renderer that never ran.
		polled, metricsErr := client.FrameMetrics(waitCtx)
		if metricsErr != nil {
			// Running out of the sampling budget is the outcome this wait exists to
			// report. Treating the expired call as a transport failure replaced that
			// report with a bare deadline error and hid every renderer counter.
			if waitCtx.Err() != nil {
				break
			}
			t.Fatalf("read frame metrics: %v", metricsErr)
		}
		metrics = polled
		for _, sample := range metrics.Recent {
			if sample.FrameID <= lastFrameID || !sample.HostCompleted || !sample.Presented {
				continue
			}
			lastFrameID = sample.FrameID
			observed = append(observed, sample)
		}
		if len(observed) >= want {
			writePerfArtifact(t, observed)
			return observed[:want]
		}
		if len(observed) > observedBeforePoll {
			// Every sample needs its own frame, so the next request goes out as soon as
			// this one lands instead of waiting for the quiet-request interval.
			requestFrame = true
			continue
		}
		requestFrame = time.Since(requestedAt) >= quietFrameRequestInterval
		select {
		case <-waitCtx.Done():
		case <-ticker.C:
		}
	}
	metrics = finalFrameMetrics(ctx, client, metrics)
	t.Fatalf("collected %d presented frames, want %d after %d frame requests over %s: %s; samples %+v",
		len(observed), want, frameRequests, time.Since(startedAt).Truncate(time.Millisecond), describeFrameMetrics(metrics), observed)
	return nil
}

// finalFrameMetrics re-reads the counters once the sampling budget is gone. The
// budget context is already expired by then, so reusing it would only produce
// another deadline error in place of the numbers that explain the failure.
func finalFrameMetrics(ctx context.Context, client *automationdriver.Client, last woxui.FrameMetricsSnapshot) woxui.FrameMetricsSnapshot {
	readCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	metrics, err := client.FrameMetrics(readCtx)
	if err != nil {
		return last
	}
	return metrics
}

// describeFrameMetrics reports what the renderer did with the requested frames. A
// presented count of zero has causes the sample list alone cannot separate: frames that
// were never produced, produced and dropped, coalesced into a newer frame, or skipped
// because every bounded native render surface was busy.
func describeFrameMetrics(metrics woxui.FrameMetricsSnapshot) string {
	return fmt.Sprintf("frames=%d nativeEncoded=%d presented=%d dropped=%d coalesced=%d backpressured=%d recent=%d",
		metrics.FrameCount, metrics.NativeEncodedFrameCount, metrics.PresentedFrameCount,
		metrics.DroppedFrameCount, metrics.CoalescedFrameCount, metrics.BackpressuredFrameCount, len(metrics.Recent))
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
	// Bound the pre-sampling wait too, leaving time to report renderer counters.
	waitCtx, cancel := context.WithTimeout(ctx, automationdriver.ActionTimeout)
	defer cancel()
	snapshot, err := client.Snapshot(waitCtx)
	if err != nil {
		t.Fatalf("read snapshot before quiet wait: %v", err)
	}
	// Generation advances on every draw, even when the content is unchanged.
	// Repaints must not keep a completed stream in the quiet wait forever.
	snapshot.Tree.Generation = 0
	quietSince := time.Now()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for waitCtx.Err() == nil {
		select {
		case <-waitCtx.Done():
		case <-ticker.C:
		}
		var current woxwidget.AutomationSnapshot
		current, err = client.Snapshot(waitCtx)
		if err != nil {
			break
		}
		current.Tree.Generation = 0
		if !reflect.DeepEqual(current.Tree, snapshot.Tree) {
			snapshot = current
			quietSince = time.Now()
			continue
		}
		if time.Since(quietSince) >= quiet {
			return
		}
	}
	metrics := finalFrameMetrics(ctx, client, woxui.FrameMetricsSnapshot{})
	t.Fatalf("wait for snapshot content quiet for %s: %v (context: %v); %s; %s",
		quiet, err, waitCtx.Err(), automationdriver.DescribeSnapshot(snapshot), describeFrameMetrics(metrics))
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
