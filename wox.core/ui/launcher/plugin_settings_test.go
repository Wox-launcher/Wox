package launcher

import (
	"reflect"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestPluginMatchesFiltersExclusiveDropdowns(t *testing.T) {
	enabled := pluginSettingsPlugin{ID: "a", Runtime: "nodejs"}
	disabled := pluginSettingsPlugin{ID: "b", Runtime: "python", IsDisable: true}
	upgradable := pluginSettingsPlugin{ID: "c", Runtime: "nodejs", IsUpgradable: true}
	system := pluginSettingsPlugin{ID: "d", Runtime: "go", IsSystem: true}
	script := pluginSettingsPlugin{ID: "e", Runtime: "script", Entry: "main.js"}
	storeInstalled := pluginSettingsPlugin{ID: "f", Runtime: "python", IsInstalled: true}

	if got := pluginMatchesFilters(disabled, pluginFilterState{enabledStatus: pluginFilterEnabled}, false); got {
		t.Fatal("disabled plugin must not match enabled status")
	}
	if got := pluginMatchesFilters(enabled, pluginFilterState{enabledStatus: pluginFilterDisabled}, false); got {
		t.Fatal("enabled plugin must not match disabled status")
	}
	if got := pluginMatchesFilters(enabled, pluginFilterState{upgradeStatus: pluginFilterUpgradable}, false); got {
		t.Fatal("non-upgradable plugin must not match upgradable status")
	}
	if got := pluginMatchesFilters(upgradable, pluginFilterState{upgradeStatus: pluginFilterNotUpgradable}, false); got {
		t.Fatal("upgradable plugin must not match not-upgradable status")
	}
	if got := pluginMatchesFilters(system, pluginFilterState{pluginType: pluginFilterThirdParty}, false); got {
		t.Fatal("system plugin must not match third-party type")
	}
	if got := pluginMatchesFilters(enabled, pluginFilterState{pluginType: pluginFilterSystem}, false); got {
		t.Fatal("third-party plugin must not match system type")
	}
	if got := pluginMatchesFilters(script, pluginFilterState{runtime: pluginFilterRuntimeScriptNodeJS}, false); !got {
		t.Fatal("javascript script plugin must match the Node.js script runtime")
	}
	if got := pluginMatchesFilters(script, pluginFilterState{runtime: pluginFilterRuntimePython}, false); got {
		t.Fatal("javascript script plugin must not match the Python runtime")
	}
	if got := pluginMatchesFilters(storeInstalled, pluginFilterState{installStatus: pluginFilterUninstalled}, true); got {
		t.Fatal("installed store plugin must not match uninstalled status")
	}
	if got := pluginMatchesFilters(enabled, pluginFilterState{}, false); !got {
		t.Fatal("empty filters must keep every plugin visible")
	}
}

func TestResetPluginFiltersClearsExclusiveDropdowns(t *testing.T) {
	app := &App{
		pluginSettings:  newPluginSettingsController(CommonDeps{}),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	}
	app.pluginSettings.SetFilters(pluginFilterState{
		enabledStatus: pluginFilterDisabled,
		pluginType:    pluginFilterThirdParty,
		runtime:       pluginFilterRuntimePython,
	})
	app.resetPluginFilters()
	if got := app.pluginSettings.Filters(); got != (pluginFilterState{}) {
		t.Fatalf("reset filters = %+v, want empty All values", got)
	}
}

func TestGroupInstalledPluginsPutsEnabledFirstAndOmitsEmptySections(t *testing.T) {
	filtered := []filteredPlugin{
		{index: 0, plugin: pluginSettingsPlugin{ID: "disabled-system", Name: "Zebra", IsSystem: true, IsDisable: true}},
		{index: 1, plugin: pluginSettingsPlugin{ID: "enabled-third", Name: "Beta"}},
		{index: 2, plugin: pluginSettingsPlugin{ID: "enabled-system", Name: "Alpha", IsSystem: true}},
		{index: 3, plugin: pluginSettingsPlugin{ID: "disabled-third", Name: "Apple", IsDisable: true}},
	}
	got := groupInstalledPlugins(filtered)
	if len(got) != 2 || got[0].ID != pluginSectionEnabled || got[1].ID != pluginSectionDisabled {
		t.Fatalf("sections = %#v", got)
	}
	if ids := installedSectionPluginIDs(got[0]); !reflect.DeepEqual(ids, []string{"enabled-system", "enabled-third"}) {
		t.Fatalf("enabled order = %v, want name order without system first", ids)
	}
	if ids := installedSectionPluginIDs(got[1]); !reflect.DeepEqual(ids, []string{"disabled-third", "disabled-system"}) {
		t.Fatalf("disabled order = %v, want name order without system first", ids)
	}

	onlyEnabled := groupInstalledPlugins(filtered[1:3])
	if len(onlyEnabled) != 1 || onlyEnabled[0].ID != pluginSectionEnabled {
		t.Fatalf("single enabled section = %#v", onlyEnabled)
	}
	onlyDisabled := groupInstalledPlugins([]filteredPlugin{filtered[0], filtered[3]})
	if len(onlyDisabled) != 1 || onlyDisabled[0].ID != pluginSectionDisabled {
		t.Fatalf("single disabled section = %#v", onlyDisabled)
	}
	if got := groupInstalledPlugins(nil); len(got) != 0 {
		t.Fatalf("empty filter = %#v", got)
	}
}

func installedSectionPluginIDs(section installedPluginSection) []string {
	ids := make([]string, len(section.Plugins))
	for index, entry := range section.Plugins {
		ids[index] = entry.plugin.ID
	}
	return ids
}

func TestFilterPluginsMatchesPinyinAndEnglishName(t *testing.T) {
	plugins := []pluginSettingsPlugin{{
		ID:            "file-search",
		Name:          "文件搜索",
		NameEn:        "File Search",
		Description:   "搜索本地文件",
		DescriptionEn: "Search local files",
	}}

	if got := filterPlugins(plugins, "wenjian", pluginFilterState{}, false, true); len(got) != 1 {
		t.Fatalf("pinyin query matches = %d, want 1", len(got))
	}
	if got := filterPlugins(plugins, "wenjian", pluginFilterState{}, false, false); len(got) != 0 {
		t.Fatalf("pinyin-disabled query matches = %d, want 0", len(got))
	}
	if got := filterPlugins(plugins, "file", pluginFilterState{}, false, false); len(got) != 1 {
		t.Fatalf("english name query matches = %d, want 1", len(got))
	}
	if got := filterPlugins(plugins, "文件", pluginFilterState{}, false, false); len(got) != 1 {
		t.Fatalf("localized name query matches = %d, want 1", len(got))
	}
}

func TestSplitPluginTriggerKeywords(t *testing.T) {
	got := splitPluginTriggerKeywords(" web, translate ,, clipboard ")
	want := []string{"web", "translate", "clipboard"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split keywords = %#v, want %#v", got, want)
	}
}

func TestPluginTriggerKeywordsUseEditableTableAndPersistCoreCSV(t *testing.T) {
	definition := pluginTriggerKeywordDefinition()
	if definition.Type != "table" || !definition.Value.InlineTable || definition.Value.MinimumRowCount != 1 {
		t.Fatalf("trigger keyword definition = %#v, want an inline table with one required row", definition)
	}
	value := encodePluginTriggerKeywordRows([]string{" dictation ", "*"})
	state := &pluginSettingsFormState{
		formFieldsState: formFieldsState{definitions: []formDefinition{definition}, values: map[string]string{"TriggerKeywords": value}},
		initial:         map[string]string{"TriggerKeywords": "[]"},
	}
	_, persisted, err := preparePluginSettingSaveValues(state)
	if err != nil {
		t.Fatalf("prepare trigger keyword save: %v", err)
	}
	if persisted["TriggerKeywords"] != "dictation,*" {
		t.Fatalf("persisted trigger keywords = %q, want core CSV", persisted["TriggerKeywords"])
	}
}

func TestValidatePluginTriggerKeywordTableRowAllowsGlobalAndRejectsInstalledConflict(t *testing.T) {
	plugins := newPluginSettingsController(CommonDeps{})
	plugins.SetPlugins([]pluginSettingsPlugin{
		{ID: "current", Name: "Current"},
		{ID: "other", Name: "Clipboard", TriggerKeywords: []string{"cb"}},
	})
	plugins.SetForm(&pluginSettingsFormState{pluginID: "current"})
	a := &App{pluginSettings: plugins, translations: map[string]string{"ui_plugin_trigger_keyword_duplicate_in_other_plugin": "%s conflict"}}
	state := &formTableEditorState{
		definition: pluginTriggerKeywordDefinition(),
		rowForm:    &formFieldsState{values: map[string]string{"keyword": " cb "}},
		rowIndex:   -1,
	}

	if got := a.validatePluginTriggerKeywordTableRow(state); got != "Clipboard conflict" {
		t.Fatalf("cross-plugin validation = %q, want named conflict", got)
	}
	state.rowForm.values["keyword"] = "*"
	if got := a.validatePluginTriggerKeywordTableRow(state); got != "" {
		t.Fatalf("global trigger validation = %q, want shared global trigger", got)
	}
}

func TestPluginSettingKeepVisibleUsesMeasuredRowKey(t *testing.T) {
	fields := formFieldsSnapshot{definitions: []formDefinition{{}, {}, {}}, focused: 2}
	if got := pluginSettingKeepVisibleKey(fields, 0); got != "" {
		t.Fatalf("inactive plugin form scroll target = %q, want no automatic scrolling", got)
	}
	fields.active = true
	if got := pluginSettingKeepVisibleKey(fields, 1); got != woxwidget.Key("plugin-setting-row-2") {
		t.Fatalf("keep-visible key = %q, want measured third row", got)
	}
	if got := pluginSettingKeepVisibleKey(fields, 3); got != "" {
		t.Fatalf("hidden focused row key = %q, want empty", got)
	}
}

func TestPluginSettingTabMovesOneHostFocusPerPress(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{
		{Type: "textbox", Value: formDefinitionValue{Key: "days"}},
		{Type: "checkbox", Value: formDefinitionValue{Key: "enabled"}},
		{Type: "textbox", Value: formDefinitionValue{Key: "imageDays"}},
		{Type: "checkbox", Value: formDefinitionValue{Key: "ocr"}},
	}, nil, true)
	deps := CommonDeps{}
	plugins := newPluginSettingsController(deps)
	plugins.SetForm(&pluginSettingsFormState{formFieldsState: fields})
	app := &App{settingTab: "plugins", pluginSettings: plugins, hotkeySettings: newHotkeySettingsController(deps)}
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Focusable{Key: "plugin-settings-field-0", OnKey: app.onPluginSettingsKey, Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "plugin-settings-field-1", Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "plugin-settings-field-2", OnKey: app.onPluginSettingsKey, OnFocusChange: func(focused bool) {
				if focused {
					app.focusPluginFormField(2)
				}
			}, Child: woxwidget.Container{Width: 100, Height: 30}},
			woxwidget.Focusable{Key: "plugin-settings-field-3", Child: woxwidget.Container{Width: 100, Height: 30}},
		}}
	})
	host.AttachServices(formTableHostServices{})
	app.settingsHost = host
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 100, Height: 120}, PixelSize: woxui.PixelSize{Width: 100, Height: 120}, Scale: 1})
	host.RequestFocus("plugin-settings-field-0")

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("plugin-settings-field-1") {
		t.Fatal("Tab from a plugin text field did not focus the next plugin setting")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyTab, Down: true}) || !host.HasFocus("plugin-settings-field-2") {
		t.Fatal("Tab from a plugin checkbox did not focus the next plugin setting")
	}
	host.Key(woxui.KeyEvent{Key: woxui.KeyTab})
	if !host.HasFocus("plugin-settings-field-2") {
		t.Fatal("Tab key release advanced plugin focus a second time")
	}
}

