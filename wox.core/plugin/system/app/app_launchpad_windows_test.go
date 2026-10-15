package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"wox/common"

	"golang.org/x/sys/windows"
)

// TestLaunchpadCustomDirectoryScope shares scanner recursion rules and verifies
// removing a configured directory stops admitting its otherwise unrelated apps.
func TestLaunchpadCustomDirectoryScope(t *testing.T) {
	root := appDirectory{Path: `D:\Portable`, Recursive: true, RecursiveDepth: 3}
	for _, tt := range []struct {
		path string
		want bool
	}{
		{`D:\Portable\App.exe`, true},
		{`d:\portable\one\two\three\App.exe`, true},
		{`D:\Portable\one\two\three\four\App.exe`, false},
		{`D:\PortableOther\App.exe`, false},
	} {
		info := appInfo{Path: tt.path, Type: AppTypeDesktop}
		if got := windowsLaunchpadEntry(info, nil, root); got != tt.want {
			t.Errorf("custom scope(%s) = %v", tt.path, got)
		}
		if windowsLaunchpadEntry(info, nil) {
			t.Errorf("removed root retained %s", tt.path)
		}
	}
	var defaults []appDirectory
	if err := json.Unmarshal([]byte(defaultAppDirectories()), &defaults); err != nil {
		t.Fatal(err)
	}
	desktop, err := windows.KnownFolderPath(windows.FOLDERID_Desktop, windows.KF_FLAG_DONT_VERIFY)
	if err != nil {
		t.Fatal(err)
	}
	if len(defaults) != 1 || defaults[0].Path != desktop {
		t.Fatalf("desktop default = %+v", defaults)
	}
}

// TestWindowsLaunchpadEntryScope covers redirected and case-insensitive Start
// Menu roots without accidentally admitting desktop or installation helpers.
func TestWindowsLaunchpadEntryScope(t *testing.T) {
	roots := []string{`D:\Redirected\Start Menu`, `C:\ProgramData\Microsoft\Windows\Start Menu`}
	for _, tt := range []struct {
		path string
		kind AppType
		want bool
	}{
		{`d:\redirected\START MENU\Programs\Music.lnk`, AppTypeDesktop, true},
		{`C:\ProgramData\Microsoft\Windows\Start Menu\Programs\Vendor\App.lnk`, AppTypeDesktop, true},
		{`D:\Redirected\Start Menu Backup\App.lnk`, AppTypeDesktop, false},
		{`C:\Users\user\Desktop\App.lnk`, AppTypeDesktop, false},
		{`C:\Program Files\App\helper.exe`, AppTypeDesktop, false},
		{`C:\Windows\System32\tool.exe`, AppTypeDesktop, false},
		{`shell:AppsFolder\Music!App`, AppTypeUWP, true},
		{`shell:AppsFolder\Tools`, AppTypeAppsFolder, true},
		{"ms-settings:display", AppTypeWindowsSetting, false},
	} {
		if got := windowsLaunchpadEntry(appInfo{Path: tt.path, Type: tt.kind}, roots); got != tt.want {
			t.Errorf("scope(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestLaunchpadRejectsGenericExecutableIcon checks that a shell fallback cannot
// pass as dedicated artwork, while ordinary search metadata remains intact.
func TestLaunchpadRejectsGenericExecutableIcon(t *testing.T) {
	source := filepath.Join(t.TempDir(), "helper.exe")
	if err := os.WriteFile(source, []byte("no embedded icon"), 0600); err != nil {
		t.Fatal(err)
	}
	icon := common.NewWoxImageAbsolutePath(`C:\cache\generic.png`)
	info := appInfo{Path: extraAppPath("Tools"), Type: AppTypeAppsFolder, IconSourcePath: source, Icon: icon}
	populateLaunchpadIconMetadata(context.Background(), &info)
	populateLaunchpadVisibility(context.Background(), &info)
	app := &ApplicationPlugin{}
	if app.shouldShowInLaunchpad(info) {
		t.Fatal("generic executable icon is visible in Launchpad")
	}
	if info.IsDefaultIcon || app.getQueryResultIcon(info, false) != icon {
		t.Fatal("Launchpad classification changed ordinary search metadata")
	}
	info = appInfo{Path: extraAppPath("Music!App"), Type: AppTypeUWP, Icon: icon}
	populateLaunchpadIconMetadata(context.Background(), &info)
	populateLaunchpadVisibility(context.Background(), &info)
	if !app.shouldShowInLaunchpad(info) {
		t.Fatal("packaged app artwork was hidden")
	}
	info.IsDefaultIcon = true
	if app.shouldShowInLaunchpad(info) {
		t.Fatal("default packaged app artwork was shown")
	}
}

// TestLaunchpadRejectsAppsFolderFilesWithoutArtwork covers management consoles
// that previously bypassed the executable-only check simply by being in AppsFolder.
func TestLaunchpadRejectsAppsFolderFilesWithoutArtwork(t *testing.T) {
	for _, name := range []string{"printmanagement.msc", "services.msc", "comexp.msc", "WF.msc", "missing.ico"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), name)
			if err := os.WriteFile(source, []byte("no embedded artwork"), 0600); err != nil {
				t.Fatal(err)
			}
			info := appInfo{Path: extraAppPath(appsFolderSystem32GUID + name), Type: AppTypeAppsFolder, IconSourcePath: source, Icon: common.NewWoxImageAbsolutePath(`C:\cache\generic.png`)}
			populateLaunchpadIconMetadata(context.Background(), &info)
			populateLaunchpadVisibility(context.Background(), &info)
			if (&ApplicationPlugin{}).shouldShowInLaunchpad(info) {
				t.Fatal("AppsFolder file without dedicated artwork was shown")
			}
		})
	}
}

