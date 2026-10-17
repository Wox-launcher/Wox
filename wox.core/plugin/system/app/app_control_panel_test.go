package app

import "testing"

func TestMergeControlPanelEntriesKeepsAppletAndUniqueTasks(t *testing.T) {
	applet := controlPanelEntry{
		Name: "Pen and Touch",
		Path: controlPanelRootParsingName + `\0\::{F82DF8F7-8B9F-442E-A48C-818EA735FF9B}`,
	}
	duplicateTask := controlPanelEntry{
		Name: "Pen and Touch",
		Path: controlPanelRootParsingName + `\0\::` + controlPanelAllTasksGUID + `\{11111111-1111-1111-1111-111111111111}`,
	}
	task := controlPanelEntry{
		Name: "Edit the system environment variables",
		Path: controlPanelRootParsingName + `\0\::` + controlPanelAllTasksGUID + `\{E2394C16-F45A-496F-83CC-49E163281662}`,
	}

	got := mergeControlPanelEntries(
		[]controlPanelEntry{
			applet,
			{Name: "  ", Path: applet.Path},
			{Name: "Not a panel", Path: `C:\Windows\notepad.exe`},
			{Name: applet.Name, Path: applet.Path},
		},
		[]controlPanelEntry{duplicateTask, task, task},
	)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2: %#v", len(got), got)
	}
	if got[0] != applet || got[1] != task {
		t.Fatalf("got %#v", got)
	}
}

func TestWindowsResultSubtitleKeySeparatesControlPanel(t *testing.T) {
	panel := appInfo{Name: "Pen and Touch", Path: controlPanelRootParsingName + `\0\::{F82DF8F7-8B9F-442E-A48C-818EA735FF9B}`, Type: AppTypeWindowsSetting}
	if got := windowsResultSubtitleKey(panel); got != "i18n:plugin_app_windows_control_panel_subtitle" {
		t.Fatalf("control panel subtitle = %q", got)
	}
	setting := appInfo{Name: "i18n:plugin_app_windows_settings_system", Path: "ms-settings:system", Type: AppTypeWindowsSetting}
	if got := windowsResultSubtitleKey(setting); got != "i18n:plugin_app_windows_settings_subtitle" {
		t.Fatalf("settings subtitle = %q", got)
	}
	if got := windowsResultSubtitleKey(appInfo{Name: "Notes", Path: `C:\Apps\Notes.exe`, Type: AppTypeDesktop}); got != "" {
		t.Fatalf("desktop subtitle = %q", got)
	}
}
