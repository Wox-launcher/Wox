package woxui

import (
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

func TestFrameMetricsRecorderTracksCoalescedAndResetFrames(t *testing.T) {
	recorder := newFrameMetricsRecorder()
	droppedID := recorder.beginFrame()
	recorder.dropFrame(droppedID)
	recorder.dropFrame(droppedID)
	if snapshot := recorder.current(); snapshot.DroppedFrameCount != 1 || len(snapshot.Recent) != 1 || !snapshot.Recent[0].Dropped {
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
