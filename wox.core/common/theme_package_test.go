package common

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/util"
)

const surfaceThemeJSON = `{"SchemaVersion":2,"ThemeId":"ming","ThemeName":"Ming","BaseBackgroundColor":"#721D16","BaseTextColor":"#FFF0CA","BaseAccentColor":"#F4C453","Surfaces":{"App":{"Background":{"Source":"assets/red.png","Mode":"tile"},"ContentInsets":{"Top":80}},"Toolbar":{"Background":{"Source":"assets/red.png","Mode":"stretch"}}},"windows":{"Surfaces":{"Toolbar":null}},"linux":{"Surfaces":{"App":{"Decorations":[{"Source":"assets/red.png","Anchor":"topCenter","Size":{"Width":40,"Height":20}}]}}}}`

// TestSurfaceThemeRoundTrip covers sparse persistence, per-surface overrides and runtime-only assets.
func TestSurfaceThemeRoundTrip(t *testing.T) {
	var theme Theme
	if err := json.Unmarshal([]byte(surfaceThemeJSON), &theme); err != nil {
		t.Fatal(err)
	}
	theme.AssetFiles = map[string][]byte{"assets/red.png": {1, 2, 3}}
	resolved, err := theme.ResolveForTarget("windows", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Surfaces) != 1 || resolved.Surfaces["App"].ContentInsets.Top != 80 || len(resolved.AssetFiles) != 1 {
		t.Fatalf("lost inherited surface or assets: %+v", resolved.Surfaces)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "AssetFiles") || !strings.Contains(string(encoded), `"Toolbar":null`) {
		t.Fatal(string(encoded))
	}
	var again Theme
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatal(err)
	}
	if len(again.Surfaces) != 2 {
		t.Fatal("resolved theme flattened its authored surfaces")
	}
	for _, bad := range []string{`"Mode":"unknown"`, `"Source":"../red.png"`, `"Size":{"Width":0,"Height":10}`} {
		input := `{"SchemaVersion":2,"ThemeId":"x","ThemeName":"x","BaseBackgroundColor":"#000000","BaseTextColor":"#ffffff","BaseAccentColor":"#ffffff","Surfaces":{"App":{"Background":{"Source":"a.png","Mode":"tile",` + bad + `}}}}`
		if json.Unmarshal([]byte(input), &again) == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

// TestThemePackageValidation rejects traversal, missing assets and oversized version floors before extraction.
func TestThemePackageValidation(t *testing.T) {
	original := util.ProdEnv
	util.ProdEnv = "true"
	t.Cleanup(func() { util.ProdEnv = original })
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, document, asset string
		wantError             bool
	}{
		{"valid", surfaceThemeJSON, "assets/red.png", false},
		{"traversal", surfaceThemeJSON, "../red.png", true},
		{"windows path", surfaceThemeJSON, `assets\red.png`, true},
		{"missing", surfaceThemeJSON, "assets/other.png", true},
		{"reserved", surfaceThemeJSON, "assets/CON.png", true},
		{"future", `{"SchemaVersion":999,"MinWoxVersion":"999.0.0"}`, "assets/red.png", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var data bytes.Buffer
			writer := zip.NewWriter(&data)
			manifest, _ := writer.Create("theme.json")
			manifest.Write([]byte(tc.document))
			asset, _ := writer.Create(tc.asset)
			asset.Write(pngData.Bytes())
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "ming.wox-theme")
			if err := os.WriteFile(file, data.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			theme, err := ReadThemePackage(file, "2.4.4")
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if err == nil && len(theme.AssetFiles) != 1 {
				t.Fatal("missing decoded package assets")
			}
			if tc.name == "future" && !strings.Contains(err.Error(), "requires Wox") {
				t.Fatal(err)
			}
		})
	}
}

// TestThemeRepeatValidation preserves omitted defaults and rejects unsupported declarations.
func TestThemeRepeatValidation(t *testing.T) {
	for _, tc := range []struct {
		mode, x, y string
		valid      bool
	}{
		{"nineSlice", "", "", true}, {"nineSlice", "tile", "stretch", true}, {"nineSlice", "stretch", "tile", true}, {"nineSlice", "tile", "tile", true}, {"nineSlice", "repeat", "", false}, {"tile", "tile", "", false},
	} {
		layer := &ThemeSurfaceImage{Source: "frame.png", Mode: tc.mode, Repeat: &ThemeRepeat{X: tc.x, Y: tc.y}}
		if tc.mode == "nineSlice" {
			layer.Slice = &ThemeInsets{Top: 4}
			layer.Insets = &ThemeInsets{Top: 2}
		}
		err := (ThemeSurfaces{"App": &ThemeSurface{Frame: layer}}).Validate()
		if (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
