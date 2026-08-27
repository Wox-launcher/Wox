package system

import (
	"os"
	"path/filepath"
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

func TestResolveClipboardFilesystemPathAcceptsFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	dir, isDir := resolveClipboardFilesystemPath(`"` + root + `"`)
	if !isDir || dir != filepath.Clean(root) {
		t.Fatalf("directory path = %q isDir=%v", dir, isDir)
	}
	file, isDir := resolveClipboardFilesystemPath(filePath)
	if isDir || file != filepath.Clean(filePath) {
		t.Fatalf("file path = %q isDir=%v", file, isDir)
	}
	if path, ok := resolveClipboardFilesystemPath("relative/path"); path != "" || ok {
		t.Fatalf("relative path should be rejected: %q %v", path, ok)
	}
}

func TestParseIgnoredClipboardApplicationsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseIgnoredClipboardApplications("["); err == nil {
		t.Fatal("expected malformed setting to fail")
	}
}
