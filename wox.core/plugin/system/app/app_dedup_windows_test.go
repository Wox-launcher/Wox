package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// TestAppsFolderExecutableDeduplication covers each resolvable namespace and source priority.
func TestAppsFolderExecutableDeduplication(t *testing.T) {
	t.Setenv("SystemRoot", `D:\Windows`)
	for _, test := range []struct {
		appID  string
		target string
	}{
		{`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\cleanmgr.exe`, `D:\Windows\System32\cleanmgr.exe`},
		{`{d65231b0-b2f1-4857-a4ce-a8e7c6ea7d27}\odbcad32.EXE`, `D:\Windows\SysWOW64\odbcad32.EXE`},
		{`{F38BF404-1D43-42F2-9305-67DE0B28FC23}\regedit.exe`, `D:\Windows\regedit.exe`},
	} {
		t.Run(test.appID, func(t *testing.T) {
			executable := appInfo{Name: "Executable", Path: test.target, Type: AppTypeDesktop}
			folder := appInfo{Name: "Shell entry", Path: extraAppPath(test.appID), Type: AppTypeAppsFolder}
			populateAppLaunchKey(context.Background(), &executable)
			populateAppLaunchKey(context.Background(), &folder)
			if folder.launchKey == "" || folder.launchKey != executable.launchKey || folder.Path != extraAppPath(test.appID) {
				t.Fatalf("expected shared executable identity without changing Shell activation: %+v", folder)
			}
			// An opaque/custom shortcut may have a different launch key or none at all.
			// Its original launch semantics must survive replacing the default entries.
			shortcut := appInfo{Name: "Shortcut", Path: `Z:\Custom\Tool.lnk`, Type: AppTypeDesktop, shortcutTarget: appPathMatchKey(test.target)}
			variant := appInfo{Name: "Custom arguments", Path: `Z:\Custom\Variant.lnk`, Type: AppTypeDesktop, shortcutTarget: shortcut.shortcutTarget, launchKey: "custom arguments"}
			for _, test := range []struct {
				name    string
				apps    []appInfo
				want    []string
				aliases []string
			}{
				{"all sources", []appInfo{folder, executable, variant, shortcut}, []string{shortcut.Path, variant.Path}, []string{folder.Name, executable.Name}},
				{"without shortcut", []appInfo{executable, folder}, []string{folder.Path}, []string{executable.Name}},
				{"without executable", []appInfo{folder, shortcut}, []string{shortcut.Path}, []string{folder.Name}},
				{"Shell only", []appInfo{folder}, []string{folder.Path}, nil},
			} {
				t.Run(test.name, func(t *testing.T) {
					got := deduplicateWindowsApps(test.apps, nil)
					paths := make([]string, len(got))
					for i := range got {
						paths[i] = got[i].Path
					}
					if !slices.Equal(paths, test.want) {
						t.Fatalf("got %v, want %v", paths, test.want)
					}
					for _, alias := range test.aliases {
						if !slices.Contains(got[0].SearchableNames, alias) {
							t.Fatalf("lost merged alias %q: %+v", alias, got[0])
						}
					}
					slices.Reverse(test.apps)
					if reversed := deduplicateWindowsApps(test.apps, nil); !reflect.DeepEqual(got, reversed) {
						t.Fatal("discovery order changed AppsFolder deduplication")
					}
				})
			}
		})
	}
}

// TestAppsFolderDeduplicationPreservesOpaqueEntries rejects names/icons as executable identities.
func TestAppsFolderDeduplicationPreservesOpaqueEntries(t *testing.T) {
	target := filepath.Join(getWindowsSystemRoot(), "System32", "cleanmgr.exe")
	apps := []appInfo{{Name: "Disk Cleanup", Path: target, Type: AppTypeDesktop}}
	for _, appID := range []string{
		`Microsoft.Windows.DiskCleanup`,
		`Example.Package_123!App`,
		`https://example.com/cleanmgr.exe`,
		`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\tools.msc`,
		`{UNKNOWN}\cleanmgr.exe`,
	} {
		info := appInfo{Name: "Disk Cleanup", Path: extraAppPath(appID), Type: AppTypeAppsFolder, IconSourcePath: target, launchKey: "stale", shortcutTarget: "stale"}
		populateAppLaunchKey(context.Background(), &info)
		if info.launchKey != "" || info.shortcutTarget != "" {
			t.Fatalf("opaque entry acquired an executable identity: %+v", info)
		}
		apps = append(apps, info)
	}
	if got := deduplicateWindowsApps(apps, nil); len(got) != len(apps) {
		t.Fatalf("opaque entries were merged: %+v", got)
	}
}

