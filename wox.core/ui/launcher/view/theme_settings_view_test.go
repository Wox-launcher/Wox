package view

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeListUsesSharedSearchFieldGeometry(t *testing.T) {
	icon := &woxui.Image{}
	list := themeList(ThemeSettingsProps{Mode: "installed", Search: woxui.TextEditingState{Text: "query"}, LocateIcon: icon, OnClear: func() {}}, 260, 400).(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	stack := search.Child.(woxwidget.Stack)
	children := stack.Children[1].Child.(woxwidget.Flex).Children
	input := stack.Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	clear := children[1].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	action := children[2].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)

	if search.Height != woxcomponent.SettingsSearchHeight || input.Height != woxcomponent.SettingsSearchHeight || clear.ID != "theme-search-clear" || action.Width != 30 || action.Height != 30 || action.Radius != 15 {
		t.Fatalf("theme search geometry = field %v input %v action %vx%v radius %v, want shared 40px field and circular 30px action", search.Height, input.Height, action.Width, action.Height, action.Radius)
	}
	if inset := children[3].(woxwidget.Container).Width; inset != 4 {
		t.Fatalf("theme search trailing inset = %v, want 4", inset)
	}
}

func TestThemeApplyUsesIntrinsicOutlinedButton(t *testing.T) {
	actions := themeActions(ThemeSettingsProps{ApplyLabel: "应用", Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}}}, ThemeCatalogItem{IsInstalled: true, IsSystem: true})
	button := focusedControlGesture(actions[0]).Child.(woxwidget.Container)

	if button.Width != 0 || button.Height != 32 || button.Color.A != 0 || button.BorderWidth != 1 {
		t.Fatalf("apply button = width %v height %v background alpha %v border %v, want intrinsic shared outlined button", button.Width, button.Height, button.Color.A, button.BorderWidth)
	}
}

func TestThemeDetailKeepsVersionBesideTitle(t *testing.T) {
	detail := ThemeCatalogItem{Name: "Aquarium", Version: "1.1.0"}
	view := themeDetail(ThemeSettingsProps{Detail: &detail}, 600, 700).(woxwidget.Flex)
	header := view.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	titleRow := header.Children[0].(woxwidget.Container).Child.(woxwidget.Clip).Child.(woxwidget.Flex)
	author := header.Children[1].(woxwidget.Flex).Children[0]

	if titleRow.Gap != 10 || titleRow.CrossAxisAlignment != woxwidget.CrossAxisCenter || titleRow.Children[0].(woxwidget.Text).Value != "Aquarium" || titleRow.Children[1].(woxwidget.Text).Value != "1.1.0" {
		t.Fatal("theme version should follow the title with the same alignment as plugin details")
	}
	if _, ok := author.(woxwidget.Expanded); !ok {
		t.Fatalf("theme author slot = %T, want Expanded", author)
	}
}

func TestThemeDetailUsesCatalogEmptyState(t *testing.T) {
	empty := themeDetail(ThemeSettingsProps{EmptyTitle: "No themes", EmptyDescription: "Refresh to load themes", EmptyIcon: &woxui.Image{}}, 600, 700).(woxwidget.Align)
	if empty.Width != 600 || empty.Height != 700 || empty.Horizontal != 0.5 || empty.Vertical != 0.42 {
		t.Fatalf("theme detail empty state = %#v, want the centered catalog empty state", empty)
	}
}

func TestThemeDetailWebsiteUsesSharedButtonHover(t *testing.T) {
	detail := ThemeCatalogItem{Name: "Aquarium", URL: "https://example.com"}
	view := themeDetail(ThemeSettingsProps{
		Detail: &detail, WebsiteLabel: "Website", ExternalIcon: &woxui.Image{}, OnOpenWebsite: func() {},
	}, 600, 700).(woxwidget.Flex)
	header := view.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	website := header.Children[1].(woxwidget.Flex).Children[1].(woxwidget.Align)
	button := focusedControlGesture(website.Child)

	if button.ID != "theme-website" || button.OnTap == nil || button.OnHoverAt == nil {
		t.Fatalf("theme website control = id %q tap %v hover %v, want shared hoverable button", button.ID, button.OnTap != nil, button.OnHoverAt != nil)
	}
}