func TestPluginCommandsUseHintAndReadonlyTable(t *testing.T) {
	plugins := newPluginSettingsController(CommonDeps{})
	plugins.SetPlugins([]pluginSettingsPlugin{{
		ID: "ai", Name: "AI Commands", Commands: []pluginCommand{
			{Command: "translate", Description: "Translate selection"},
			{Command: "fix", Description: "Fix selection"},
		},
	}})
	plugins.SetSelected(0)
	plugins.SetDetailTab("commands")
	plugins.SetForm(&pluginSettingsFormState{formFieldsState: formFieldsState{definitions: []formDefinition{pluginTriggerKeywordDefinition()}}})
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.uiCall = func(callback func()) error {
		callback()
		return nil
	}
	a.pluginSettings = plugins
	a.translations = map[string]string{
		"ui_plugin_commands_tip":         "Command help",
		"ui_plugin_command_name_column":  "Name",
		"ui_plugin_command_desc_column":  "Description",
		"ui_plugin_no_commands":          "No commands",
		"ui_plugin_tab_commands":         "Commands",
		"ui_plugin_tab_settings":         "Settings",
		"ui_plugin_tab_description":      "Description",
		"ui_plugin_tab_trigger_keywords": "Keywords",
		"ui_plugin_tab_privacy":          "Privacy",
	}
	props := a.pluginDetailProps(settingsSnapshot{plugins: plugins.Snapshot()}, 800, 600, 1)

	if props.Editor == nil || props.Editor.Form == nil || props.Editor.Keywords == nil || props.Editor.Metadata == nil || props.Editor.DescriptionDetail == nil {
		t.Fatal("all installed plugin sections must be built regardless of the previous tab selection")
	}
	if props.Editor == nil || props.Editor.Commands == nil || props.Editor.Commands.Intro != "" || len(props.Editor.Commands.Rows) != 1 {
		t.Fatalf("command form = %#v, want one shared table without a separate hint box", props.Editor)
	}
	table := props.Editor.Commands.Rows[0].(woxwidget.Keyed).Child.(woxwidget.Container)
	tableRows := table.Child.(woxwidget.Flex).Children
	titleBlock := tableRows[0].(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if props.Editor.Commands.SectionLabel != "Commands" || len(titleBlock.Children) != 1 || titleBlock.Children[0].(woxwidget.TextBlock).Value != "Command help" {
		t.Fatal("command group must own the title, with only description above the table")
	}
	grid := tableRows[1].(woxwidget.Stateful)
	state := grid.CreateState()
	state.InitState(woxwidget.StateContext{}, grid.Widget)
	rendered := state.Build(woxwidget.StateContext{}, grid.Widget).(woxwidget.Stack).Children[1].Child.(woxwidget.Flex)
	header := rendered.Children[0].(woxwidget.Flex).Children[0].(woxwidget.ScrollView).Child.(woxwidget.Flex)
	if header.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock).Value != "Name" {
		t.Fatal("command table should expose Flutter's localized name column")
	}
	bodyScroll := rendered.Children[1].(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	body := bodyScroll.Child.(woxwidget.Flex).Children[0].(woxwidget.ScrollView).Child.(woxwidget.Flex)
	firstCell := body.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Clip).Child.(woxwidget.Align).Child.(woxwidget.Semantics)
	if firstCell.Role != woxui.AccessibilityRoleText || firstCell.Label != "fix" || firstCell.Value != "fix" {
		t.Fatalf("first command semantics = %#v, want accessible command text", firstCell)
	}
	firstCommand := firstCell.Child.(woxwidget.TextBlock).Value
	if firstCommand != "fix" {
		t.Fatalf("first command = %q, want Flutter's command sort order", firstCommand)
	}
}

