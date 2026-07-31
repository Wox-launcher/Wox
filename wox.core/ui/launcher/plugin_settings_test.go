package launcher

import (
	"reflect"
	"testing"
)

func TestSplitPluginTriggerKeywords(t *testing.T) {
	got := splitPluginTriggerKeywords(" web, translate ,, clipboard ")
	want := []string{"web", "translate", "clipboard"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("split keywords = %#v, want %#v", got, want)
	}
}

func TestFocusPluginFormHotkeyReleasesSearchFocus(t *testing.T) {
	pluginController := newPluginSettingsController(CommonDeps{})
	hotkeyController := newHotkeySettingsController(CommonDeps{})
	form := newFormFieldsState([]formDefinition{{
		Type: "dictationHotkey", Value: formDefinitionValue{Key: "Hotkey", Label: "Hotkey"},
	}}, map[string]string{"Hotkey": "cmd+a"}, true)
	pluginController.SetForm(&pluginSettingsFormState{formFieldsState: form})
	pluginController.SetSearchFocused(true)
	app := &App{pluginSettings: pluginController, hotkeySettings: hotkeyController}

	app.focusPluginFormField(0)

	if pluginController.SearchFocused() {
		t.Fatal("plugin search should lose focus when the hotkey field takes focus")
	}
	if focused := pluginController.Form().focused; focused != 0 {
		t.Fatalf("plugin form focused field = %d, want 0", focused)
	}
}
