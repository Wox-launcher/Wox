package app

import "testing"

func TestWindowsNewWindowCapabilityUsesExecutableNotDisplayName(t *testing.T) {
	for _, tc := range []struct {
		info appInfo
		want bool
	}{
		{appInfo{Name: "Renamed Browser", Path: `C:\Apps\firefox.exe`, Type: AppTypeDesktop}, true},
		{appInfo{Name: "Firefox", Path: `C:\Apps\unknown.exe`, Type: AppTypeDesktop}, false},
		{appInfo{Path: `C:\Apps\missing.lnk`, Type: AppTypeDesktop}, false},
		{appInfo{Path: `C:\Apps\firefox.url`, Type: AppTypeDesktop}, false},
		{appInfo{Path: "ms-settings:display", Type: AppTypeWindowsSetting}, false},
		{appInfo{Path: `shell:AppsFolder\Example!App`, Type: AppTypeUWP}, false},
	} {
		if got := supportsAppNewWindow(tc.info); got != tc.want {
			t.Errorf("%+v: supported=%t, want %t", tc.info, got, tc.want)
		}
	}
}
