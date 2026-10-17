package app

import (
	"encoding/json"
	"testing"

	"wox/plugin"
)

func TestDesktopAppsReadsLaunchTarget(t *testing.T) {
	key, err := json.Marshal(struct {
		File       string
		Parameters string
	}{File: `c:\apps\tool.exe`, Parameters: "-open"})
	if err != nil {
		t.Fatal(err)
	}
	application := &ApplicationPlugin{apps: []appInfo{
		{Name: "Tool", Path: `C:\Start\Tool.lnk`, Type: AppTypeDesktop, launchKey: string(key)},
		{Name: "Settings", Path: "ms-settings:display", Type: AppTypeWindowsSetting},
		{Name: "Bare", Path: `C:\Apps\Bare.exe`, Type: AppTypeDesktop, IconSourcePath: `C:\Apps\Bare.exe`},
	}}
	got := application.DesktopApps()
	if len(got) != 2 {
		t.Fatalf("apps = %+v", got)
	}
	if got[0] != (plugin.DesktopApp{Name: "Tool", Path: `C:\Start\Tool.lnk`, Executable: `c:\apps\tool.exe`, Arguments: "-open"}) {
		t.Fatalf("shortcut = %+v", got[0])
	}
	if got[1].Name != "Bare" || got[1].Executable != `C:\Apps\Bare.exe` || got[1].Arguments != "" {
		t.Fatalf("bare = %+v", got[1])
	}
}

func TestSettingsAppsListsControlPanelItems(t *testing.T) {
	panel := `::{26EE0668-A00A-44D7-9371-BEB064C98683}\0\::{ED7BA470-8E54-465E-825C-99712043E01C}\{3D8638E3-EC96-4CE8-8279-36B0065B5F2C}`
	application := &ApplicationPlugin{apps: []appInfo{
		{Name: "Tool", Path: `C:\Start\Tool.lnk`, Type: AppTypeDesktop},
		{Name: "Display", Path: "ms-settings:display", Type: AppTypeWindowsSetting},
		{Name: "使用放大镜放大部分屏幕", Path: panel, Type: AppTypeWindowsSetting},
		{Name: " ", Path: panel, Type: AppTypeWindowsSetting},
	}}
	got := application.SettingsApps()
	if len(got) != 1 || got[0].Name != "使用放大镜放大部分屏幕" || got[0].Path != panel || got[0].Executable != "" {
		t.Fatalf("settings = %+v", got)
	}
}
