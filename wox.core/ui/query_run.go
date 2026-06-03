package ui

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/plugin"
	"wox/util"
	"wox/util/timetracking"
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
	// acceptedResultIds keeps the UI snapshot aligned with responses queryRun has actually received.
	acceptedResultIds []string
	// acceptedResultIdSet prevents duplicate ids from expanding filtered snapshots.
	acceptedResultIdSet map[string]struct{}
	// acceptedResultMux protects accepted result ids because debounced flushes run concurrently with addResponse.
	acceptedResultMux sync.Mutex
	// latestResponse keeps the newest context, layout, and refinement metadata for the next flush.
	latestResponse plugin.QueryResponseUI
	// fallbackHandled prevents duplicate fallback checks after fallbackReady and done both fire.
	fallbackHandled bool
	// resultDebouncer batches plugin results before flushing snapshots back to Flutter.
	resultDebouncer *util.Debouncer[plugin.QueryResultUI]
}

func newQueryRun(ctx context.Context, request WebsocketMsg, query plugin.Query, ownerPlugin *plugin.Instance) *queryRun {
	return &queryRun{
		ctx:                 ctx,
		request:             request,
		sessionId:           request.SessionId,
		queryId:             query.Id,
		query:               query,
		ownerPlugin:         ownerPlugin,
		acceptedResultIdSet: map[string]struct{}{},
		latestResponse: plugin.QueryResponseUI{
			Context: plugin.BuildQueryContext(query, ownerPlugin),
		},
	}
}

func (r *queryRun) start() {
	r.startTimestamp = util.GetSystemTimestamp()
	r.firstFlushDelayMs = plugin.GetPluginManager().GetQueryFirstFlushDelayMs(r.query)
	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, first flush delay: %d ms", r.query.Type, r.query.String(), r.firstFlushDelayMs))
	if tracker := timetracking.New("query_run_start"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetRawString("query", r.query.String())
		tracker.SetInt64("firstFlushDelayMs", r.firstFlushDelayMs)
		tracker.Log(r.ctx)
	}

	debouncerStart := util.GetSystemTimestamp()
	r.resultDebouncer = util.NewDebouncer(r.firstFlushDelayMs, resultDebounceIntervalMs, r.flush)
	r.resultDebouncer.Start(r.ctx)
	if tracker := timetracking.New("debouncer_start"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-debouncerStart)
		tracker.Log(r.ctx)
	}
	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, result flushed (new start)", r.query.Type, r.query.String()))

	// Bug diagnostics: Manager.Query starts plugin work and returns the result
	// channels used by the select loop below. If a future log has "query pipeline
	// starting" but not "ready", the stall is inside scheduler setup rather than
	// UI result handling.
	managerQueryStart := util.GetSystemTimestamp()
	if tracker := timetracking.New("manager_query_call"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetRawString("query", r.query.String())
		tracker.SetRawString("ownerPlugin", queryPipelinePluginLabel(r.ctx, r.ownerPlugin))
		tracker.Log(r.ctx)
	}
	resultChan, fallbackReadyChan, doneChan := plugin.GetPluginManager().Query(r.ctx, r.query)
	if tracker := timetracking.New("manager_query_return"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetRawString("query", r.query.String())
		tracker.SetRawString("ownerPlugin", queryPipelinePluginLabel(r.ctx, r.ownerPlugin))
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-managerQueryStart)
		tracker.Log(r.ctx)
	}

	for {
		select {
		case response := <-resultChan:
			r.addResponse(response)
		case <-fallbackReadyChan:
			if tracker := timetracking.New("fallback_ready"); tracker.Enabled() {
				tracker.SetRawString("queryId", r.queryId)
				tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-r.startTimestamp)
				tracker.SetInt("totalResults", r.totalResultCount)
				tracker.Log(r.ctx)
			}
			// Consume any already-produced results before checking whether fallback is needed.
			r.drainPendingResults(resultChan)
			r.showFallbackResults()
		case <-doneChan:
			if tracker := timetracking.New("done_signal"); tracker.Enabled() {
				tracker.SetRawString("queryId", r.queryId)
				tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-r.startTimestamp)
				tracker.SetInt("totalResults", r.totalResultCount)
				tracker.Log(r.ctx)
			}
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
	receivedElapsed := util.GetSystemTimestamp() - r.startTimestamp
	if tracker := timetracking.New("query_run_receive"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt64("elapsedMs", receivedElapsed)
		tracker.Log(r.ctx)
	}

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
	r.recordAcceptedResultIds(response.Results)
	recordStart := util.GetSystemTimestamp()
	plugin.GetPluginManager().RecordQueryResultQueryElapsed(r.sessionId, r.queryId, response.Results, receivedElapsed)
	if tracker := timetracking.New("record_query_elapsed"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-recordStart)
		tracker.Log(r.ctx)
	}
	r.totalResultCount += len(response.Results)
	addStart := util.GetSystemTimestamp()
	r.resultDebouncer.Add(r.ctx, response.Results)
	if tracker := timetracking.New("debouncer_add"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt("totalResults", r.totalResultCount)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-addStart)
		tracker.Log(r.ctx)
	}
}

// recordAcceptedResultIds marks results that have crossed the queryRun receive boundary.
func (r *queryRun) recordAcceptedResultIds(results []plugin.QueryResultUI) {
	r.acceptedResultMux.Lock()
	defer r.acceptedResultMux.Unlock()

	for _, result := range results {
		if result.Id == "" {
			continue
		}
		if _, exists := r.acceptedResultIdSet[result.Id]; exists {
			continue
		}
		r.acceptedResultIdSet[result.Id] = struct{}{}
		r.acceptedResultIds = append(r.acceptedResultIds, result.Id)
	}
}

