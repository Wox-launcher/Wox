package plugin

import (
	"context"
	"fmt"

	"wox/common"
	"wox/util"
)

// ClearSessionState drops query-owned caches after an independent UI session is destroyed.
func (m *Manager) ClearSessionState(ctx context.Context, sessionId string) {
	if sessionId == "" {
		return
	}
	// Closing a launcher also leaves its plugin query. Hosts use the existing
	// leave callbacks to release command processes owned by this window.
	m.HandleQueryLifecycle(ctx, Query{SessionId: sessionId}, nil)
	m.resultStore.clearSession(sessionId)
	m.sessionPluginQueries.Delete(sessionId)
	logger.Info(ctx, fmt.Sprintf("cleared plugin session state: %s", sessionId))
}

// TrimHiddenSessionQueryCache retains the newest query without ending the plugin session.
func (m *Manager) TrimHiddenSessionQueryCache(ctx context.Context, sessionId string) {
	keepQueryId, dropped := m.resultStore.trimHidden(sessionId)
	if dropped > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("trimmed hidden session query cache: session=%s kept=%s dropped=%d", sessionId, keepQueryId, dropped))
	}
}

func (m *Manager) GetSessionIdByQueryId(queryId string) string {
	return m.resultStore.sessionIDForQuery(queryId)
}

func (m *Manager) LoadLazyResultIcon(ctx context.Context, token string) (common.WoxImage, error) {
	return m.resultStore.loadLazyIcon(ctx, token)
}

func (m *Manager) RecordQueryResultQueryElapsed(sessionId string, queryId string, results []QueryResultUI, elapsedMs int64, batchQueueElapsedMs float64) {
	m.resultStore.recordQueryElapsed(sessionId, queryId, results, elapsedMs, batchQueueElapsedMs)
}

func (m *Manager) RecordQueryResultPluginQueryElapsed(sessionId string, queryId string, resultId string, elapsedMs int64) {
	m.resultStore.recordPluginQueryElapsed(sessionId, queryId, resultId, elapsedMs)
}

func (m *Manager) RecordQueryResultFlushBatch(sessionId string, queryId string, results []QueryResultUI, batch int) {
	m.resultStore.recordFlushBatch(sessionId, queryId, results, batch)
}

func (m *Manager) GetQueryResultDebugInfo(sessionId string, queryId string, resultId string) (batch int, queryElapsed int64, batchQueueElapsed float64, batchQueueElapsedSet bool, pluginQueryElapsed int64, pluginQueryElapsedSet bool, ok bool) {
	return m.resultStore.debugInfo(sessionId, queryId, resultId)
}

// PluginIDForCachedResult returns the plugin that produced a currently cached result.
func (m *Manager) PluginIDForCachedResult(sessionId string, queryId string, resultId string) string {
	resultCache, found := m.resultStore.findInSession(sessionId, queryId, resultId)
	if !found || resultCache == nil || resultCache.PluginInstance == nil {
		return ""
	}
	return resultCache.PluginInstance.Metadata.Id
}

func (m *Manager) GetResultSessionId(resultId string) string {
	resultCache, found := m.resultStore.findByID(resultId)
	if !found {
		return ""
	}
	return resultCache.Query.SessionId
}

func (m *Manager) GetQueryInfoByResultId(resultId string) (string, string) {
	resultCache, found := m.resultStore.findByID(resultId)
	if !found {
		return "", ""
	}
	return resultCache.Query.SessionId, resultCache.Query.Id
}
