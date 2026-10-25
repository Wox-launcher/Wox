package plugin

import (
	"context"
	"sort"
	"sync"

	"wox/util"
)

// queryResultStore owns query-scoped result sets and their deferred image tokens.
// Returned cache entries keep their existing shared-pointer semantics; presentation
// and patch processing still run in the manager without introducing new locks.
type queryResultStore struct {
	sessions        *util.HashMap[string, *util.HashMap[string, *QueryResultSet]]
	lazyResultIcons *util.HashMap[string, *lazyResultIconEntry]
}

const maxCachedQueriesPerSession = 32

// newQueryResultStore initializes both maps before any query or image work starts.
func newQueryResultStore() *queryResultStore {
	return &queryResultStore{
		sessions:        util.NewHashMap[string, *util.HashMap[string, *QueryResultSet]](),
		lazyResultIcons: util.NewHashMap[string, *lazyResultIconEntry](),
	}
}

// clearSession releases results and tokens together, without invoking plugin callbacks.
func (s *queryResultStore) clearSession(sessionId string) {
	s.sessions.Delete(sessionId)
	s.clearLazyIconsExcept(sessionId, "")
}

type QueryResultCache struct {
	Result         QueryResult // store the full QueryResult including actions with callbacks
	PluginInstance *Instance
	Query          Query
	Layout         QueryLayout // query layout used when polishing this result, so later updates keep the same surface sizing
	// FlushBatch is the debouncer batch that first sent this result in a visible response.
	FlushBatch int
	// BatchQueueElapsed is the elapsed time when queryRun made this result ready for the debouncer queue.
	BatchQueueElapsed    float64
	BatchQueueElapsedSet bool
	// QueryElapsed is the elapsed time when queryRun received the plugin response, measured from the end-to-end query start.
	QueryElapsed    int64
	QueryElapsedSet bool
	// PluginQueryElapsed is only the raw Plugin.Query duration, excluding manager polish and UI conversion.
	PluginQueryElapsed    int64
	PluginQueryElapsedSet bool
	// SourceMRUHash is the MRU identity fixed when the result is cached.
	// Restored rows retain their saved identity instead of hashing the alias query.
	SourceMRUHash string
	// AliasMatchKind ranks alias-restored rows above ordinary plugin results.
	AliasMatchKind aliasMatchKind
}

// QueryResultSet retains one query independently of later queries in the session.
type QueryResultSet struct {
	// storeMu makes alias precedence atomic when plugin responses arrive concurrently.
	storeMu   sync.Mutex
	Query     Query
	StartedAt int64
	Results   *util.HashMap[string, *QueryResultCache]
}

// newQueryResultSet creates the per-query result map and records its retention age.
func newQueryResultSet(query Query) *QueryResultSet {
	set := &QueryResultSet{
		Query:     query,
		StartedAt: util.GetSystemTimestamp(),
		Results:   util.NewHashMap[string, *QueryResultCache](),
	}
	return set
}

// startQuery publishes a separate result set before expiring older image tokens.
func (s *queryResultStore) startQuery(query Query) {
	if query.Id == "" {
		return
	}

	sessionQueries, _ := s.sessions.LoadOrStore(query.SessionId, util.NewHashMap[string, *QueryResultSet]())

	// A UI session can have multiple backend query pipelines in flight because UI
	// requests and plugin responses are handled concurrently.
	// Store every query under its own query id so a late old query cannot erase
	// the result cache required to send the newer query's final response.
	sessionQueries.Store(query.Id, newQueryResultSet(query))
	s.clearLazyIconsExcept(query.SessionId, query.Id)
	s.pruneQueries(sessionQueries, query.Id)
}

// pruneQueries bounds retained queries while protecting the query just started.
func (s *queryResultStore) pruneQueries(sessionQueries *util.HashMap[string, *QueryResultSet], keepQueryId string) {
	if sessionQueries.Len() <= maxCachedQueriesPerSession {
		return
	}

	type cachedQueryEntry struct {
		queryId   string
		startedAt int64
	}
	var entries []cachedQueryEntry
	sessionQueries.Range(func(queryId string, set *QueryResultSet) bool {
		if queryId != keepQueryId {
			entries = append(entries, cachedQueryEntry{queryId: queryId, startedAt: set.StartedAt})
		}
		return true
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].startedAt < entries[j].startedAt
	})

	// Query caches are now keyed by query id, so without a small cap every typed
	// character would leave a completed result set behind. Keep the newest sets
	// plus the query that was just started; old non-current responses are still
	// safe to drop because UI ignores them by query id.
	for sessionQueries.Len() > maxCachedQueriesPerSession && len(entries) > 0 {
		if entries[0].queryId == keepQueryId {
			entries = entries[1:]
			continue
		}
		sessionQueries.Delete(entries[0].queryId)
		entries = entries[1:]
	}
}