func TestThemeDetailAnchorsErrorToBodyBottom(t *testing.T) {
	detail := ThemeCatalogItem{Name: "Aquarium"}
	view := themeDetail(ThemeSettingsProps{Detail: &detail, Error: "Unable to load preview"}, 600, 700).(woxwidget.Flex)
	body := view.Children[2].(woxwidget.Stack)
	errorLayer := body.Children[1]
	if !errorLayer.AnchorBottom || !errorLayer.StretchWidth || errorLayer.Left != 16 || errorLayer.Right != 16 || errorLayer.Bottom != 4 {
		t.Fatalf("theme error layout = %+v, want bottom-anchored 16px insets", errorLayer)
	}
}

func TestThemePreviewUsesWallpaperBackdrop(t *testing.T) {
	wallpaper := &woxui.Image{}
	blurred := &woxui.Image{}
	preview := themePreviewTab(ThemeSettingsProps{Wallpaper: wallpaper, WallpaperBlurred: blurred}, ThemeCatalogItem{}, 600, 700).(woxwidget.Container)
	stage := preview.Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Stack)
	window := stage.Children[2].Child.(woxwidget.Stack)

	if stage.Children[1].Child.(woxwidget.Image).Source != wallpaper || window.Children[0].Child.(woxwidget.Image).Source != blurred {
		t.Fatal("theme preview did not reuse the loaded wallpaper layers")
	}
	expectedRadius := 29 * stage.Width / 1440
	if stage.Height != stage.Width*420/900 || stage.Children[0].Child.(woxwidget.Container).Radius != expectedRadius || stage.Children[3].Child.(woxwidget.Container).Radius != expectedRadius {
		t.Fatal("theme preview wallpaper should preserve the cached image aspect ratio and rounded corners")
	}
}

func TestThemeCatalogToolbarMatchesFlutterGeometry(t *testing.T) {
	toolbar := themeCatalogToolbar(ThemeSettingsProps{PreviewOpenLabel: "打开"}, woxcomponent.Theme{ToolbarText: woxui.Color{A: 255}}, 600, true).(woxwidget.Stack)
	body := toolbar.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Flex)
	action := row.Children[0].(woxwidget.Container)
	keycaps := action.Child.(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	keyText := keycaps.Children[0].(woxwidget.Stack).Children[2].Child.(woxwidget.Text)
	if body.Height != 40 || body.Padding.Top != 6 || toolbar.Children[1].Child.(woxwidget.Container).Height != 1 {
		t.Fatalf("theme toolbar = height %v padding %+v, want Flutter 40px footer with top divider", body.Height, body.Padding)
	}
	if keyText.Value != "Enter" || len(keycaps.Children) != 1 {
		t.Fatalf("theme toolbar keycap = %q, want Flutter Enter key label", keyText.Value)
	}
	if row.MainAxisAlignment != woxwidget.MainAxisEnd {
		t.Fatal("theme toolbar action does not use trailing main-axis alignment")
	}
}

