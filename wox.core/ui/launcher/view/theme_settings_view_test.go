package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeListUsesSharedSearchFieldGeometry(t *testing.T) {
	icon := &woxui.Image{}
	list := themeList(ThemeSettingsProps{Mode: "installed", Search: woxui.TextEditingState{Text: "query"}, LocateIcon: icon, OnClear: func() {}}, 260, 400).(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	children := search.Child.(woxwidget.Flex).Children
	input := children[0].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	clear := children[1].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	action := children[2].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)

	if search.Height != 42 || input.Height != 42 || clear.ID != "theme-search-clear" || action.Width != 30 || action.Height != 30 || action.Radius != 15 {
		t.Fatalf("theme search geometry = field %v input %v action %vx%v radius %v, want shared 42px field and circular 30px action", search.Height, input.Height, action.Width, action.Height, action.Radius)
	}
	if inset := children[3].(woxwidget.Container).Width; inset != 4 {
		t.Fatalf("theme search trailing inset = %v, want 4", inset)
	}
}

func TestThemeApplyUsesIntrinsicOutlinedButton(t *testing.T) {
	actions := themeActions(ThemeSettingsProps{ApplyLabel: "应用", Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}}}, ThemeCatalogItem{IsInstalled: true, IsSystem: true})
	button := actions[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if button.Width != 0 || button.Height != 36 || button.Color.A != 0 || button.BorderWidth != 1 {
		t.Fatalf("apply button = width %v height %v background alpha %v border %v, want intrinsic 36px outlined button", button.Width, button.Height, button.Color.A, button.BorderWidth)
	}
}

func TestThemeAutoPreviewUsesSplitVariantsAndFlutterHint(t *testing.T) {
	accent := woxui.Color{R: 64, G: 196, B: 255, A: 255}
	preview := themePreviewTab(ThemeSettingsProps{
		Theme:              woxcomponent.Theme{Background: woxui.Color{R: 20, G: 20, B: 20, A: 255}, ResultTitle: woxui.Color{A: 255}},
		AutoAppearanceHint: "Switches automatically", AutoAppearanceAccent: accent, AutoAppearanceIcon: &woxui.Image{},
	}, ThemeCatalogItem{IsAuto: true, LightPreviewTheme: woxcomponent.Theme{Background: woxui.Color{R: 255, G: 255, B: 255, A: 255}}, DarkPreviewTheme: woxcomponent.Theme{Background: woxui.Color{A: 255}}}, 600, 700).(woxwidget.Container)
	children := preview.Child.(woxwidget.Flex).Children
	hint := children[0].(woxwidget.Container)
	autoPreview := children[1].(woxwidget.Stack)

	if hint.Radius != 10 || hint.BorderWidth != 1 || hint.Color.A != 36 || hint.BorderColor.A != 89 {
		t.Fatalf("AUTO hint = radius %v fill %v border %v/%v, want Flutter dark hint treatment", hint.Radius, hint.Color.A, hint.BorderWidth, hint.BorderColor.A)
	}
	hintContent := hint.Child.(woxwidget.Flex)
	if hint.Padding.Top != 14 || hintContent.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(hintContent.Children) != 2 || len(autoPreview.Children) != 2 {
		t.Fatalf("AUTO content = hint children %d preview layers %d, want icon/text and split background/content", len(hint.Child.(woxwidget.Flex).Children), len(autoPreview.Children))
	}
}

func TestThemeDiagonalRectPolygonSplitsFullBounds(t *testing.T) {
	bounds := woxui.Rect{Width: 100, Height: 60}
	light := themeDiagonalRectPolygon(bounds, bounds, true)
	dark := themeDiagonalRectPolygon(bounds, bounds, false)

	if len(light) != 3 || len(dark) != 3 {
		t.Fatalf("diagonal polygons = %d/%d points, want two triangles", len(light), len(dark))
	}
}

func TestThemeAutoSwatchUsesRoundedOutline(t *testing.T) {
	points := themeRoundedRectPoints(woxui.Rect{Width: 32, Height: 32}, 8)
	if len(points) != 16 || points[0] == (woxui.Point{X: 32}) || points[15] == (woxui.Point{}) {
		t.Fatalf("rounded swatch points = %#v, want curved corners without square vertices", points)
	}
}

func TestThemeSystemTagCentersLabel(t *testing.T) {
	tagColor := woxui.Color{R: 80, G: 90, B: 100, A: 255}
	trailing, _ := themeListTrailing(ThemeSettingsProps{Mode: "installed", SystemLabel: "系统"}, ThemeCatalogItem{IsSystem: true}, tagColor)
	slot := trailing.(woxwidget.Align)
	if slot.Horizontal != 1 || slot.Vertical != 0.5 {
		t.Fatalf("system tag slot alignment = (%v, %v), want trailing and vertically centered", slot.Horizontal, slot.Vertical)
	}
	tag := slot.Child.(woxwidget.Container)
	wantPadding := woxwidget.Insets{Left: 4, Top: 1, Right: 4, Bottom: 1}
	if tag.Padding != wantPadding || tag.BorderWidth != 0.5 {
		t.Fatalf("system tag geometry = padding %+v border %v, want shared Flutter tag", tag.Padding, tag.BorderWidth)
	}
	if label := tag.Child.(woxwidget.Text); tag.BorderColor != tagColor || label.Color != tagColor {
		t.Fatalf("system tag colors = border %#v text %#v, want %#v", tag.BorderColor, label.Color, tagColor)
	}
}
