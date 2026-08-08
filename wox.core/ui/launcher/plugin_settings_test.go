package launcher

import (
	"reflect"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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

	if props.Editor == nil || props.Editor.Form == nil || props.Editor.Form.Intro != "Command help" || len(props.Editor.Form.Rows) != 1 {
		t.Fatalf("command form = %#v, want hint and one shared table", props.Editor)
	}
	table := props.Editor.Form.Rows[0].(woxwidget.Keyed).Child.(woxwidget.Container)
	grid := table.Child.(woxwidget.Flex).Children[0].(woxwidget.Stateful)
	state := grid.CreateState()
	state.InitState(woxwidget.StateContext{}, grid.Widget)
	rendered := state.Build(woxwidget.StateContext{}, grid.Widget).(woxwidget.Container).Child.(woxwidget.Flex)
	header := rendered.Children[0].(woxwidget.Flex).Children[0].(woxwidget.ScrollView).Child.(woxwidget.Flex)
	if header.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock).Value != "Name" {
		t.Fatal("command table should expose Flutter's localized name column")
	}
	bodyScroll := rendered.Children[1].(woxwidget.Gesture).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.ScrollView)
	body := bodyScroll.Child.(woxwidget.Flex).Children[0].(woxwidget.ScrollView).Child.(woxwidget.Flex)
	firstCommand := body.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.TextBlock).Value
	if firstCommand != "fix" {
		t.Fatalf("first command = %q, want Flutter's command sort order", firstCommand)
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
