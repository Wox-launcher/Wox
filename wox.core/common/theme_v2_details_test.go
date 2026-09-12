package common

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestThemeV2DetailColors covers inheritance, transparency, platform overrides and sparse saves.
func TestThemeV2DetailColors(t *testing.T) {
	fields := []string{"PreviewBackgroundColor", "PreviewBorderColor", "PreviewTagFontColor", "PreviewTagBackgroundColor", "PreviewTagBorderColor", "GlanceFontColor", "GlanceIconColor", "GlanceBackgroundColor", "GlanceHoverBackgroundColor", "ActionContainerDividerColor", "ToolbarHotkeyFontColor", "ToolbarHotkeyBackgroundColor", "ToolbarHotkeyBorderColor", "ResultItemHoverBackgroundColor"}
	for _, field := range fields {
		for _, value := range []string{"null", `"transparent"`, `"#12345680"`, `"invalid"`} {
			input := strings.TrimSuffix(minimalV2Theme, "}") + `,"` + field + `":` + value + `}`
			var theme Theme
			err := json.Unmarshal([]byte(input), &theme)
			if value == `"invalid"` {
				if err == nil {
					t.Fatalf("accepted invalid %s", field)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			color := theme.ResolvedColors()[field]
			if color == "" || (value == `"transparent"` && color != "#00000000") || (value == `"#12345680"` && color != "#12345680") {
				t.Fatalf("%s %s resolved to %q", field, value, color)
			}
			encoded, err := json.Marshal(theme)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			if value == "null" && raw[field] != nil {
				t.Fatalf("materialized inherited %s", field)
			}
		}
	}
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"windows":{"ToolbarHotkeyBorderColor":"transparent","variants":{"test":{"ResultItemHoverBackgroundColor":"#12345680"}}}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	resolved, err := theme.ResolveForTarget("windows", "test")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ResolvedColors()["ToolbarHotkeyBorderColor"] != "#00000000" || resolved.ResolvedColors()["ResultItemHoverBackgroundColor"] != "#12345680" {
		t.Fatal("platform colors were not applied")
	}
	before, _ := json.Marshal(theme)
	after, _ := json.Marshal(resolved)
	if string(before) != string(after) {
		t.Fatal("platform resolution changed authored source")
	}
	var legacy Theme
	if err := json.Unmarshal([]byte(`{"ToolbarHotkeyBorderColor":"#12345680"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if len(legacy.ResolvedColors()) != 0 {
		t.Fatal("v2 colors leaked into v1")
	}
}
