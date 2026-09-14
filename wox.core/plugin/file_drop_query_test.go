package plugin

import (
	"context"
	"testing"

	"wox/common"
	"wox/database"
	"wox/setting"
	"wox/util/selection"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestCanOperateQueryRequiresSelectionFeatureForKeywordAndScope(t *testing.T) {
	manager := &Manager{}
	ctx := context.Background()
	withFeature := &Instance{
		Metadata: Metadata{
			Id:              "with-selection",
			TriggerKeywords: []string{"sel"},
			Features:        []MetadataFeature{{Name: MetadataFeatureQuerySelection}},
		},
	}
	withoutFeature := &Instance{
		Metadata: Metadata{
			Id:              "without-selection",
			TriggerKeywords: []string{"plain"},
		},
	}

	keywordSelection := Query{Type: QueryTypeSelection, TriggerKeyword: "sel"}
	if !manager.canOperateQuery(ctx, withFeature, keywordSelection) {
		t.Fatal("keyword selection must reach a plugin that owns the keyword and declares querySelection")
	}
	if manager.canOperateQuery(ctx, withoutFeature, Query{Type: QueryTypeSelection, TriggerKeyword: "plain"}) {
		t.Fatal("keyword selection must not reach a plugin that never declared querySelection")
	}
	if manager.canOperateQuery(ctx, withFeature, Query{Type: QueryTypeSelection, TriggerKeyword: "plain"}) {
		t.Fatal("keyword selection must not reach a different plugin")
	}

	scoped := Query{
		Type:  QueryTypeSelection,
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: withFeature.Metadata.Id}}},
	}
	if !manager.canOperateQuery(ctx, withFeature, scoped) {
		t.Fatal("scoped selection must require querySelection")
	}
	scopedUnsupported := Query{
		Type:  QueryTypeSelection,
		Scope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: withoutFeature.Metadata.Id}}},
	}
	if manager.canOperateQuery(ctx, withoutFeature, scopedUnsupported) {
		t.Fatal("scoped selection without querySelection must not run")
	}

	global := Query{Type: QueryTypeSelection}
	if !manager.canOperateQuery(ctx, withFeature, global) {
		t.Fatal("global selection should still reach plugins that declare querySelection")
	}
	if manager.canOperateQuery(ctx, withoutFeature, global) {
		t.Fatal("global selection must not reach plugins without querySelection")
	}
}

func TestBuildFileDropQueryRoutesExplicitPluginAndFallsBack(t *testing.T) {
	manager := &Manager{}
	supported := &Instance{
		Metadata: Metadata{
			Id:              "file-plugin",
			TriggerKeywords: []string{"file"},
			Commands:        []MetadataCommand{{Command: "preview"}},
			Features:        []MetadataFeature{{Name: MetadataFeatureQuerySelection}},
		},
	}
	unsupported := &Instance{
		Metadata: Metadata{
			Id:              "calc-plugin",
			TriggerKeywords: []string{"calc"},
		},
	}
	if !manager.appendPluginInstance(supported) || !manager.appendPluginInstance(unsupported) {
		t.Fatal("failed to register test plugins")
	}
	paths := []string{`C:\tmp\report.pdf`}

	scoped := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{
		QueryText: "notes inside scope",
		QueryScope: common.QueryScope{Plugins: []common.QueryScopePlugin{{
			PluginID: supported.Metadata.Id,
			Command:  "preview",
		}}},
		QueryRefinements: map[string]string{"sort": "name"},
		ContextData:      common.ContextData{"ai_chat_active_id": "chat-1"},
	}, paths)
	if scoped.QueryType != QueryTypeSelection || scoped.QueryText != "notes inside scope" {
		t.Fatalf("scoped query = %+v", scoped)
	}
	if len(scoped.QueryScope.Plugins) != 1 || scoped.QueryScope.Plugins[0].PluginID != supported.Metadata.Id || scoped.QueryScope.Plugins[0].Command != "preview" {
		t.Fatalf("scoped target = %+v", scoped.QueryScope)
	}
	if scoped.QuerySelection.Type != selection.SelectionTypeFile || len(scoped.QuerySelection.FilePaths) != 1 {
		t.Fatalf("selection = %+v", scoped.QuerySelection)
	}
	if len(scoped.QueryRefinements) != 0 || scoped.QueryHint != nil || scoped.ContextData["ai_chat_active_id"] != "" {
		t.Fatalf("scoped query leaked previous input state: %+v", scoped)
	}

	fallback := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{
		QueryText: "calc 1+1",
		QueryScope: common.QueryScope{Plugins: []common.QueryScopePlugin{{
			PluginID: unsupported.Metadata.Id,
		}}},
		ContextData: common.ContextData{"plugin": "private"},
	}, paths)
	if !fallback.QueryScope.IsEmpty() || fallback.QueryText != "" || fallback.ContextData["plugin"] != "" {
		t.Fatalf("unsupported scope must fall back to a clean global selection: %+v", fallback)
	}

	keyword := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{QueryText: "file preview leftover"}, paths)
	if keyword.QueryType != QueryTypeSelection || keyword.QueryText != "file preview leftover" || !keyword.QueryScope.IsEmpty() {
		t.Fatalf("keyword entry must keep the typed query and send selection: %+v", keyword)
	}
	if keyword.QuerySelection.Type != selection.SelectionTypeFile || len(keyword.QuerySelection.FilePaths) != 1 {
		t.Fatalf("keyword selection = %+v", keyword.QuerySelection)
	}

	keywordFallback := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{QueryText: "calc 2+2"}, paths)
	if !keywordFallback.QueryScope.IsEmpty() || keywordFallback.QueryText != "" {
		t.Fatalf("unsupported keyword must fall back globally: %+v", keywordFallback)
	}

	global := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{QueryText: "hello world"}, paths)
	if !global.QueryScope.IsEmpty() || global.QueryText != "" || global.QueryType != QueryTypeSelection {
		t.Fatalf("global launcher drop = %+v", global)
	}

	missing := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{
		QueryScope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: "missing-plugin"}}},
	}, paths)
	if len(missing.QueryScope.Plugins) != 1 || missing.QueryScope.Plugins[0].PluginID != "missing-plugin" {
		t.Fatalf("invalid scope must not expand to global: %+v", missing)
	}

	multi := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{
		QueryScope: common.QueryScope{Plugins: []common.QueryScopePlugin{
			{PluginID: unsupported.Metadata.Id},
			{PluginID: supported.Metadata.Id},
		}},
	}, paths)
	if len(multi.QueryScope.Plugins) != 2 {
		t.Fatalf("multi-plugin scope must keep the allowlist: %+v", multi)
	}
}

