package launcher

import (
	"encoding/json"
	"reflect"
	"testing"
	"wox/setting/definition"
)

func TestFilePathDefinitionAndPicker(t *testing.T) {
	var item definition.PluginSettingDefinitionItem
	if err := json.Unmarshal([]byte(`{"Type":"filePath","Value":{"Key":"file","Label":"File","Extensions":["json",".yaml"],"DefaultValue":"example.json"}}`), &item); err != nil {
		t.Fatal(err)
	}
	form, ok := fromCoreFormDefinition(item)
	if !ok || form.Type != "filePath" || form.Value.DefaultValue != "example.json" || !reflect.DeepEqual(form.Value.Extensions, []string{"json", ".yaml"}) {
		t.Fatalf("converted definition = %+v", form)
	}
	options := formPathDialogOptions(form)
	if options.Directory || options.Filter != "*.json;*.yaml" {
		t.Fatalf("picker options = %+v", options)
	}
	column, ok := formTableColumnDefinition(formTableColumn{Key: "file", Type: "filePath", Extensions: []string{"txt"}}, nil)
	if !ok || formPathDialogOptions(column).Filter != "*.txt" || !formDefinitionFocusable(column) {
		t.Fatalf("table column = %+v", column)
	}
	if got := formPathDialogOptions(formDefinition{Type: "filePath", Value: formDefinitionValue{Extensions: []string{"*", "", "x;*.exe"}}}); got.Filter != "" {
		t.Fatalf("invalid extensions = %+v", got)
	}
	if got := formPathDialogOptions(formDefinition{Type: "dirPath"}); !got.Directory || got.Filter != "" {
		t.Fatalf("directory options = %+v", got)
	}
}
