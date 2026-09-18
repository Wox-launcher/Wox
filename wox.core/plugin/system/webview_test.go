package system

import (
	"testing"

	"wox/setting/definition"
	"wox/setting/validator"
)

func TestIsWebViewURL(t *testing.T) {
	if !isWebViewURL("https://m.nmc.cn/publish/forecast/AGD/guangzhou.html") {
		t.Fatal("absolute HTTPS URL was rejected")
	}
	if isWebViewURL("m.nmc.cn/publish/forecast/AGD/guangzhou.html") {
		t.Fatal("URL without a scheme was accepted")
	}
}

func TestWebViewURLValidatorIsBoundToURLColumn(t *testing.T) {
	metadata := (&WebViewPlugin{}).GetMetadata()
	if len(metadata.SettingDefinitions) != 1 {
		t.Fatalf("setting definitions = %d, want 1", len(metadata.SettingDefinitions))
	}
	table, ok := metadata.SettingDefinitions[0].Value.(*definition.PluginSettingValueTable)
	if !ok {
		t.Fatalf("setting value type = %T, want table", metadata.SettingDefinitions[0].Value)
	}
	for _, column := range table.Columns {
		hasURLValidator := false
		for _, item := range column.Validators {
			if item.Type == validator.PluginSettingValidatorTypeIsURL {
				hasURLValidator = true
			}
		}
		if column.Key == "Url" && !hasURLValidator {
			t.Fatal("Url column is missing is_url validator")
		}
		if column.Key != "Url" && hasURLValidator {
			t.Fatalf("is_url validator is incorrectly bound to %s column", column.Key)
		}
	}
}

func TestResolveWebviewUserAgent(t *testing.T) {
	for _, test := range []struct {
		value string
		want  string
	}{
		{value: "", want: ""},
		{value: "auto", want: ""},
		{value: webviewUserAgentDesktopSafari, want: ""},
		{value: webviewUserAgentMobileSafari, want: webviewMobileSafariUserAgent},
		{value: " custom-agent ", want: "custom-agent"},
	} {
		if got := resolveWebviewUserAgent(test.value); got != test.want {
			t.Fatalf("resolveWebviewUserAgent(%q) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestParseWebviewSitesMapsLegacyCacheDisabled(t *testing.T) {
	sites, err := parseWebviewSites(`[{"Keyword":"x","CacheDisabled":false},{"Keyword":"fresh","CacheDisabled":true},{"Keyword":"legacy"},{"Keyword":"kept","KeepInBackground":false,"CacheDisabled":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 4 {
		t.Fatalf("sites = %d, want 4", len(sites))
	}
	if !sites[0].KeepInBackground || sites[1].KeepInBackground || !sites[2].KeepInBackground || sites[3].KeepInBackground {
		t.Fatalf("KeepInBackground = %+v", []bool{sites[0].KeepInBackground, sites[1].KeepInBackground, sites[2].KeepInBackground, sites[3].KeepInBackground})
	}
}

func TestWebViewKeepInBackgroundSettingIsAvailable(t *testing.T) {
	metadata := (&WebViewPlugin{}).GetMetadata()
	table := metadata.SettingDefinitions[0].Value.(*definition.PluginSettingValueTable)
	for _, column := range table.Columns {
		if column.Key == "KeepInBackground" {
			if column.Type != definition.PluginSettingValueTableColumnTypeCheckbox || !column.HideInTable {
				t.Fatalf("KeepInBackground column = %+v", column)
			}
			return
		}
		if column.Key == "CacheDisabled" {
			t.Fatal("CacheDisabled must be replaced by KeepInBackground")
		}
	}
	t.Fatal("KeepInBackground setting column is missing")
}

func TestWebViewUserAgentSettingIsAvailable(t *testing.T) {
	metadata := (&WebViewPlugin{}).GetMetadata()
	table := metadata.SettingDefinitions[0].Value.(*definition.PluginSettingValueTable)
	for _, column := range table.Columns {
		if column.Key == "UserAgent" {
			if column.Type != definition.PluginSettingValueTableColumnTypeSelect || len(column.SelectOptions) < 3 {
				t.Fatalf("UserAgent column = %+v", column)
			}
			return
		}
	}
	t.Fatal("UserAgent setting column is missing")
}
