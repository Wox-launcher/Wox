package ui

import (
	"context"
	"time"

	woxui "wox/ui/runtime"
	"wox/util/screen"
)

type RenderMode string

const (
	RenderModeGPU      RenderMode = "gpu"
	RenderModeSoftware RenderMode = "software"
)

// DisplayDiagnosticInfo keeps logical desktop coordinates distinct from physical pixels.
type DisplayDiagnosticInfo struct {
	LogicalBounds screen.Rect `json:"logicalBounds"`
	PixelBounds   screen.Rect `json:"pixelBounds"`
	Scale         float64     `json:"scale"`
	Primary       bool        `json:"primary"`
}

type FramePhaseDiagnostic struct {
	Phase woxui.FrameMetricPhase `json:"phase"`
	woxui.FramePhaseMetrics
}

// UIPerformanceSnapshot contains detached, content-free diagnostics from the primary launcher.
type UIPerformanceSnapshot struct {
	RenderMode          RenderMode              `json:"renderMode"`
	Displays            []DisplayDiagnosticInfo `json:"displays"`
	WakeMilliseconds    []float64               `json:"wakeMilliseconds"`
	PresentedFrameCount uint64                  `json:"presentedFrameCount"`
	FramePhases         [6]FramePhaseDiagnostic `json:"framePhases"`
}

// SnapshotPerformanceDiagnostics is available in release builds through the primary UI view.
func (m *Manager) SnapshotPerformanceDiagnostics(_ context.Context) UIPerformanceSnapshot {
	var wake []time.Duration
	var frames woxui.FrameMetricsSnapshot
	software := woxui.SoftwareRenderingEnabled()
	if view, ok := m.getView().(interface {
		LauncherPerformanceDiagnostics() ([]time.Duration, woxui.FrameMetricsSnapshot, bool)
	}); ok {
		wake, frames, software = view.LauncherPerformanceDiagnostics()
	}
	displays, _ := screen.ListDisplays()
	mode := RenderModeGPU
	if software {
		mode = RenderModeSoftware
	}
	return buildPerformanceSnapshot(mode, displays, wake, frames)
}

// buildPerformanceSnapshot projects the native data into stable diagnostics without display identifiers.
func buildPerformanceSnapshot(mode RenderMode, displays []screen.Display, wake []time.Duration, frames woxui.FrameMetricsSnapshot) UIPerformanceSnapshot {
	snapshot := UIPerformanceSnapshot{
		RenderMode: mode, PresentedFrameCount: frames.PresentedFrameCount,
		Displays:         make([]DisplayDiagnosticInfo, 0, len(displays)),
		WakeMilliseconds: make([]float64, 0, len(wake)),
		FramePhases: [6]FramePhaseDiagnostic{
			{woxui.FrameMetricSnapshot, frames.Snapshot},
			{woxui.FrameMetricBuildLayout, frames.BuildLayout},
			{woxui.FrameMetricDrawRecord, frames.DrawRecord},
			{woxui.FrameMetricAccessibility, frames.Accessibility},
			{woxui.FrameMetricNativeEncode, frames.NativeEncode},
			{woxui.FrameMetricNativePresent, frames.NativePresent},
		},
	}
	for _, display := range displays {
		snapshot.Displays = append(snapshot.Displays, DisplayDiagnosticInfo{display.Bounds, display.PixelBounds, display.Scale, display.Primary})
	}
	for _, duration := range wake {
		snapshot.WakeMilliseconds = append(snapshot.WakeMilliseconds, float64(duration)/float64(time.Millisecond))
	}
	return snapshot
}
