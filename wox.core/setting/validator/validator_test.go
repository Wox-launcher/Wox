package validator

import (
	"encoding/json"
	"testing"
)

func TestPluginSettingValidatorUnmarshalsIsURL(t *testing.T) {
	var item PluginSettingValidator
	if err := json.Unmarshal([]byte(`{"Type":"is_url","Value":{}}`), &item); err != nil {
		t.Fatalf("unmarshal is_url validator: %v", err)
	}
	if item.Type != PluginSettingValidatorTypeIsURL {
		t.Fatalf("validator type = %q, want %q", item.Type, PluginSettingValidatorTypeIsURL)
	}
	if _, ok := item.Value.(*PluginSettingValidatorIsURL); !ok {
		t.Fatalf("validator value type = %T, want *PluginSettingValidatorIsURL", item.Value)
	}
}