// acceptedResultSnapshotIds returns a stable copy for concurrent debounced flushes.
func (r *queryRun) acceptedResultSnapshotIds() []string {
	r.acceptedResultMux.Lock()
	defer r.acceptedResultMux.Unlock()

	ids := make([]string, len(r.acceptedResultIds))
	copy(ids, r.acceptedResultIds)
	return ids
}

func (r *queryRun) drainPendingResults(resultChan <-chan plugin.QueryResponseUI) {
	// Drain queued results first so fallback does not race ahead of already-finished plugins.
	for {
		select {
		case response := <-resultChan:
			if tracker := timetracking.New("drain_pending"); tracker.Enabled() {
				tracker.SetRawString("queryId", r.queryId)
				tracker.SetInt("resultCount", len(response.Results))
				tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-r.startTimestamp)
				tracker.Log(r.ctx)
			}
			r.addResponse(response)
		default:
			return
		}
	}
}

func (r *queryRun) showFallbackResults() {
	if tracker := timetracking.New("fallback_check"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetBool("handled", r.fallbackHandled)
		tracker.SetInt("totalResults", r.totalResultCount)
		tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-r.startTimestamp)
		tracker.Log(r.ctx)
	}

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
	flushStart := util.GetSystemTimestamp()
	isFinal := reason == "done"
	if !isFinal && len(results) == 0 {
		return
	}

	logger.Info(r.ctx, fmt.Sprintf("query %s: %s, result flushed (reason: %s, isFinal: %v), current: %d, total results: %d", r.query.Type, r.query.String(), reason, isFinal, len(results), r.totalResultCount))
	if tracker := timetracking.New("flush_start"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetRawString("reason", reason)
		tracker.SetBool("isFinal", isFinal)
		tracker.SetInt("pendingCount", len(results))
		tracker.SetInt("totalResults", r.totalResultCount)
		tracker.SetInt64("elapsedMs", flushStart-r.startTimestamp)
		tracker.SetInt("batchBefore", r.resultFlushBatch)
		tracker.Log(r.ctx)
	}
	if len(results) > 0 {
		if r.resultFlushBatch == 0 {
			r.firstVisibleFlushElapsedMs = util.GetSystemTimestamp() - r.startTimestamp
		}
		r.resultFlushBatch++
		recordBatchStart := util.GetSystemTimestamp()
		plugin.GetPluginManager().RecordQueryResultFlushBatch(r.sessionId, r.queryId, results, r.resultFlushBatch)
		if tracker := timetracking.New("record_flush_batch"); tracker.Enabled() {
			tracker.SetRawString("queryId", r.queryId)
			tracker.SetInt("batch", r.resultFlushBatch)
			tracker.SetInt("resultCount", len(results))
			tracker.SetInt64("firstVisibleFlushElapsedMs", r.firstVisibleFlushElapsedMs)
			tracker.SetInt64("costMs", util.GetSystemTimestamp()-recordBatchStart)
			tracker.Log(r.ctx)
		}
	}

	// Bug fix: core query pipelines are concurrent, so backend "current query"
	// state can move while an older or newer pipeline is still flushing. Send
	// the queryId-specific snapshot and let Flutter decide whether it is still
	// the visible query; otherwise the current query can miss its final response
	// and keep the loading indicator alive.
	//
	// Snapshot visibility must still stay behind the queryRun receive boundary.
	// PolishResult writes to the query cache before the plugin response is sent
	// through resultChan, so a full cache snapshot can expose results before
	// their response elapsed time and flush batch have been recorded.
	snapshotStart := util.GetSystemTimestamp()
	snapshot := plugin.GetPluginManager().BuildQueryResultsSnapshotForResultIds(r.sessionId, r.queryId, r.acceptedResultSnapshotIds())
	if tracker := timetracking.New("build_snapshot"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt("snapshotCount", len(snapshot))
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-snapshotStart)
		tracker.Log(r.ctx)
	}
	responseSnapshot := snapshot
	if util.IsDev() {
		debugTailStart := util.GetSystemTimestamp()
		responseSnapshot = appendQueryDebugTails(r.ctx, r.sessionId, r.queryId, snapshot, r.firstVisibleFlushElapsedMs)
		if tracker := timetracking.New("append_debug_tails"); tracker.Enabled() {
			tracker.SetRawString("queryId", r.queryId)
			tracker.SetInt("snapshotCount", len(snapshot))
			tracker.SetInt("responseCount", len(responseSnapshot))
			tracker.SetInt64("costMs", util.GetSystemTimestamp()-debugTailStart)
			tracker.Log(r.ctx)
		}
	}
	sendStart := util.GetSystemTimestamp()
	responseUIQueryResponse(r.ctx, r.request, r.queryId, plugin.QueryResponseUI{
		Results:     responseSnapshot,
		Refinements: r.latestResponse.Refinements,
		Layout:      r.latestResponse.Layout,
		Context:     r.latestResponse.Context,
	}, isFinal)
	if tracker := timetracking.New("send_ui_response"); tracker.Enabled() {
		tracker.SetRawString("queryId", r.queryId)
		tracker.SetInt("responseCount", len(responseSnapshot))
		tracker.SetBool("isFinal", isFinal)
		tracker.SetInt64("sendCostMs", util.GetSystemTimestamp()-sendStart)
		tracker.SetInt64("flushCostMs", util.GetSystemTimestamp()-flushStart)
		tracker.Log(r.ctx)
	}
}
