package plugin

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestPluginQueryMetricsIgnoreEmptyWindowsAndRankSpikes(t *testing.T) {
	ctx := context.Background()
	for _, manager := range []*Manager{nil, {}, {queryScheduler: &queryScheduler{}}} {
		if got := manager.SnapshotPluginQueryMetrics(ctx); got != nil {
			t.Fatalf("uninitialized manager returned metrics: %+v", got)
		}
	}
	manager := newManager()
	windows := manager.queryScheduler.pluginQueryMetrics
	windows.Store("empty", pluginQueryMetricWindow{})
	for id, peak := range map[string]time.Duration{"a": 5, "b": 10, "c": 10} {
		window := pluginQueryMetricWindow{total: pluginQueryMetricSamples}
		for i := range window.samples {
			window.samples[i] = time.Millisecond
		}
		window.samples[0] = peak * time.Millisecond
		windows.Store(id, window)
	}
	var ids []string
	for _, metrics := range manager.SnapshotPluginQueryMetrics(ctx) {
		ids = append(ids, metrics.PluginID)
	}
	if !slices.Equal(ids, []string{"b", "c", "a"}) {
		t.Fatalf("ranking must break equal P95 by Max, then ID: %v", ids)
	}
}

func TestPluginQueryMetricsWindow(t *testing.T) {
	manager := newManager()
	scheduler := manager.queryScheduler
	scheduler.recordPluginQuery("recent", time.Now().Add(-time.Second), true)
	for range pluginQueryMetricSamples {
		scheduler.recordPluginQuery("recent", time.Now().Add(-time.Millisecond), false)
	}
	scheduler.recordPluginQuery("slow", time.Now().Add(-200*time.Millisecond), false)

	got := manager.SnapshotPluginQueryMetrics(context.Background())
	if len(got) != 2 || got[0].PluginID != "slow" || got[1].PluginID != "recent" {
		t.Fatalf("unexpected ranking: %+v", got)
	}
	if row := got[1]; row.Samples != 32 || row.TotalQueries != 33 || row.SlowQueries != 1 ||
		row.PanicCount != 1 || row.LastRun.IsZero() || row.P50 < time.Millisecond || row.Max >= time.Second {
		t.Fatalf("bad rolling summary: %+v", row)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		scheduler.recordPluginQuery("recent", time.Now(), false)
	}); allocs != 0 {
		t.Fatalf("steady query metrics recorded %.2f allocations per job", allocs)
	}
}

func TestPluginQueryMetricsRecordAllJobs(t *testing.T) {
	manager := newManager()
	ctx := context.Background()
	instance := &Instance{Metadata: Metadata{Id: "test", Name: "Test"}}
	callbacks := queryExecutionCallbacks{
		queryPlugin: func(_ context.Context, _ *Instance, query Query) (QueryResponse, bool) {
			if query.Id == "panic" {
				panic("scheduler catches this")
			}
			return QueryResponse{}, query.Id == "recovered"
		},
		onPluginResponse: func(context.Context, Query, QueryResponse) {},
	}
	var jobs sync.WaitGroup
	for _, id := range []string{"empty", "recovered", "panic"} {
		jobs.Go(func() {
			execution := manager.queryScheduler.start(ctx, Query{Id: id, SessionId: id}, []queryPluginJob{{pluginInstance: instance}}, 0, callbacks)
			select {
			case <-execution.Done:
			case <-time.After(2 * time.Second):
				t.Errorf("scheduler did not finish %q", id)
			}
		})
	}
	jobs.Wait()
	got := manager.SnapshotPluginQueryMetrics(ctx)
	if len(got) != 1 || got[0].TotalQueries != 3 || got[0].PanicCount != 2 {
		t.Fatalf("unexpected job counts: %+v", got)
	}
}
