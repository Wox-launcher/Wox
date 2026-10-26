package plugin

import (
	"context"
	"sync"
	"wox/util"
)

type queryResultLifetimeKey struct{}

// queryResultLifetime deliberately contains no result data: plugins may retain
// their query context after Core has evicted the expensive result set.
type queryResultLifetime struct {
	mu        sync.Mutex
	released  bool
	callbacks map[any]func()
}

// withQueryResultLifetime binds asynchronous host work to the original result set,
// including a missing (already evicted) set. Completion must not recreate it.
func (s *queryResultStore) withQueryResultLifetime(ctx context.Context, sessionID, queryID string) context.Context {
	if s == nil || sessionID == "" || queryID == "" {
		return ctx
	}
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionID), queryID)
	var lifetime *queryResultLifetime
	if set, ok := s.getQuery(sessionID, queryID); ok {
		lifetime = set.lifetime
	}
	return context.WithValue(ctx, queryResultLifetimeKey{}, lifetime)
}

// QueryResultLifetime exposes internal host cleanup registration, not a plugin SDK API.
// Hosts register after sending work so release cannot reach the host before that work.
// Missing contexts retain compatibility with callers outside managed query execution.
func QueryResultLifetime(ctx context.Context) (register func(owner any, release func()), managed bool) {
	set, managed := ctx.Value(queryResultLifetimeKey{}).(*queryResultLifetime)
	if !managed {
		return nil, false
	}
	return func(owner any, release func()) {
		if set == nil {
			release()
			return
		}
		set.mu.Lock()
		if set.released {
			set.mu.Unlock()
			release()
			return
		}
		if set.callbacks == nil {
			set.callbacks = make(map[any]func())
		}
		set.callbacks[owner] = release
		set.mu.Unlock()
	}, true
}

// release invalidates registration before invoking callbacks outside the lock.
func (s *queryResultLifetime) release() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.released = true
	callbacks := s.callbacks
	s.callbacks = nil
	s.mu.Unlock()
	for _, release := range callbacks {
		release()
	}
}
