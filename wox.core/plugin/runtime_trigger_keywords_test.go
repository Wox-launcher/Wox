package plugin

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"wox/database"
	"wox/setting"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestAPIGetTriggerKeywords covers effective user/runtime triggers and protects the routing list from caller mutation.
func TestAPIGetTriggerKeywords(t *testing.T) {
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"*", "f"}}}
	api := &APIImpl{pluginInstance: instance}
	result := api.GetTriggerKeywords(t.Context(), GetTriggerKeywordsOption{})
	if !slices.Equal(result.TriggerKeywords, []string{"*", "f"}) {
		t.Fatalf("metadata triggers = %v", result.TriggerKeywords)
	}
	result.TriggerKeywords[1] = "changed"
	if !slices.Equal(instance.GetTriggerKeywords(), []string{"*", "f"}) {
		t.Fatal("caller mutated metadata triggers")
	}

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "settings.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatal(err)
	}
	instance.Setting = setting.NewPluginSetting(setting.NewPluginSettingStore(db, "file-test"), nil)
	if err := instance.Setting.TriggerKeywords.SetLocal([]string{"*", "files"}); err != nil {
		t.Fatal(err)
	}
	instance.setRuntimeTriggerKeywords([]string{"find"})
	result = api.GetTriggerKeywords(t.Context(), GetTriggerKeywordsOption{})
	if !slices.Equal(result.TriggerKeywords, []string{"*", "files", "find"}) {
		t.Fatalf("effective triggers = %v", result.TriggerKeywords)
	}
	result.TriggerKeywords[1], result.TriggerKeywords[2] = "changed", "changed"
	if !slices.Equal(instance.GetTriggerKeywords(), []string{"*", "files", "find"}) {
		t.Fatal("caller mutated configured or runtime triggers")
	}
	instance.setRuntimeTriggerKeywords(nil)
	if got := api.GetTriggerKeywords(t.Context(), GetTriggerKeywordsOption{}).TriggerKeywords; !slices.Equal(got, []string{"*", "files"}) {
		t.Fatalf("removed runtime triggers survived: %v", got)
	}
	encoded, err := json.Marshal((&APIImpl{pluginInstance: &Instance{}}).GetTriggerKeywords(t.Context(), GetTriggerKeywordsOption{}))
	if err != nil || string(encoded) != `{"TriggerKeywords":[]}` {
		t.Fatalf("empty wire result = %s, error = %v", encoded, err)
	}
}

// TestRuntimeTriggerKeywords verifies registration, replacement and lifecycle cleanup through query parsing.
func TestRuntimeTriggerKeywords(t *testing.T) {
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"*"}}}
	other := &Instance{Metadata: Metadata{TriggerKeywords: []string{"occupied"}}}
	manager := &Manager{lifecycle: pluginLifecycle{instances: []*Instance{instance, other}}}
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
	(&pluginLifecycle{}).clearRuntimeCallbacks(instance)
	if got := instance.GetTriggerKeywords(); !slices.Equal(got, []string{"*"}) {
		t.Fatalf("runtime keywords survived unload: %v", got)
	}
}

// TestConcurrentTriggerRegistration ensures that an ownership check cannot race another registration.
func TestConcurrentTriggerRegistration(t *testing.T) {
	first, second := &Instance{}, &Instance{}
	manager := &Manager{lifecycle: pluginLifecycle{instances: []*Instance{first, second}}}
	var group sync.WaitGroup
	results := make(chan bool, 2)
	for _, instance := range manager.lifecycle.instances {
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
