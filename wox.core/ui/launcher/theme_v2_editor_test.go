package launcher

import (
	"encoding/json"
	"testing"
	"wox/common"
)

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
