package plugin

import (
	"context"
	"testing"
	"time"

	"wox/setting"
	"wox/util"
)

func TestGetQueryFirstFlushDeadlineMsSkipsAliasRestoreJob(t *testing.T) {
	m := &Manager{pluginResultDeliveryLatency: util.NewHashMap[string, *util.EWMA]()}
	deadline := m.getQueryFirstFlushDeadlineMs([]queryPluginJob{{aliasRestore: true}}, 0)
	if deadline != 20 {
		t.Fatalf("deadline = %d, want the no-history first-flush bound", deadline)
	}
}

func TestQueryResultToUICopiesTitleTags(t *testing.T) {
	result := QueryResult{
		Title: "Open Wox Settings",
		TitleTags: []QueryResultTitleTag{
			{Text: "fdj", Kind: QueryResultTitleTagKindAlias, Tooltip: "Result alias"},
			{Text: "ctrl+alt+s", Kind: QueryResultTitleTagKindHotkey, Tooltip: "Result hotkey"},
		},
	}
	ui := result.ToUI()
	if len(ui.TitleTags) != 2 || ui.TitleTags[0].Text != "fdj" || ui.TitleTags[0].Kind != QueryResultTitleTagKindAlias ||
		ui.TitleTags[1].Text != "ctrl+alt+s" || ui.TitleTags[1].Kind != QueryResultTitleTagKindHotkey {
		t.Fatalf("TitleTags = %+v, want alias then hotkey chips", ui.TitleTags)
	}
}

