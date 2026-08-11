package plugin

import (
	"context"
	"testing"
	"time"
	"wox/common"
	"wox/setting"
)

func TestAutoQueryHistoryRecorderRecordsStableQueryForCompletion(t *testing.T) {
	recorded := make(chan common.PlainQuery, 1)
	recorder := newAutoQueryHistoryRecorder(10*time.Millisecond, func(ctx context.Context, query common.PlainQuery) {
		recorded <- query
	})
	query := Query{Id: "query", SessionId: "session", Type: QueryTypeInput, RawQuery: "time in ny"}

	recorder.beginQuery(query)
	response := QueryResponse{
		Results:                []QueryResult{{Title: "12:24"}},
		AutoRecordQueryHistory: true,
	}
	recorder.schedule(context.Background(), query, response)
	recorder.schedule(context.Background(), query, response)

	var history common.PlainQuery
	select {
	case history = <-recorded:
	case <-time.After(time.Second):
		t.Fatal("stable display query was not recorded")
	}
	if history.QueryText != "time in ny" || history.QueryType != QueryTypeInput {
		t.Fatalf("recorded query = %#v", history)
	}
	select {
	case duplicate := <-recorded:
		t.Fatalf("recorded duplicate query: %#v", duplicate)
	case <-time.After(30 * time.Millisecond):
	}

	prefix, _ := newQueryInputWithPlugins("time in", getFakePluginInstances())
	hint := BuildQueryCompletionHint(prefix, nil, []setting.QueryHistory{{Query: history, Timestamp: 1}})
	if hint == nil || hint.CompletionText != "time in ny" || hint.Suffix != " ny" {
		t.Fatalf("completion hint = %#v", hint)
	}
}

func TestAutoQueryHistoryRecorderCancelsOlderQuery(t *testing.T) {
	recorded := make(chan common.PlainQuery, 1)
	recorder := newAutoQueryHistoryRecorder(20*time.Millisecond, func(ctx context.Context, query common.PlainQuery) {
		recorded <- query
	})
	oldQuery := Query{Id: "old", SessionId: "session", Type: QueryTypeInput, RawQuery: "time in ny"}
	newQuery := Query{Id: "new", SessionId: "session", Type: QueryTypeInput, RawQuery: "time in lon"}
	response := QueryResponse{Results: []QueryResult{{Title: "time"}}, AutoRecordQueryHistory: true}

	recorder.beginQuery(oldQuery)
	recorder.schedule(context.Background(), oldQuery, response)
	recorder.beginQuery(newQuery)
	// A response from the old plugin pipeline must not revive the cancelled query.
	recorder.schedule(context.Background(), oldQuery, response)
	recorder.schedule(context.Background(), newQuery, response)

	select {
	case query := <-recorded:
		if query.QueryText != newQuery.RawQuery {
			t.Fatalf("recorded stale query %q", query.QueryText)
		}
	case <-time.After(time.Second):
		t.Fatal("latest display query was not recorded")
	}
	select {
	case query := <-recorded:
		t.Fatalf("recorded an extra query: %#v", query)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestAutoQueryHistoryRecorderRejectsIneligibleResponses(t *testing.T) {
	recorded := make(chan common.PlainQuery, 1)
	recorder := newAutoQueryHistoryRecorder(5*time.Millisecond, func(ctx context.Context, query common.PlainQuery) {
		recorded <- query
	})
	tests := []struct {
		name     string
		query    Query
		response QueryResponse
	}{
		{name: "not opted in", query: Query{Id: "one", SessionId: "one", Type: QueryTypeInput, RawQuery: "1+1"}, response: QueryResponse{Results: []QueryResult{{Title: "2"}}}},
		{name: "empty results", query: Query{Id: "two", SessionId: "two", Type: QueryTypeInput, RawQuery: "1+1"}, response: QueryResponse{AutoRecordQueryHistory: true}},
		{name: "selection query", query: Query{Id: "three", SessionId: "three", Type: QueryTypeSelection, RawQuery: "1+1"}, response: QueryResponse{Results: []QueryResult{{Title: "2"}}, AutoRecordQueryHistory: true}},
		{name: "blank input", query: Query{Id: "four", SessionId: "four", Type: QueryTypeInput, RawQuery: "   "}, response: QueryResponse{Results: []QueryResult{{Title: "2"}}, AutoRecordQueryHistory: true}},
		{name: "no session", query: Query{Id: "five", Type: QueryTypeInput, RawQuery: "1+1"}, response: QueryResponse{Results: []QueryResult{{Title: "2"}}, AutoRecordQueryHistory: true}},
		{name: "no query id", query: Query{SessionId: "six", Type: QueryTypeInput, RawQuery: "1+1"}, response: QueryResponse{Results: []QueryResult{{Title: "2"}}, AutoRecordQueryHistory: true}},
	}

	for _, test := range tests {
		recorder.beginQuery(test.query)
		recorder.schedule(context.Background(), test.query, test.response)
	}
	select {
	case query := <-recorded:
		t.Fatalf("recorded ineligible query: %#v", query)
	case <-time.After(30 * time.Millisecond):
	}
}