func TestPluginSelectionRefreshKeepsDetailTabForSamePlugin(t *testing.T) {
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.pluginSettings.SetPlugins([]pluginSettingsPlugin{{ID: "app", Name: "Apps"}, {ID: "sys", Name: "System"}})
	a.setPluginSelectionLocked(0)
	a.pluginSettings.SetDetailTab("keywords")

	// Saving a trigger keyword refreshes the same plugin's form; the tab must survive.
	a.setPluginSelectionLocked(0)
	if tab := a.pluginSettings.DetailTab(); tab != "keywords" {
		t.Fatalf("detail tab after same-plugin refresh = %q, want keywords", tab)
	}

	a.setPluginSelectionLocked(1)
	if tab := a.pluginSettings.DetailTab(); tab != "settings" {
		t.Fatalf("detail tab after selecting another plugin = %q, want settings", tab)
	}
}

func TestSetPluginSelectionAppliesCachedAIModels(t *testing.T) {
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.aiSettings.SetModels([]aiModel{{Name: "deepseek-chat", Provider: "deepseek"}})
	a.pluginSettings.SetPlugins([]pluginSettingsPlugin{{
		ID: dictationPluginID,
		SettingDefinitions: []formDefinition{{
			Type: "selectAIModel", Value: formDefinitionValue{Key: dictationDefaultAIModelKey, Label: "AI model"},
		}},
		Setting: pluginSettingsData{Settings: map[string]string{
			dictationActionsKey: `[{"id":"default","type":"default","name":"default","hotkey":"","output":"input","aiRefineEnabled":true}]`,
		}},
	}})
	a.setPluginSelectionLocked(0)

	form := a.pluginSettings.Form()
	if form == nil {
		t.Fatal("plugin form should be built")
	}
	found := false
	for _, definition := range form.definitions {
		if definition.Type != "selectAIModel" {
			continue
		}
		found = true
		if len(definition.Value.Options) == 0 {
			t.Fatal("selectAIModel options should be filled from the cached catalog")
		}
	}
	if !found {
		t.Fatal("dictation form should keep the AI model field")
	}
	if form.values[dictationDefaultAIRefineKey] != "true" {
		t.Fatalf("AI Polish checkbox = %q, want true from persisted actions", form.values[dictationDefaultAIRefineKey])
	}
}

