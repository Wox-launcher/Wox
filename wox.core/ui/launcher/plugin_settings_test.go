package launcher

import (
	"reflect"
	"testing"

	woxwidget "wox/ui/widget"
)

func TestSplitPluginTriggerKeywords(t *testing.T) {
	got := splitPluginTriggerKeywords(" web, translate ,, clipboard ")
	want := []string{"web", "translate", "clipboard"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split keywords = %#v, want %#v", got, want)
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
