package launcher

import (
	"sync"
	"time"

	woxui "wox/ui/runtime"
)

// launcherWakeMetrics correlates consecutive primary launcher shows with native presents.
// The renderer can present on a different thread from the Show and Hide callers.
type launcherWakeMetrics struct {
	mu          sync.Mutex
	started     time.Time
	afterFrame  uint64
	samples     [16]time.Duration
	next, count int
}

func (m *launcherWakeMetrics) begin(started time.Time, afterFrame uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started, m.afterFrame = started, afterFrame
}

// cancel discards an unfinished show when it fails or the launcher hides again.
func (m *launcherWakeMetrics) cancel(started time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if started.IsZero() || m.started == started {
		m.started = time.Time{}
	}
}

// presented records the first successful frame begun after the native show was requested.
func (m *launcherWakeMetrics) presented(frameID uint64, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.started.IsZero() || frameID <= m.afterFrame {
		return
	}
	m.samples[m.next] = at.Sub(m.started)
	m.next = (m.next + 1) % len(m.samples)
	m.count = min(m.count+1, len(m.samples))
	m.started = time.Time{}
}

// snapshot returns wake samples oldest first in caller-owned storage.
func (m *launcherWakeMetrics) snapshot() []time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]time.Duration, m.count)
	for i := range result {
		result[i] = m.samples[(m.next+len(m.samples)-m.count+i)%len(m.samples)]
	}
	return result
}

// LauncherPerformanceDiagnostics returns measurements for this app's native launcher window.
func (a *App) LauncherPerformanceDiagnostics() ([]time.Duration, woxui.FrameMetricsSnapshot, bool) {
	var window *woxui.Window
	software := woxui.SoftwareRenderingEnabled()
	_ = a.runOnUI("read launcher performance", func() {
		window = a.window
		software = window.SoftwareRendering()
	})
	return a.wakeMetrics.snapshot(), window.FrameMetrics(), software
}
