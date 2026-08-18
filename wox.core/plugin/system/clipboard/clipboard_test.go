package system

import (
	"testing"
	"wox/setting/definition"
	"wox/util"
)

func TestClipboardIgnoredApplicationsSettingUsesSharedAppPicker(t *testing.T) {
	settings := (&ClipboardPlugin{}).GetMetadata().SettingDefinitions
	var privacyHeadFound bool
	var ignoredAppsFound bool
	for _, item := range settings {
		switch value := item.Value.(type) {
		case *definition.PluginSettingValueHead:
			if value.Content == "i18n:plugin_clipboard_privacy" {
				privacyHeadFound = true
			}
		case *definition.PluginSettingValueTable:
			if value.Key != ignoredApplicationsSettingKey {
				continue
			}
			ignoredAppsFound = true
			if !value.InlineTable || value.DefaultValue != "[]" || len(value.Columns) != 1 {
				t.Fatalf("ignored applications table = %+v", value)
			}
			if value.Columns[0].Type != definition.PluginSettingValueTableColumnTypeApp {
				t.Fatalf("ignored applications column type = %q, want app", value.Columns[0].Type)
			}
			if !item.IsPlatformSpecific {
				t.Fatal("ignored applications must be platform specific")
			}
			if len(item.DisabledInPlatforms) != 1 || item.DisabledInPlatforms[0] != util.PlatformLinux {
				t.Fatalf("disabled platforms = %v, want Linux", item.DisabledInPlatforms)
			}
		}
	}
	if !privacyHeadFound || !ignoredAppsFound {
		t.Fatalf("privacy head found=%v ignored applications found=%v", privacyHeadFound, ignoredAppsFound)
	}
}

func TestIgnoredClipboardApplicationMatching(t *testing.T) {
	rows, err := parseIgnoredClipboardApplications(`[{"App":{"Name":"TextEdit","Identity":"com.apple.TextEdit","Path":"/System/Applications/TextEdit.app"}}]`)
	if err != nil {
		t.Fatalf("parse ignored applications: %v", err)
	}
	if !isIgnoredClipboardApplication(rows, " COM.APPLE.TEXTEDIT ") {
		t.Fatal("expected identity to match case-insensitively")
	}
	if isIgnoredClipboardApplication(rows, "com.apple.Safari") {
		t.Fatal("unexpected identity match")
	}
}

func TestParseIgnoredClipboardApplicationsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseIgnoredClipboardApplications("["); err == nil {
		t.Fatal("expected malformed setting to fail")
	}
}
