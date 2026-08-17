package woxui

import (
	"sync"
	"time"
)

const recentFrameMetricsLimit = 240

// FrameMetricPhase identifies one measured part of the portable or native frame pipeline.
type FrameMetricPhase string

const (
	FrameMetricSnapshot FrameMetricPhase = "snapshot"
	// BuildLayout includes snapshot preparation so the nested snapshot value can be subtracted during analysis.
	FrameMetricBuildLayout   FrameMetricPhase = "build_layout"
	FrameMetricDrawRecord    FrameMetricPhase = "draw_record"
	FrameMetricAccessibility FrameMetricPhase = "accessibility"
	FrameMetricNativeEncode  FrameMetricPhase = "native_encode"
	FrameMetricNativePresent FrameMetricPhase = "native_present"
)

// FramePhaseMetrics summarizes one phase since the most recent metrics reset.
type FramePhaseMetrics struct {
	Samples           uint64 `json:"samples"`
	TotalMicroseconds int64  `json:"totalMicroseconds"`
	LastMicroseconds  int64  `json:"lastMicroseconds"`
	MaxMicroseconds   int64  `json:"maxMicroseconds"`
}

// FrameWorkMetrics counts portable Host work actually performed for one frame.
type FrameWorkMetrics struct {
	LayoutVisits       int `json:"layoutVisits"`
	IdentityVisits     int `json:"identityVisits"`
	PaintVisits        int `json:"paintVisits"`
	A11yVisits         int `json:"a11yVisits"`
	BoundaryBuilds     int `json:"boundaryBuilds"`
	BoundaryReuses     int `json:"boundaryReuses"`
	PaintSegmentReuses int `json:"paintSegmentReuses"`
	IdentityUpserts    int `json:"identityUpserts"`
	A11yUpserts        int `json:"a11yUpserts"`
	TextDraws          int `json:"textDraws"`
	ImageDraws         int `json:"imageDraws"`
}

// FrameRendererResourceMetrics counts native encode-time resource work for one frame.
type FrameRendererResourceMetrics struct {
	TextRasterizations int   `json:"textRasterizations"`
	ImageCreates       int   `json:"imageCreates"`
	ImageUploads       int   `json:"imageUploads"`
	CacheHits          int   `json:"cacheHits"`
	CacheEvictions     int   `json:"cacheEvictions"`
	ResidentBytes      int64 `json:"residentBytes"`
}

// FrameMetricsSample keeps correlated timings and tree sizes for one recent frame.
type FrameMetricsSample struct {
	FrameID                   uint64                       `json:"frameId"`
	SnapshotMicroseconds      int64                        `json:"snapshotMicroseconds"`
	BuildLayoutMicroseconds   int64                        `json:"buildLayoutMicroseconds"`
	DrawRecordMicroseconds    int64                        `json:"drawRecordMicroseconds"`
	AccessibilityMicroseconds int64                        `json:"accessibilityMicroseconds"`
	NativeEncodeMicroseconds  int64                        `json:"nativeEncodeMicroseconds"`
	NativePresentMicroseconds int64                        `json:"nativePresentMicroseconds"`
	NodeCount                 int                          `json:"nodeCount"`
	DisplayCommandCount       int                          `json:"displayCommandCount"`
	AccessibilityNodeCount    int                          `json:"accessibilityNodeCount"`
	LogicalDamage             Rect                         `json:"logicalDamage"`
	HostCompleted             bool                         `json:"hostCompleted"`
	NativeEncodingCompleted   bool                         `json:"nativeEncodingCompleted"`
	Presented                 bool                         `json:"presented"`
	Dropped                   bool                         `json:"dropped"`
	Coalesced                 bool                         `json:"coalesced"`
	Backpressured             bool                         `json:"backpressured"`
	Work                      FrameWorkMetrics             `json:"work"`
	RendererResources         FrameRendererResourceMetrics `json:"rendererResources"`
}

