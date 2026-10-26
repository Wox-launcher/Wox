package plugin

import (
	"context"
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"wox/util"
	"wox/util/timetracking"
)

// queryScheduler owns cross-query timing state, while queryExecution owns each run.
// Business callbacks do not receive scheduler channels or completion counters.
type queryScheduler struct {
	debounceQueryTimer *util.HashMap[queryDebounceKey, *debounceTimer]
	// Learn from result-producing jobs through channel delivery, not just plugin execution.
	pluginResultDeliveryLatency *util.HashMap[string, *util.EWMA]
}

// queryExecutionCallbacks keep query business rules outside the scheduler.
// Every callback is required; Manager supplies them when starting a planned query.
type queryExecutionCallbacks struct {
	queryPlugin      func(context.Context, *Instance, Query) QueryResponse
	onPluginResponse func(context.Context, Query, QueryResponse)
	restoreAliases   func(context.Context, Query) QueryResponse
}

// newQueryScheduler initializes the state shared by all executions of one Manager.
func newQueryScheduler() *queryScheduler {
	return &queryScheduler{
		debounceQueryTimer:          util.NewHashMap[queryDebounceKey, *debounceTimer](),
		pluginResultDeliveryLatency: util.NewHashMap[string, *util.EWMA](),
	}
}

type debounceTimer struct {
	timer  *time.Timer
	onStop func()
}

// queryDebounceKey keeps independent launcher windows from cancelling one another's searches.
type queryDebounceKey struct {
	sessionID string
	pluginID  string
}

// queryPluginJob is the scheduler's normalized decision for one plugin.
// Keeping the debounce and fallback flags together makes the lifecycle rule
// explicit: debounced jobs still affect done, but they do not block fallback.
type queryPluginJob struct {
	// pluginInstance is the runnable plugin selected by Manager's query plan.
	pluginInstance *Instance
	// blocksFallback decides whether this job must finish before fallback can be shown.
	blocksFallback bool
	// debounced means the job starts from a debounce timer instead of immediately.
	debounced bool
	// intervalMs is only used for debounced jobs and stores the timer delay.
	intervalMs int
	// aliasRestore is the core job that restores result-alias matches.
	aliasRestore bool
}

// QueryExecution exposes one planned query run to the UI pipeline without leaking scheduler internals.
type QueryExecution struct {
	Results              <-chan QueryResponseUI
	ImmediateDone        <-chan bool
	Done                 <-chan bool
	FirstFlushDeadlineMs int64
}

// queryExecution owns the already-planned plugin jobs for one Manager.Query call.
type queryExecution struct {
	// ctx carries query trace/session metadata through scheduling and plugin goroutines.
	ctx context.Context
	// scheduler owns shared debounce and delivery-latency state.
	scheduler *queryScheduler
	// callbacks provide business operations without exposing Manager state.
	callbacks queryExecutionCallbacks
	// query is the parsed backend query shared by all scheduled plugin jobs.
	query Query
	// resultsChan receives normalized plugin responses in the public Manager.Query contract.
	resultsChan chan QueryResponseUI
	// tracker emits fallback-ready and done lifecycle signals for this query.
	tracker *queryTracker
	// jobs are fully registered with tracker before any plugin can finish.
	jobs []queryPluginJob
}

// queryTracker splits query completion into two phases:
// 1. fallbackReady: all fallback-blocking jobs have finished.
// 2. done: all jobs have finished, including debounced jobs that may return later.
//
// Debounced plugins are counted only in remaining. This lets the UI show fallback
// once the immediate plugins are done, while still keeping the query open for late
// debounced results to arrive.
type queryTracker struct {
	remaining         *atomic.Int32
	fallbackRemaining *atomic.Int32
	fallbackReady     chan bool
	done              chan bool
}

