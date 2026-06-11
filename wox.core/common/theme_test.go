package common

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestThemePlatformOverrideVariantsAllowStyleFields(t *testing.T) {
	themeJSON := []byte(`{
		"ThemeId": "variant-theme",
		"windows": {
			"AppBackgroundColor": "#111111",
			"variants": {
				"win11": {
					"ToolbarBackgroundColor": "#222222"
				}
			}
		}
	}`)

	var theme Theme
	if err := json.Unmarshal(themeJSON, &theme); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if theme.Windows == nil {
		t.Fatal("expected windows override to be preserved")
	}
	if _, ok := (*theme.Windows)["variants"]; !ok {
		t.Fatal("expected variants node to be preserved in raw platform override")
	}
}

func TestThemePlatformOverrideVariantsRejectNonStyleFields(t *testing.T) {
	themeJSON := []byte(`{
		"ThemeId": "variant-theme",
		"windows": {
			"variants": {
				"win11": {
					"ThemeName": "Bad Variant"
				}
			}
		}
	}`)

	var theme Theme
	err := json.Unmarshal(themeJSON, &theme)
	if err == nil {
		t.Fatal("expected non-style variant field to be rejected")
	}
	if !strings.Contains(err.Error(), `platform theme override "windows" variant "win11" contains non-style field "ThemeName"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestThemePlatformOverrideVariantsMustBeObjects(t *testing.T) {
	tests := []struct {
		name      string
		themeJSON string
		wantError string
	}{
		{
			name: "variants node is not object",
			themeJSON: `{
				"ThemeId": "variant-theme",
				"windows": {
					"variants": []
				}
			}`,
			wantError: `platform theme override "windows" variants must be a JSON object`,
		},
		{
			name: "variant node is not object",
			themeJSON: `{
				"ThemeId": "variant-theme",
				"windows": {
					"variants": {
						"win11": []
					}
				}
			}`,
			wantError: `platform theme override "windows" variant "win11" must be a JSON object`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var theme Theme
			err := json.Unmarshal([]byte(tt.themeJSON), &theme)
			if err == nil {
				t.Fatal("expected invalid variants shape to be rejected")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
