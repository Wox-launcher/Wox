package plugin

import (
	"context"
	"encoding/json"
	"testing"

	"wox/util"
)

// TestWoxViewPreviewEventScope protects callbacks from other queries and replaced documents.
func TestWoxViewPreviewEventScope(t *testing.T) {
	manager := &Manager{resultStore: newQueryResultStore()}
	query := Query{SessionId: "session", Id: "query"}
	manager.resultStore.startQuery(query)
	set, _ := manager.resultStore.getQuery(query.SessionId, query.Id)
	called := 0
	result := QueryResult{Id: "row", Preview: WoxPreview{PreviewType: WoxPreviewTypeWoxView, WoxViewID: "doc", OnWoxViewEvent: func(ctx context.Context, event WoxViewPreviewEvent) {
		if util.GetContextSessionId(ctx) != "session" || util.GetContextQueryId(ctx) != "query" || event.Handler != "click" || string(event.Args) != `[1]` {
			t.Errorf("unexpected callback context or data: %+v", event)
		}
		called++
	}}}
	cache := &QueryResultCache{Result: result, PluginInstance: &Instance{}, Query: query}
	cache.previewSnapshot.Store(&result.Preview)
	set.Results.Store("row", cache)
	event := WoxViewPreviewEvent{Handler: "click", Args: json.RawMessage(`[1]`)}
	for _, scope := range [][4]string{{"other", "query", "row", "doc"}, {"session", "other", "row", "doc"}, {"session", "query", "other", "doc"}, {"session", "query", "row", "old"}} {
		if err := manager.DispatchWoxViewPreviewEvent(context.Background(), scope[0], scope[1], scope[2], scope[3], event); err == nil {
			t.Fatalf("accepted stale scope %v", scope)
		}
	}
	if err := manager.DispatchWoxViewPreviewEvent(context.Background(), "session", "query", "row", "doc", event); err != nil || called != 1 {
		t.Fatalf("current callback: count=%d error=%v", called, err)
	}
	manager.resultStore.clearSession("session")
	if err := manager.DispatchWoxViewPreviewEvent(context.Background(), "session", "query", "row", "doc", event); err == nil {
		t.Fatal("accepted event after query release")
	}
}