// TestAppsFolderDeduplicationRebuild restores launch keys from cache and promotes visible fallbacks.
func TestAppsFolderDeduplicationRebuild(t *testing.T) {
	ctx := context.Background()
	target := filepath.Join(getWindowsSystemRoot(), "System32", "cleanmgr.exe")
	folder := appInfo{Name: "Shell entry", Path: extraAppPath(`{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\cleanmgr.exe`), Type: AppTypeAppsFolder}
	executable := appInfo{Name: "Executable", Path: target, Type: AppTypeDesktop}
	encoded, err := json.Marshal(appCacheFile{Version: appCacheVersion, Apps: []appInfo{folder, executable}})
	if err != nil {
		t.Fatal(err)
	}
	cached, err := parseAppCacheContent(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for i := range cached {
		populateAppLaunchKey(ctx, &cached[i])
	}
	shortcut := appInfo{Name: "Shortcut", Path: `Z:\Custom\Cleanup.lnk`, Type: AppTypeDesktop, shortcutTarget: appPathMatchKey(target)}
	a := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever, apps: append(cached, shortcut)}
	for _, test := range []struct {
		ignored []ignoredApp
		want    string
	}{
		{nil, shortcut.Path},
		{[]ignoredApp{{Path: shortcut.Path}}, folder.Path},
		{[]ignoredApp{{Path: shortcut.Path}, {Path: folder.Path}}, executable.Path},
	} {
		a.ignoredApps = test.ignored
		a.rebuildQueryEntries(ctx)
		entries, _ := a.getQueryEntriesSnapshot()
		if len(entries) != 1 || entries[0].info.Path != test.want || len(a.apps) != 3 {
			t.Fatalf("expected fallback %q and all original sources, got %+v", test.want, entries)
		}
	}
	if cached[0].launchKey == "" || cached[0].launchKey != cached[1].launchKey {
		t.Fatal("cached AppsFolder launch identity was not rebuilt")
	}
}

func TestShortcutReplacesBareExeWithoutMergingLaunchVariants(t *testing.T) {
	target := `c:\apps\pixpin.exe`
	apps := []appInfo{
		{Name: "Bare exe", Path: target, Type: AppTypeDesktop, launchKey: "default"},
		{Name: "PixPin", Path: `C:\Start\PixPin.lnk`, Type: AppTypeDesktop, launchKey: "explicit working directory", shortcutTarget: target},
		{Name: "Profile", Path: `C:\Other\Profile.lnk`, Type: AppTypeDesktop, launchKey: "custom arguments", shortcutTarget: target},
		{Name: "Special", Path: `C:\Other\Special.lnk`, Type: AppTypeDesktop, shortcutTarget: target},
	}
	result := deduplicateWindowsApps(apps, []string{`c:\start`})
	if len(result) != 3 || result[0].Name != "PixPin" || !slices.Contains(result[0].SearchableNames, "Bare exe") {
		t.Fatalf("expected all shortcuts but no bare exe: %+v", result)
	}
	// A shortcut rejected by the launch fast path can still replace a bare exe because
	// execution keeps using the original .lnk, rather than discarding its special data.
	result = deduplicateWindowsApps([]appInfo{apps[0], apps[3]}, nil)
	if len(result) != 1 || result[0].Name != "Special" {
		t.Fatalf("opaque shortcut should replace its bare exe: %+v", result)
	}
	a := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever, apps: []appInfo{apps[0], apps[1]}}
	a.ignoredApps = []ignoredApp{{Path: apps[1].Path}}
	a.rebuildQueryEntries(context.Background())
	entries, _ := a.getQueryEntriesSnapshot()
	if len(entries) != 1 || entries[0].info.Path != target {
		t.Fatal("hidden shortcut must not suppress the remaining executable")
	}
}

