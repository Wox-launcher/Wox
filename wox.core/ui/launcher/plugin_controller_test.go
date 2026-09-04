package launcher

import (
	"context"
	"testing"

	"wox/setting/definition"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type pluginFakeService struct {
	plugins  map[contract.PluginCatalog][]contract.PluginCatalogItem
	storeErr error
	instErr  error
	started  chan struct{}
	release  chan struct{}
}

func (f *pluginFakeService) Plugins(_ context.Context, _ string, catalog contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		<-f.release
	}
	switch catalog {
	case contract.PluginCatalogStore:
		if f.storeErr != nil {
			return nil, f.storeErr
		}
	case contract.PluginCatalogInstalled:
		if f.instErr != nil {
			return nil, f.instErr
		}
	}
	return append([]contract.PluginCatalogItem(nil), f.plugins[catalog]...), nil
}

func newPluginControllerDeps() (CommonDeps, *int) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &invalidateCalled
}

func TestPluginControllerPlugins(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetPlugins([]pluginSettingsPlugin{{ID: "a"}, {ID: "b"}})
	got := c.Plugins()
	if len(got) != 2 {
		t.Fatalf("Plugins len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("Plugins = %+v, want [a, b]", got)
	}
}

func TestPluginControllerSelected(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetSelected(3)
	if got := c.Selected(); got != 3 {
		t.Fatalf("Selected = %d, want 3", got)
	}
}

func TestPluginControllerSearchEditor(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	editor := woxui.NewTextEditor("query")
	c.SetSearchEditor(editor)
	if got := c.SearchEditor(); got != editor {
		t.Fatalf("SearchEditor mismatch: got %v, want %v", got, editor)
	}
}

func TestPluginControllerForm(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	form := &pluginSettingsFormState{pluginID: "p1"}
	c.SetForm(form)
	if got := c.Form(); got != form {
		t.Fatalf("Form mismatch: got %v, want %v", got, form)
	}
}

func TestPluginSettingsFormDefinitionsKeepMetadataControlsOutOfSettings(t *testing.T) {
	definitions := pluginSettingsFormDefinitions(pluginSettingsPlugin{
		SettingDefinitions: []formDefinition{{Type: "checkbox", Value: formDefinitionValue{Key: "FeatureEnabled"}}},
	})

	if len(definitions) != 2 {
		t.Fatalf("definitions len = %d, want trigger keywords plus one plugin setting", len(definitions))
	}
	if definitions[0].Value.Key != "TriggerKeywords" {
		t.Fatalf("first definition key = %q, want TriggerKeywords", definitions[0].Value.Key)
	}
	for _, definition := range definitions {
		if definition.Value.Key == "Disabled" {
			t.Fatalf("Disabled should be rendered as a header action, not a Settings field")
		}
		if definition.Type == "head" && definition.Value.Content == "Plugin controls" {
			t.Fatalf("Plugin controls heading should not be part of the Settings form")
		}
	}
}

func TestPluginSettingsFormDefinitionsUseInlineLayoutForTableOnlyPlugins(t *testing.T) {
	definitions := pluginSettingsFormDefinitions(pluginSettingsPlugin{
		SettingDefinitions: []formDefinition{
			{Type: "table", Value: formDefinitionValue{Key: "commands"}},
			{Type: "table", Value: formDefinitionValue{Key: "aliases"}},
		},
	})

	for index, definition := range definitions[1:] {
		if !definition.Value.InlineTable {
			t.Fatalf("plugin table definition %d should use the full-width inline layout", index)
		}
	}
}

func TestPluginSettingsFormDefinitionsKeepMixedFormsInLabelLayout(t *testing.T) {
	definitions := pluginSettingsFormDefinitions(pluginSettingsPlugin{
		SettingDefinitions: []formDefinition{
			{Type: "table", Value: formDefinitionValue{Key: "commands"}},
			{Type: "checkbox", Value: formDefinitionValue{Key: "enabled"}},
		},
	})

	if definitions[1].Value.InlineTable {
		t.Fatalf("table in a mixed plugin form should keep the label-and-control layout")
	}
}

func TestPluginSettingsPluginsFromContractPreservesEnglishNames(t *testing.T) {
	items := []contract.PluginCatalogItem{{
		ID: "file-search", Name: "文件搜索", NameEn: "File Search",
		Description: "搜索本地文件", DescriptionEn: "Search local files",
	}}

	plugins, err := pluginSettingsPluginsFromContract(items)
	if err != nil {
		t.Fatalf("adapt plugin settings: %v", err)
	}
	if plugins[0].NameEn != "File Search" || plugins[0].DescriptionEn != "Search local files" {
		t.Fatalf("english fields = %+v, want NameEn and DescriptionEn preserved", plugins[0])
	}
}

func TestPluginControllerPreservesDirPathSettingDefinition(t *testing.T) {
	items := []contract.PluginCatalogItem{{
		ID: "shell",
		SettingDefinitions: definition.PluginSettingDefinitions{{
			Type: definition.PluginSettingDefinitionTypeDirPath,
			Value: &definition.PluginSettingValueDirPath{
				Key: "default_working_directory", Label: "Default working directory", DefaultValue: `C:\Users\test`,
			},
		}},
	}}

	plugins, err := pluginSettingsPluginsFromContract(items)
	if err != nil {
		t.Fatalf("adapt plugin settings: %v", err)
	}
	definition := plugins[0].SettingDefinitions[0]
	if definition.Type != "dirPath" || definition.Value.Key != "default_working_directory" || definition.Value.DefaultValue != `C:\Users\test` {
		t.Fatalf("dirPath setting definition = %+v", definition)
	}
}

func TestPluginControllerPreservesIgnoreRuleTableSearchAndPreview(t *testing.T) {
	items := []contract.PluginCatalogItem{{
		ID: "app",
		SettingDefinitions: definition.PluginSettingDefinitions{{
			Type: definition.PluginSettingDefinitionTypeTable,
			Value: &definition.PluginSettingValueTable{
				Key: "IgnoreRules", EnableSearch: true, SearchColumnKey: "Pattern",
				Columns: []definition.PluginSettingValueTableColumn{{
					Key: "Pattern", Type: definition.PluginSettingValueTableColumnTypeText, PreviewMatchedApps: true,
				}},
			},
		}},
	}}

	plugins, err := pluginSettingsPluginsFromContract(items)
	if err != nil {
		t.Fatalf("adapt plugin settings: %v", err)
	}
	value := plugins[0].SettingDefinitions[0].Value
	if !value.EnableSearch || value.SearchColumnKey != "Pattern" {
		t.Fatalf("ignore rules search = %+v", value)
	}
	if len(value.Columns) != 1 || !value.Columns[0].PreviewMatchedApps {
		t.Fatalf("ignore rules columns = %+v", value.Columns)
	}
}

func TestPluginControllerPreservesInlineApplicationTableDefinition(t *testing.T) {
	items := []contract.PluginCatalogItem{{
		ID: "clipboard",
		SettingDefinitions: definition.PluginSettingDefinitions{{
			Type: definition.PluginSettingDefinitionTypeTable,
			Value: &definition.PluginSettingValueTable{
				Key: "ignored_applications", InlineTable: true,
				Columns: []definition.PluginSettingValueTableColumn{{Key: "App", Type: definition.PluginSettingValueTableColumnTypeApp}},
			},
		}},
	}}

	plugins, err := pluginSettingsPluginsFromContract(items)
	if err != nil {
		t.Fatalf("adapt plugin settings: %v", err)
	}
	value := plugins[0].SettingDefinitions[0].Value
	if !value.InlineTable || len(value.Columns) != 1 || value.Columns[0].Type != "app" {
		t.Fatalf("application table definition = %+v", value)
	}
}

func TestPluginControllerPreservesStatsSettingDefinition(t *testing.T) {
	items := []contract.PluginCatalogItem{{
		ID: "file-search",
		SettingDefinitions: definition.PluginSettingDefinitions{{
			Type: definition.PluginSettingDefinitionTypeStats,
			Value: &definition.PluginSettingValueStats{
				Key:   "indexStats",
				Title: "Index Stats",
				Rows: []definition.PluginSettingValueStatsRow{
					{Label: "Disk Usage", Value: "29.4 MB"},
					{Label: "Files", Value: "130,945"},
				},
			},
		}},
	}}

	plugins, err := pluginSettingsPluginsFromContract(items)
	if err != nil {
		t.Fatalf("adapt plugin settings: %v", err)
	}
	definition := plugins[0].SettingDefinitions[0]
	if definition.Type != "stats" || definition.Value.Key != "indexStats" || definition.Value.Title != "Index Stats" {
		t.Fatalf("stats setting definition = %+v", definition)
	}
	if len(definition.Value.Rows) != 2 || definition.Value.Rows[0].Label != "Disk Usage" || definition.Value.Rows[0].Value != "29.4 MB" {
		t.Fatalf("stats rows = %+v", definition.Value.Rows)
	}
}

func TestPluginControllerReloadPluginsSuccess(t *testing.T) {
	deps, invalidateCalled := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	plugins := []contract.PluginCatalogItem{{ID: "t1", Name: "Plugin One"}, {ID: "t2", Name: "Plugin Two"}}
	service := &pluginFakeService{plugins: map[contract.PluginCatalog][]contract.PluginCatalogItem{contract.PluginCatalogInstalled: plugins}}
	if err := c.ReloadPlugins(context.Background(), service, "session", false, ""); err != nil {
		t.Fatalf("ReloadPlugins error: %v", err)
	}
	if !c.PluginsLoaded() {
		t.Fatalf("PluginsLoaded should be true after successful reload")
	}
	if c.PluginsError() != "" {
		t.Fatalf("PluginsError should be empty, got %q", c.PluginsError())
	}
	if c.PluginsLoading() {
		t.Fatalf("PluginsLoading should be false after reload completes")
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestPluginControllerPreloadPluginsCachesWithoutReplacingActiveCatalog(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetPlugins([]pluginSettingsPlugin{{ID: "installed"}})
	service := &pluginFakeService{plugins: map[contract.PluginCatalog][]contract.PluginCatalogItem{
		contract.PluginCatalogStore: {{ID: "store", Name: "Store Plugin"}},
	}}

	if err := c.PreloadPlugins(context.Background(), service, "session", true); err != nil {
		t.Fatalf("PreloadPlugins error: %v", err)
	}
	if got := c.Plugins(); len(got) != 1 || got[0].ID != "installed" {
		t.Fatalf("active plugins = %+v, want installed catalog unchanged", got)
	}
	cached, loaded := c.CachedPlugins(true)
	if !loaded || len(cached) != 1 || cached[0].ID != "store" {
		t.Fatalf("cached store plugins = %+v, loaded = %v", cached, loaded)
	}
}

func TestPluginControllerReleaseWindowMemoryInvalidatesPreload(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	service := &pluginFakeService{
		plugins: map[contract.PluginCatalog][]contract.PluginCatalogItem{
			contract.PluginCatalogStore: {{ID: "store", Name: "Store Plugin"}},
		},
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	done := make(chan error, 1)
	go func() {
		done <- c.PreloadPlugins(context.Background(), service, "session", true)
	}()
	<-service.started
	c.ReleaseWindowMemory()
	close(service.release)
	if err := <-done; err != nil {
		t.Fatalf("PreloadPlugins error: %v", err)
	}
	if cached, loaded := c.CachedPlugins(true); loaded || len(cached) != 0 {
		t.Fatalf("released store cache was repopulated: loaded=%v plugins=%+v", loaded, cached)
	}
}

func TestPluginControllerPluginsStore(t *testing.T) {
	deps, _ := newPluginControllerDeps()
	c := newPluginSettingsController(deps)
	c.SetPluginsStore(true)
	if got := c.PluginsStore(); got != true {
		t.Fatalf("PluginsStore = %v, want true", got)
	}
}
