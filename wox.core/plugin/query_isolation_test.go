package plugin

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
)

func TestSlowPluginDoesNotBlockFastResultsOrAccumulateWorkers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := newQueryScheduler()
		slow := &Instance{Metadata: Metadata{Id: "slow", Name: "slow"}}
		fast := &Instance{Metadata: Metadata{Id: "fast", Name: "fast"}}
		release := make(chan struct{})
		defer close(release)
		started := make(chan context.Context, 4)
		callbacks := queryExecutionCallbacks{
			queryPlugin: func(ctx context.Context, instance *Instance, _ Query) (QueryResponse, bool) {
				if instance == slow {
					started <- ctx
					<-release
					return NewQueryResponse([]QueryResult{{Title: "late"}}), false
				}
				return NewQueryResponse([]QueryResult{{Title: "fast"}}), false
			},
			onPluginResponse: func(context.Context, Query, QueryResponse) {},
			timeoutPlugin:    func(context.Context, *Instance, Query) QueryResponse { return QueryResponse{} },
		}
		jobs := []queryPluginJob{{pluginInstance: slow}, {pluginInstance: fast}}
		for _, id := range []string{"one", "two", "three"} {
			start := time.Now()
			execution := scheduler.start(context.Background(), Query{Id: id, SessionId: "same", Type: QueryTypeInput}, jobs, 0, callbacks)
			if row := <-execution.Results; len(row.Results) != 1 || row.Results[0].Title != "fast" || time.Since(start) != 0 {
				t.Fatalf("fast result was delayed by slow plugin: %+v", row.Results)
			}
			<-execution.Done
			if time.Since(start) != globalPluginQueryTimeout {
				t.Fatalf("slow job completed after %s", time.Since(start))
			}
		}
		if len(started) != 1 || context.Cause(<-started) != context.DeadlineExceeded {
			t.Fatal("non-cooperative plugin acquired multiple workers or did not observe the deadline")
		}
		state, _ := scheduler.pluginCircuits.Load(slow.Metadata.Id)
		if state.timeouts != 3 || !state.openUntil.After(time.Now()) {
			t.Fatalf("three timeouts did not open circuit: %+v", state)
		}
		start := time.Now()
		execution := scheduler.start(context.Background(), Query{Id: "four", SessionId: "same", Type: QueryTypeInput}, jobs, 0, callbacks)
		<-execution.Done
		if time.Since(start) != 0 {
			t.Fatal("open circuit did not skip global job")
		}
		execution = scheduler.start(context.Background(), Query{Id: "five", SessionId: "other", Type: QueryTypeInput, TriggerKeyword: "slow"}, []queryPluginJob{{pluginInstance: slow}}, 0, queryExecutionCallbacks{
			queryPlugin:      func(context.Context, *Instance, Query) (QueryResponse, bool) { return QueryResponse{}, false },
			onPluginResponse: callbacks.onPluginResponse,
		})
		<-execution.Done
		state, _ = scheduler.pluginCircuits.Load(slow.Metadata.Id)
		if state.consecutive != 0 || !state.openUntil.IsZero() || state.timeouts != 3 {
			t.Fatalf("successful explicit query did not recover circuit: %+v", state)
		}
	})
}

func TestExplicitPluginProgressRenewsOnlyUpToLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := newQueryScheduler()
		instance := &Instance{Metadata: Metadata{Id: "stream", Name: "stream"}}
		query := Query{Id: "stream", SessionId: "launcher", Type: QueryTypeInput, TriggerKeyword: "stream"}
		start := time.Now()
		execution := scheduler.start(context.Background(), query, []queryPluginJob{{pluginInstance: instance}}, 0, queryExecutionCallbacks{
			queryPlugin: func(ctx context.Context, _ *Instance, _ Query) (QueryResponse, bool) {
				// A targeted plugin can take longer than five seconds to publish its first batch.
				for range 2 {
					select {
					case <-ctx.Done():
						return QueryResponse{}, false
					case <-time.After(7 * time.Second):
						scheduler.notePluginQueryProgress(query, instance.Metadata.Id)
					}
				}
				<-ctx.Done()
				return QueryResponse{}, false
			},
			onPluginResponse: func(context.Context, Query, QueryResponse) {},
			timeoutPlugin: func(context.Context, *Instance, Query) QueryResponse {
				return NewQueryResponse([]QueryResult{{Title: "query timed out"}})
			},
		})
		<-execution.Done
		if time.Since(start) != targetedPluginQueryLimit {
			t.Fatalf("stream lasted %s", time.Since(start))
		}
		if row := <-execution.Results; len(row.Results) != 1 || row.Results[0].Title != "query timed out" {
			t.Fatalf("missing explicit timeout result: %+v", row.Results)
		}
	})
}

