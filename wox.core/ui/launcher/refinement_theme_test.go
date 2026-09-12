package launcher

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"wox/common"
	woxui "wox/ui/runtime"
)

// TestRefinementThemeColorsReachRenderer covers sparse defaults and explicit transparent overrides through both loading paths.
func TestRefinementThemeColorsReachRenderer(t *testing.T) {
	document := map[string]any{"SchemaVersion": 2, "ThemeId": "filters", "ThemeName": "Filters", "BaseBackgroundColor": "#182020", "BaseTextColor": "#E0F0E8", "BaseAccentColor": "#70D6A6"}
	var fields []string
	for i := 0; i < reflect.TypeFor[common.ThemeSchemaV2]().NumField(); i++ {
		name := reflect.TypeFor[common.ThemeSchemaV2]().Field(i).Name
		if strings.HasPrefix(name, "Refinement") {
			fields = append(fields, name)
		}
	}
	for _, transparent := range []bool{false, true} {
		for _, name := range fields {
			if transparent {
				document[name] = "transparent"
			}
		}
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		var core common.Theme
		if err := json.Unmarshal(data, &core); err != nil {
			t.Fatal(err)
		}
		var preview themeData
		if err := json.Unmarshal(data, &preview); err != nil {
			t.Fatal(err)
		}
		for _, input := range []themeData{fromCoreTheme(core), preview} {
			theme := paletteForTheme(input).componentTheme()
			for _, name := range fields {
				value := reflect.ValueOf(theme).FieldByName(name).Interface().(*woxui.Color)
				if value == nil || (transparent && value.A != 0) {
					t.Fatalf("%s lost its resolved color", name)
				}
			}
			if transparent {
				for _, active := range []bool{false, true} {
					for _, hovered := range []bool{false, true} {
						text, icon, background, border := theme.RefinementButtonColors(active, hovered)
						if text.A != 0 || icon.A != 0 || background.A != 0 || border.A != 0 {
							t.Fatal("filter button lost explicit transparency")
						}
					}
				}
			}
		}
	}
}