// start allocates one query's channels and launches its already-planned jobs.
func (s *queryScheduler) start(ctx context.Context, query Query, jobs []queryPluginJob, firstFlushDeadlineMs int64, callbacks queryExecutionCallbacks) QueryExecution {
	channelCapacity := len(jobs)
	if channelCapacity < 10 {
		channelCapacity = 10
	}
	resultsChan := make(chan QueryResponseUI, channelCapacity)
	immediateDoneChan := make(chan bool, 1)
	doneChan := make(chan bool, 1)
	execution := &queryExecution{
		ctx:         ctx,
		scheduler:   s,
		callbacks:   callbacks,
		query:       query,
		resultsChan: resultsChan,
		tracker:     newQueryTracker(immediateDoneChan, doneChan),
		jobs:        jobs,
	}
	// Install pending timers before returning so consecutive calls cannot replace
	// a newer query with an older one merely because their setup goroutines raced.
	execution.start()
	return QueryExecution{
		Results:              resultsChan,
		ImmediateDone:        immediateDoneChan,
		Done:                 doneChan,
		FirstFlushDeadlineMs: firstFlushDeadlineMs,
	}
}

// start registers all jobs before any can complete.
func (e *queryExecution) start() {
	// Register every job before starting any goroutine so fast plugins cannot
	// complete the lifecycle before later jobs have been counted.
	for _, job := range e.jobs {
		e.tracker.startJob(job.blocksFallback)
	}
	e.tracker.signalReadyIfEmpty()

	for _, job := range e.jobs {
		e.startPluginJob(job)
	}
}

// startPluginJob dispatches alias, immediate, and debounced jobs with their existing policies.
func (e *queryExecution) startPluginJob(job queryPluginJob) {
	if job.aliasRestore {
		e.runResultAliasJob(job)
		return
	}
	pluginLabel := queryDiagnosticPluginLabel(job.pluginInstance)
	if job.debounced {
		if tracker := timetracking.New("start_plugin_job"); tracker.Enabled() {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetRawString("mode", "debounced")
			tracker.SetInt("intervalMs", job.intervalMs)
			tracker.Log(e.ctx)
		}
		e.replaceDebouncedJob(job)
		return
	}
	e.scheduler.debounceQueryTimer.Compute(queryDebounceKey{e.query.SessionId, job.pluginInstance.Metadata.Id}, func(pending *debounceTimer, _ bool) (*debounceTimer, bool) {
		if pending != nil && pending.timer.Stop() {
			pending.onStop()
		}
		return nil, false
	})
	e.runPluginJob(job)
}