// getQuery only resolves the requested query within its session.
func (s *queryResultStore) getQuery(sessionId string, queryId string) (*QueryResultSet, bool) {
	if queryId == "" {
		return nil, false
	}
	sessionQueries, ok := s.sessions.Load(sessionId)
	if !ok {
		return nil, false
	}
	return sessionQueries.Load(queryId)
}

// storeResult drops late writes to evicted queries and preserves alias precedence.
func (s *queryResultStore) storeResult(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, resultOriginal QueryResult, sourceHash string, match aliasMatchKind) {
	if query.Id == "" {
		util.GetLogger().Warn(ctx, "query id is empty, skip result cache")
		return
	}
	if query.SessionId == "" {
		util.GetLogger().Warn(ctx, "query session id is empty, skip result cache")
		return
	}

	set, ok := s.getQuery(query.SessionId, query.Id)
	if !ok {
		return
	}

	set.storeMu.Lock()
	defer set.storeMu.Unlock()
	// A plugin can return the same stable result ID after its alias restore.
	// Keep the restored row and its priority regardless of completion order.
	if existing, found := set.Results.Load(resultOriginal.Id); found && existing != nil && existing.AliasMatchKind != aliasMatchNone && match == aliasMatchNone {
		return
	}
	cache := &QueryResultCache{
		Result:         resultOriginal,
		PluginInstance: pluginInstance,
		Query:          query,
		Layout:         layout,
	}
	cache.SourceMRUHash = sourceHash
	cache.AliasMatchKind = match
	set.Results.Store(resultOriginal.Id, cache)

}

// findInSession only scans other queries when the caller omits the query ID.
func (s *queryResultStore) findInSession(sessionId string, queryId string, resultId string) (*QueryResultCache, bool) {
	if resultId == "" {
		return nil, false
	}
	if queryId != "" {
		set, found := s.getQuery(sessionId, queryId)
		if !found {
			return nil, false
		}
		resultCache, found := set.Results.Load(resultId)
		if !found {
			return nil, false
		}
		return resultCache, true
	}

	sessionQueries, found := s.sessions.Load(sessionId)
	if !found {
		return nil, false
	}
	var foundCache *QueryResultCache
	sessionQueries.Range(func(_ string, set *QueryResultSet) bool {
		resultCache, found := set.Results.Load(resultId)
		if !found {
			return true
		}
		foundCache = resultCache
		return false
	})
	if foundCache == nil {
		return nil, false
	}
	return foundCache, true
}

// findByID preserves the unscoped lookup used by legacy plugin calls.
func (s *queryResultStore) findByID(resultId string) (*QueryResultCache, bool) {
	if resultId == "" {
		return nil, false
	}

	var foundCache *QueryResultCache
	s.sessions.Range(func(_ string, sessionQueries *util.HashMap[string, *QueryResultSet]) bool {
		sessionQueries.Range(func(_ string, set *QueryResultSet) bool {
			resultCache, found := set.Results.Load(resultId)
			if !found {
				return true
			}
			foundCache = resultCache
			return false
		})
		if foundCache == nil {
			return true
		}
		return false
	})

	if foundCache == nil {
		return nil, false
	}

	return foundCache, true
}

// findWithContext keeps explicit session/query misses from escaping their scope.
func (s *queryResultStore) findWithContext(ctx context.Context, resultId string) (*QueryResultCache, bool) {
	sessionId := util.GetContextSessionId(ctx)
	queryId := util.GetContextQueryId(ctx)
	if sessionId != "" {
		resultCache, ok := s.findInSession(sessionId, queryId, resultId)
		// An explicit query scope must never fall back to another query that
		// happens to reuse the same stable result id.
		if ok || queryId != "" {
			return resultCache, ok
		}
	}
	return s.findByID(resultId)
}

// sessionIDForQuery resolves the owner without changing query retention.
func (s *queryResultStore) sessionIDForQuery(queryId string) string {
	if queryId == "" {
		return ""
	}
	var sessionId string
	s.sessions.Range(func(candidateSessionId string, sessionQueries *util.HashMap[string, *QueryResultSet]) bool {
		if _, ok := sessionQueries.Load(queryId); ok {
			sessionId = candidateSessionId
			return false
		}
		return true
	})
	return sessionId
}

