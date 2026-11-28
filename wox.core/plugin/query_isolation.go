package plugin

import (
	"context"
	"time"
	"wox/util"
)

const (
	globalPluginQueryTimeout   = 1500 * time.Millisecond
	targetedPluginQueryTimeout = 10 * time.Second
	targetedPluginQueryLimit   = 15 * time.Second
	pluginQueryRetryDelay      = 125 * time.Millisecond
	pluginCircuitWindow        = time.Minute
	pluginCircuitThreshold     = 3
)

type pluginCircuitState struct {
	firstTimeout time.Time
	openUntil    time.Time
	consecutive  int
	timeouts     uint64
}

type pluginQueryLease struct {
	id       string
	cancel   context.CancelCauseFunc
	done     chan struct{}
	progress chan time.Time
}

type isolatedQueryResponse struct {
	response  QueryResponse
	recovered bool
}

// The scheduler owns cancellation while the transport can bound blocking writes
// by the absolute limit, including the allowance for streaming results.
type pluginQueryContext struct {
	context.Context
	deadline time.Time
}

type querySessionContextKey struct{}

// QuerySessionContext returns the lifetime of the current launcher query for
// built-in plugins that publish results after their Query method returns.
func QuerySessionContext(ctx context.Context) context.Context {
	if sessionCtx, ok := ctx.Value(querySessionContextKey{}).(context.Context); ok {
		return sessionCtx
	}
	return ctx
}

func (c pluginQueryContext) Deadline() (time.Time, bool) { return c.deadline, true }

// queryPlugin bounds completion independently of plugin cooperation. The execution
// lease remains occupied until the worker exits, preventing stuck jobs from piling up.
func (e *queryExecution) queryPlugin(instance *Instance) (isolatedQueryResponse, error) {
	s := e.scheduler
	pluginID := instance.Metadata.Id
	now := time.Now()
	global := e.query.IsGlobalQuery()
	if state, ok := s.pluginCircuits.Load(pluginID); ok && global && now.Before(state.openUntil) {
		return isolatedQueryResponse{}, context.Canceled
	}
	timeout, limit := targetedPluginQueryTimeout, targetedPluginQueryLimit
	if global {
		timeout, limit = globalPluginQueryTimeout, globalPluginQueryTimeout
	}
	deadline := now.Add(limit)
	if parentDeadline, ok := e.ctx.Deadline(); ok && parentDeadline.Before(deadline) {
		deadline = parentDeadline
	}
	cancellable, cancel := context.WithCancelCause(e.ctx)
	defer cancel(nil)
	ctx := pluginQueryContext{context.WithValue(cancellable, querySessionContextKey{}, e.ctx), deadline}
	lease := &pluginQueryLease{id: e.query.Id, cancel: cancel, done: make(chan struct{}), progress: make(chan time.Time, 1)}
	key := queryDebounceKey{e.query.SessionId, pluginID}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		if ctx.Err() != nil {
			return isolatedQueryResponse{}, ctx.Err()
		}
		previous, loaded := s.pluginQueryLeases.LoadOrStore(key, lease)
		if !loaded {
			break
		}
		if ctx.Err() != nil {
			return isolatedQueryResponse{}, ctx.Err()
		}
		previous.cancel(context.Canceled)
		select {
		case <-previous.done:
		case <-ctx.Done():
			return isolatedQueryResponse{}, ctx.Err()
		case <-timer.C:
			cancel(context.DeadlineExceeded)
			if s.recordPluginTimeout(pluginID, time.Now()) && e.callbacks.onCircuitOpen != nil {
				e.callbacks.onCircuitOpen(e.ctx, instance, pluginCircuitWindow)
			}
			return isolatedQueryResponse{}, context.DeadlineExceeded
		}
	}
	responses := make(chan isolatedQueryResponse, 1)
	run := func() {
		defer close(lease.done)
		defer s.pluginQueryLeases.Delete(key)
		for ctx.Err() == nil {
			response, recovered := e.callbacks.queryPlugin(ctx, instance, e.query)
			if recovered || !response.QueryBackpressured {
				responses <- isolatedQueryResponse{response, recovered}
				return
			}
			// Retry a busy host within the original budget and hold the lease
			// between attempts so a newer query can preempt the retry.
			retry := time.NewTimer(pluginQueryRetryDelay)
			select {
			case <-retry.C:
			case <-ctx.Done():
				retry.Stop()
				return
			}
		}
	}
	util.Go(ctx, "execute isolated plugin query", run, func() {
		responses <- isolatedQueryResponse{recovered: true}
	})
	for {
		select {
		case <-ctx.Done():
			return isolatedQueryResponse{}, ctx.Err()
		case at := <-lease.progress:
			if !global {
				timer.Reset(max(0, min(timeout, time.Until(deadline), time.Until(at.Add(timeout)))))
			}
		case <-timer.C:
			cancel(context.DeadlineExceeded)
			if s.recordPluginTimeout(pluginID, time.Now()) && e.callbacks.onCircuitOpen != nil {
				e.callbacks.onCircuitOpen(e.ctx, instance, pluginCircuitWindow)
			}
			return isolatedQueryResponse{}, context.DeadlineExceeded
		case response := <-responses:
			if ctx.Err() != nil {
				return isolatedQueryResponse{}, ctx.Err()
			}
			if !response.recovered && !response.response.QueryFailed {
				s.resetPluginCircuit(pluginID, global)
			}
			return response, nil
		}
	}
}

// recordPluginTimeout updates the streak and lifetime count in one transaction.
// It returns true only when this timeout transitions the circuit from closed to open.
func (s *queryScheduler) recordPluginTimeout(pluginID string, now time.Time) (opened bool) {
	s.pluginCircuits.Compute(pluginID, func(state pluginCircuitState, _ bool) (pluginCircuitState, bool) {
		wasOpen := now.Before(state.openUntil)
		state.timeouts++
		if now.Sub(state.firstTimeout) >= pluginCircuitWindow {
			state.firstTimeout, state.consecutive = now, 0
		}
		state.consecutive++
		if state.consecutive >= pluginCircuitThreshold {
			state.openUntil = now.Add(pluginCircuitWindow)
			if !wasOpen {
				opened = true
			}
		}
		return state, true
	})
	return opened
}

// resetPluginCircuit accepts an explicit successful probe even during cooldown.
func (s *queryScheduler) resetPluginCircuit(pluginID string, global bool) {
	s.pluginCircuits.Compute(pluginID, func(state pluginCircuitState, exists bool) (pluginCircuitState, bool) {
		if !global || time.Now().After(state.openUntil) {
			state = pluginCircuitState{timeouts: state.timeouts}
		}
		return state, exists
	})
}

func (s *queryScheduler) notePluginQueryProgress(query Query, pluginID string) {
	lease, ok := s.pluginQueryLeases.Load(queryDebounceKey{query.SessionId, pluginID})
	if ok && lease.id == query.Id {
		select {
		case lease.progress <- time.Now():
		default:
		}
	}
}