// replaceDebouncedJob completes a stopped job before installing its replacement.
// Timer replacement is atomic within a session and plugin.
func (e *queryExecution) replaceDebouncedJob(job queryPluginJob) {
	pluginInstance := job.pluginInstance
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	util.GetLogger().Debug(e.ctx, fmt.Sprintf("[%s] debounce query, will execute in %d ms", pluginInstance.GetName(e.ctx), job.intervalMs))
	if tracker := timetracking.New("debounce_schedule"); tracker.Enabled() {
		tracker.SetRawString("queryId", e.query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetInt("intervalMs", job.intervalMs)
		tracker.Log(e.ctx)
	}
	key := queryDebounceKey{e.query.SessionId, pluginInstance.Metadata.Id}
	e.scheduler.debounceQueryTimer.Compute(key, func(previous *debounceTimer, _ bool) (*debounceTimer, bool) {
		if previous != nil && previous.timer.Stop() {
			previous.onStop()
		}
		pending := &debounceTimer{}
		debounceScheduledAt := util.GetSystemTimestamp()
		pending.timer = time.AfterFunc(time.Duration(job.intervalMs)*time.Millisecond, func() {
			e.scheduler.debounceQueryTimer.Compute(key, func(current *debounceTimer, exists bool) (*debounceTimer, bool) {
				return current, exists && current != pending
			})
			if tracker := timetracking.New("debounce_fire"); tracker.Enabled() {
				tracker.SetRawString("queryId", e.query.Id)
				tracker.SetRawString("plugin", pluginLabel)
				tracker.SetInt64("waitedMs", util.GetSystemTimestamp()-debounceScheduledAt)
				tracker.Log(e.ctx)
			}
			e.runPluginJob(job)
		})
		pending.onStop = func() {
			// A newer query replaced this debounced run before it started. Mark this
			// job as finished for the current query lifecycle so counters do not hang.
			util.GetLogger().Debug(e.ctx, fmt.Sprintf("[%s] previous debounced query cancelled", pluginInstance.GetName(e.ctx)))
			if tracker := timetracking.New("debounce_cancel"); tracker.Enabled() {
				tracker.SetRawString("queryId", e.query.Id)
				tracker.SetRawString("plugin", pluginLabel)
				tracker.SetInt64("waitedMs", util.GetSystemTimestamp()-debounceScheduledAt)
				tracker.Log(e.ctx)
			}
			e.tracker.finishJob(job.blocksFallback)
		}
		return pending, true
	})
}

// runPluginJob delivers the response before signaling completion and learning its latency.
func (e *queryExecution) runPluginJob(job queryPluginJob) {
	pluginInstance := job.pluginInstance
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	util.Go(e.ctx, fmt.Sprintf("[%s] parallel query", pluginInstance.GetName(e.ctx)), func() {
		jobStart := util.GetSystemTimestamp()
		jobTimingStart := time.Now()
		// QueryResponse keeps result rows and query-scoped UI metadata together.
		// Sending one normalized response through the query pipeline prevents the
		// UI from applying refinements or layout from a different query execution.
		queryForPluginStart := util.GetSystemTimestamp()
		queryResponse := e.callbacks.queryPlugin(e.ctx, pluginInstance, e.query)
		queryForPluginCost := util.GetSystemTimestamp() - queryForPluginStart
		e.callbacks.onPluginResponse(e.ctx, e.query, queryResponse)
		if tracker := timetracking.New("query_for_plugin_done"); tracker.Enabled() && (len(queryResponse.Results) > 0 || queryForPluginCost >= 5) {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt("resultCount", len(queryResponse.Results))
			tracker.SetInt64("costMs", queryForPluginCost)
			tracker.Log(e.ctx)
		}
		// Bug diagnostics: queryForPlugin logs before response conversion and
		// tracker completion. These boundaries make it clear whether a future
		// spinner is stuck while converting/sending results or while marking the
		// plugin as finished for the query lifecycle.
		toUIStart := util.GetSystemTimestamp()
		toUITimingStart := time.Now()
		queryResponseUI := queryResponse.ToUI()
		toUICost := util.GetSystemTimestamp() - toUIStart
		toUICostUs := time.Since(toUITimingStart).Microseconds()
		if tracker := timetracking.New("to_ui"); tracker.Enabled() && len(queryResponseUI.Results) > 0 {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt("resultCount", len(queryResponseUI.Results))
			tracker.SetInt64("costMs", toUICost)
			tracker.SetInt64("costUs", toUICostUs)
			tracker.Log(e.ctx)
		}
		sendStart := util.GetSystemTimestamp()
		sendTimingStart := time.Now()
		e.resultsChan <- queryResponseUI
		sendCost := util.GetSystemTimestamp() - sendStart
		sendCostUs := time.Since(sendTimingStart).Microseconds()
		deliveryElapsedUs := time.Since(jobTimingStart).Microseconds()
		if len(queryResponseUI.Results) > 0 {
			e.scheduler.updatePluginResultDeliveryLatency(pluginInstance.Metadata.Id, float64(deliveryElapsedUs)/1000)
			util.GetLogger().Debug(e.ctx, fmt.Sprintf("<%s> query response delivered to query pipeline, result count: %d", pluginInstance.GetName(e.ctx), len(queryResponseUI.Results)))
		}
		if tracker := timetracking.New("channel_send"); tracker.Enabled() && (len(queryResponseUI.Results) > 0 || deliveryElapsedUs >= 5000) {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt("resultCount", len(queryResponseUI.Results))
			tracker.SetInt64("costMs", sendCost)
			tracker.SetInt64("costUs", sendCostUs)
			tracker.SetInt64("elapsedSinceJobStartMs", util.GetSystemTimestamp()-jobStart)
			tracker.SetInt64("elapsedSinceJobStartUs", deliveryElapsedUs)
			tracker.Log(e.ctx)
		}
		finishStart := util.GetSystemTimestamp()
		e.tracker.finishJob(job.blocksFallback)
		finishCost := util.GetSystemTimestamp() - finishStart
		if tracker := timetracking.New("tracker_finish_job"); tracker.Enabled() && len(queryResponseUI.Results) > 0 {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetBool("blocksFallback", job.blocksFallback)
			tracker.SetInt("remaining", int(e.tracker.remaining.Load()))
			tracker.SetInt("fallbackRemaining", int(e.tracker.fallbackRemaining.Load()))
			tracker.SetInt64("costMs", finishCost)
			tracker.Log(e.ctx)
		}
	}, func() {
		util.GetLogger().Warn(e.ctx, fmt.Sprintf("<%s> query goroutine recovered, force finishing tracker", pluginInstance.GetName(e.ctx)))
		if tracker := timetracking.New("plugin_job_recovered"); tracker.Enabled() {
			tracker.SetRawString("queryId", e.query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.Log(e.ctx)
		}
		e.tracker.finishJob(job.blocksFallback)
	})
}

// runResultAliasJob keeps alias restoration cancellable without changing ordinary plugin delivery.
func (e *queryExecution) runResultAliasJob(job queryPluginJob) {
	util.Go(e.ctx, "result alias restore", func() {
		defer e.tracker.finishJob(job.blocksFallback)
		response := e.callbacks.restoreAliases(e.ctx, e.query)
		if len(response.Results) == 0 {
			return
		}
		select {
		case e.resultsChan <- response.ToUI():
		case <-e.ctx.Done():
		}
	})
}

// newQueryTracker separates fallback readiness from completion of every scheduled job.
func newQueryTracker(fallbackReady chan bool, done chan bool) *queryTracker {
	return &queryTracker{
		remaining:         &atomic.Int32{},
		fallbackRemaining: &atomic.Int32{},
		fallbackReady:     fallbackReady,
		done:              done,
	}
}

// startJob accounts for one planned job before execution begins.
func (t *queryTracker) startJob(blocksFallback bool) {
	t.remaining.Add(1)
	if blocksFallback {
		t.fallbackRemaining.Add(1)
	}
}

// signalReadyIfEmpty covers queries with no jobs or only debounced jobs.
func (t *queryTracker) signalReadyIfEmpty() {
	if t.remaining.Load() == 0 {
		t.done <- true
	}
	if t.fallbackRemaining.Load() == 0 {
		t.fallbackReady <- true
	}
}

// finishJob preserves the final-before-fallback notification order.
func (t *queryTracker) finishJob(blocksFallback bool) {
	fallbackReady := blocksFallback && t.fallbackRemaining.Add(-1) == 0
	done := t.remaining.Add(-1) == 0
	// Queue done first when the last immediate job also completes the whole run.
	// The UI can then coalesce immediate-ready and final into one response.
	if done {
		t.done <- true
	}
	if fallbackReady {
		t.fallbackReady <- true
	}
}

// updatePluginResultDeliveryLatency learns only from jobs that produced visible results.
func (s *queryScheduler) updatePluginResultDeliveryLatency(pluginId string, costMs float64) {
	ewma, ok := s.pluginResultDeliveryLatency.Load(pluginId)
	if !ok {
		ewma, _ = s.pluginResultDeliveryLatency.LoadOrStore(pluginId, util.NewEWMA(0.3))
	}
	ewma.Add(costMs)
}

// getQueryFirstFlushDeadlineMs bounds first-batch waiting while favoring historically complete local results.
func (s *queryScheduler) getQueryFirstFlushDeadlineMs(jobs []queryPluginJob, planElapsedMs int64) int64 {
	const minDeadlineMs int64 = 12
	const maxDeadlineMs int64 = 20
	const deliveryGuardMs int64 = 4

	var maxDeliveryMs float64
	hasDeliveryHistory := false
	for _, job := range jobs {
		if job.aliasRestore || job.debounced || job.pluginInstance == nil {
			continue
		}
		if ewma, ok := s.pluginResultDeliveryLatency.Load(job.pluginInstance.Metadata.Id); ok {
			if avg, hasValue := ewma.Value(); hasValue {
				hasDeliveryHistory = true
				if avg > maxDeliveryMs {
					maxDeliveryMs = avg
				}
			}
		}
	}

	if !hasDeliveryHistory {
		return maxDeadlineMs
	}

	deadlineMs := planElapsedMs + int64(math.Ceil(maxDeliveryMs)) + deliveryGuardMs
	if deadlineMs < minDeadlineMs {
		return minDeadlineMs
	}
	if deadlineMs > maxDeadlineMs {
		return maxDeadlineMs
	}
	return deadlineMs
}
