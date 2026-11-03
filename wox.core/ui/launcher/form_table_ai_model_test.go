package launcher

import (
	"encoding/json"
	"testing"

	woxui "wox/ui/runtime"
)

func TestFormTableAIModelSelection(t *testing.T) {
	models := []aiModel{{Provider: "first", Name: "one"}, {Provider: "first", Name: "two"}, {Provider: "second", Name: "three"}}
	fields := newFormFieldsState([]formDefinition{{Type: "selectAIModel", Value: formDefinitionValue{Key: "model"}}, {Type: "textbox", Value: formDefinitionValue{Key: "name"}}}, map[string]string{"model": "", "name": "untouched"}, true)
	applyAIModelOptionsLocked(&fields, models)
	app := bindTestApp(&App{formTables: formTableController{launcherEditor: &formTableEditorState{rowForm: &fields}}})
	c := &app.formTables
	c.openFormTableAIModelChoice(0, true, woxui.Rect{})
	if c.launcherEditor.choicePicker == nil || len(c.launcherEditor.choicePicker.options) != 2 {
		t.Fatal("expected two providers")
	}
	c.chooseFormTableChoice(1)
	var selected aiModel
	if err := json.Unmarshal([]byte(fields.values["model"]), &selected); err != nil || selected.Provider != "second" || selected.Name != "three" {
		t.Fatalf("provider selection: %+v, %v", selected, err)
	}
	c.openFormTableAIModelChoice(0, false, woxui.Rect{})
	if c.launcherEditor.choicePicker == nil || len(c.launcherEditor.choicePicker.options) != 1 || !formTableChoicePickerFilterable(c.launcherEditor) {
		t.Fatal("expected only the selected provider's models")
	}
	c.chooseFormTableChoice(0)
	c.setFormTableAIModelName(0, "custom")
	if err := json.Unmarshal([]byte(fields.values["model"]), &selected); err != nil || selected.Name != "custom" || fields.values["name"] != "untouched" {
		t.Fatalf("custom model: %+v, %v", selected, err)
	}
	c.finishFormTableAIModelEdit(0, "custom")
	if err := json.Unmarshal([]byte(fields.values["model"]), &selected); err != nil || selected.Name != "three" {
		t.Fatalf("configured model: %+v, %v", selected, err)
	}
}