func TestOpenPluginAIModelChoiceUsesCachedCatalogAndEmptyState(t *testing.T) {
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.translations = map[string]string{
		"ui_ai_model_selector_no_models_title":  "No AI provider configured",
		"ui_ai_model_selector_open_ai_settings": "Open AI settings",
		"ui_ai_model_selector_no_models_desc":   "Configure a provider first.",
	}
	definition := formDefinition{Type: "selectAIModel", Value: formDefinitionValue{Key: dictationDefaultAIModelKey, Label: "AI model"}}
	a.pluginSettings.SetForm(&pluginSettingsFormState{
		pluginID:        dictationPluginID,
		formFieldsState: formFieldsState{definitions: []formDefinition{definition}, values: map[string]string{dictationDefaultAIModelKey: ""}, focused: 0, active: true},
		initial:         map[string]string{dictationDefaultAIModelKey: ""},
	})

	a.aiSettings.SetModels([]aiModel{{Name: "deepseek-chat", Provider: "deepseek"}})
	a.openPluginAIModelChoice(0, true, woxui.Rect{Width: 40, Height: 24})
	picker := a.generalSettings.ChoicePicker()
	if picker == nil || len(picker.item.choices) != 1 || picker.item.choices[0].label != "deepseek" {
		t.Fatalf("cached catalog picker = %#v, want one deepseek provider", picker)
	}

	a.generalSettings.SetChoicePicker(nil)
	a.aiSettings.SetModels(nil)
	a.pluginSettings.Form().definitions[0].Value.Options = nil
	a.openPluginAIModelChoice(0, true, woxui.Rect{Width: 40, Height: 24})
	picker = a.generalSettings.ChoicePicker()
	if picker == nil || len(picker.item.choices) != 1 || picker.item.choices[0].value != pluginAIModelOpenSettingsValue {
		t.Fatalf("empty picker = %#v, want an Open AI settings action", picker)
	}
}

