package launcher

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"wox/common"
)

// TestBuiltinThemesUseV2 preserves every previously authored style across the format conversion.
func TestBuiltinThemesUseV2(t *testing.T) {
	for _, name := range []string{"auto", "dark", "light", "glass"} {
		data, err := os.ReadFile("../../resource/themes/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var theme common.Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if theme.SchemaVersion != 2 || !theme.HasAuthoredStyles() || theme.MinWoxVersion != "2.4.3" {
			t.Fatalf("%s is not a complete v2 theme", name)
		}
		oldData, err := os.ReadFile("../../common/testdata/theme_v1/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var old common.Theme
		if err := json.Unmarshal(oldData, &old); err != nil {
			t.Fatal(err)
		}
		if name == "auto" {
			if !theme.IsAutoAppearance || theme.LightThemeId != old.LightThemeId || theme.DarkThemeId != old.DarkThemeId {
				t.Fatal("auto appearance routing changed")
			}
			continue
		}
		for _, target := range [][2]string{{"windows", ""}, {"windows", "win10"}, {"macos", ""}, {"linux", ""}, {"linux", "hyprland"}} {
			before, err := old.ResolveForTarget(target[0], target[1])
			if err != nil {
				t.Fatal(err)
			}
			after, err := theme.ResolveForTarget(target[0], target[1])
			if err != nil {
				t.Fatal(err)
			}
			oldIndicator := paletteForTheme(fromCoreTheme(before)).componentTheme().ResultIndicator()
			newIndicator := paletteForTheme(fromCoreTheme(after)).componentTheme().ResultIndicator()
			if oldIndicator != newIndicator {
				t.Fatalf("%s: indicator appearance changed", name)
			}
			left, right := reflect.ValueOf(fromCoreTheme(before)), reflect.ValueOf(fromCoreTheme(after))
			for i := 0; i < left.NumField(); i++ {
				field := left.Type().Field(i).Name
				if strings.HasPrefix(field, "ResultItemActiveIndicator") || strings.HasPrefix(field, "ResultItemActiveBorderLeft") || strings.HasPrefix(field, "QueryBoxBorderBottom") {
					continue
				}
				if left.Field(i).Kind() == reflect.String && left.Field(i).String() == "" {
					continue
				}
				if strings.HasSuffix(field, "Color") {
					want, ok := decodeThemeColor(left.Field(i).String())
					got, valid := decodeThemeColor(right.Field(i).String())
					if !ok || !valid || want != got {
						t.Fatalf("%s/%v: %s color changed", name, target, field)
					}
				} else if !reflect.DeepEqual(left.Field(i).Interface(), right.Field(i).Interface()) {
					t.Fatalf("%s/%v: %s changed", name, target, field)
				}
			}
		}
	}
}