func TestDeduplicateWindowsAppsPreservesVariantsAndAliases(t *testing.T) {
	roots := []string{`c:\user\start`, `c:\common\start`, `c:\user\desktop`, `c:\public\desktop`}
	apps := []appInfo{
		{Name: "Executable", Path: `C:\Apps\Editor.exe`, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "Other", Path: `C:\Other\Editor.lnk`, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "Desktop", Path: `C:\User\Desktop\Editor.lnk`, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "Common", Path: `C:\Common\Start\Editor.lnk`, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "User", Path: `C:\User\Start\Editor.lnk`, SearchableNames: []string{"ExistingAlias"}, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "Profile", Path: `C:\User\Start\Profile.lnk`, Type: AppTypeDesktop, launchKey: "profile"},
		{Name: "Admin", Path: `C:\User\Start\Admin.lnk`, Type: AppTypeDesktop, launchKey: "admin"},
		{Name: "Opaque", Path: `C:\Other\Special.lnk`, Type: AppTypeDesktop},
		{Name: "Opaque alias", Path: `c:\other\SPECIAL.lnk`, Type: AppTypeDesktop},
		{Name: "Different opaque", Path: `C:\Other\Special2.lnk`, Type: AppTypeDesktop},
	}
	result := deduplicateWindowsApps(apps, roots)
	if len(result) != 5 {
		t.Fatalf("expected 5 launch groups, got %+v", result)
	}
	var representative appInfo
	for _, info := range result {
		if info.launchKey == "plain" {
			representative = info
		}
	}
	if representative.Name != "User" || representative.Path != apps[4].Path {
		t.Fatalf("wrong representative: %+v", representative)
	}
	for _, alias := range []string{"Executable", "Other", "Desktop", "Common", "ExistingAlias", "Editor", "Editor.exe"} {
		if !slices.Contains(representative.GetSearchCandidates(representative.Name), alias) {
			t.Fatalf("lost searchable alias %q", alias)
		}
	}
	if !reflect.DeepEqual(apps[4].SearchableNames, []string{"ExistingAlias"}) {
		t.Fatal("merging aliases mutated the original index")
	}
	remaining := slices.Clone(apps[:5])
	for _, want := range []string{"User", "Common", "Desktop", "Other", "Executable"} {
		groups := deduplicateWindowsApps(remaining, roots)
		if len(groups) != 1 || groups[0].Name != want {
			t.Fatalf("priority promotion: wanted %s, got %+v", want, groups)
		}
		remaining = slices.DeleteFunc(remaining, func(info appInfo) bool { return info.Name == want })
	}
	slices.Reverse(apps)
	if reordered := deduplicateWindowsApps(apps, roots); !reflect.DeepEqual(result, reordered) {
		t.Fatal("parallel discovery order changed the representatives or aliases")
	}
	// A similarly prefixed directory is not inside the user's Start Menu.
	if windowsAppEntryPriority(`C:\User\StartOther\Editor.lnk`, roots) != 3 {
		t.Fatal("directory prefix matched without a path boundary")
	}
}

func TestDeduplicationRebuildKeepsSourcesAndRemovesStaleAliases(t *testing.T) {
	ctx := context.Background()
	a := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever}
	a.apps = []appInfo{
		{Name: "Shortcut", Path: `C:\AppDedupTest\Editor.lnk`, Type: AppTypeDesktop, launchKey: "plain"},
		{Name: "Executable", Path: `C:\AppDedupTest\Editor.exe`, Type: AppTypeDesktop, launchKey: "plain"},
	}
	a.rebuildQueryEntries(ctx)
	entries, generation := a.getQueryEntriesSnapshot()
	if len(entries) != 1 || entries[0].info.Name != "Shortcut" || len(a.apps) != 2 {
		t.Fatalf("expected a merged snapshot and both original sources: %+v", entries)
	}
	// Filtering one path must leave the other launchable and exclude the hidden name from its aliases.
	a.ignoredApps = []ignoredApp{{Path: a.apps[0].Path}}
	a.rebuildQueryEntries(ctx)
	entries, _ = a.getQueryEntriesSnapshot()
	if len(entries) != 1 || entries[0].info.Name != "Executable" || slices.Contains(entries[0].info.SearchableNames, "Shortcut") {
		t.Fatalf("ignored source was merged back into results: %+v", entries)
	}
	a.ignoredApps = nil
	if !a.removeIndexedAppByPath(ctx, a.apps[0].Path) {
		t.Fatal("failed to remove representative")
	}
	a.rebuildQueryEntries(ctx)
	entries, nextGeneration := a.getQueryEntriesSnapshot()
	if len(entries) != 1 || entries[0].info.Name != "Executable" || slices.Contains(entries[0].info.SearchableNames, "Shortcut") || nextGeneration <= generation {
		t.Fatalf("deleted representative/alias survived rebuild: %+v", entries)
	}
	// Recreating an entry with different arguments adds a result instead of folding it into the old group.
	a.apps = append(a.apps, appInfo{Name: "Profile", Path: `C:\AppDedupTest\Editor.lnk`, Type: AppTypeDesktop, launchKey: "profile"})
	a.rebuildQueryEntries(ctx)
	entries, _ = a.getQueryEntriesSnapshot()
	if len(entries) != 2 {
		t.Fatalf("modified launch should have split into its own group: %+v", entries)
	}
	before := a.apps[0]
	after := before
	after.launchKey = "new launch"
	if before.equals(after) {
		t.Fatal("launch changes must invalidate the query snapshot")
	}
	// Runtime launch identities are reconstructed, not persisted as stale decisions in the app cache.
	encoded, err := json.Marshal(appCacheFile{Version: appCacheVersion, Apps: a.apps})
	if err != nil {
		t.Fatal(err)
	}
	cached, err := parseAppCacheContent(encoded)
	if err != nil || len(cached) != 2 || cached[0].launchKey != "" || cached[1].launchKey != "" {
		t.Fatalf("cache did not preserve source entries independently: %+v, %v", cached, err)
	}
}
