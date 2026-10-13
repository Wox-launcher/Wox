package ui

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"wox/plugin"
	"wox/ui/contract"
	"wox/util"
)

// TestQueryRunFallbackDeadline exercises the receive loop with virtual time and pending plugin jobs.
func TestQueryRunFallbackDeadline(t *testing.T) {
	logger = util.GetLogger()
	plugin.GetPluginManager()
	for _, tc := range []struct {
		name         string
		elapsed      time.Duration
		queuedResult bool
		pluginScoped bool
		earlyReady   bool
	}{
		{name: "pending plugins"},
		{name: "includes dispatch time", elapsed: 30 * time.Millisecond},
		{name: "overdue", elapsed: 80 * time.Millisecond},
		{name: "queued result wins", queuedResult: true},
		{name: "plugin scope", pluginScoped: true},
		{name: "early completion", earlyReady: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()
				results := make(chan plugin.QueryResponseUI, 2)
				ready := make(chan bool, 1)
				done := make(chan bool, 1)
				query := plugin.Query{Id: tc.name, Type: plugin.QueryTypeInput, RawQuery: "unmatched"}
				if tc.pluginScoped {
					query.TriggerKeyword = "apps"
				}
				run := newQueryRun(ctx, contract.QueryRequest{}, nil, query, nil)
				run.startTime = time.Now().Add(-tc.elapsed)
				run.startTimestamp = run.startTime.UnixMilli()
				finals, deadlines := 0, 0
				var delivered []plugin.QueryResultUI
				run.resultDebouncer = util.NewDebouncer(1000, 1000, func(items []plugin.QueryResultUI, reason string) {
					delivered = append(delivered, items...)
					if reason == "done" {
						finals++
					}
					if reason == "fallback_deadline" {
						deadlines++
					}
				})
				run.resultDebouncer.Start(ctx)
				if tc.queuedResult {
					results <- plugin.QueryResponseUI{Results: []plugin.QueryResultUI{{Id: "early"}}}
				}
				if tc.earlyReady {
					ready <- true
				}
				returned := false
				go func() {
					run.waitForResults(plugin.QueryExecution{Results: results, ImmediateDone: ready, Done: done})
					returned = true
				}()
				synctest.Wait()
				remaining := max(0, fallbackMaxWait-tc.elapsed)
				if remaining > 0 {
					time.Sleep(remaining - time.Millisecond)
					synctest.Wait()
					if run.fallbackHandled != tc.earlyReady {
						t.Fatalf("fallback before deadline = %t", run.fallbackHandled)
					}
					time.Sleep(time.Millisecond)
				}
				synctest.Wait()
				wantFallback := !tc.queuedResult && !tc.pluginScoped
				if run.fallbackHandled != wantFallback || finals != 0 || returned {
					t.Fatalf("at deadline: fallback=%t want=%t finals=%d returned=%t", run.fallbackHandled, wantFallback, finals, returned)
				}
				wantDeadlines := 0
				if wantFallback && !tc.earlyReady {
					wantDeadlines = 1
				}
				if deadlines != wantDeadlines {
					t.Fatalf("deadline flushes=%d want=%d", deadlines, wantDeadlines)
				}
				results <- plugin.QueryResponseUI{Results: []plugin.QueryResultUI{{Id: "late"}}}
				ready <- true
				synctest.Wait()
				if finals != 0 || returned {
					t.Fatal("immediate completion ended pending jobs")
				}
				done <- true
				synctest.Wait()
				wantCount := 1
				if tc.queuedResult {
					wantCount++
				}
				if !returned || finals != 1 || len(delivered) != wantCount || run.totalResultCount != wantCount {
					t.Fatalf("completion: returned=%t finals=%d delivered=%d total=%d", returned, finals, len(delivered), run.totalResultCount)
				}
			})
		})
	}
}
