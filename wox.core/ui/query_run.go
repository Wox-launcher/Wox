package ui

import (
	"context"
	"fmt"
	"time"
	"wox/plugin"
	"wox/util"
)

// This value cannot be too small; otherwise frequent result-update requests
// cause constant window resizing and flickering. 32ms is roughly 30fps.
const resultDebounceIntervalMs = 32

// queryRun owns the per-query execution state that used to live in
// handleWebsocketQuery. Keeping this state together makes it clear that result
// batching, fallback, and final response delivery are scoped by query id, not by
// the session's latest visible query.
type queryRun struct {
	ctx     context.Context
	request WebsocketMsg
	// sessionId scopes the query result cache to the launcher window session.
	sessionId string
	// queryId scopes result snapshots so concurrent queries in the same session do not overwrite each other.
	queryId string
	// query is the parsed backend query passed to plugin scheduling and fallback handling.
	query plugin.Query
	// ownerPlugin is set for plugin-scoped queries and nil for global queries.
	ownerPlugin *plugin.Instance
	// startTimestamp records the query start time for elapsed metrics and debug tails.
	startTimestamp int64
	// firstFlushDelayMs records the first visible flush delay chosen for this query.
	firstFlushDelayMs int64
	// firstVisibleFlushElapsedMs records when the first non-empty snapshot is sent.
	firstVisibleFlushElapsedMs int64
	// resultFlushBatch tracks the visible snapshot batch number shown in dev performance tails.
	resultFlushBatch int
	// totalResultCount counts all accepted results for fallback decisions and completion logging.
	totalResultCount int
	// latestResponse keeps the newest context, layout, and refinement metadata for the next flush.
	latestResponse plugin.QueryResponseUI
	// fallbackHandled prevents duplicate fallback checks after fallbackReady and done both fire.
	fallbackHandled bool
	// resultDebouncer batches plugin results before flushing snapshots back to Flutter.
	resultDebouncer *util.Debouncer[plugin.QueryResultUI]
}

func newQueryRun(ctx context.Context, request WebsocketMsg, query plugin.Query, ownerPlugin *plugin.Instance) *queryRun {
	return &queryRun{
		ctx:         ctx,
		request:     request,
		sessionId:   request.SessionId,
		queryId:     query.Id,
		query:       query,
		ownerPlugin: ownerPlugin,
		latestResponse: plugin.QueryResponseUI{
			Context: plugin.BuildQueryContext(query, ownerPlugin),
		},
	}
}

func (r *queryRun) start() {
	r.startTimestamp = util.GetSystemTimestamp()
	r.firstFlushDelayMs = plugin.GetPluginManager().GetQueryFirstFlushDelayMs(r.query)
	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, first flush delay: %d ms", r.query.Type, r.query.String(), r.firstFlushDelayMs))

	r.resultDebouncer = util.NewDebouncer(r.firstFlushDelayMs, resultDebounceIntervalMs, r.flush)
	r.resultDebouncer.Start(r.ctx)
	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, result flushed (new start)", r.query.Type, r.query.String()))

	// Bug diagnostics: Manager.Query starts plugin work and returns the result
	// channels used by the select loop below. If a future log has "query pipeline
	// starting" but not "ready", the stall is inside scheduler setup rather than
	// UI result handling.
	logger.Debug(r.ctx, fmt.Sprintf("query pipeline starting: queryId=%s query=%s ownerPlugin=%s", r.queryId, r.query.String(), queryPipelinePluginLabel(r.ctx, r.ownerPlugin)))
	resultChan, fallbackReadyChan, doneChan := plugin.GetPluginManager().Query(r.ctx, r.query)
	logger.Debug(r.ctx, fmt.Sprintf("query pipeline ready: queryId=%s query=%s ownerPlugin=%s", r.queryId, r.query.String(), queryPipelinePluginLabel(r.ctx, r.ownerPlugin)))

	for {
		select {
		case response := <-resultChan:
			r.addResponse(response)
		case <-fallbackReadyChan:
			// Consume any already-produced results before checking whether fallback is needed.
			r.drainPendingResults(resultChan)
			r.showFallbackResults()
		case <-doneChan:
			// Run the same fallback check at final completion so queries without any
			// fallback-blocking plugins still get a fallback result when appropriate.
			r.drainPendingResults(resultChan)
			r.showFallbackResults()
			logger.Info(r.ctx, fmt.Sprintf("query done, total results: %d, cost %d ms", r.totalResultCount, util.GetSystemTimestamp()-r.startTimestamp))
			r.resultDebouncer.Done(r.ctx)
			return
		case <-time.After(time.Minute):
			logger.Info(r.ctx, fmt.Sprintf("query timeout, query: %s, request id: %s", r.query.String(), r.request.RequestId))
			r.resultDebouncer.Done(r.ctx)
			responseUIError(r.ctx, r.request, fmt.Sprintf("query timeout, query: %s, request id: %s", r.query.String(), r.request.RequestId))
			return
		}
	}
}

