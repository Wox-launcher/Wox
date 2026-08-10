package app

import "testing"

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
		})
	}
}
