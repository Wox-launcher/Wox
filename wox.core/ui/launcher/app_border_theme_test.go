package launcher

import (
	"encoding/json"
	"testing"
)

func TestAppBorderThemePreservesZeroAndTransparency(t *testing.T) {
	var theme themeData
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"border","ThemeName":"Border","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","AppBorderColor":"transparent","AppBorderWidth":0,"AppBorderRadius":0}`), &theme); err != nil {
		t.Fatal(err)
	}
	component := paletteForTheme(theme).componentTheme()
	if component.AppBorderColor == nil || component.AppBorderColor.A != 0 || component.AppBorderWidth == nil || *component.AppBorderWidth != 0 || component.AppBorderRadius == nil || *component.AppBorderRadius != 0 {
		t.Fatal("window chrome lost an explicit override")
	}
	if !component.AppWindowChrome {
		t.Fatal("authored outline kept system window material")
	}
}

// TestPreviewCornerTheme checks schema validation and transport to the shared preview.
func TestPreviewCornerTheme(t *testing.T) {
	for _, extra := range []string{``, `,"PreviewBorderRadius":0,"PreviewTagBorderRadius":16`, `,"PreviewBorderRadius":-1`} {
		var theme themeData
		err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"preview","ThemeName":"Preview","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6"`+extra+`}`), &theme)
		if extra == `,"PreviewBorderRadius":-1` {
			if err == nil {
				t.Fatal("accepted negative radius")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		component := paletteForTheme(theme).componentTheme()
		if extra == "" {
			if component.PreviewBorderRadius != nil || component.PreviewTagBorderRadius != nil {
				t.Fatal("lost omission")
			}
		} else if component.PreviewBorderRadius == nil || *component.PreviewBorderRadius != 0 || component.PreviewTagBorderRadius == nil || *component.PreviewTagBorderRadius != 16 {
			t.Fatal("lost preview radii")
		}
	}
}
