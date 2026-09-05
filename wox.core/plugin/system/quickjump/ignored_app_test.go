package quickjump

import (
	"testing"
	"wox/common"
	"wox/setting/definition"
	"wox/util/window"
)

func TestQuickJumpPluginUsesDedicatedIcon(t *testing.T) {
	icon := (&QuickJumpPlugin{}).GetMetadata().Icon
	if icon != common.PluginQuickJumpIcon.String() {
		t.Fatalf("plugin icon = %q, want dedicated quick jump icon", icon)
	}
}

func TestExplorerSettingsOmitTypeToSearchToggle(t *testing.T) {
	for _, item := range (&QuickJumpPlugin{}).GetMetadata().SettingDefinitions {
		if item.Value != nil && item.Value.GetKey() == "enableTypeToSearch" {
			t.Fatal("type-to-search is always on; the enable setting must not appear")
		}
	}
}

func TestExplorerIgnoredApplicationsSettingUsesSharedAppPicker(t *testing.T) {
	settings := (&QuickJumpPlugin{}).GetMetadata().SettingDefinitions
	var ignoredAppsFound bool
	for _, item := range settings {
		value, ok := item.Value.(*definition.PluginSettingValueTable)
		if !ok || value.Key != ignoredApplicationsSettingKey {
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
	}
	if !ignoredAppsFound {
		t.Fatal("ignored applications setting is missing")
	}
}

func TestIgnoredExplorerApplicationMatching(t *testing.T) {
	rows, err := parseIgnoredExplorerApplications(`[{"App":{"Name":"TextEdit","Identity":"com.apple.TextEdit","Path":"/System/Applications/TextEdit.app"}}]`)
	if err != nil {
		t.Fatalf("parse ignored applications: %v", err)
	}
	if !isIgnoredExplorerApplication(rows, " COM.APPLE.TEXTEDIT ") {
		t.Fatal("expected identity to match case-insensitively")
	}
	if isIgnoredExplorerApplication(rows, "com.apple.Safari") {
		t.Fatal("unexpected identity match")
	}
}

func TestParseIgnoredExplorerApplicationsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseIgnoredExplorerApplications("["); err == nil {
		t.Fatal("expected malformed setting to fail")
	}
}

func TestIgnoredApplicationPidFailClosed(t *testing.T) {
	plugin := &QuickJumpPlugin{}
	plugin.ignoredApps.failClosed = true
	if !plugin.isIgnoredApplicationPid(1) {
		t.Fatal("corrupt ignore list must fail closed")
	}

	plugin.ignoredApps.failClosed = false
	if plugin.isIgnoredApplicationPid(1) {
		t.Fatal("empty ignore list should not ignore")
	}
}

func TestParseIgnoredExplorerApplicationsTreatsEmptyAsNone(t *testing.T) {
	rows, err := parseIgnoredExplorerApplications("   ")
	if err != nil {
		t.Fatalf("empty setting should parse: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("empty setting rows = %+v", rows)
	}
}

func TestIsMonitoredAppIgnoredUsesInstalledChecker(t *testing.T) {
	t.Cleanup(func() { setIgnoreMonitoredApp(nil) })

	if isMonitoredAppIgnored() {
		t.Fatal("empty checker should not ignore")
	}

	called := false
	setIgnoreMonitoredApp(func(pid int) bool {
		called = true
		return pid > 0
	})
	ignored := isMonitoredAppIgnored()
	if window.GetActiveWindowPid() <= 0 {
		if called || ignored {
			t.Fatal("missing foreground pid should not consult checker")
		}
		return
	}
	if !called || !ignored {
		t.Fatal("expected foreground pid to be ignored")
	}

	setIgnoreMonitoredApp(func(int) bool { return false })
	if isMonitoredAppIgnored() {
		t.Fatal("checker that rejects all pids should not ignore")
	}
}
