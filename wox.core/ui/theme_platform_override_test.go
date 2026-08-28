package ui

import (
	"context"
	"encoding/json"
	"testing"
	"wox/common"
)

func TestResolvePlatformThemeForTargetAppliesPlatformThenVariant(t *testing.T) {
	theme := common.Theme{
		ThemeId:                         "variant-theme",
		AppBackgroundColor:              "#000000",
		ToolbarBackgroundColor:          "#111111",
		ResultItemActiveBorderLeftWidth: 1,
		Windows: &common.ThemePlatformOverride{
			"AppBackgroundColor": json.RawMessage(`"#222222"`),
			"variants": json.RawMessage(`{
				"win11": {
					"ToolbarBackgroundColor": "#333333",
					"ResultItemActiveBorderLeft": 7
				}
			}`),
		},
	}

	resolvedTheme := resolvePlatformThemeForTarget(context.Background(), theme, "windows", "win11")

	if resolvedTheme.AppBackgroundColor != "#222222" {
		t.Fatalf("AppBackgroundColor = %q, want platform override", resolvedTheme.AppBackgroundColor)
	}
	if resolvedTheme.ToolbarBackgroundColor != "#333333" {
		t.Fatalf("ToolbarBackgroundColor = %q, want variant override", resolvedTheme.ToolbarBackgroundColor)
	}
	if resolvedTheme.ResultItemActiveBorderLeftWidth != 7 {
		t.Fatalf("ResultItemActiveBorderLeftWidth = %d, want legacy variant alias", resolvedTheme.ResultItemActiveBorderLeftWidth)
	}
	if resolvedTheme.Windows != nil || resolvedTheme.MacOS != nil || resolvedTheme.Linux != nil {
		t.Fatal("expected resolved theme to clear raw platform overrides")
	}
}

func TestResolvePlatformThemeForTargetFallsBackToPlatformWhenVariantMissing(t *testing.T) {
	theme := common.Theme{
		ThemeId:                "variant-theme",
		AppBackgroundColor:     "#000000",
		ToolbarBackgroundColor: "#111111",
		Windows: &common.ThemePlatformOverride{
			"AppBackgroundColor": json.RawMessage(`"#222222"`),
			"variants": json.RawMessage(`{
				"win10": {
					"ToolbarBackgroundColor": "#333333"
				}
			}`),
		},
	}

	resolvedTheme := resolvePlatformThemeForTarget(context.Background(), theme, "windows", "win11")

	if resolvedTheme.AppBackgroundColor != "#222222" {
		t.Fatalf("AppBackgroundColor = %q, want platform override", resolvedTheme.AppBackgroundColor)
	}
	if resolvedTheme.ToolbarBackgroundColor != "#111111" {
		t.Fatalf("ToolbarBackgroundColor = %q, want base value when variant is missing", resolvedTheme.ToolbarBackgroundColor)
	}
}

func TestResolvePlatformThemeForTargetAppliesLinuxHyprlandVariant(t *testing.T) {
	theme := common.Theme{
		ThemeId:            "variant-theme",
		AppBackgroundColor: "rgba(35, 41, 51, 0.75)",
		Linux: &common.ThemePlatformOverride{
			"variants": json.RawMessage(`{
				"hyprland": {
					"AppBackgroundColor": "rgba(35, 41, 51, 0.98)"
				}
			}`),
		},
	}

	hyprlandTheme := resolvePlatformThemeForTarget(context.Background(), theme, "linux", "hyprland")
	if hyprlandTheme.AppBackgroundColor != "rgba(35, 41, 51, 0.98)" {
		t.Fatalf("hyprland AppBackgroundColor = %q, want 98%% wash", hyprlandTheme.AppBackgroundColor)
	}

	otherLinuxTheme := resolvePlatformThemeForTarget(context.Background(), theme, "linux", "")
	if otherLinuxTheme.AppBackgroundColor != "rgba(35, 41, 51, 0.75)" {
		t.Fatalf("other linux AppBackgroundColor = %q, want the shared wash", otherLinuxTheme.AppBackgroundColor)
	}
}

func TestResolvePlatformThemeForTargetKeepsLegacyPlatformOverride(t *testing.T) {
	theme := common.Theme{
		ThemeId:                "variant-theme",
		AppBackgroundColor:     "#000000",
		ToolbarBackgroundColor: "#111111",
		Windows: &common.ThemePlatformOverride{
			"AppBackgroundColor":     json.RawMessage(`"#222222"`),
			"ToolbarBackgroundColor": json.RawMessage(`"#333333"`),
		},
	}

	resolvedTheme := resolvePlatformThemeForTarget(context.Background(), theme, "windows", "")

	if resolvedTheme.AppBackgroundColor != "#222222" {
		t.Fatalf("AppBackgroundColor = %q, want platform override", resolvedTheme.AppBackgroundColor)
	}
	if resolvedTheme.ToolbarBackgroundColor != "#333333" {
		t.Fatalf("ToolbarBackgroundColor = %q, want platform override", resolvedTheme.ToolbarBackgroundColor)
	}
	if resolvedTheme.Windows != nil || resolvedTheme.MacOS != nil || resolvedTheme.Linux != nil {
		t.Fatal("expected resolved theme to clear raw platform overrides")
	}
}
