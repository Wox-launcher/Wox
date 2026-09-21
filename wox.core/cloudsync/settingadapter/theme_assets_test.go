package settingadapter

import (
	"context"
	"encoding/json"
	"testing"
	"wox/cloudsync"
	"wox/common"
)

// TestInstalledThemeSyncSkipsResources covers unloaded and inactive platform assets.
func TestInstalledThemeSyncSkipsResources(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		want        bool
	}{
		{"plain", "", true},
		{"layout only", `,"Surfaces":{"App":{"ContentInsets":{"Top":20}}}`, true},
		{"resource", `,"Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}`, false},
		{"inactive platform", `,"macos":{"Surfaces":{"App":{"Background":{"Source":"assets/bg.png","Mode":"stretch"}}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"SchemaVersion":2,"ThemeId":"sync-test","ThemeName":"Test","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ffffff"` + tc.extra + `}`
			var theme common.Theme
			if err := json.Unmarshal([]byte(document), &theme); err != nil {
				t.Fatal(err)
			}
			if got := theme.CanSyncWithoutAssets(); got != tc.want {
				t.Fatalf("sync eligibility = %v, want %v", got, tc.want)
			}
			data, err := json.Marshal(cloudsync.InstalledThemeValue{ID: theme.ThemeId, Theme: json.RawMessage(document)})
			if err != nil {
				t.Fatal(err)
			}
			_, ok, err := decodeInstalledTheme(context.Background(), theme.ThemeId, string(data))
			if err != nil || ok != tc.want {
				t.Fatalf("decode = %v, %v; want %v", ok, err, tc.want)
			}
			theme.AssetFiles = map[string][]byte{"assets/unused.png": {1}}
			if theme.CanSyncWithoutAssets() {
				t.Fatal("loaded assets must remain local")
			}
		})
	}
}
