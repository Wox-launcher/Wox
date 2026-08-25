package app

import (
	"testing"
	"wox/util"
)

func TestSplitUninstallCommand(t *testing.T) {
	testCases := []struct {
		name       string
		command    string
		wantFile   string
		wantParams string
	}{
		{name: "quoted path with args", command: `"C:\Program Files\PixPin\unins000.exe" /SILENT`, wantFile: `C:\Program Files\PixPin\unins000.exe`, wantParams: "/SILENT"},
		{name: "quoted path only", command: `"C:\Program Files\PixPin\unins000.exe"`, wantFile: `C:\Program Files\PixPin\unins000.exe`},
		{name: "unquoted path with spaces", command: `C:\Program Files\PixPin\unins000.exe`, wantFile: `C:\Program Files\PixPin\unins000.exe`},
		{name: "unquoted path with spaces and args", command: `C:\Program Files (x86)\Foo\uninstall.exe /S`, wantFile: `C:\Program Files (x86)\Foo\uninstall.exe`, wantParams: "/S"},
		{name: "unquoted msiexec", command: `MsiExec.exe /I{B21F6F1C-1111-2222-3333-444444444444}`, wantFile: "MsiExec.exe", wantParams: "/I{B21F6F1C-1111-2222-3333-444444444444}"},
		{name: "plain exe", command: `C:\Apps\uninstall.exe`, wantFile: `C:\Apps\uninstall.exe`},
		{name: "empty", command: "   "},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			file, params := splitUninstallCommand(testCase.command)
			if file != testCase.wantFile || params != testCase.wantParams {
				t.Fatalf("splitUninstallCommand(%q) = (%q, %q), want (%q, %q)", testCase.command, file, params, testCase.wantFile, testCase.wantParams)
			}
		})
	}
}

func TestRewriteMsiUninstallCommand(t *testing.T) {
	file, params := rewriteMsiUninstallCommand("MsiExec.exe", "/I{B21F6F1C-1111-2222-3333-444444444444}", true, "{B21F6F1C-1111-2222-3333-444444444444}")
	if file != "msiexec.exe" || params != "/X{B21F6F1C-1111-2222-3333-444444444444}" {
		t.Fatalf("got %s %s", file, params)
	}

	file, params = rewriteMsiUninstallCommand(`C:\Apps\unins000.exe`, "", false, "PixPin")
	if file != `C:\Apps\unins000.exe` || params != "" {
		t.Fatalf("non-MSI command was rewritten: %s %s", file, params)
	}
}

func TestNormalizeUninstallAppName(t *testing.T) {
	if got := normalizeUninstallAppName("PixPin 版本 3.4.3.2"); got != "pixpin" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeUninstallAppName("PixPin"); got != "pixpin" {
		t.Fatalf("got %q", got)
	}
}

func TestFindWindowsUninstallEntryMatchesInstallLocation(t *testing.T) {
	info := appInfo{Name: "PixPin", Path: `C:\Users\me\AppData\Local\PixPin\PixPin.exe`, Type: AppTypeDesktop}
	entries := []windowsUninstallEntry{
		{
			DisplayName:     "Unrelated App 1.0",
			UninstallString: `C:\Other\uninstall.exe`,
			InstallLocation: `C:\Other`,
		},
		{
			DisplayName:     "PixPin 版本 3.4.3.2",
			UninstallString: `"C:\Users\me\AppData\Local\PixPin\unins000.exe"`,
			InstallLocation: `C:\Users\me\AppData\Local\PixPin`,
			DisplayIcon:     `C:\Users\me\AppData\Local\PixPin\PixPin.exe,0`,
		},
	}

	entry := findWindowsUninstallEntry(info, []string{info.Path}, entries)
	if entry == nil || entry.DisplayName != "PixPin 版本 3.4.3.2" {
		t.Fatal("expected PixPin ARP entry")
	}
}

func TestFindWindowsUninstallEntryRequiresUniqueNameWhenPathMissing(t *testing.T) {
	info := appInfo{Name: "Chrome", Path: `C:\Users\me\Desktop\Chrome.lnk`, Type: AppTypeDesktop}
	entries := []windowsUninstallEntry{
		{DisplayName: "Google Chrome", UninstallString: `C:\Program Files\Google\Chrome\uninstall.exe`},
		{DisplayName: "Chrome Remote Desktop", UninstallString: `C:\Program Files\Chrome Remote\uninstall.exe`},
	}

	if entry := findWindowsUninstallEntry(info, []string{info.Path}, entries); entry != nil {
		t.Fatal("name-only ambiguous match must be rejected")
	}

	entries = []windowsUninstallEntry{
		{DisplayName: "PixPin 版本 3.4.3.2", UninstallString: `C:\Users\me\AppData\Local\PixPin\unins000.exe`},
	}
	info = appInfo{Name: "PixPin", Path: `C:\Users\me\Desktop\PixPin.lnk`, Type: AppTypeDesktop}
	if entry := findWindowsUninstallEntry(info, []string{info.Path}, entries); entry == nil {
		t.Fatal("unique normalized name should match")
	}
}

func TestFindWindowsUninstallEntrySkipsNoRemove(t *testing.T) {
	info := appInfo{Name: "System App", Path: `C:\Program Files\SystemApp\app.exe`, Type: AppTypeDesktop}
	entries := []windowsUninstallEntry{
		{
			DisplayName:     "System App",
			UninstallString: `C:\Program Files\SystemApp\uninstall.exe`,
			InstallLocation: `C:\Program Files\SystemApp`,
			NoRemove:        true,
		},
	}

	if entry := findWindowsUninstallEntry(info, []string{info.Path}, entries); entry != nil {
		t.Fatal("NoRemove entries must not be uninstallable")
	}
}

func TestShouldOfferWindowsUninstall(t *testing.T) {
	testCases := []struct {
		name string
		info appInfo
		want bool
	}{
		{name: "exe", info: appInfo{Path: `C:\Apps\Editor.exe`, Type: AppTypeDesktop}, want: true},
		{name: "shortcut", info: appInfo{Path: `C:\Apps\Editor.lnk`, Type: AppTypeDesktop}, want: true},
		{name: "url shortcut", info: appInfo{Path: `C:\Apps\Game.url`, Type: AppTypeDesktop}},
		{name: "uwp", info: appInfo{Path: `shell:AppsFolder\Example.App_123!App`, Type: AppTypeUWP}, want: true},
		{name: "windows setting", info: appInfo{Path: "ms-settings:display", Type: AppTypeWindowsSetting}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldOfferWindowsUninstall(testCase.info); got != testCase.want {
				t.Fatalf("got %t, want %t", got, testCase.want)
			}
		})
	}
}

func TestUwpPackageFamilyName(t *testing.T) {
	if got := uwpPackageFamilyName(`shell:AppsFolder\Microsoft.WindowsCalculator_8wekyb3d8bbwe!App`); got != "Microsoft.WindowsCalculator_8wekyb3d8bbwe" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildAppActionsIncludesUninstallOnWindows(t *testing.T) {
	plugin := &ApplicationPlugin{}
	actions := plugin.buildAppActions(appInfo{Path: `C:\Apps\Editor.exe`, Type: AppTypeDesktop}, "Editor", nil)

	hasUninstall := false
	for _, action := range actions {
		if action.Name == "i18n:plugin_app_uninstall" {
			hasUninstall = true
			break
		}
	}

	if hasUninstall != util.IsWindows() {
		t.Fatalf("uninstall action presence = %t, want %t", hasUninstall, util.IsWindows())
	}
}
