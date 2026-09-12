package launcher

import (
	"encoding/json"
	"testing"
	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestV2IndicatorAndQueryBorder checks authored geometry, platform inheritance, and painted dimensions.
func TestV2IndicatorAndQueryBorder(t *testing.T) {
	raw := `{"SchemaVersion":2,"ThemeId":"indicator","ThemeName":"Indicator","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","ResultItemActiveIndicatorWidth":3,"ResultItemActiveIndicatorInsetLeft":8,"ResultItemActiveIndicatorInsetTop":10,"ResultItemActiveIndicatorInsetBottom":12,"ResultItemActiveIndicatorBorderRadius":2,"QueryBoxBorderBottomWidth":1,"windows":{"QueryBoxBorderBottomColor":"transparent"}}`
	var core common.Theme
	if err := json.Unmarshal([]byte(raw), &core); err != nil {
		t.Fatal(err)
	}
	resolved, err := core.ResolveForTarget("windows", "win11")
	if err != nil {
		t.Fatal(err)
	}
	theme := paletteForTheme(fromCoreTheme(resolved)).componentTheme()
	s := theme.ResultIndicator()
	if s.Width != 3 || s.Left != 8 || s.Top != 10 || s.Bottom != 12 || s.Radius != 2 || s.Color.A != 255 {
		t.Fatalf("indicator lost: %+v", s)
	}
	row := woxcomponent.ResultIndicatorBackground(200, 60, 8, woxui.Color{}, s).(woxwidget.Stack)
	marker := row.Children[1]
	box := marker.Child.(woxwidget.Container)
	if marker.Left != 8 || marker.Top != 10 || box.Width != 3 || box.Height != 38 || box.Radius != 1.5 {
		t.Fatalf("wrong marker: %+v", box)
	}
	color, width := theme.QueryBottomBorder()
	if width != 1 || color.A != 0 {
		t.Fatal("query border override lost")
	}
	saved, err := json.Marshal(core)
	if err != nil {
		t.Fatal(err)
	}
	var round common.Theme
	if err = json.Unmarshal(saved, &round); err != nil {
		t.Fatal(err)
	}
	if round.ResultItemActiveIndicatorInsetTop == nil || *round.ResultItemActiveIndicatorInsetTop != 10 {
		t.Fatal("round trip lost inset")
	}
}
