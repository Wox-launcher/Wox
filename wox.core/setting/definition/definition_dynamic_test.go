package definition

import (
	"context"
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

func TestTranslateReturnsSamePointerWhenUnchanged(t *testing.T) {
	noop := func(_ context.Context, key string) string { return key }
	prefix := func(_ context.Context, key string) string { return "tr:" + key }

	tb := &PluginSettingValueTextBox{Key: "k", Label: "Label", Suffix: "Suffix", Tooltip: "Tip"}
	if got := tb.Translate(noop); got != tb {
		t.Fatal("TextBox.Translate allocated a copy when all strings were unchanged")
	}
	if got := tb.Translate(prefix); got == tb {
		t.Fatal("TextBox.Translate mutated receiver instead of copying on change")
	}

	hk := &PluginSettingValueHotkey{Key: "k", Label: "Label", Tooltip: "Tip"}
	if got := hk.Translate(noop); got != hk {
		t.Fatal("Hotkey.Translate allocated a copy when all strings were unchanged")
	}
	if got := hk.Translate(prefix); got == hk {
		t.Fatal("Hotkey.Translate mutated receiver instead of copying on change")
	}
}