func TestShouldFallbackUnsupportedSelectionKeepsMissingAndDisabled(t *testing.T) {
	if shouldFallbackUnsupportedSelection(nil) {
		t.Fatal("missing plugin must not use the unsupported-selection fallback")
	}
	if shouldFallbackUnsupportedSelection(&Instance{Metadata: Metadata{Features: []MetadataFeature{{Name: MetadataFeatureQuerySelection}}}}) {
		t.Fatal("supported plugin must not fall back")
	}
	if !shouldFallbackUnsupportedSelection(&Instance{Metadata: Metadata{Id: "plain"}}) {
		t.Fatal("enabled plugin without querySelection must fall back")
	}
	disabled := newDisabledPluginInstance(t, "disabled-plain", false)
	if shouldFallbackUnsupportedSelection(disabled) {
		t.Fatal("a disabled plugin must keep existing protection instead of the unsupported fallback")
	}
}

func TestBuildFileDropQueryKeepsDisabledPluginScope(t *testing.T) {
	manager := &Manager{}
	disabled := newDisabledPluginInstance(t, "disabled-plugin", false)
	if !manager.appendPluginInstance(disabled) {
		t.Fatal("failed to register the disabled plugin")
	}
	paths := []string{`C:\tmp\report.pdf`}

	scoped := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{
		QueryScope: common.QueryScope{Plugins: []common.QueryScopePlugin{{PluginID: disabled.Metadata.Id}}},
	}, paths)
	if len(scoped.QueryScope.Plugins) != 1 || scoped.QueryScope.Plugins[0].PluginID != disabled.Metadata.Id {
		t.Fatalf("disabled plugin scope must not expand to global: %+v", scoped)
	}
	if manager.canOperateQuery(context.Background(), disabled, Query{Type: QueryTypeSelection, Scope: scoped.QueryScope}) {
		t.Fatal("a disabled plugin must not receive the scoped selection")
	}

	keyword := manager.BuildFileDropQuery(context.Background(), common.PlainQuery{QueryText: "off leftover"}, paths)
	if len(keyword.QueryScope.Plugins) != 1 || keyword.QueryScope.Plugins[0].PluginID != disabled.Metadata.Id || keyword.QueryText != "leftover" {
		t.Fatalf("disabled keyword must stay scoped, not fall back globally: %+v", keyword)
	}
}

func newDisabledPluginInstance(t *testing.T, id string, supportSelection bool) *Instance {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open plugin setting db: %v", err)
	}
	if err := db.AutoMigrate(&database.PluginSetting{}, &database.Oplog{}); err != nil {
		t.Fatalf("migrate plugin setting db: %v", err)
	}
	pluginSetting := setting.NewPluginSetting(setting.NewPluginSettingStore(db, id), nil)
	if err := pluginSetting.Disabled.SetLocal(true); err != nil {
		t.Fatalf("disable plugin: %v", err)
	}
	instance := &Instance{
		Metadata: Metadata{Id: id, TriggerKeywords: []string{"off"}},
		Setting:  pluginSetting,
	}
	if supportSelection {
		instance.Metadata.Features = []MetadataFeature{{Name: MetadataFeatureQuerySelection}}
	}
	return instance
}
