package woxui

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFrameMetricsRecorderCorrelatesPortableAndNativePhases(t *testing.T) {
	recorder := newFrameMetricsRecorder()
	frameID := recorder.beginFrame()
	recorder.recordPhase(frameID, FrameMetricSnapshot, 120*time.Microsecond)
	recorder.recordPhase(frameID, FrameMetricBuildLayout, 800*time.Microsecond)
	recorder.recordPhase(frameID, FrameMetricDrawRecord, 240*time.Microsecond)
	recorder.recordPhase(frameID, FrameMetricAccessibility, 160*time.Microsecond)
	logicalDamage := Rect{X: 10, Y: 12, Width: 120, Height: 40}
	recorder.recordCounts(frameID, 31, 18, 9, logicalDamage)
	recorder.finishNativeFrame(frameID, 900*time.Microsecond, 100*time.Microsecond, true)

	snapshot := recorder.current()
	if snapshot.FrameCount != 1 || snapshot.NativeEncodedFrameCount != 1 || snapshot.PresentedFrameCount != 1 || snapshot.DroppedFrameCount != 0 {
		t.Fatalf("unexpected frame counters: %+v", snapshot)
	}
	if snapshot.BuildLayout.Samples != 1 || snapshot.BuildLayout.TotalMicroseconds != 800 || snapshot.NativeEncode.TotalMicroseconds != 900 {
		t.Fatalf("unexpected phase summaries: build=%+v native=%+v", snapshot.BuildLayout, snapshot.NativeEncode)
	}
	if len(snapshot.Recent) != 1 {
		t.Fatalf("recent sample count = %d, want 1", len(snapshot.Recent))
	}
	sample := snapshot.Recent[0]
	if sample.FrameID != frameID || sample.SnapshotMicroseconds != 120 || sample.NodeCount != 31 || sample.DisplayCommandCount != 18 || sample.AccessibilityNodeCount != 9 || sample.LogicalDamage != logicalDamage || !sample.HostCompleted || !sample.Presented || sample.Dropped {
		t.Fatalf("unexpected correlated sample: %+v", sample)
	}
}

func TestFrameMetricsRecorderStoresNestedWorkAndRendererResources(t *testing.T) {
	recorder := newFrameMetricsRecorder()
	frameID := recorder.beginFrame()
	work := FrameWorkMetrics{LayoutVisits: 12, IdentityVisits: 11, PaintVisits: 10, A11yVisits: 8, BoundaryBuilds: 2, BoundaryReuses: 3, TextDraws: 6, ImageDraws: 4}
	resources := FrameRendererResourceMetrics{TextRasterizations: 6, ImageCreates: 4, ImageUploads: 4, CacheHits: 1, ResidentBytes: 2048}
	recorder.recordWork(frameID, work)
	recorder.recordRendererResources(frameID, resources)
	recorder.recordCounts(frameID, 12, 9, 8, Rect{Width: 20, Height: 10})

	sample := recorder.current().Recent[0]
	if sample.NodeCount != 12 || sample.Work != work || sample.RendererResources != resources {
		t.Fatalf("nested metrics = %+v", sample)
	}
}