// FrameMetricsSnapshot is the detached metrics view exposed to automation and diagnostics.
type FrameMetricsSnapshot struct {
	FrameCount              uint64               `json:"frameCount"`
	NativeEncodedFrameCount uint64               `json:"nativeEncodedFrameCount"`
	PresentedFrameCount     uint64               `json:"presentedFrameCount"`
	DroppedFrameCount       uint64               `json:"droppedFrameCount"`
	CoalescedFrameCount     uint64               `json:"coalescedFrameCount"`
	BackpressuredFrameCount uint64               `json:"backpressuredFrameCount"`
	Snapshot                FramePhaseMetrics    `json:"snapshot"`
	BuildLayout             FramePhaseMetrics    `json:"buildLayout"`
	DrawRecord              FramePhaseMetrics    `json:"drawRecord"`
	Accessibility           FramePhaseMetrics    `json:"accessibility"`
	NativeEncode            FramePhaseMetrics    `json:"nativeEncode"`
	NativePresent           FramePhaseMetrics    `json:"nativePresent"`
	Recent                  []FrameMetricsSample `json:"recent"`
}

type frameMetricsRecorder struct {
	mu             sync.Mutex
	nextFrameID    uint64
	minimumFrameID uint64
	order          []uint64
	samples        map[uint64]*FrameMetricsSample
	snapshot       FrameMetricsSnapshot
}

func newFrameMetricsRecorder() *frameMetricsRecorder {
	return &frameMetricsRecorder{minimumFrameID: 1, samples: map[uint64]*FrameMetricsSample{}}
}

func (r *frameMetricsRecorder) beginFrame() uint64 {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextFrameID++
	frameID := r.nextFrameID
	r.samples[frameID] = &FrameMetricsSample{FrameID: frameID}
	r.order = append(r.order, frameID)
	r.snapshot.FrameCount++
	if len(r.order) > recentFrameMetricsLimit {
		oldest := r.order[0]
		r.order = r.order[1:]
		delete(r.samples, oldest)
	}
	return frameID
}

func (r *frameMetricsRecorder) recordPhase(frameID uint64, phase FrameMetricPhase, duration time.Duration) {
	if r == nil || frameID == 0 {
		return
	}
	microseconds := max(int64(0), duration.Microseconds())
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	if sample := r.samples[frameID]; sample != nil {
		switch phase {
		case FrameMetricSnapshot:
			sample.SnapshotMicroseconds = microseconds
		case FrameMetricBuildLayout:
			sample.BuildLayoutMicroseconds = microseconds
		case FrameMetricDrawRecord:
			sample.DrawRecordMicroseconds = microseconds
		case FrameMetricAccessibility:
			sample.AccessibilityMicroseconds = microseconds
		case FrameMetricNativeEncode:
			sample.NativeEncodeMicroseconds = microseconds
		case FrameMetricNativePresent:
			sample.NativePresentMicroseconds = microseconds
		}
	}
	metrics := r.phaseMetrics(phase)
	if metrics == nil {
		return
	}
	metrics.Samples++
	metrics.TotalMicroseconds += microseconds
	metrics.LastMicroseconds = microseconds
	metrics.MaxMicroseconds = max(metrics.MaxMicroseconds, microseconds)
}

func (r *frameMetricsRecorder) phaseMetrics(phase FrameMetricPhase) *FramePhaseMetrics {
	switch phase {
	case FrameMetricSnapshot:
		return &r.snapshot.Snapshot
	case FrameMetricBuildLayout:
		return &r.snapshot.BuildLayout
	case FrameMetricDrawRecord:
		return &r.snapshot.DrawRecord
	case FrameMetricAccessibility:
		return &r.snapshot.Accessibility
	case FrameMetricNativeEncode:
		return &r.snapshot.NativeEncode
	case FrameMetricNativePresent:
		return &r.snapshot.NativePresent
	default:
		return nil
	}
}

func (r *frameMetricsRecorder) recordCounts(frameID uint64, nodes, commands, accessibilityNodes int, logicalDamage Rect) {
	if r == nil || frameID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	if sample := r.samples[frameID]; sample != nil {
		sample.NodeCount = nodes
		sample.DisplayCommandCount = commands
		sample.AccessibilityNodeCount = accessibilityNodes
		sample.LogicalDamage = logicalDamage
		sample.HostCompleted = true
	}
}

