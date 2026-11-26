package plugin

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"
)

const pluginQueryMetricSamples = 32

type PluginHealthState string

const (
	PluginHealthHealthy     PluginHealthState = "healthy"
	PluginHealthDegraded    PluginHealthState = "degraded"
	PluginHealthCircuitOpen PluginHealthState = "circuit_open"
)

// pluginQueryMetricWindow keeps bounded latency history and process-lifetime counts.
type pluginQueryMetricWindow struct {
	samples [pluginQueryMetricSamples]time.Duration
	total   uint64
	slow    uint64
	panics  uint64
	lastRun time.Time
}

// PluginQueryMetrics summarizes completed jobs, with percentiles over the latest 32.
type PluginQueryMetrics struct {
	TimeoutCount     uint64
	HealthState      PluginHealthState
	CircuitOpenUntil time.Time
	PluginID         string
	Samples          int
	TotalQueries     uint64
	SlowQueries      uint64
	PanicCount       uint64
	LastRun          time.Time
	P50              time.Duration
	P95              time.Duration
	Max              time.Duration
}

// recordPluginQuery keeps steady-state updates allocation-free and serializes concurrent jobs.
func (s *queryScheduler) recordPluginQuery(pluginID string, started time.Time, recovered bool) {
	finished := time.Now()
	duration := finished.Sub(started)
	s.pluginQueryMetrics.Compute(pluginID, func(window pluginQueryMetricWindow, _ bool) (pluginQueryMetricWindow, bool) {
		window.samples[window.total%pluginQueryMetricSamples] = duration
		window.total++
		if duration >= 200*time.Millisecond {
			window.slow++
		}
		if recovered {
			window.panics++
		}
		if finished.After(window.lastRun) {
			window.lastRun = finished
		}
		return window, true
	})
}

// SnapshotPluginQueryMetrics detaches the windows before sorting them for diagnostics.
func (m *Manager) SnapshotPluginQueryMetrics(_ context.Context) []PluginQueryMetrics {
	if m == nil || m.queryScheduler == nil || m.queryScheduler.pluginQueryMetrics == nil {
		return nil
	}
	windows := m.queryScheduler.pluginQueryMetrics.ToMap()
	output := make([]PluginQueryMetrics, 0, len(windows))
	for pluginID, window := range windows {
		circuit, _ := m.queryScheduler.pluginCircuits.Load(pluginID)
		health := PluginHealthHealthy
		now := time.Now()
		if now.Before(circuit.openUntil) {
			health = PluginHealthCircuitOpen
		} else if now.Sub(circuit.firstTimeout) < pluginCircuitWindow {
			health = PluginHealthDegraded
		}
		samples := window.samples[:min(window.total, pluginQueryMetricSamples)]
		n := len(samples)
		if n == 0 {
			continue
		}
		slices.Sort(samples)
		output = append(output, PluginQueryMetrics{
			TimeoutCount: circuit.timeouts, HealthState: health, CircuitOpenUntil: circuit.openUntil,
			PluginID: pluginID, Samples: n, TotalQueries: window.total, SlowQueries: window.slow,
			PanicCount: window.panics, LastRun: window.lastRun,
			P50: samples[(50*n+99)/100-1], P95: samples[(95*n+99)/100-1], Max: samples[n-1],
		})
	}
	slices.SortFunc(output, func(a, b PluginQueryMetrics) int {
		if order := cmp.Compare(b.P95, a.P95); order != 0 {
			return order
		}
		if order := cmp.Compare(b.Max, a.Max); order != 0 {
			return order
		}
		return strings.Compare(a.PluginID, b.PluginID)
	})
	return output
}
