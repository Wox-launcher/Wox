package common

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewWoxImageThemeUsesSettingsSwatchSVG(t *testing.T) {
	image := NewWoxImageTheme(Theme{
		AppBackgroundColor:              "#112233",
		QueryBoxBackgroundColor:         "#445566",
		ResultItemActiveBackgroundColor: "#778899",
	})
	if image.ImageType != WoxImageTypeSvg {
		t.Fatalf("image type = %q, want svg", image.ImageType)
	}
	if !strings.Contains(image.ImageData, `rx="8"`) || !strings.Contains(image.ImageData, `fill="#112233"`) {
		t.Fatalf("swatch SVG = %s, want the Settings rounded catalog icon", image.ImageData)
	}
	if strings.Contains(image.ImageData, `y="17"`) {
		t.Fatalf("swatch SVG still uses the old square window preview: %s", image.ImageData)
	}
}

func TestNewWoxImageThemeAutoUsesDiagonalMask(t *testing.T) {
	image := NewWoxImageThemeAuto(
		Theme{AppBackgroundColor: "#F5F5F5", QueryBoxBackgroundColor: "#E8E8E8", ResultItemActiveBackgroundColor: "#D8D8D8"},
		Theme{AppBackgroundColor: "#2B2B2B", QueryBoxBackgroundColor: "#3D3D3D", ResultItemActiveBackgroundColor: "#4A4A4A"},
	)
	if image.ImageType != WoxImageTypeSvg {
		t.Fatalf("image type = %q, want svg", image.ImageType)
	}
	if !strings.Contains(image.ImageData, `id="theme-auto-swatch"`) || !strings.Contains(image.ImageData, `fill="#F5F5F5"`) || !strings.Contains(image.ImageData, `fill="#2B2B2B"`) {
		t.Fatalf("auto swatch SVG = %s, want a masked light/dark catalog icon", image.ImageData)
	}
}

func TestThemeSwatchSVGPaintsAuthoredWindowBorder(t *testing.T) {
	var theme Theme
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"jade","ThemeName":"Jade","BaseBackgroundColor":"#1C2325","BaseTextColor":"#E5ECE9","BaseAccentColor":"#70D6A6","AppBackgroundColor":"#1C2325FF","AppBorderColor":"#4FAE85FF","AppBorderWidth":3}`), &theme); err != nil {
		t.Fatalf("parse jade theme: %v", err)
	}
	svg := ThemeSwatchSVG(theme)
	if !strings.Contains(svg, `stroke="#4FAE85FF"`) || !strings.Contains(svg, `stroke-width="2.25"`) {
		t.Fatalf("swatch SVG = %s, want Jade's authored window outline", svg)
	}
}

func TestThemeSwatchSVGSkipsRadiusOnlyChrome(t *testing.T) {
	var theme Theme
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"round","ThemeName":"Round","BaseBackgroundColor":"#1C2325","BaseTextColor":"#E5ECE9","BaseAccentColor":"#70D6A6","AppBorderRadius":8}`), &theme); err != nil {
		t.Fatalf("parse radius-only theme: %v", err)
	}
	if strings.Contains(ThemeSwatchSVG(theme), `stroke=`) {
		t.Fatal("corner-only chrome should not paint a window outline on the swatch")
	}
}

func TestThemeSwatchSVGSkipsDefaultDividerChrome(t *testing.T) {
	var theme Theme
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"plain","ThemeName":"Plain","BaseBackgroundColor":"#1C2325","BaseTextColor":"#E5ECE9","BaseAccentColor":"#70D6A6"}`), &theme); err != nil {
		t.Fatalf("parse plain theme: %v", err)
	}
	if strings.Contains(ThemeSwatchSVG(theme), `stroke=`) {
		t.Fatal("ordinary themes should not paint the resolved divider as a window outline")
	}
}

func TestThemeSwatchDiagonalPolygonSplitsFullBounds(t *testing.T) {
	bounds := []themeSwatchPoint{{0, 0}, {ThemeSwatchSize, 0}, {ThemeSwatchSize, ThemeSwatchSize}, {0, ThemeSwatchSize}}
	light := themeSwatchDiagonalPolygon(bounds, true)
	dark := themeSwatchDiagonalPolygon(bounds, false)
	if len(light) != 3 || len(dark) != 3 {
		t.Fatalf("diagonal polygons = %d/%d points, want two triangles", len(light), len(dark))
	}
}
