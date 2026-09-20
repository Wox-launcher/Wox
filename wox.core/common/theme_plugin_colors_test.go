package common

import (
	"encoding/json"
	"os"
	"testing"
)

// TestPluginColorsSurfaceComposition covers layered fills and legacy fallbacks.
func TestPluginColorsSurfaceComposition(t *testing.T) {
	tests := []struct {
		name, input, background, text, accentText string
		dark                                      bool
	}{
		{
			name:       "content beneath translucent preview",
			input:      `{"SchemaVersion":2,"ThemeId":"test","ThemeName":"Test","BaseBackgroundColor":"#000000","BaseTextColor":"#FFFFFF80","BaseAccentColor":"#008800","AppContentBackgroundColor":"#FFFFFF","PreviewBackgroundColor":"#00000080","ResultItemActiveBackgroundColor":"#000000","ResultItemActiveTitleColor":"#FFFFFF80"}`,
			background: "#7F7F7F", text: "#BFBFBF", accentText: "#808080", dark: true,
		},
		{
			name:       "transparent preview preserves content",
			input:      `{"SchemaVersion":2,"ThemeId":"test","ThemeName":"Test","BaseBackgroundColor":"#000000","BaseTextColor":"#00000080","BaseAccentColor":"#008800","AppContentBackgroundColor":"#FFFFFF","PreviewBackgroundColor":"transparent","ResultItemActiveBackgroundColor":"#FFFFFF","ResultItemActiveTitleColor":"#00000080"}`,
			background: "#FFFFFF", text: "#7F7F7F", accentText: "#7F7F7F", dark: true,
		},
		{
			name:       "legacy query background fallback",
			input:      `{"AppBackgroundColor":"transparent","QueryBoxBackgroundColor":"#FFFFFF","PreviewFontColor":"#000000","ResultItemActiveTitleColor":"#000000"}`,
			background: "#FFFFFF", text: "#000000", accentText: "#000000", dark: false,
		},
		{
			name:       "legacy window alpha applied once",
			input:      `{"AppBackgroundColor":"#00000080","PreviewFontColor":"#FFFFFF","ResultItemActiveTitleColor":"#FFFFFF"}`,
			background: "#09090A", text: "#FFFFFF", accentText: "#FFFFFF", dark: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var theme Theme
			if err := json.Unmarshal([]byte(tt.input), &theme); err != nil {
				t.Fatal(err)
			}
			got := theme.PluginColors()
			if got.Background != tt.background || got.Text != tt.text || got.AccentText != tt.accentText || got.Dark != tt.dark {
				t.Fatalf("palette = %+v; want background=%s text=%s accentText=%s dark=%t", got, tt.background, tt.text, tt.accentText, tt.dark)
			}
		})
	}
}

func TestPluginColorsFlattensDarkPreviewPalette(t *testing.T) {
	theme := Theme{
		AppBackgroundColor:              "#1C232580",
		PreviewFontColor:                "#E5ECE9",
		ResultItemSubTitleColor:         "#9BB0A8",
		PreviewSplitLineColor:           "#3A4A46",
		ResultItemActiveBackgroundColor: "#70D6A6",
		ResultItemActiveTitleColor:      "#0A1A16",
		PreviewTextSelectionColor:       "#264F78",
	}

	colors := theme.PluginColors()
	if !colors.Dark {
		t.Fatalf("dark theme reported as light: %+v", colors)
	}
	if colors.Background[1] > '8' {
		t.Fatalf("background = %s, want a dark fill", colors.Background)
	}
	if colors.Text != "#E5ECE9" || colors.SecondaryText != "#9BB0A8" || colors.Border != "#3A4A46" {
		t.Fatalf("text colors = %+v", colors)
	}
	if colors.Accent != "#70D6A6" || colors.AccentText != "#0A1A16" || colors.Selection != "#264F78" {
		t.Fatalf("accent colors = %+v", colors)
	}
}

func TestPluginColorsFlattensLightPreviewPalette(t *testing.T) {
	theme := Theme{
		AppBackgroundColor:              "#F7F2E2",
		PreviewFontColor:                "#1C1C1E",
		ResultItemSubTitleColor:         "#6B6258",
		PreviewSplitLineColor:           "#E4D9C4",
		ResultItemActiveBackgroundColor: "#C45C26",
		ResultItemActiveTitleColor:      "#FFFFFF",
		PreviewTextSelectionColor:       "#F3D5B5",
	}

	colors := theme.PluginColors()
	if colors.Dark {
		t.Fatalf("light theme reported as dark: %+v", colors)
	}
	if colors.Background != "#F7F2E2" || colors.Text != "#1C1C1E" {
		t.Fatalf("light surface = %+v", colors)
	}
}

func TestPluginColorsWoxLightPreviewWashStaysLight(t *testing.T) {
	data, err := os.ReadFile("../resource/themes/light.json")
	if err != nil {
		t.Fatal(err)
	}
	var theme Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		t.Fatal(err)
	}
	colors := theme.PluginColors()
	if colors.Dark {
		t.Fatalf("Wox Light classified as dark: %+v", colors)
	}
	if colors.Background == "#1E1E1E" || colors.Background == "#121214" {
		t.Fatalf("preview background stayed on the dark fallback: %+v", colors)
	}
	if colors.Text == "#E6E6E6" {
		t.Fatalf("text stayed on the dark fallback: %+v", colors)
	}
}

func TestPluginColorsIgnoresTransparentAndFallsBack(t *testing.T) {
	theme := Theme{
		AppBackgroundColor:    "#111111",
		PreviewFontColor:      "transparent",
		ResultItemTitleColor:  "#EEEEEE",
		PreviewSplitLineColor: "transparent",
	}

	colors := theme.PluginColors()
	if colors.Text != "#EEEEEE" {
		t.Fatalf("text fallback = %s", colors.Text)
	}
	if colors.Border == "transparent" || len(colors.Border) != 7 {
		t.Fatalf("border should be opaque fallback, got %q", colors.Border)
	}
}
