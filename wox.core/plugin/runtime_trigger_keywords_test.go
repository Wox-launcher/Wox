package plugin

import (
	"context"
	"slices"
	"sync"
	"testing"
)

// TestRuntimeTriggerKeywords verifies registration, replacement and lifecycle cleanup through query parsing.
func TestRuntimeTriggerKeywords(t *testing.T) {
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"*"}}}
	other := &Instance{Metadata: Metadata{TriggerKeywords: []string{"occupied"}}}
	manager := &Manager{instances: []*Instance{instance, other}}
	for _, keyword := range []string{"", "*", "two words", "tab\tword", "occupied"} {
		if manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: keyword}) {
			t.Fatalf("invalid or occupied keyword %q succeeded", keyword)
		}
	}
	if !manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "g"}) || !manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "g"}) {
		t.Fatal("registration or repeat registration failed")
	}
	if manager.registerTriggerKeyword(other, RegisterTriggerKeywordOption{Keyword: "g"}) {
		t.Fatal("another plugin claimed an occupied runtime keyword")
	}
	if got := instance.GetTriggerKeywords(); !slices.Equal(got, []string{"*", "g"}) {
		t.Fatalf("keywords = %v", got)
	}
	query, owner := newQueryInputWithPlugins("g hello world", []*Instance{instance})
	if owner != instance || query.TriggerKeyword != "g" || query.Search != "hello world" || query.IsGlobalQuery() {
		t.Fatalf("query was not scoped: %+v", query)
	}
	api := &APIImpl{pluginInstance: instance}
	for range 2 {
		if !api.UnregisterTriggerKeyword(context.Background(), UnregisterTriggerKeywordOption{Keyword: "g"}).Success {
			t.Fatal("unregistration or repeated unregistration failed")
		}
	}
	if !manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "b"}) || !manager.registerTriggerKeyword(other, RegisterTriggerKeywordOption{Keyword: "g"}) {
		t.Fatal("released or new keyword could not be registered")
	}
	if _, owner := newQueryInputWithPlugins("g test", []*Instance{instance}); owner != nil {
		t.Fatal("removed keyword still routes to the plugin")
	}
	if _, owner := newQueryInputWithPlugins("b test", []*Instance{instance}); owner != instance {
		t.Fatal("replacement keyword does not route to the plugin")
	}
	instance.unregisterTriggerKeyword("*")
	(&Manager{}).clearRuntimeCallbacks(instance)
	if got := instance.GetTriggerKeywords(); !slices.Equal(got, []string{"*"}) {
		t.Fatalf("runtime keywords survived unload: %v", got)
	}
}

// TestConcurrentTriggerRegistration ensures that an ownership check cannot race another registration.
func TestConcurrentTriggerRegistration(t *testing.T) {
	first, second := &Instance{}, &Instance{}
	manager := &Manager{instances: []*Instance{first, second}}
	var group sync.WaitGroup
	results := make(chan bool, 2)
	for _, instance := range manager.instances {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "same"})
		}()
	}
	group.Wait()
	if first, second := <-results, <-results; first == second {
		t.Fatalf("expected exactly one owner, got %v, %v", first, second)
	}
}
