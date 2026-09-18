package app

import (
	"path/filepath"
	"testing"
	"wox/util"
)

func TestBuildAppActionsIncludesAdministratorActionForExecutableApps(t *testing.T) {
	plugin := &ApplicationPlugin{}

	testCases := []struct {
		name            string
		info            appInfo
		wantAdminAction bool
	}{
		{name: "executable", info: appInfo{Path: `C:\Apps\Editor.exe`, Type: AppTypeDesktop}, wantAdminAction: true},
		{name: "shortcut", info: appInfo{Path: `C:\Apps\Editor.lnk`, Type: AppTypeDesktop}, wantAdminAction: true},
		{name: "url shortcut", info: appInfo{Path: `C:\Apps\Editor.url`, Type: AppTypeDesktop}},
		{name: "full trust packaged app", info: appInfo{Path: `shell:AppsFolder\Example.App_123!App`, Type: AppTypeUWP, CanRunAsAdministrator: true}, wantAdminAction: true},
		{name: "sandboxed UWP app", info: appInfo{Path: `shell:AppsFolder\Example.Sandbox_123!App`, Type: AppTypeUWP}},
		{name: "browser web app", info: appInfo{Path: `shell:AppsFolder\https://example.com`, Type: AppTypeAppsFolder}},
		{name: "Windows setting", info: appInfo{Path: "ms-settings:display", Type: AppTypeWindowsSetting}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actions := plugin.buildAppActions(testCase.info, "Editor", nil)
			hasAdminAction := false
			for _, action := range actions {
				if action.Name == "i18n:plugin_app_open_as_administrator" {
					hasAdminAction = true
					break
				}
			}

			if hasAdminAction != testCase.wantAdminAction {
				t.Fatalf("administrator action presence = %t, want %t", hasAdminAction, testCase.wantAdminAction)
			}

			hasUninstallAction := false
			for _, action := range actions {
				if action.Name == "i18n:plugin_app_uninstall" {
					hasUninstallAction = true
					break
				}
			}
			wantUninstallAction := shouldOfferWindowsUninstall(testCase.info) && util.IsWindows()
			if hasUninstallAction != wantUninstallAction {
				t.Fatalf("uninstall action presence = %t, want %t", hasUninstallAction, wantUninstallAction)
			}
		})
	}
}

func TestBuildAppActionsOmitsFileActionsForAppsFolderEntries(t *testing.T) {
	actions := (&ApplicationPlugin{}).buildAppActions(appInfo{Path: `shell:AppsFolder\https://example.com`, Type: AppTypeAppsFolder}, "Example", nil)
	for _, action := range actions {
		if action.Name == "i18n:plugin_app_open_containing_folder" || action.Name == "i18n:plugin_file_show_context_menu" {
			t.Fatalf("unexpected file action %q", action.Name)
		}
	}
}

func TestBuildAppActionsIncludesCopyName(t *testing.T) {
	actions := (&ApplicationPlugin{}).buildAppActions(appInfo{Name: "Notes", Path: `C:\Apps\Notes.exe`}, "Notes", nil)
	hasCopyPath := false
	hasCopyName := false
	for _, action := range actions {
		if action.Name == "i18n:plugin_app_copy_path" {
			hasCopyPath = true
		}
		if action.Name == "i18n:plugin_app_copy_name" {
			hasCopyName = true
		}
	}
	if !hasCopyPath {
		t.Fatal("expected copy path action")
	}
	if !hasCopyName {
		t.Fatal("expected copy name action")
	}
}

func TestAppCopyNamePrefersDisplayName(t *testing.T) {
	info := appInfo{Name: "Code", Path: filepath.Join("Apps", "Code.exe")}
	if got := appCopyName("Visual Studio Code", info); got != "Visual Studio Code" {
		t.Fatalf("display name = %q, want Visual Studio Code", got)
	}
	if got := appCopyName("", info); got != "Code" {
		t.Fatalf("indexed name = %q, want Code", got)
	}
	if got := appCopyName("", appInfo{Name: "i18n:plugin_app_windows_settings_system_display", Path: "ms-settings:display"}); got != "ms-settings:display" {
		t.Fatalf("i18n fallback = %q, want ms-settings:display", got)
	}
	if got := appCopyName("", appInfo{Path: filepath.Join("Apps", "Notes.exe")}); got != "Notes.exe" {
		t.Fatalf("path leaf = %q, want Notes.exe", got)
	}
}
