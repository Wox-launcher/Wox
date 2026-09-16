package websearch

import (
	"encoding/json"
	"testing"
	"wox/setting/definition"
)

func TestWebSearchLegacyEnabledJSON(t *testing.T) {
	var enabled webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g","Enabled":true}`), &enabled); err != nil || enabled.Disabled {
		t.Fatalf("Enabled true should stay active: %+v err=%v", enabled, err)
	}
	var disabled webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g","Enabled":false}`), &disabled); err != nil || !disabled.Disabled {
		t.Fatalf("Enabled false should become disabled: %+v err=%v", disabled, err)
	}
	var migrated webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g","Disabled":true,"Enabled":true}`), &migrated); err != nil || !migrated.Disabled {
		t.Fatalf("existing Disabled must win: %+v err=%v", migrated, err)
	}
	var fresh webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g"}`), &fresh); err != nil || fresh.Disabled {
		t.Fatalf("new rows without either flag stay on: %+v err=%v", fresh, err)
	}
}

func TestWebSearchWebViewFieldsDefaultEmpty(t *testing.T) {
	var search webSearch
	if err := json.Unmarshal([]byte(`{"Keyword":"g","Title":"Google","Urls":["https://www.google.com/search?q=wox"],"Enabled":true}`), &search); err != nil {
		t.Fatal(err)
	}
	if search.WebViewWidth != 0 || search.WebViewHeight != 0 || search.InjectCSS != "" {
		t.Fatalf("legacy rows should keep empty WebView chrome, got %+v", search)
	}
}

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

func TestWebSearchMetadataGroupsAdvancedAndWebViewColumns(t *testing.T) {
	metadata := (&WebSearchPlugin{}).GetMetadata()
	var table *definition.PluginSettingValueTable
	for _, item := range metadata.SettingDefinitions {
		value, ok := item.Value.(*definition.PluginSettingValueTable)
		if ok && value.Key == webSearchesSettingKey {
			table = value
			break
		}
	}
	if table == nil || len(table.Groups) != 2 || table.Groups[0].Key != webSearchTableGroupAdvanced || !table.Groups[0].CollapsedByDefault ||
		table.Groups[1].Key != webSearchTableGroupWebView || !table.Groups[1].CollapsedByDefault {
		t.Fatalf("groups = %#v", table)
	}
	got := map[string]string{}
	for _, column := range table.Columns {
		got[column.Key] = column.Group
		if column.Key == "Keyword" || column.Key == "Title" || column.Key == "Urls" || column.Key == "Browser" || column.Key == "Disabled" {
			if column.Group != "" {
				t.Fatalf("%s should stay ungrouped", column.Key)
			}
		}
		if column.Key == "Disabled" && (column.Type != definition.PluginSettingValueTableColumnTypeCheckbox || column.Label != "i18n:ui_disabled") {
			t.Fatalf("Disabled column = %#v", column)
		}
	}
	if _, ok := got["Disabled"]; !ok {
		t.Fatal("Disabled column was not registered")
	}
	if got["OpenInPrivate"] != webSearchTableGroupAdvanced || got["IsFallback"] != webSearchTableGroupAdvanced {
		t.Fatalf("advanced columns = %#v", got)
	}
	if got["WebViewWidth"] != webSearchTableGroupWebView || got["WebViewHeight"] != webSearchTableGroupWebView || got["InjectCSS"] != webSearchTableGroupWebView {
		t.Fatalf("webview columns = %#v", got)
	}
}
