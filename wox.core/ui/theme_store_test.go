package ui

import (
	"context"
	"errors"
	"testing"
	"wox/common"
	"wox/util"
)

func TestResolveThemeManifestPreservesFetchFailure(t *testing.T) {
	store := &Store{manifests: []common.StoreThemeManifest{{Id: "cached"}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, ok, err := store.ResolveThemeManifest(ctx, "new-theme"); ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("missing theme on failed refresh: found=%v, err=%v", ok, err)
	}
	if manifest, ok, err := store.ResolveThemeManifest(ctx, "cached"); err != nil || !ok || manifest.Id != "cached" {
		t.Fatalf("failed refresh discarded cached theme: %#v, %v, %v", manifest, ok, err)
	}
}

func TestFindThemeManifestIgnoresLocalTheme(t *testing.T) {
	store := &Store{manifests: []common.StoreThemeManifest{{
		Id: "store-theme", Name: "Omarchy", Version: "1.0.0", DownloadUrl: "https://example.com/omarchy.json",
	}}}
	if _, ok := store.FindThemeManifest(context.Background(), "local-theme"); ok {
		t.Fatal("local theme should not be treated as a store theme")
	}
	manifest, ok := store.FindThemeManifest(context.Background(), "store-theme")
	if !ok || manifest.Id != "store-theme" {
		t.Fatalf("store theme = %#v, %v", manifest, ok)
	}
}

func TestParseStoreThemesSkipsInvalidEntries(t *testing.T) {
	themes, err := parseStoreThemes(context.Background(), []byte(`[
		{"Id":"ok","Name":"Ok","Version":"1.0.0","DownloadUrl":"https://example.com/ok.json","IconColors":{"Background":"#111111","Query":"#222222","Selected":"#333333"}},
		{"Id":"incomplete"},
		{"Id":"pkg","Name":"Pkg","Version":"1.0.0","DownloadUrl":"https://example.com/pkg.wox-theme","IconColors":{"Background":"#111111","Query":"#222222","Selected":"#333333"}},
		{"Id":"nocolor","Name":"No Color","Version":"1.0.0","DownloadUrl":"https://example.com/nocolor.json"}
	]`))
	if err != nil || len(themes) != 2 || themes[0].Id != "ok" || themes[1].Id != "pkg" {
		t.Fatalf("catalog isolation: %#v, %v", themes, err)
	}
}

func TestIsThemeUpgradable(t *testing.T) {
	manager := &Manager{themes: util.NewHashMap[string, common.Theme]()}
	manager.themes.Store("a", common.Theme{ThemeId: "a", Version: "1.0.0"})
	if !manager.IsThemeUpgradable("a", "1.0.1") {
		t.Fatal("newer store version should be upgradable")
	}
	if manager.IsThemeUpgradable("a", "1.0.0") || manager.IsThemeUpgradable("a", "0.9.0") || manager.IsThemeUpgradable("missing", "2.0.0") {
		t.Fatal("same, older, or missing themes are not upgradable")
	}
}