func TestCancellationCompletesPendingDebouncedJobs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		instance := &Instance{Metadata: Metadata{Id: "debounced", Name: "debounced"}}
		execution := newQueryScheduler().start(ctx, Query{}, []queryPluginJob{{pluginInstance: instance, debounced: true, intervalMs: 10000}}, 0, queryExecutionCallbacks{
			queryPlugin: func(context.Context, *Instance, Query) (QueryResponse, bool) {
				t.Error("canceled debounce executed plugin")
				return QueryResponse{}, false
			},
		})
		start := time.Now()
		cancel()
		<-execution.Done
		if elapsed := time.Since(start); elapsed != 0 {
			t.Fatalf("debounced query cancellation took %s", elapsed)
		}
	})
}

func TestHostBackpressureCountsTowardCircuitBreaker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := newQueryScheduler()
		instance := &Instance{Metadata: Metadata{Id: "host", Name: "host"}}
		calls := 0
		callbacks := queryExecutionCallbacks{
			queryPlugin: func(context.Context, *Instance, Query) (QueryResponse, bool) {
				calls++
				return QueryResponse{QueryBackpressured: true}, false
			},
		}
		for range pluginCircuitThreshold {
			start := time.Now()
			execution := scheduler.start(context.Background(), Query{Type: QueryTypeInput}, []queryPluginJob{{pluginInstance: instance}}, 0, callbacks)
			<-execution.Done
			if elapsed := time.Since(start); elapsed != globalPluginQueryTimeout {
				t.Fatalf("busy host finished after %s, want %s", elapsed, globalPluginQueryTimeout)
			}
		}
		synctest.Wait()
		attempts := calls
		execution := scheduler.start(context.Background(), Query{Type: QueryTypeInput}, []queryPluginJob{{pluginInstance: instance}}, 0, callbacks)
		<-execution.Done
		state, _ := scheduler.pluginCircuits.Load(instance.Metadata.Id)
		if attempts <= pluginCircuitThreshold || calls != attempts || state.timeouts != pluginCircuitThreshold || !state.openUntil.After(time.Now()) {
			t.Fatalf("busy host calls=%d circuit=%+v", calls, state)
		}
	})
}

func TestHostBackpressureRecoversAndReleasesExecutionContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		instance := &Instance{Metadata: Metadata{Id: "host", Name: "host"}}
		var executionCtx, sessionCtx context.Context
		calls := 0
		start := time.Now()
		execution := newQueryScheduler().start(parent, Query{Type: QueryTypeInput, TriggerKeyword: "host"}, []queryPluginJob{{pluginInstance: instance}}, 0, queryExecutionCallbacks{
			queryPlugin: func(ctx context.Context, _ *Instance, _ Query) (QueryResponse, bool) {
				calls++
				if calls == 1 {
					return QueryResponse{QueryBackpressured: true}, false
				}
				executionCtx, sessionCtx = ctx, QuerySessionContext(ctx)
				return NewQueryResponse([]QueryResult{{Title: "recovered"}}), false
			},
			onPluginResponse: func(context.Context, Query, QueryResponse) {},
		})
		<-execution.Done
		if row := <-execution.Results; len(row.Results) != 1 || row.Results[0].Title != "recovered" || time.Since(start) != pluginQueryRetryDelay {
			t.Fatalf("busy host did not recover on retry: elapsed=%s results=%+v", time.Since(start), row.Results)
		}
		if executionCtx.Err() != context.Canceled || sessionCtx.Err() != nil {
			t.Fatal("completion must release only the execution context")
		}
		cancel()
		if sessionCtx.Err() != context.Canceled {
			t.Fatal("session cancellation did not reach background work")
		}
	})
}

