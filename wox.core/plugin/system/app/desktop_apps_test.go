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