func (r *frameMetricsRecorder) recordWork(frameID uint64, work FrameWorkMetrics) {
	if r == nil || frameID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	if sample := r.samples[frameID]; sample != nil {
		sample.Work = work
	}
}

func (r *frameMetricsRecorder) recordRendererResources(frameID uint64, resources FrameRendererResourceMetrics) {
	if r == nil || frameID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	if sample := r.samples[frameID]; sample != nil {
		sample.RendererResources = resources
	}
}

func (r *frameMetricsRecorder) recordEncodedResources(displayList *DisplayList) {
	if r == nil || displayList == nil {
		return
	}
	r.recordRendererResources(displayList.frameID, displayList.EncodedRendererResources())
}

func (r *frameMetricsRecorder) finishNativeFrame(frameID uint64, encode, present time.Duration, presented bool) {
	if r == nil || frameID == 0 {
		return
	}
	r.recordPhase(frameID, FrameMetricNativeEncode, encode)
	if present >= 0 {
		r.recordPhase(frameID, FrameMetricNativePresent, present)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	r.snapshot.NativeEncodedFrameCount++
	if sample := r.samples[frameID]; sample != nil {
		sample.NativeEncodingCompleted = true
		sample.Presented = presented
		sample.Dropped = !presented
	}
	if presented {
		r.snapshot.PresentedFrameCount++
	} else {
		r.snapshot.DroppedFrameCount++
	}
}

// finishBackpressuredFrame records partial native work that could not acquire a bounded surface.
func (r *frameMetricsRecorder) finishBackpressuredFrame(frameID uint64, encode, present time.Duration) {
	if r == nil || frameID == 0 {
		return
	}
	r.recordPhase(frameID, FrameMetricNativeEncode, encode)
	if present >= 0 {
		r.recordPhase(frameID, FrameMetricNativePresent, present)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	r.snapshot.NativeEncodedFrameCount++
	if sample := r.samples[frameID]; sample != nil {
		sample.NativeEncodingCompleted = true
		sample.Dropped = true
		sample.Backpressured = true
	}
	r.snapshot.DroppedFrameCount++
	r.snapshot.BackpressuredFrameCount++
}

func (r *frameMetricsRecorder) dropFrame(frameID uint64) {
	r.markDroppedFrame(frameID, false, false)
}

// coalesceFrame records an obsolete queued frame separately while preserving the legacy dropped total.
func (r *frameMetricsRecorder) coalesceFrame(frameID uint64) {
	r.markDroppedFrame(frameID, true, false)
}

// backpressureFrame records a frame skipped because every bounded native render surface is busy.
func (r *frameMetricsRecorder) backpressureFrame(frameID uint64) {
	r.markDroppedFrame(frameID, false, true)
}

func (r *frameMetricsRecorder) markDroppedFrame(frameID uint64, coalesced, backpressured bool) {
	if r == nil || frameID == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if frameID < r.minimumFrameID {
		return
	}
	if sample := r.samples[frameID]; sample != nil {
		if sample.Dropped || sample.Presented {
			return
		}
		sample.Dropped = true
		sample.Coalesced = coalesced
		sample.Backpressured = backpressured
	}
	r.snapshot.DroppedFrameCount++
	if coalesced {
		r.snapshot.CoalescedFrameCount++
	}
	if backpressured {
		r.snapshot.BackpressuredFrameCount++
	}
}

func (r *frameMetricsRecorder) current() FrameMetricsSnapshot {
	if r == nil {
		return FrameMetricsSnapshot{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	result := r.snapshot
	result.Recent = make([]FrameMetricsSample, 0, len(r.order))
	for _, frameID := range r.order {
		if sample := r.samples[frameID]; sample != nil {
			result.Recent = append(result.Recent, *sample)
		}
	}
	return result
}

func (r *frameMetricsRecorder) reset() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.minimumFrameID = r.nextFrameID + 1
	r.order = nil
	r.samples = map[uint64]*FrameMetricsSample{}
	r.snapshot = FrameMetricsSnapshot{}
}