func TestCircuitOpenNotifiesOncePerEpisode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		scheduler := newQueryScheduler()
		slow := &Instance{Metadata: Metadata{Id: "slow-notify", Name: "Slow Notify"}}
		openEvents := 0
		var openCooldown time.Duration
		callbacks := queryExecutionCallbacks{
			queryPlugin: func(ctx context.Context, _ *Instance, _ Query) (QueryResponse, bool) {
				<-ctx.Done()
				return QueryResponse{}, false
			},
			onPluginResponse: func(context.Context, Query, QueryResponse) {},
			timeoutPlugin:    func(context.Context, *Instance, Query) QueryResponse { return QueryResponse{} },
			onCircuitOpen: func(_ context.Context, instance *Instance, cooldown time.Duration) {
				if instance == slow {
					openEvents++
					openCooldown = cooldown
				}
			},
		}
		jobs := []queryPluginJob{{pluginInstance: slow}}
		for attempt := 1; attempt <= pluginCircuitThreshold; attempt++ {
			execution := scheduler.start(context.Background(), Query{Id: "g", SessionId: "s", Type: QueryTypeInput}, jobs, 0, callbacks)
			<-execution.Done
			want := 0
			if attempt == pluginCircuitThreshold {
				want = 1
			}
			if openEvents != want {
				t.Fatalf("after timeout %d: openEvents=%d, want %d", attempt, openEvents, want)
			}
		}
		if openCooldown != pluginCircuitWindow {
			t.Fatalf("openCooldown=%s, want %s", openCooldown, pluginCircuitWindow)
		}

		// Skipped global query while open must not re-notify.
		execution := scheduler.start(context.Background(), Query{Id: "skip", SessionId: "s", Type: QueryTypeInput}, jobs, 0, callbacks)
		<-execution.Done
		// Explicit timeout while already open must not re-notify either.
		execution = scheduler.start(context.Background(), Query{Id: "explicit-timeout", SessionId: "s2", Type: QueryTypeInput, TriggerKeyword: "slow"}, jobs, 0, callbacks)
		<-execution.Done
		if openEvents != 1 {
			t.Fatalf("duplicate notifications while circuit remained open: openEvents=%d", openEvents)
		}

		// Successful explicit query resets the circuit so a future episode notifies again.
		execution = scheduler.start(context.Background(), Query{Id: "recover", SessionId: "s2", Type: QueryTypeInput, TriggerKeyword: "slow"}, jobs, 0, queryExecutionCallbacks{
			queryPlugin:      func(context.Context, *Instance, Query) (QueryResponse, bool) { return QueryResponse{}, false },
			onPluginResponse: callbacks.onPluginResponse,
		})
		<-execution.Done
		for range pluginCircuitThreshold {
			execution = scheduler.start(context.Background(), Query{Id: "g2", SessionId: "s", Type: QueryTypeInput}, jobs, 0, callbacks)
			<-execution.Done
		}
		if openEvents != 2 {
			t.Fatalf("second circuit-open episode did not notify: openEvents=%d", openEvents)
		}
	})
}

type circuitNoticeTestUI struct {
	common.UI
	toast  common.ToastMsg
	toasts int
	query  common.PlainQuery
	shown  int
}

func (u *circuitNoticeTestUI) ShowToast(_ context.Context, msg common.ToastMsg) {
	u.toast = msg
	u.toasts++
}

func (u *circuitNoticeTestUI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	u.query = query
}

func (u *circuitNoticeTestUI) ShowApp(_ context.Context, _ common.ShowContext) {
	u.shown++
}

func TestNotifyPluginCircuitOpenShowsToastWithDiagnosticsAction(t *testing.T) {
	ctx := context.Background()
	if err := i18n.GetI18nManager().UpdateLang(ctx, i18n.LangCodeEnUs); err != nil {
		t.Fatalf("UpdateLang(en_US): %v", err)
	}
	slow := &Instance{Metadata: Metadata{Id: "slow-ui", Name: "Slow Plugin"}}

	// Nil UI and lifecycle must not panic.
	(&Manager{}).notifyPluginCircuitOpen(ctx, slow, pluginCircuitWindow)

	fakeUI := &circuitNoticeTestUI{}
	lifecycle := newPluginLifecycle()
	doctor := &Instance{Metadata: Metadata{Id: common.DoctorPluginID, TriggerKeywords: []string{"diag"}}}
	lifecycle.appendPluginInstance(doctor)
	manager := &Manager{ui: fakeUI, lifecycle: lifecycle}

	manager.notifyPluginCircuitOpen(ctx, slow, pluginCircuitWindow)
	if fakeUI.toasts != 1 {
		t.Fatalf("toasts=%d, want 1", fakeUI.toasts)
	}
	if fakeUI.toast.Icon != icons.Get(icons.StatusWarning).String() {
		t.Fatalf("toast icon=%q, want warning icon", fakeUI.toast.Icon)
	}
	if !strings.Contains(fakeUI.toast.Text, "Slow Plugin") || !strings.Contains(fakeUI.toast.Text, "60") {
		t.Fatalf("unexpected toast text: %q", fakeUI.toast.Text)
	}
	if fakeUI.toast.Action == nil || fakeUI.toast.Action.Action == nil {
		t.Fatal("missing diagnostics toast action")
	}
	fakeUI.toast.Action.Action(ctx)
	if fakeUI.query.QueryType != QueryTypeInput || fakeUI.query.QueryText != "diag perf " {
		t.Fatalf("unexpected diagnostics query: %+v", fakeUI.query)
	}
	if fakeUI.shown != 1 {
		t.Fatalf("ShowApp count=%d, want 1", fakeUI.shown)
	}
}
