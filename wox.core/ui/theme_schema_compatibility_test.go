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
	themes, err := parseStoreThemes(context.Background(), []byte(`[{"ThemeName":"Legacy"},{"SchemaVersion":999},{"ThemeName":"Future","MinWoxVersion":"999.0.0"}]`))
	if err != nil || len(themes) != 2 {
		t.Fatalf("catalog isolation: %d themes, %v", len(themes), err)
	}
}
