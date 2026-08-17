//go:build wox_ui_smoke

package perf

import (
	"context"
	"runtime"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
)

// Test005WarmCacheResources verifies repeated text/image frames reach a warm native cache.
// Flow: query wox-smoke warm-cache -> collect 8 presented frames of the same icons and titles.
// Evidence: later frames report cache hits and stop creating every drawn image.
func Test005WarmCacheResources(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		snapshot := runQueryFixture(t, ctx, client, fixtureCommandQuery("warm-cache"))
		if _, found := automationdriver.Find(snapshot, "launcher.result.perf-warm-0"); !found {
			t.Fatal("expected warm-cache fixture result")
		}
		samples := waitForPresentedSamples(t, ctx, client)
		assertSettledWork(t, samples)
		for _, sample := range samples {
			if sample.Work.ImageDraws <= 0 {
				t.Fatalf("frame %d recorded no image draws: %+v", sample.FrameID, sample.Work)
			}
			resources := sample.RendererResources
			if resources.TextRasterizations <= 0 && resources.CacheHits <= 0 && resources.ImageCreates <= 0 {
				t.Fatalf("frame %d missing renderer resource counters: %+v", sample.FrameID, resources)
			}
		}
		assertWarmCacheSteadyState(t, samples[len(samples)/2:])
		assertNoDroppedFrames(t, ctx, client)
	})
}

// assertWarmCacheSteadyState requires repeated frames to stop re-encoding native resources.
// The image and text caches are asserted separately because CacheHits aggregates both, so a
// working image cache alone keeps that counter positive while text caching regresses silently.
func assertWarmCacheSteadyState(t *testing.T, samples []woxui.FrameMetricsSample) {
	t.Helper()
	if len(samples) == 0 {
		t.Fatal("no steady frames to assert warm cache")
	}
	if runtime.GOOS == "windows" {
		// Windows still records the uncached encode baseline (creates==draws, hits==0).
		return
	}
	for _, sample := range samples {
		resources := sample.RendererResources
		if resources.CacheHits <= 0 {
			t.Fatalf("steady frame %d had no native cache hits: %+v", sample.FrameID, resources)
		}
		if sample.Work.ImageDraws > 0 && resources.ImageCreates >= sample.Work.ImageDraws {
			t.Fatalf("steady frame %d still created every image: draws %d creates %d hits %d", sample.FrameID, sample.Work.ImageDraws, resources.ImageCreates, resources.CacheHits)
		}
		if runtime.GOOS == "darwin" {
			// macOS builds a CTLine per draw until its text cache lands, so rasterizations
			// legitimately track draws one for one here.
			continue
		}
		// The fixture repeats eight short results, well inside the Linux text cache capacity,
		// so a healthy steady frame rasterizes almost nothing. The margin only tolerates runs
		// the cache refuses, such as strings past its per-entry character limit.
		if sample.Work.TextDraws > 0 && resources.TextRasterizations*4 > sample.Work.TextDraws {
			t.Fatalf("steady frame %d re-rasterized too much text: draws %d rasterizations %d hits %d", sample.FrameID, sample.Work.TextDraws, resources.TextRasterizations, resources.CacheHits)
		}
	}
}
