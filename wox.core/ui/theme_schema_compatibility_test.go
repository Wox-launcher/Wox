package ui

import (
	"context"
	"testing"
	"wox/common"
)

// TestThemeCompatibilityBoundaries ensures incompatible themes cannot load or reach persistence.
func TestThemeCompatibilityBoundaries(t *testing.T) {
	manager := &Manager{}
	if _, err := manager.parseTheme(`{"ThemeName":"Future","MinWoxVersion":"999.0.0"}`); err == nil {
		t.Fatal("loaded incompatible theme")
	}
	store := &Store{}
	for _, install := range []func(context.Context, common.Theme) error{store.Install, store.InstallLocal} {
		if err := install(context.Background(), common.Theme{ThemeName: "Future", MinWoxVersion: "999.0.0"}); err == nil {
			t.Fatal("installed incompatible theme")
		}
	}
	themes, err := parseStoreThemes(context.Background(), []byte(`[
		{"Id":"ok","Name":"Ok","Version":"1.0.0","DownloadUrl":"https://example.com/ok.json","IconColors":{"Background":"#111111","Query":"#222222","Selected":"#333333"}},
		{"Id":"incomplete"},
		{"Id":"pkg","Name":"Pkg","Version":"1.0.0","DownloadUrl":"https://example.com/pkg.wox-theme","IconColors":{"Background":"#111111","Query":"#222222","Selected":"#333333"}}
	]`))
	if err != nil || len(themes) != 2 || themes[0].Id != "ok" || themes[1].Id != "pkg" {
		t.Fatalf("catalog isolation: %#v, %v", themes, err)
	}
}