func TestThemeAutoPreviewUsesSplitVariantsAndFlutterHint(t *testing.T) {
	accent := woxui.Color{R: 64, G: 196, B: 255, A: 255}
	wallpaper := &woxui.Image{}
	blurred := &woxui.Image{}
	preview := themePreviewTab(ThemeSettingsProps{
		Theme:              woxcomponent.Theme{Background: woxui.Color{R: 20, G: 20, B: 20, A: 255}, ResultTitle: woxui.Color{A: 255}},
		AutoAppearanceHint: "Switches automatically", AutoAppearanceAccent: accent, AutoAppearanceIcon: &woxui.Image{},
		Wallpaper: wallpaper, WallpaperBlurred: blurred,
	}, ThemeCatalogItem{IsAuto: true, LightPreviewTheme: woxcomponent.Theme{Background: woxui.Color{R: 255, G: 255, B: 255, A: 255}}, DarkPreviewTheme: woxcomponent.Theme{Background: woxui.Color{A: 255}}}, 600, 700).(woxwidget.Container)
	children := preview.Child.(woxwidget.Flex).Children
	hint := children[0].(woxwidget.Container)
	stage := children[1].(woxwidget.Align).Child.(woxwidget.Stack)
	autoPreview := stage.Children[2].Child.(woxwidget.Stack)

	if hint.Radius != 10 || hint.BorderWidth != 1 || hint.Color.A != 36 || hint.BorderColor.A != 89 {
		t.Fatalf("AUTO hint = radius %v fill %v border %v/%v, want Flutter dark hint treatment", hint.Radius, hint.Color.A, hint.BorderWidth, hint.BorderColor.A)
	}
	hintContent := hint.Child.(woxwidget.Flex)
	if hint.Padding != woxwidget.UniformInsets(12) || hintContent.CrossAxisAlignment != woxwidget.CrossAxisStart || len(hintContent.Children) != 2 || len(stage.Children) != 4 || len(autoPreview.Children) != 3 {
		t.Fatalf("AUTO content = hint children %d preview layers %d, want icon/text and split background/content", len(hint.Child.(woxwidget.Flex).Children), len(autoPreview.Children))
	}
	if stage.Children[1].Child.(woxwidget.Image).Source != wallpaper || autoPreview.Children[0].Child.(woxwidget.Image).Source != blurred {
		t.Fatal("theme preview did not reuse the loaded wallpaper layers")
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
	props := ThemeSettingsProps{Mode: "installed", SystemLabel: "系统", Items: []ThemeCatalogItem{{ID: "light", Name: "Wox Light", IsSystem: true}}}
	trailing, _ := themeListTrailing(props, props.Items[0], tagColor)
	slot := trailing.(woxwidget.Align)
	if slot.Horizontal != 1 || slot.Vertical != 0.5 {
		t.Fatalf("system tag slot alignment = (%v, %v), want trailing and vertically centered", slot.Horizontal, slot.Vertical)
	}
	tag := slot.Child.(woxwidget.Container)
	wantPadding := woxwidget.Insets{Left: 4, Top: 2, Right: 4, Bottom: 2}
	if tag.Padding != wantPadding || tag.BorderWidth != 1 {
		t.Fatalf("system tag geometry = padding %+v border %v, want shared 1px outlined tag", tag.Padding, tag.BorderWidth)
	}
	if label := tag.Child.(woxwidget.Text); tag.BorderColor != tagColor || label.Color != tagColor {
		t.Fatalf("system tag colors = border %#v text %#v, want %#v", tag.BorderColor, label.Color, tagColor)
	}
	list := themeList(props, 260, 400).(woxwidget.Flex)
	scrollProps := list.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rowSlot := scrollProps.Content.(woxwidget.LazyList).ItemBuilder(0).(woxwidget.Container)
	row := focusedControlGesture(rowSlot.Child).Child.(woxwidget.Container)
	alignment := row.Child.(woxwidget.Align)
	content := alignment.Child.(woxwidget.Flex)
	_, textExpanded := content.Children[1].(woxwidget.Expanded)
	tagSlot := content.Children[2].(woxwidget.Align)
	if row.Padding.Top != 0 || alignment.Vertical != 0.5 || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("theme row alignment = padding %#v slot %#v flex %v, want a full-height centered icon row", row.Padding, alignment, content.CrossAxisAlignment)
	}
	if !textExpanded || tagSlot.Width != 44 {
		t.Fatalf("theme row slots = text expanded %v tag %.0f, want true/44", textExpanded, tagSlot.Width)
	}
}

func TestThemeListUsesSharedScrollbarWhenOverflowing(t *testing.T) {
	items := make([]ThemeCatalogItem, 10)
	for index := range items {
		items[index] = ThemeCatalogItem{ID: fmt.Sprint(index), Name: fmt.Sprint(index)}
	}
	list := themeList(ThemeSettingsProps{Items: items, Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}}}, 260, 300).(woxwidget.Flex)
	scrollbar := list.Children[1].(woxwidget.Stateful)
	props := scrollbar.Widget.(woxcomponent.ScrollViewProps)

	if props.ContentHeight != 0 || props.ThumbColor.A != 255 {
		t.Fatalf("theme scrollbar hint = %.0f color alpha %d, want measured shared scrollbar", props.ContentHeight, props.ThumbColor.A)
	}
}