func (r *queryRun) addResponse(response plugin.QueryResponseUI) {
	// QueryContext is backend-owned and remains valid for both global and plugin
	// queries. Refinements and layout stay single-plugin only because global
	// queries aggregate many plugins and one plugin should not control the whole
	// result surface.
	r.latestResponse.Context = response.Context
	if r.ownerPlugin != nil {
		r.latestResponse.Refinements = response.Refinements
		r.latestResponse.Layout = response.Layout
	}
	if len(response.Results) == 0 {
		return
	}

	for index := range response.Results {
		response.Results[index].QueryId = r.queryId
	}
	plugin.GetPluginManager().RecordQueryResultQueryElapsed(r.sessionId, r.queryId, response.Results, util.GetSystemTimestamp()-r.startTimestamp)
	r.totalResultCount += len(response.Results)
	r.resultDebouncer.Add(r.ctx, response.Results)
}

func (r *queryRun) drainPendingResults(resultChan <-chan plugin.QueryResponseUI) {
	// Drain queued results first so fallback does not race ahead of already-finished plugins.
	for {
		select {
		case response := <-resultChan:
			r.addResponse(response)
		default:
			return
		}
	}
}

func (r *queryRun) showFallbackResults() {
	// fallbackReady only means "all fallback-blocking plugins are done". Late
	// debounced plugins may still return afterward, so fallback is shown as an
	// early best-effort option, not as a guarantee that no more real results exist.
	if r.fallbackHandled || r.totalResultCount > 0 {
		return
	}
	r.fallbackHandled = true
	if !r.query.IsGlobalQuery() {
		// Fallback is for global discovery only. Once a trigger keyword has put
		// the query inside one plugin, empty results should stay owned by that
		// plugin instead of falling back to command suggestions.
		logger.Info(r.ctx, "no result, skip fallback for plugin-scoped query")
		return
	}

	fallbackResponse := plugin.GetPluginManager().QueryFallback(r.ctx, r.query, r.ownerPlugin)
	if len(fallbackResponse.Results) > 0 {
		r.addResponse(fallbackResponse)
		logger.Info(r.ctx, fmt.Sprintf("no result yet, show %d fallback results", len(fallbackResponse.Results)))
		return
	}

	logger.Info(r.ctx, "no result, no fallback results")
}

func (r *queryRun) flush(results []plugin.QueryResultUI, reason string) {
	isFinal := reason == "done"
	if !isFinal && len(results) == 0 {
		return
	}

	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, result flushed (reason: %s, isFinal: %v), current: %d, total results: %d", r.query.Type, r.query.String(), reason, isFinal, len(results), r.totalResultCount))
	if len(results) > 0 {
		if r.resultFlushBatch == 0 {
			r.firstVisibleFlushElapsedMs = util.GetSystemTimestamp() - r.startTimestamp
		}
		r.resultFlushBatch++
		plugin.GetPluginManager().RecordQueryResultFlushBatch(r.sessionId, r.queryId, results, r.resultFlushBatch)
	}

	// Bug fix: core query pipelines are concurrent, so backend "current query"
	// state can move while an older or newer pipeline is still flushing. Send
	// the queryId-specific snapshot and let Flutter decide whether it is still
	// the visible query; otherwise the current query can miss its final response
	// and keep the loading indicator alive.
	snapshot := plugin.GetPluginManager().BuildQueryResultsSnapshot(r.sessionId, r.queryId)
	responseSnapshot := snapshot
	if util.IsDev() {
		responseSnapshot = appendQueryDebugTails(r.ctx, r.sessionId, r.queryId, snapshot, r.firstVisibleFlushElapsedMs)
	}
	responseUIQueryResponse(r.ctx, r.request, r.queryId, plugin.QueryResponseUI{
		Results:     responseSnapshot,
		Refinements: r.latestResponse.Refinements,
		Layout:      r.latestResponse.Layout,
		Context:     r.latestResponse.Context,
	}, isFinal)
}
