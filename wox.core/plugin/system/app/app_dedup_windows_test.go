package app

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

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
