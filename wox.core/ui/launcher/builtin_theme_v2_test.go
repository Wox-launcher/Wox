package launcher

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"wox/common"
)

// TestBuiltinThemesUseV2 preserves legacy styles except for intentional theme updates.
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
			expected := fromCoreTheme(before)
			if target[0] == "linux" && target[1] == "" {
				// Ordinary Linux desktops now use rounded fallback chrome; Hyprland
				// clears the override so its compositor keeps ownership of the material.
				radius := 8
				expected.AppWindowChrome = true
				expected.AppBorderRadius = &radius
			}
			left, right := reflect.ValueOf(expected), reflect.ValueOf(fromCoreTheme(after))
			for i := 0; i < left.NumField(); i++ {
				field := left.Type().Field(i).Name
				if strings.HasPrefix(field, "ResultItemActiveIndicator") || strings.HasPrefix(field, "ResultItemActiveBorderLeft") || strings.HasPrefix(field, "QueryBoxBorderBottom") {
					continue
				}
				if left.Field(i).Kind() == reflect.String && left.Field(i).String() == "" {
					continue
				}
				if strings.HasSuffix(field, "Color") {
					expected := left.Field(i).String()
					// Glass material and selection updates supersede the legacy fixture.
					if name == "glass" && field == "ToolbarBackgroundColor" {
						expected = "#16161A04"
						if target[0] == "windows" {
							expected = "#12121604"
						} else if target[0] == "linux" {
							expected = "#0D0D0DA5"
							if target[1] == "hyprland" {
								expected = "#15151AB3"
							}
						}
					}
					if name == "glass" && field == "ActionContainerBackgroundColor" {
						expected = "#15151558"
						if target[0] == "linux" {
							expected = "#131518B2"
						}
					}
					if name == "glass" && field == "ResultItemActiveBackgroundColor" {
						expected = "#FFFFFF19"
					}
					if name == "glass" && field == "ToolbarFontColor" {
						expected = "#A3A3A3FF"
					}
					want, ok := decodeThemeColor(expected)
					got, valid := decodeThemeColor(right.Field(i).String())
					if !ok || !valid || want != got {
						t.Errorf("%s/%v: %s = %q, want %q", name, target, field, right.Field(i).String(), expected)
					}
				} else if !reflect.DeepEqual(left.Field(i).Interface(), right.Field(i).Interface()) {
					t.Errorf("%s/%v: %s = %v, want %v", name, target, field, right.Field(i).Interface(), left.Field(i).Interface())
				}
			}
		}
	}
}
