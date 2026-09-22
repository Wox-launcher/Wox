package launcher

import (
	"encoding/json"
	"reflect"
	"testing"
	"wox/common"
	"wox/util"
	"wox/util/osvariant"
)

// TestThemeEditorGeometryRoundTrip exercises actual schema parsing rather than string-only form state.
func TestThemeEditorGeometryRoundTrip(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"geometry","ThemeName":"Geometry","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","AppBorderRadius":28}`), &raw); err != nil {
		t.Fatal(err)
	}
	platform := util.GetCurrentPlatform()
	if platform == "darwin" {
		platform = "macos"
	}
	raw[platform] = map[string]any{"AppBorderRadius": nil, "ToolbarBorderWidth": float64(2)}
	_, values := themeEditorForm(raw)
	if values["AppBorderRadius"] != "" {
		t.Fatal("null must clear the parent's radius")
	}
	if merged := mergeThemeEditorDraft(raw, values); !reflect.DeepEqual(raw, merged) {
		t.Fatal("opening and saving changed the source")
	}
	for _, key := range []string{"AppBorderWidth", "QueryBoxBorderRadius", "ResultItemBorderRadius", "PreviewBorderRadius", "ToolbarBorderWidth"} {
		for _, value := range []string{"0", "12", ""} {
			values[key] = value
			if _, err := themeEditorDraftTheme(raw, values); err != nil {
				t.Fatalf("%s=%q: %v", key, value, err)
			}
		}
		for _, value := range []string{"-1", "1.5", "abc", "999999999999999999999999999"} {
			values[key] = value
			if _, err := themeEditorDraftTheme(raw, values); err == nil {
				t.Fatalf("accepted invalid %s=%q", key, value)
			}
		}
		values[key] = ""
	}
	values["ToolbarBorderWidth"] = "0"
	merged := mergeThemeEditorDraft(raw, values)
	if merged[platform].(map[string]any)["ToolbarBorderWidth"] != 0 {
		t.Fatal("explicit zero must remain numeric on the active layer")
	}
	if _, ok := merged[platform].(map[string]any)["AppBorderRadius"]; !ok {
		t.Fatal("unrelated null override was lost")
	}
	values["AppBorderRadius"] = "0"
	preview, err := themeEditorDraftTheme(raw, values)
	if err != nil || preview.AppBorderRadius == nil || *preview.AppBorderRadius != 0 {
		t.Fatal("editing a null override did not affect the preview")
	}
	if raw[platform].(map[string]any)["AppBorderRadius"] != nil {
		t.Fatal("draft mutated original source")
	}
}

// TestThemeEditorActiveOverride keeps the picker, preview and saved document on the same authored layer.
func TestThemeEditorActiveOverride(t *testing.T) {
	for _, useVariant := range []bool{false, true} {
		platform, variant := util.GetCurrentPlatform(), osvariant.GetCurrentPlatformVariant()
		if platform == "darwin" {
			platform = "macos"
		}
		if useVariant && variant == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"override","ThemeName":"Override","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","ToolbarBackgroundColor":"#101010FF"}`), &raw); err != nil {
			t.Fatal(err)
		}
		node := map[string]any{"ToolbarBackgroundColor": "#202020FF", "ToolbarBorderWidth": float64(0)}
		raw[platform] = node
		wantInitial, wantReset := "#202020FF", "#101010FF"
		if useVariant {
			node["variants"] = map[string]any{variant: map[string]any{"ToolbarBackgroundColor": "#303030FF"}}
			wantInitial, wantReset = "#303030FF", "#202020FF"
		}
		original, _ := json.Marshal(raw)
		_, values := themeEditorForm(raw)
		if values["ToolbarBackgroundColor"] != wantInitial {
			t.Fatal("picker ignored the active override")
		}
		values["ToolbarBackgroundColor"] = "#FF00FF80"
		preview, err := themeEditorDraftTheme(raw, values)
		if err != nil || preview.ToolbarBackgroundColor != "#FF00FF80" {
			t.Fatalf("preview ignored edited override: %s, %v", preview.ToolbarBackgroundColor, err)
		}
		merged := mergeThemeEditorDraft(raw, values)
		if merged["ToolbarBackgroundColor"] != "#101010FF" || merged[platform].(map[string]any)["ToolbarBorderWidth"] != float64(0) {
			t.Fatal("editing a platform color changed the root color or unrelated geometry")
		}
		encoded, _ := json.Marshal(merged)
		var saved common.Theme
		if err := json.Unmarshal(encoded, &saved); err != nil {
			t.Fatal(err)
		}
		resolved, err := saved.ResolveForTarget(platform, variant)
		if err != nil || resolved.ToolbarBackgroundColor != preview.ToolbarBackgroundColor {
			t.Fatal("saved theme disagrees with preview")
		}
		values["ToolbarBackgroundColor"] = ""
		preview, err = themeEditorDraftTheme(raw, values)
		if err != nil || preview.ToolbarBackgroundColor != wantReset {
			t.Fatalf("reset did not inherit next layer: %s, %v", preview.ToolbarBackgroundColor, err)
		}
		after, _ := json.Marshal(raw)
		if string(original) != string(after) {
			t.Fatal("editing mutated the source theme")
		}
	}
}

// TestV2ThemeEditorInheritance verifies the real draft path keeps empty overrides sparse and base changes live.
func TestV2ThemeEditorInheritance(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"editor-v2","ThemeName":"Editor","BaseBackgroundColor":"#182020B8","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","AppPaddingLeft":0,"windows":{"ToolbarBorderWidth":0}}`), &raw); err != nil {
		t.Fatal(err)
	}
	_, values := themeEditorForm(raw)
	if values["QueryBoxFontColor"] != "" || values["BaseAccentColor"] != "#70D6A6" {
		t.Fatal("editor filled inherited overrides or omitted base colors")
	}
	values["BaseTextColor"] = "#ABCDEF"
	draft, err := themeEditorDraftTheme(raw, values)
	if err != nil {
		t.Fatal(err)
	}
	if draft.QueryBoxFontColor != "#ABCDEFFF" || draft.AppPaddingLeft != 0 {
		t.Fatalf("draft did not resolve base color/zero: %#v", draft)
	}
	if swatch := themeEditorColorValue(raw, values, "QueryBoxFontColor"); swatch != "#ABCDEFFF" {
		t.Fatalf("wrong inherited swatch %s", swatch)
	}
	values["QueryBoxFontColor"] = "#112233"
	draft, err = themeEditorDraftTheme(raw, values)
	if err != nil {
		t.Fatal(err)
	}
	if draft.QueryBoxFontColor != "#112233FF" {
		t.Fatal("explicit override ignored")
	}
	values["QueryBoxFontColor"] = ""
	merged := mergeThemeEditorDraft(raw, values)
	if _, exists := merged["QueryBoxFontColor"]; exists {
		t.Fatal("reset did not remove override")
	}
	encoded, err := json.Marshal(merged)
	if err != nil {
		t.Fatal(err)
	}
	var saved common.Theme
	if err := json.Unmarshal(encoded, &saved); err != nil {
		t.Fatal(err)
	}
	var authored common.ThemeSchemaV2
	if err := json.Unmarshal(encoded, &authored); err != nil {
		t.Fatal(err)
	}
	if saved.Windows == nil || authored.AppPaddingLeft == nil || *authored.AppPaddingLeft != 0 {
		t.Fatal("save lost platform source or explicit zero")
	}
}
