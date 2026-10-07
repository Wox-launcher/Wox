package woxui

import (
	window "wox/ui/runtime/internal/window"
)

// FrameMetricPhase identifies one measured part of the portable or native frame pipeline.
type FrameMetricPhase = window.FrameMetricPhase

const (
	FrameMetricSnapshot      = window.FrameMetricSnapshot
	FrameMetricBuildLayout   = window.FrameMetricBuildLayout
	FrameMetricDrawRecord    = window.FrameMetricDrawRecord
	FrameMetricAccessibility = window.FrameMetricAccessibility
	FrameMetricNativeEncode  = window.FrameMetricNativeEncode
	FrameMetricNativePresent = window.FrameMetricNativePresent
)

// FramePhaseMetrics summarizes one phase since the most recent metrics reset.
type FramePhaseMetrics = window.FramePhaseMetrics

// FrameWorkMetrics counts portable Host work actually performed for one frame.
type FrameWorkMetrics = window.FrameWorkMetrics

// FrameRendererResourceMetrics counts native encode-time resource work for one frame.
type FrameRendererResourceMetrics = window.FrameRendererResourceMetrics

// FrameMetricsSample keeps correlated timings and tree sizes for one recent frame.
type FrameMetricsSample = window.FrameMetricsSample

// FrameMetricsSnapshot is the detached metrics view exposed to automation and diagnostics.
type FrameMetricsSnapshot = window.FrameMetricsSnapshot
