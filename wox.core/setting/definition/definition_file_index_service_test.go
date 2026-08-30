package definition

import (
	"encoding/json"
	"testing"
)

func TestFileIndexServiceSettingTypeUnmarshal(t *testing.T) {
	var item PluginSettingDefinitionItem
	err := json.Unmarshal([]byte(`{"Type":"fileIndexService","Value":{"Key":"index-service"}}`), &item)
	if err != nil {
		t.Fatal(err)
	}
	if item.Type != PluginSettingDefinitionTypeFileIndexService {
		t.Fatalf("type = %q", item.Type)
	}
	if _, ok := item.Value.(*PluginSettingValueService); !ok {
		t.Fatalf("value = %T", item.Value)
	}
}
