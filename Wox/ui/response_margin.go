package ui

import (
	"context"
	"wox/plugin"
	"wox/util"
)

type ResponseMargin struct {
	lastFlushedTimestamp int64
	results              []plugin.QueryResultUI
	maxFlushInterval     int64
	onFlush              func([]plugin.QueryResultUI, string)
}

func NewResponseMargin(maxFlushInterval int64, onFlush func([]plugin.QueryResultUI, string)) *ResponseMargin {
	return &ResponseMargin{
		maxFlushInterval:     maxFlushInterval,
		onFlush:              onFlush,
		lastFlushedTimestamp: util.GetSystemTimestamp(),
	}
}

func (r *ResponseMargin) Add(ctx context.Context, result []plugin.QueryResultUI) {
	r.results = append(r.results, result...)
	if util.GetSystemTimestamp()-r.lastFlushedTimestamp > r.maxFlushInterval {
		r.Flush(ctx, "timeout")
	}
}

func (r *ResponseMargin) Flush(ctx context.Context, reason string) {
	r.lastFlushedTimestamp = util.GetSystemTimestamp()
	flushedResults := r.results
	r.results = nil
	r.onFlush(flushedResults, reason)
}
