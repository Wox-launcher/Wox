package plugin

import (
	"context"
	"strings"
	"sync"
	"time"
	"wox/common"
)

const autoRecordQueryHistoryDelay = 800 * time.Millisecond

type autoQueryHistorySession struct {
	queryID string
	timer   *time.Timer
}

// autoQueryHistoryRecorder records stable display-only queries without learning intermediate input.
type autoQueryHistoryRecorder struct {
	mu       sync.Mutex
	delay    time.Duration
	sessions map[string]*autoQueryHistorySession
	record   func(context.Context, common.PlainQuery)
}

// newAutoQueryHistoryRecorder creates a recorder with an injectable sink for focused tests.
func newAutoQueryHistoryRecorder(delay time.Duration, record func(context.Context, common.PlainQuery)) *autoQueryHistoryRecorder {
	return &autoQueryHistoryRecorder{
		delay:    delay,
		sessions: map[string]*autoQueryHistorySession{},
		record:   record,
	}
}

// beginQuery makes the query current for its launcher session and cancels pending older input.
func (r *autoQueryHistoryRecorder) beginQuery(query Query) {
	if query.SessionId == "" || query.Id == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if current := r.sessions[query.SessionId]; current != nil && current.timer != nil {
		current.timer.Stop()
	}
	r.sessions[query.SessionId] = &autoQueryHistorySession{queryID: query.Id}
}

// schedule records the latest eligible query after the user leaves it unchanged for the debounce window.
func (r *autoQueryHistoryRecorder) schedule(ctx context.Context, query Query, response QueryResponse) {
	if !response.AutoRecordQueryHistory || len(response.Results) == 0 || query.Type != QueryTypeInput || query.SessionId == "" || query.Id == "" || strings.TrimSpace(query.RawQuery) == "" {
		return
	}

	r.mu.Lock()
	current := r.sessions[query.SessionId]
	if current == nil || current.queryID != query.Id {
		r.mu.Unlock()
		return
	}
	if current.timer != nil {
		current.timer.Stop()
	}

	plainQuery := common.PlainQuery{QueryType: query.Type, QueryText: query.RawQuery}
	var timer *time.Timer
	timer = time.AfterFunc(r.delay, func() {
		r.mu.Lock()
		latest := r.sessions[query.SessionId]
		if latest != current || latest.queryID != query.Id || latest.timer != timer {
			r.mu.Unlock()
			return
		}
		latest.timer = nil
		r.mu.Unlock()

		if r.record != nil {
			r.record(context.WithoutCancel(ctx), plainQuery)
		}
	})
	current.timer = timer
	r.mu.Unlock()
}