func TestFrameMetricsSnapshotKeepsLegacyJSONFields(t *testing.T) {
	payload, err := json.Marshal(FrameMetricsSnapshot{
		FrameCount: 3, PresentedFrameCount: 2, DroppedFrameCount: 1,
		Recent: []FrameMetricsSample{{
			FrameID: 9, NodeCount: 4, DisplayCommandCount: 7, HostCompleted: true,
			Work: FrameWorkMetrics{LayoutVisits: 4, TextDraws: 2},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"frameCount", "presentedFrameCount", "droppedFrameCount", "recent"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("legacy field %q missing from %s", key, payload)
		}
	}
	recent := decoded["recent"].([]any)[0].(map[string]any)
	if recent["nodeCount"] != float64(4) || recent["displayCommandCount"] != float64(7) {
		t.Fatalf("legacy sample fields = %v", recent)
	}
	if _, ok := recent["work"]; !ok {
		t.Fatalf("nested work field missing from %s", payload)
	}
}

func TestFrameMetricsRecorderTracksCoalescedAndResetFrames(t *testing.T) {
	recorder := newFrameMetricsRecorder()
	coalescedID := recorder.beginFrame()
	recorder.coalesceFrame(coalescedID)
	recorder.coalesceFrame(coalescedID)
	if snapshot := recorder.current(); snapshot.DroppedFrameCount != 1 || snapshot.CoalescedFrameCount != 1 || len(snapshot.Recent) != 1 || !snapshot.Recent[0].Dropped || !snapshot.Recent[0].Coalesced {
		t.Fatalf("unexpected coalesced metrics: %+v", snapshot)
	}

	backpressuredID := recorder.beginFrame()
	recorder.backpressureFrame(backpressuredID)
	if snapshot := recorder.current(); snapshot.DroppedFrameCount != 2 || snapshot.BackpressuredFrameCount != 1 || len(snapshot.Recent) != 2 || !snapshot.Recent[1].Dropped || !snapshot.Recent[1].Backpressured {
		t.Fatalf("unexpected backpressure metrics: %+v", snapshot)
	}
	finishedBackpressureID := recorder.beginFrame()
	recorder.finishBackpressuredFrame(finishedBackpressureID, 2*time.Millisecond, -1)
	if snapshot := recorder.current(); snapshot.DroppedFrameCount != 3 || snapshot.BackpressuredFrameCount != 2 || snapshot.NativeEncodedFrameCount != 1 || !snapshot.Recent[2].NativeEncodingCompleted || !snapshot.Recent[2].Backpressured {
		t.Fatalf("unexpected finished backpressure metrics: %+v", snapshot)
	}

	droppedID := recorder.beginFrame()
	recorder.dropFrame(droppedID)
	recorder.dropFrame(droppedID)
	if snapshot := recorder.current(); snapshot.DroppedFrameCount != 4 || snapshot.CoalescedFrameCount != 1 || snapshot.BackpressuredFrameCount != 2 || len(snapshot.Recent) != 4 || !snapshot.Recent[3].Dropped || snapshot.Recent[3].Coalesced || snapshot.Recent[3].Backpressured {
		t.Fatalf("unexpected dropped metrics: %+v", snapshot)
	}

	recorder.reset()
	recorder.recordPhase(droppedID, FrameMetricNativeEncode, time.Millisecond)
	if snapshot := recorder.current(); snapshot.FrameCount != 0 || snapshot.NativeEncode.Samples != 0 || len(snapshot.Recent) != 0 {
		t.Fatalf("stale in-flight frame crossed reset: %+v", snapshot)
	}

	currentID := recorder.beginFrame()
	recorder.finishNativeFrame(currentID, 2*time.Millisecond, -1, false)
	snapshot := recorder.current()
	if snapshot.FrameCount != 1 || snapshot.NativeEncodedFrameCount != 1 || snapshot.DroppedFrameCount != 1 || len(snapshot.Recent) != 1 || !snapshot.Recent[0].NativeEncodingCompleted {
		t.Fatalf("unexpected post-reset metrics: %+v", snapshot)
	}
}

func TestFrameMetricsRecorderBoundsRecentSamples(t *testing.T) {
	recorder := newFrameMetricsRecorder()
	for range recentFrameMetricsLimit + 5 {
		recorder.beginFrame()
	}
	snapshot := recorder.current()
	if snapshot.FrameCount != recentFrameMetricsLimit+5 || len(snapshot.Recent) != recentFrameMetricsLimit {
		t.Fatalf("bounded metrics = frames %d recent %d", snapshot.FrameCount, len(snapshot.Recent))
	}
	if snapshot.Recent[0].FrameID != 6 {
		t.Fatalf("oldest retained frame id = %d, want 6", snapshot.Recent[0].FrameID)
	}
}
