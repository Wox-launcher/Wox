package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wox/common"
)

// TestDeclaredDebounceLevels exercises real plugin.json loading, query parsing and scheduling precedence.
func TestDeclaredDebounceLevels(t *testing.T) {
	directory := t.TempDir()
	payload := `{"Id":"search","Runtime":"nodejs","TriggerKeywords":["web","fast","inherit","*"],
		"Features":[{"Name":"debounce","Params":{"IntervalMs":200}},{"Name":"querySelection"}],
		"TriggerDebounceMs":{"web":500,"fast":0},
		"Commands":[{"Command":"remote","DebounceMs":800},{"Command":"now","DebounceMs":0},{"Command":"default"}]}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(payload), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := (&pluginLifecycle{}).ParseMetadata(context.Background(), directory)
	if err != nil {
		t.Fatal(err)
	}
	instance := &Instance{Metadata: metadata}
	manager := &Manager{lifecycle: pluginLifecycle{instances: []*Instance{instance}}}
	check := func(query Query, want int) {
		t.Helper()
		jobs, _, _ := manager.buildQueryPlan(context.Background(), query)
		if len(jobs) != 1 || jobs[0].intervalMs != want || jobs[0].debounced != (want > 0) || jobs[0].blocksFallback != (want == 0) {
			t.Fatalf("query=%+v jobs=%+v, want %d ms", query, jobs, want)
		}
	}
	for _, test := range []struct {
		text string
		want int
	}{
		{"web words", 500}, {"fast words", 0}, {"inherit words", 200}, {"global words", 200},
		{"web remote words", 800}, {"fast remote words", 800}, {"web now words", 0},
		{"web default words", 500}, {"inherit default words", 200},
		{"web remote", 500}, // Command policy starts only once the query parser recognizes the command.
	} {
		query, _ := newQueryInputWithPlugins(test.text, []*Instance{instance})
		check(query, test.want)
	}
	delay := 600
	manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "web", DebounceMs: &delay})
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web"}, 600)
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web", Command: "remote"}, 800)
	manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "web"})
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web"}, 500)
	instance.unregisterTriggerKeyword("web")
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web"}, 500)
	check(Query{Type: QueryTypeSelection, TriggerKeyword: "web", Command: "remote"}, 200)
	check(Query{Type: QueryTypeInput, Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "search", Command: "remote"}}}}, 800)
	check(Query{Type: QueryTypeInput, Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "search"}}}}, 500)

	api := &APIImpl{pluginInstance: instance}
	api.RegisterQueryCommands(context.Background(), []MetadataCommand{{Command: "live", DebounceMs: &delay}})
	delay = 900
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web", Command: "live"}, 600)
	invalid := -1
	api.RegisterQueryCommands(context.Background(), []MetadataCommand{{Command: "live", DebounceMs: &invalid}})
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web", Command: "live"}, 600)
	api.RegisterQueryCommands(context.Background(), nil)
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web", Command: "live"}, 500)
	instance.Metadata.Features[0].Params["IntervalMs"] = 0
	check(Query{Type: QueryTypeInput, TriggerKeyword: "inherit"}, 0)
	instance.Metadata.Features = nil
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web"}, 500)
	check(Query{Type: QueryTypeInput, TriggerKeyword: "web", Command: "remote"}, 800)
}

// TestQueryDebounceOverride checks keyword routing, metadata inheritance and explicit immediate execution.
func TestQueryDebounceOverride(t *testing.T) {
	instance := &Instance{Metadata: Metadata{Id: "search", TriggerKeywords: []string{"*"}, Features: []MetadataFeature{
		{Name: MetadataFeatureDebounce, Params: map[string]any{"IntervalMs": 200}},
		{Name: MetadataFeatureQuerySelection},
	}}}
	manager := &Manager{lifecycle: pluginLifecycle{instances: []*Instance{instance}}}
	delay, immediate, invalid := 500, 0, -1
	for _, option := range []RegisterTriggerKeywordOption{{Keyword: "web", DebounceMs: &delay}, {Keyword: "browser", DebounceMs: &immediate}, {Keyword: "inherit"}} {
		if !manager.registerTriggerKeyword(instance, option) {
			t.Fatal("registration failed")
		}
	}
	delay = 900 // Registration must own a snapshot, not the caller's pointer.
	if manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "web", DebounceMs: &invalid}) {
		t.Fatal("negative interval accepted")
	}
	for _, test := range []struct {
		keyword string
		kind    QueryType
		want    int
	}{
		{"web", QueryTypeInput, 500}, {"browser", QueryTypeInput, 0},
		{"inherit", QueryTypeInput, 200}, {"", QueryTypeInput, 200},
		{"web", QueryTypeSelection, 200},
	} {
		jobs, _, _ := manager.buildQueryPlan(context.Background(), Query{Type: test.kind, TriggerKeyword: test.keyword})
		if len(jobs) != 1 || jobs[0].intervalMs != test.want || jobs[0].debounced != (test.want > 0) || jobs[0].blocksFallback != (test.want == 0) {
			t.Fatalf("%s/%s: jobs=%+v, want %d ms", test.kind, test.keyword, jobs, test.want)
		}
	}
	manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "web"})
	if _, overridden := instance.queryDebounceOverride(Query{Type: QueryTypeInput, TriggerKeyword: "web"}); overridden {
		t.Fatal("re-registering without a delay should restore metadata inheritance")
	}
}

// TestQueryDebounceReplacement checks that typing replaces pending work without crossing window sessions.
func TestQueryDebounceReplacement(t *testing.T) {
	scheduler := newQueryScheduler()
	instance := &Instance{Metadata: Metadata{Id: "search", Name: "Search"}}
	calls := make(chan string, 4)
	start := func(id, session string, delay int) QueryExecution {
		return scheduler.start(context.Background(), Query{Id: id, SessionId: session},
			[]queryPluginJob{{pluginInstance: instance, debounced: delay > 0, intervalMs: delay, blocksFallback: delay == 0}}, 0,
			queryExecutionCallbacks{
				queryPlugin: func(_ context.Context, _ *Instance, query Query) QueryResponse {
					calls <- query.Id
					return QueryResponse{}
				},
				onPluginResponse: func(context.Context, Query, QueryResponse) {},
			})
	}
	wait := func(execution QueryExecution) {
		t.Helper()
		select {
		case <-execution.Done:
		case <-time.After(2 * time.Second):
			t.Fatal("query completion stalled")
		}
	}
	old := start("old", "main", 60000)
	other := start("other", "panel", 20)
	latest := start("latest", "main", 20)
	wait(old)
	wait(other)
	wait(latest)
	seen := map[string]bool{}
	for range 2 {
		seen[<-calls] = true
	}
	if !seen["latest"] || !seen["other"] || seen["old"] {
		t.Fatalf("executed queries = %v", seen)
	}
	pending := start("pending", "main", 60000)
	direct := start("direct", "main", 0)
	wait(pending)
	wait(direct)
	if got := <-calls; got != "direct" {
		t.Fatalf("executed %q instead of immediate query", got)
	}
	if scheduler.debounceQueryTimer.Len() != 0 {
		t.Fatal("finished timers were retained")
	}
}
