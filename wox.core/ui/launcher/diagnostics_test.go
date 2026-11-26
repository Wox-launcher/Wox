package launcher

import (
	"testing"
	"time"
)

func TestLauncherWakeMeasuresFirstNewPresentAndKeepsRecentSamples(t *testing.T) {
	var metrics launcherWakeMetrics
	start := time.Now()
	metrics.begin(start, 10)
	metrics.presented(10, start.Add(time.Millisecond))
	metrics.cancel(start)
	metrics.presented(11, start.Add(time.Millisecond))
	if len(metrics.snapshot()) != 0 {
		t.Fatal("stale or canceled frame recorded as a wake")
	}
	for i := 1; i <= 20; i++ {
		metrics.begin(start, 20)
		metrics.presented(21, start.Add(time.Duration(i)*time.Millisecond))
		metrics.presented(22, start.Add(time.Second))
	}
	got := metrics.snapshot()
	if len(got) != 16 || got[0] != 5*time.Millisecond || got[15] != 20*time.Millisecond {
		t.Fatalf("unexpected recent wake durations: %v", got)
	}
}