// TestResultBindingRestoreCancellation exercises a callback that ignores cancellation.
func TestResultBindingRestoreCancellation(t *testing.T) {
	m := &Manager{}
	entered, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	instance := &Instance{MRURestoreCallbacks: []func(context.Context, MRUData) (*QueryResult, error){
		func(context.Context, MRUData) (*QueryResult, error) {
			close(entered)
			<-release
			defer close(finished)
			return &QueryResult{Title: "late"}, nil
		},
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() { close(release); <-finished }()
	done := make(chan *QueryResult, 1)
	go func() {
		done <- m.restoreResultBinding(ctx, instance, setting.ResultBinding{Hash: "bound"}, QueryEnv{})
	}()
	<-entered
	cancel()
	select {
	case result := <-done:
		if result != nil {
			t.Fatal("canceled restore published a result")
		}
	case <-time.After(time.Second):
		t.Fatal("restore kept waiting for an uncooperative callback")
	}
}

// TestResultBindingRestoreFiltersEnv preserves the plugin opt-in boundary.
func TestResultBindingRestoreFiltersEnv(t *testing.T) {
	m := &Manager{}
	instance := &Instance{MRURestoreCallbacks: []func(context.Context, MRUData) (*QueryResult, error){
		func(_ context.Context, data MRUData) (*QueryResult, error) {
			if data.Env.ActiveBrowserUrl != "" || data.Env.ActiveWindowTitle != "" || data.Env.ActiveWindowPid != 0 {
				t.Error("undeclared desktop context was exposed")
			}
			return &QueryResult{Title: "restored"}, nil
		},
	}}
	result := m.restoreResultBinding(context.Background(), instance, setting.ResultBinding{}, QueryEnv{ActiveBrowserUrl: "private", ActiveWindowTitle: "private", ActiveWindowPid: 123})
	if result == nil || result.Title != "restored" {
		t.Fatal("restore failed")
	}
}

func TestDropResultCachesDuplicateOfAlias(t *testing.T) {
	pluginInstance := &Instance{Metadata: Metadata{Id: "sys"}}
	alias := &QueryResultCache{
		PluginInstance: pluginInstance,
		Result:         QueryResult{Id: "alias", Title: "Open Wox Settings"},
		SourceMRUHash:  "settings-hash",
		AliasMatchKind: aliasMatchExact,
	}
	pluginRow := &QueryResultCache{
		PluginInstance: pluginInstance,
		Result:         QueryResult{Id: "plugin", Title: "Open Wox Settings"},
		SourceMRUHash:  "settings-hash",
	}
	other := &QueryResultCache{
		PluginInstance: pluginInstance,
		Result:         QueryResult{Id: "volume", Title: "Set Volume"},
	}
	got := dropResultCachesDuplicateOfAlias([]*QueryResultCache{alias}, []*QueryResultCache{pluginRow, other})
	if len(got) != 1 || got[0].Result.Id != "volume" {
		t.Fatalf("ordinary results = %+v, want only the unrelated row", got)
	}
	deduped := dedupeResultBindingCaches([]*QueryResultCache{alias, {
		PluginInstance: pluginInstance,
		Result:         QueryResult{Id: "alias-2", Title: "Open Wox Settings"},
		SourceMRUHash:  "settings-hash",
		AliasMatchKind: aliasMatchFuzzy,
	}})
	if len(deduped) != 1 || deduped[0].Result.Id != "alias" {
		t.Fatalf("alias results = %+v, want the first identity", deduped)
	}
}

func TestAliasMatchForQuery(t *testing.T) {
	ctx := context.Background()
	if aliasMatchForQuery(ctx, "汇率", "汇率") != aliasMatchExact {
		t.Fatal("equal-fold alias must be exact")
	}
	if aliasMatchForQuery(ctx, "Rate", "rate") != aliasMatchExact {
		t.Fatal("case-insensitive alias must be exact")
	}
	if aliasMatchForQuery(ctx, "", "rate") != aliasMatchNone || aliasMatchForQuery(ctx, "rate", "") != aliasMatchNone {
		t.Fatal("empty alias or search must not match")
	}
}

func TestIsResultBindingMaintenanceAction(t *testing.T) {
	if !isResultBindingMaintenanceAction(systemActionSetResultHotkeyID) ||
		!isResultBindingMaintenanceAction(systemActionSetResultAliasID) ||
		!isResultBindingMaintenanceAction(systemActionRemoveFromMRUID) {
		t.Fatal("binding and remove-from-MRU actions must skip ranking bookkeeping")
	}
	if isResultBindingMaintenanceAction(systemActionPinInQueryID) {
		t.Fatal("pin must keep existing ranking bookkeeping")
	}
}

func TestDefaultPluginActionSkipsSystemAndFormActions(t *testing.T) {
	result := QueryResult{Actions: []QueryResultAction{
		{Id: "sys", IsSystemAction: true, IsDefault: true, Action: func(context.Context, ActionContext) {}},
		{Id: "form", Type: QueryResultActionTypeForm, Action: func(context.Context, ActionContext) {}},
		{Id: "run", IsDefault: true, Action: func(context.Context, ActionContext) {}},
	}}
	action := defaultPluginAction(result)
	if action == nil || action.Id != "run" {
		t.Fatalf("defaultPluginAction = %+v, want run", action)
	}
	if isDirectlyExecutableAction(&result.Actions[1]) {
		t.Fatal("form actions must not be treated as directly executable")
	}
	if !isDirectlyExecutableAction(action) {
		t.Fatal("plain plugin actions must be directly executable")
	}
}

func TestMRUIdentityHashPrefersStoredSource(t *testing.T) {
	instance := &Instance{Metadata: Metadata{Id: "converter", Features: []MetadataFeature{{Name: MetadataFeatureMRU, Params: map[string]any{"HashBy": "rawquery"}}}}}
	query := Query{RawQuery: "100 usd to cny", Search: "汇率"}
	result := QueryResult{Title: "100 USD = 700 CNY"}
	if got := resolveResultSourceHash(instance, query, result, "stored-hash"); got != "stored-hash" {
		t.Fatalf("source hash = %q, want stored-hash", got)
	}
	if got := mruIdentityHash(instance.Metadata, query, result); got == "" {
		t.Fatal("rawquery hash must be non-empty")
	}
}

func TestValidateQueryAliasAndTriggerAgainstAliases(t *testing.T) {
	bindings := []setting.ResultBinding{{Hash: "h1", PluginID: "app", Alias: "ch"}}
	if err := ValidateQueryAliasAgainstResultAliases(setting.QueryAlias{Alias: "ch"}, bindings); err == nil {
		t.Fatal("query alias must conflict with result alias")
	}
	if err := ValidateQueryAliasAgainstResultAliases(setting.QueryAlias{Alias: "ok"}, bindings); err != nil {
		t.Fatalf("unrelated query alias = %v", err)
	}
	if err := ValidateTriggerKeywordsAgainstAliases([]string{"ch"}, bindings); err == nil {
		t.Fatal("trigger keyword must conflict with result alias")
	}
	if err := ValidateTriggerKeywordsAgainstAliases([]string{"*"}, bindings); err != nil {
		t.Fatalf("wildcard trigger = %v", err)
	}
}

func TestValidateQueryHotkeysAgainstBindings(t *testing.T) {
	bindings := []setting.ResultBinding{{Hash: "h1", PluginID: "app", Hotkey: "Ctrl+Alt+C"}}
	if err := ValidateQueryHotkeysAgainstBindings([]setting.QueryHotkey{{Hotkey: "alt + ctrl + c"}}, bindings); err == nil {
		t.Fatal("query hotkey must conflict with result hotkey")
	}
	if err := ValidateQueryHotkeysAgainstBindings([]setting.QueryHotkey{{Hotkey: "Ctrl+Alt+D"}}, bindings); err != nil {
		t.Fatalf("unrelated query hotkey = %v", err)
	}
}

// TestAliasDedupePreservesDifferentIdentities protects same-title notes and actions.
func TestAliasDedupePreservesDifferentIdentities(t *testing.T) {
	instance := &Instance{Metadata: Metadata{Id: "notes", Features: []MetadataFeature{{Name: MetadataFeatureMRU, Params: map[string]any{"HashBy": "scorekey"}}}}}
	alias := &QueryResultCache{PluginInstance: instance, Result: QueryResult{Title: "Shopping", ScoreKey: "note:1"}, AliasMatchKind: aliasMatchExact}
	other := &QueryResultCache{PluginInstance: instance, Result: QueryResult{Title: "Shopping", ScoreKey: "note:2"}}
	duplicate := &QueryResultCache{PluginInstance: instance, Result: QueryResult{Title: "Shopping", ScoreKey: "note:1"}}
	got := dropResultCachesDuplicateOfAlias([]*QueryResultCache{alias}, []*QueryResultCache{other, duplicate})
	if len(got) != 1 || got[0] != other {
		t.Fatal("dedupe hid a different note or kept the duplicate")
	}
}

// TestResultBindingHashMatchesTranslatedMRU checks both pre-polish forms and cached rows.
func TestResultBindingHashMatchesTranslatedMRU(t *testing.T) {
	ctx := context.Background()
	query := Query{Id: "query", SessionId: "session", Type: QueryTypeInput, Search: "settings"}
	original := QueryResult{Id: "settings", Title: "i18n:plugin_sys_open_wox_settings"}
	manager, instance := newTestManagerWithCachedResult(query, original)
	instance.Metadata.Features = []MetadataFeature{{Name: MetadataFeatureMRU}}
	translated := original
	translated.Title = manager.translatePlugin(ctx, instance, original.Title)
	if translated.Title == original.Title {
		t.Fatal("test title was not translated")
	}
	hash := manager.resultBindingHash(ctx, instance, query, original, "")
	if hash != mruIdentityHash(instance.Metadata, query, translated) {
		t.Fatal("form and execution use different identities")
	}
	manager.storeQueryResult(ctx, instance, query, QueryLayout{}, translated, hash, aliasMatchExact)
	cache, _ := manager.findResultCacheInSession(query.SessionId, query.Id, original.Id)
	if cache.SourceMRUHash != hash || cache.AliasMatchKind != aliasMatchExact {
		t.Fatal("source identity was not published with the cache")
	}
	manager.storeQueryResult(ctx, instance, query, QueryLayout{}, translated, "ordinary", aliasMatchNone)
	cache, _ = manager.findResultCacheInSession(query.SessionId, query.Id, original.Id)
	if cache.SourceMRUHash != hash || cache.AliasMatchKind != aliasMatchExact {
		t.Fatal("late ordinary row erased alias priority")
	}
}

type bindingProxyPlugin struct{ Plugin }

func (bindingProxyPlugin) CreateActionProxy(string) func(context.Context, ActionContext) {
	return func(context.Context, ActionContext) {}
}

func TestResultBindingAcceptsExternalExecuteBeforePolish(t *testing.T) {
	instance := &Instance{Plugin: bindingProxyPlugin{}}
	result := QueryResult{Actions: []QueryResultAction{{Id: "open", Type: QueryResultActionTypeExecute}}}
	if !canBindResultHotkey(instance, result) {
		t.Fatal("host action was rejected before proxy attachment")
	}
	result.Actions[0].Type = QueryResultActionTypeForm
	if canBindResultHotkey(instance, result) {
		t.Fatal("form action was accepted as a silent hotkey")
	}
}
