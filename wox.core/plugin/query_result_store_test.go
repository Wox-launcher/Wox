package plugin

import (
	"context"
	"testing"
	"time"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClearSessionStateLeavesOnlyDestroyedWindow(t *testing.T) {
	logger = util.GetLogger()
	manager := &Manager{
		sessionPluginQueries: util.NewHashMap[string, *sessionPluginQueryState](),
		resultStore:          newQueryResultStore(),
		pluginToolbarMsgIds:  util.NewHashMap[string, string](),
	}
	left := make(chan context.Context, 2)
	instance := &Instance{Metadata: Metadata{Id: "session-owner", Name: "Session Owner"},
		LeavePluginQueryCallbacks: []func(context.Context){func(ctx context.Context) { left <- ctx }}}
	manager.appendPluginInstance(instance)
	for _, session := range []string{"first", "second"} {
		manager.sessionPluginQueries.Store(session, &sessionPluginQueryState{PluginId: instance.Metadata.Id, QueryId: "query-" + session})
	}
	manager.ClearSessionState(context.Background(), "first")
	select {
	case ctx := <-left:
		assert.Equal(t, "first", util.GetContextSessionId(ctx))
		assert.Equal(t, "query-first", util.GetContextQueryId(ctx))
	case <-time.After(time.Second):
		t.Fatal("destroyed window did not leave the active plugin query")
	}
	assert.False(t, manager.isPluginActiveInSession("first", instance.Metadata.Id))
	assert.True(t, manager.isPluginActiveInSession("second", instance.Metadata.Id))
	manager.ClearSessionState(context.Background(), "first")
	select {
	case <-left:
		t.Fatal("duplicate leave callback")
	case <-time.After(30 * time.Millisecond):
	}
}

func newTestManagerWithCachedResult(query Query, result QueryResult) (*Manager, *Instance) {
	manager := &Manager{
		resultStore: newQueryResultStore(),
	}
	pluginInstance := &Instance{
		Metadata: Metadata{
			Id:              "test-plugin",
			TriggerKeywords: []string{"*", "media"},
		},
	}
	sessionQueries := util.NewHashMap[string, *QueryResultSet]()
	resultSet := newQueryResultSet(query)
	resultSet.Results.Store(result.Id, &QueryResultCache{
		Result:         result,
		PluginInstance: pluginInstance,
		Query:          query,
	})
	sessionQueries.Store(query.Id, resultSet)
	manager.resultStore.sessions.Store(query.SessionId, sessionQueries)

	return manager, pluginInstance
}

func TestGetUpdatableResultKeepsExplicitQueryScope(t *testing.T) {
	oldQuery := Query{Id: "old", SessionId: "session"}
	manager, instance := newTestManagerWithCachedResult(oldQuery, QueryResult{Id: "row", Title: "old title"})
	currentQuery := Query{Id: "current", SessionId: "session"}
	current := newQueryResultSet(currentQuery)
	current.Results.Store("row", &QueryResultCache{Query: currentQuery, PluginInstance: instance, Result: QueryResult{Id: "row", Title: "current title"}})
	session, _ := manager.resultStore.sessions.Load("session")
	session.Store("current", current)
	for _, query := range []Query{oldQuery, currentQuery} {
		ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), query.SessionId), query.Id)
		update := manager.GetUpdatableResult(ctx, "row")
		require.NotNil(t, update)
		assert.Equal(t, query.Id+" title", *update.Title)
	}
	for _, query := range []Query{{Id: "missing", SessionId: "session"}, {Id: "current", SessionId: "missing"}} {
		ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), query.SessionId), query.Id)
		assert.Nil(t, manager.GetUpdatableResult(ctx, "row"), "missing scope must not resolve another query's row")
	}
	assert.NotNil(t, manager.GetUpdatableResult(context.Background(), "row"), "unscoped legacy lookup remains supported")
}

func TestTrimHiddenSessionQueryCacheKeepsNewest(t *testing.T) {
	initPluginImageTestLocation(t)
	manager := &Manager{
		resultStore: newQueryResultStore(),
	}
	sessionID := "session-hide"
	oldSet := newQueryResultSet(Query{Id: "q1", SessionId: sessionID})
	oldSet.StartedAt = 100
	oldSet.Results.Store("r1", &QueryResultCache{Result: QueryResult{Id: "r1"}})
	newSet := newQueryResultSet(Query{Id: "q2", SessionId: sessionID})
	newSet.StartedAt = 200
	newSet.Results.Store("r2", &QueryResultCache{Result: QueryResult{Id: "r2"}})
	sessionQueries := util.NewHashMap[string, *QueryResultSet]()
	sessionQueries.Store("q1", oldSet)
	sessionQueries.Store("q2", newSet)
	manager.resultStore.sessions.Store(sessionID, sessionQueries)
	manager.resultStore.lazyResultIcons.Store("old-token", &lazyResultIconEntry{SessionId: sessionID, QueryId: "q1"})
	manager.resultStore.lazyResultIcons.Store("new-token", &lazyResultIconEntry{SessionId: sessionID, QueryId: "q2"})

	manager.TrimHiddenSessionQueryCache(context.Background(), sessionID)

	if sessionQueries.Len() != 1 {
		t.Fatalf("cached queries after hide = %d, want 1", sessionQueries.Len())
	}
	if _, ok := sessionQueries.Load("q2"); !ok {
		t.Fatal("newest query must remain after hide")
	}
	if _, ok := sessionQueries.Load("q1"); ok {
		t.Fatal("older query caches must be dropped on hide")
	}
	if _, ok := manager.resultStore.lazyResultIcons.Load("old-token"); ok {
		t.Fatal("stale lazy icons must be dropped with the old query")
	}
	if _, ok := manager.resultStore.lazyResultIcons.Load("new-token"); !ok {
		t.Fatal("newest query lazy icons must remain")
	}

	manager.TrimHiddenSessionQueryCache(context.Background(), sessionID)
	if sessionQueries.Len() != 1 {
		t.Fatalf("second hide trim changed the kept query set, len=%d", sessionQueries.Len())
	}
}
