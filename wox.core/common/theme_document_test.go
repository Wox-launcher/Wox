package common

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestThemeV1RuntimeMapping exercises every wire field so an explicit adapter cannot silently drop one.
func TestThemeV1RuntimeMapping(t *testing.T) {
	var wire ThemeSchemaV1
	value := reflect.ValueOf(&wire).Elem()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString(value.Type().Field(i).Name)
		case reflect.Int:
			field.SetInt(int64(i + 1))
		case reflect.Bool:
			field.SetBool(true)
		case reflect.Pointer:
			if field.Type().Elem().Kind() == reflect.Int {
				pointer := reflect.New(field.Type().Elem())
				pointer.Elem().SetInt(int64(i + 1))
				field.Set(pointer)
			} else {
				node := ThemePlatformOverride{"AppPaddingLeft": json.RawMessage(`2`)}
				field.Set(reflect.ValueOf(&node))
			}
		}
	}
	wire.SchemaVersion, wire.MinWoxVersion = 1, "2.0.0"
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var runtime Theme
	if err := json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	var saved ThemeSchemaV1
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire, saved) {
		t.Fatal("v1 field lost between wire document and runtime")
	}
}

// TestThemeV2CompleteDocument preserves metadata edits without serializing inherited styles.
func TestThemeV2CompleteDocument(t *testing.T) {
	zero := 0
	document := ThemeSchemaV2{
		SchemaVersion: 2, MinWoxVersion: "2.4.3", ThemeId: "original", ThemeName: "Original",
		ThemeAuthor: "Author", ThemeUrl: "https://example.com", Version: "1.2.3", Description: "Description",
		IsSystem: true, IsInstalled: true, IsAutoAppearance: true, DarkThemeId: "dark", LightThemeId: "light",
		BaseBackgroundColor: "#123456", BaseTextColor: "#EEEEEE", BaseAccentColor: "#00AA88", AppPaddingLeft: &zero,
	}
	node := ThemePlatformOverride{"GlanceFontColor": json.RawMessage(`"transparent"`)}
	document.Windows = &node
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	var runtime Theme
	if err := json.Unmarshal(data, &runtime); err != nil {
		t.Fatal(err)
	}
	runtime.ThemeId, runtime.ThemeName, runtime.ThemeAuthor = "copy", "Renamed", "New author"
	runtime.IsSystem = false
	data, err = json.Marshal(runtime)
	if err != nil {
		t.Fatal(err)
	}
	var saved ThemeSchemaV2
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	document.ThemeId, document.ThemeName, document.ThemeAuthor, document.IsSystem = runtime.ThemeId, runtime.ThemeName, runtime.ThemeAuthor, false
	if !reflect.DeepEqual(saved, document) {
		t.Fatalf("v2 metadata or sparse styles changed: %s", data)
	}
	if themeV2StyleFields["ThemeId"] || themeV2StyleFields["MinWoxVersion"] || themeV2DocumentFields["ResultItemBorderLeft"] {
		t.Fatal("v2 inherited legacy aliases or allowed platform metadata")
	}
}