// TestLaunchpadIconMetadataSurvivesCacheReload proves query rebuilds use the indexed
// classification even when the native source is inaccessible until the next index refresh.
func TestLaunchpadIconMetadataSurvivesCacheReload(t *testing.T) {
	info := appInfo{
		Name:             "Music",
		Path:             extraAppPath("Music!App"),
		Type:             AppTypeUWP,
		Icon:             common.NewWoxImageAbsolutePath(`C:\cache\music.png`),
		IconSourcePath:   filepath.Join(t.TempDir(), "unavailable.exe"),
		HasDedicatedIcon: true,
	}
	data, err := json.Marshal(appCacheFile{Version: appCacheVersion, Apps: []appInfo{info}})
	if err != nil {
		t.Fatal(err)
	}
	apps, err := parseAppCacheContent(data)
	if err != nil {
		t.Fatal(err)
	}
	app := &ApplicationPlugin{apps: apps}
	for range 2 {
		app.rebuildQueryEntries(context.Background())
		entries, _ := app.getQueryEntriesSnapshot()
		if len(entries) != 1 || !app.shouldShowInLaunchpad(entries[0].info) {
			t.Fatalf("query rebuild lost the cached icon classification: %+v", entries)
		}
	}
}

// TestLaunchpadUsesResolvedWindowsIconSource prevents shortcut and AppsFolder
// launch identities from replacing dedicated artwork with generic file icons.
func TestLaunchpadUsesResolvedWindowsIconSource(t *testing.T) {
	indexed := common.NewWoxImageAbsolutePath(`C:\cache\dedicated.png`)
	tests := []struct {
		name string
		info appInfo
		want common.WoxImage
	}{
		{"shortcut", appInfo{Path: `C:\Start Menu\QQ音乐.lnk`, IconSourcePath: `C:\QQMusic\QQMusic.exe`, Type: AppTypeDesktop}, common.NewWoxImageFileIcon(`C:\QQMusic\QQMusic.exe`)},
		{"url resource", appInfo{Path: `C:\Desktop\Music.url`, IconSourcePath: `C:\Music\music.ico`, Type: AppTypeDesktop}, common.NewWoxImageFileIcon(`C:\Music\music.ico`)},
		{"apps folder executable", appInfo{Path: `shell:AppsFolder\Tools`, IconSourcePath: `C:\Windows\tool.exe`, Type: AppTypeAppsFolder}, common.NewWoxImageFileIcon(`C:\Windows\tool.exe`)},
		{"packaged app", appInfo{Path: `shell:AppsFolder\Music!App`, Type: AppTypeUWP}, indexed},
		{"shell artwork", appInfo{Path: `shell:AppsFolder\Music`, Type: AppTypeAppsFolder}, indexed},
		{"settings", appInfo{Path: "ms-settings:display", Type: AppTypeWindowsSetting}, indexed},
		{"missing source", appInfo{Path: `C:\Start Menu\Music.lnk`, Type: AppTypeDesktop}, indexed},
	}
	app := &ApplicationPlugin{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.info.Icon = indexed
			if got := app.getQueryResultIcon(tt.info, true); got != tt.want {
				t.Fatalf("launchpad icon = %+v, want %+v", got, tt.want)
			}
			if got := app.getQueryResultIcon(tt.info, false); got != indexed {
				t.Fatalf("list icon changed: %+v", got)
			}
		})
	}
}
