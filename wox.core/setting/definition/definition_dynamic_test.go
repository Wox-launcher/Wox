package definition

import (
	"encoding/json"
	"testing"
)

func TestDynamicSettingTypeUnmarshal(t *testing.T) {
	var item PluginSettingDefinitionItem
	err := json.Unmarshal([]byte(`{"Type":"dynamic","Value":{"Key":"font_setting"}}`), &item)
	if err != nil {
		t.Fatal(err)
	}
	if item.Type != PluginSettingDefinitionTypeDynamic {
		t.Fatalf("type = %q", item.Type)
	}
	value, ok := item.Value.(*PluginSettingValueDynamic)
	if !ok {
		t.Fatalf("value = %T", item.Value)
	}
	if value.Key != "font_setting" {
		t.Fatalf("key = %q", value.Key)
	}
}