func TestPreparePluginSettingSaveValuesTracksDictationDerivedFields(t *testing.T) {
	definition := formDefinition{Type: "dictationHotkey"}
	definition.Value.Key = dictationDefaultHotkeyKey
	state := &pluginSettingsFormState{
		pluginID: dictationPluginID,
		formFieldsState: formFieldsState{
			definitions: []formDefinition{definition},
			values: map[string]string{
				dictationDefaultHotkeyKey:         "left_alt",
				dictationDefaultActionInternalKey: `{"id":"default","type":"default","hotkey":"ctrl+space"}`,
			},
		},
		initial: map[string]string{dictationDefaultHotkeyKey: "ctrl+space"},
	}

	submitted, persisted, err := preparePluginSettingSaveValues(state)
	if err != nil {
		t.Fatalf("prepare dictation save values: %v", err)
	}
	if submitted[dictationDefaultHotkeyKey] != "left_alt" {
		t.Fatalf("submitted hotkey = %q, want left_alt", submitted[dictationDefaultHotkeyKey])
	}
	if _, ok := persisted[dictationDefaultHotkeyKey]; ok {
		t.Fatal("derived hotkey should not be sent as a standalone plugin setting")
	}
	actions := decodeDictationActions(persisted[dictationActionsKey])
	if len(actions) != 1 || dictationString(actions[0]["hotkey"]) != "left_alt" {
		t.Fatalf("persisted actions = %#v, want default left_alt hotkey", actions)
	}

	for key, value := range submitted {
		state.initial[key] = value
	}
	if pluginFormDirty(state.definitions, state.values, state.initial) {
		t.Fatal("saved dictation hotkey should not leave the form dirty")
	}
	state.values[dictationDefaultHotkeyKey] = "cmd+x"
	if !pluginFormDirty(state.definitions, state.values, state.initial) {
		t.Fatal("a newer hotkey edit should remain dirty after the previous save completes")
	}
}

func TestPluginWebsiteButtonsCoverGitHubGistAndOtherSites(t *testing.T) {
	a := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer a.cancel()
	a.translations = map[string]string{"ui_plugin_website": "Website"}
	for _, test := range []struct{ website, label string }{
		{"https://github.com/example/plugin", "GitHub"},
		{"https://gist.github.com/example/123", "GitHub"},
		{"https://example.com/plugin", "Website"},
		{"https://example.com/github.com", "Website"},
		{"", ""},
	} {
		props := a.pluginStoreDetailProps(settingsSnapshot{}, pluginSettingsPlugin{Website: test.website}, 600, 1)
		if props.WebsiteChipLabel != test.label {
			t.Fatalf("website %q label = %q, want %q", test.website, props.WebsiteChipLabel, test.label)
		}
		if (props.OnWebsite != nil) != (test.website != "") {
			t.Fatalf("website %q has incorrect button availability", test.website)
		}
	}
}