// recordQueryElapsed stores backend receive and batch queue timings for debug tails.
func (s *queryResultStore) recordQueryElapsed(sessionId string, queryId string, results []QueryResultUI, elapsedMs int64, batchQueueElapsedMs float64) {
	if sessionId == "" || queryId == "" || len(results) == 0 {
		return
	}

	set, found := s.getQuery(sessionId, queryId)
	if !found {
		return
	}

	for _, result := range results {
		if result.Id == "" {
			continue
		}

		resultCache, ok := set.Results.Load(result.Id)
		if !ok {
			continue
		}
		if !resultCache.QueryElapsedSet {
			resultCache.QueryElapsed = elapsedMs
			resultCache.QueryElapsedSet = true
		}

		if !resultCache.BatchQueueElapsedSet {
			resultCache.BatchQueueElapsed = batchQueueElapsedMs
			resultCache.BatchQueueElapsedSet = true
		}
	}
}

// recordPluginQueryElapsed stores the raw Plugin.Query duration for debug tails.
func (s *queryResultStore) recordPluginQueryElapsed(sessionId string, queryId string, resultId string, elapsedMs int64) {
	if sessionId == "" || queryId == "" || resultId == "" {
		return
	}

	resultCache, found := s.findInSession(sessionId, queryId, resultId)
	if !found {
		return
	}
	if resultCache.PluginQueryElapsedSet {
		return
	}

	resultCache.PluginQueryElapsed = elapsedMs
	resultCache.PluginQueryElapsedSet = true
}

// recordFlushBatch records only the first visible delivery batch.
func (s *queryResultStore) recordFlushBatch(sessionId string, queryId string, results []QueryResultUI, batch int) {
	if sessionId == "" || queryId == "" || len(results) == 0 || batch <= 0 {
		return
	}

	set, found := s.getQuery(sessionId, queryId)
	if !found {
		return
	}

	for _, result := range results {
		if result.Id == "" {
			continue
		}

		resultCache, ok := set.Results.Load(result.Id)
		if !ok {
			continue
		}
		if resultCache.FlushBatch > 0 {
			continue
		}

		resultCache.FlushBatch = batch
	}
}

// debugInfo exposes timings only after a result has been delivered.
func (s *queryResultStore) debugInfo(sessionId string, queryId string, resultId string) (batch int, queryElapsed int64, batchQueueElapsed float64, batchQueueElapsedSet bool, pluginQueryElapsed int64, pluginQueryElapsedSet bool, ok bool) {
	resultCache, found := s.findInSession(sessionId, queryId, resultId)
	if !found {
		return 0, 0, 0, false, 0, false, false
	}
	if resultCache.FlushBatch <= 0 || !resultCache.QueryElapsedSet {
		return 0, 0, 0, false, 0, false, false
	}

	return resultCache.FlushBatch, resultCache.QueryElapsed, resultCache.BatchQueueElapsed, resultCache.BatchQueueElapsedSet, resultCache.PluginQueryElapsed, resultCache.PluginQueryElapsedSet, true
}

// newestCachedQueryID returns the query id with the latest StartedAt in a session.
func newestCachedQueryID(sessionQueries *util.HashMap[string, *QueryResultSet]) string {
	var newestID string
	var newestAt int64
	sessionQueries.Range(func(queryId string, set *QueryResultSet) bool {
		if set == nil {
			return true
		}
		if newestID == "" || set.StartedAt >= newestAt {
			newestID = queryId
			newestAt = set.StartedAt
		}
		return true
	})
	return newestID
}

// trimHidden keeps only the newest query result set after hide.
// Each typed character starts a new query id, so a 32-entry cap would otherwise
// retain many complete result snapshots (including action closures and icon
// strings) until the next query starts.
func (s *queryResultStore) trimHidden(sessionId string) (string, int) {
	sessionQueries, ok := s.sessions.Load(sessionId)
	if !ok || sessionQueries.Len() <= 1 {
		return "", 0
	}

	keepQueryId := newestCachedQueryID(sessionQueries)
	if keepQueryId == "" {
		return "", 0
	}

	var dropped []string
	sessionQueries.Range(func(queryId string, _ *QueryResultSet) bool {
		if queryId != keepQueryId {
			dropped = append(dropped, queryId)
		}
		return true
	})
	for _, queryId := range dropped {
		sessionQueries.Delete(queryId)
	}
	s.clearLazyIconsExcept(sessionId, keepQueryId)
	return keepQueryId, len(dropped)
}
