package websearch

import (
	"encoding/json"
	"testing"
	"wox/setting/definition"
)

func TestWebSearchOpenInPrivateDefaultsOff(t *testing.T) {
	var search webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g","Title":"Google","Urls":["https://www.google.com/search?q=wox"],"Enabled":true}`), &search); err != nil {
		t.Fatal(err)
	}
	if search.OpenInPrivate {
		t.Fatal("missing OpenInPrivate must stay unchecked")
	}

	if err := json.Unmarshal([]byte(`{"Keyword":"g","OpenInPrivate":true}`), &search); err != nil {
		t.Fatal(err)
	}
	if !search.OpenInPrivate {
		t.Fatal("saved OpenInPrivate should round-trip")
	}
}

func TestWebSearchMetadataIncludesPrivateColumn(t *testing.T) {
	metadata := (&WebSearchPlugin{}).GetMetadata()
	var table *definition.PluginSettingValueTable
	for _, item := range metadata.SettingDefinitions {
		value, ok := item.Value.(*definition.PluginSettingValueTable)
		if ok && value.Key == webSearchesSettingKey {
			table = value
			break
		}
	}
	if table == nil {
		t.Fatal("web search table setting is missing")
	}

	for _, column := range table.Columns {
		if column.Key != "OpenInPrivate" {
			continue
		}
		if column.Type != definition.PluginSettingValueTableColumnTypeCheckbox {
			t.Fatalf("OpenInPrivate type = %q", column.Type)
		}
		if column.Tooltip == "" {
			t.Fatal("OpenInPrivate is missing a tooltip")
		}
		return
	}
	t.Fatal("OpenInPrivate column was not registered")
}
