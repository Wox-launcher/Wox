package view

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeSettingsViewUsesSharedCatalogListWidth(t *testing.T) {
	const contentWidth = float32(840)
	page := ThemeSettingsView(ThemeSettingsProps{Width: contentWidth, Height: 700, Theme: woxcomponent.ControlTheme{}}).(woxwidget.Flex)
	list := page.Children[0].(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	divider := page.Children[1].(woxwidget.Container)
	want := woxcomponent.SettingsCatalogListWidth(contentWidth)
	if search.Width != want || divider.Width != woxcomponent.SettingsCatalogDividerGutter {
		t.Fatalf("theme catalog column = list %.0f gutter %.0f, want shared %.0f/%.0f", search.Width, divider.Width, want, woxcomponent.SettingsCatalogDividerGutter)
	}
}

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

func TestThemeListSearchUsesValueText(t *testing.T) {
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	list := themeList(ThemeSettingsProps{
		Mode: "installed", SearchPlaceholder: "Search 14 themes",
		Theme: woxcomponent.ControlTheme{Text: title, TextSecondary: woxui.Color{R: 255, A: 255}},
	}, 260, 400).(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	wantBorder := title
	wantBorder.A = 100
	if search.BorderColor != wantBorder {
		t.Fatalf("theme search border = %#v, want ResultTitle %#v", search.BorderColor, wantBorder)
	}
	input := search.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.Theme.TextSecondary != title {
		t.Fatalf("theme search hint token = %#v, want ResultTitle so ResultSubtitle cannot restyle it", input.Theme.TextSecondary)
	}
}

func TestThemeApplyUsesIntrinsicSecondaryButton(t *testing.T) {
	actions := themeActions(ThemeSettingsProps{ApplyLabel: "应用", Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}}}, ThemeCatalogItem{IsInstalled: true, IsSystem: true})
	button := focusedControlGesture(actions[0]).Child.(woxwidget.Container)

	if button.Width != 0 || button.Height != 32 || button.Color.A != 0 || button.BorderColor.A != 0 {
		t.Fatalf("apply button = width %v height %v background alpha %v border %v, want intrinsic shared secondary button", button.Width, button.Height, button.Color.A, button.BorderWidth)
	}
}

func TestThemeCatalogRowsShowOnlyCenteredTitle(t *testing.T) {
	row := themeCatalogRowText(200, "Result", woxui.Color{A: 255}).(woxwidget.Clip)
	centered := row.Child.(woxwidget.Align)
	if centered.Height != 42 || centered.Vertical != 0.5 || centered.Child.(woxwidget.Text).Value != "Result" {
		t.Fatalf("selected preview text = %#v", centered)
	}
}

func TestThemeDetailKeepsVersionBesideTitle(t *testing.T) {
	detail := ThemeCatalogItem{Name: "Aquarium", Version: "1.1.0"}
	view := themeDetail(ThemeSettingsProps{Detail: &detail}, 600, 700).(woxwidget.Flex)
	header := view.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(header.Children) != 2 || view.Children[0].(woxwidget.Container).Height != 80 {
		t.Fatal("theme header must use two compact rows")
	}
	titleRow := header.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Flex)
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
	body := view.Children[1].(woxwidget.Stack)
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

	stageWallpaper := stage.Children[1].Child.(woxwidget.Image)
	windowWallpaper := window.Children[0].Child.(woxwidget.Image)
	if stageWallpaper.Source != wallpaper || windowWallpaper.Source != blurred {
		t.Fatal("theme preview did not reuse the loaded wallpaper layers")
	}
	expectedRadius := 29 * stage.Width / 1440
	if stage.Height != stage.Width*420/900 || stage.Children[0].Child.(woxwidget.Container).Radius != expectedRadius || stage.Children[3].Child.(woxwidget.Container).Radius != expectedRadius || stageWallpaper.Radius != expectedRadius || windowWallpaper.Radius != 8 {
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

func TestThemeAutoPreviewUsesSplitVariantsWithoutDuplicateHelp(t *testing.T) {
	wallpaper := &woxui.Image{}
	blurred := &woxui.Image{}
	preview := themePreviewTab(ThemeSettingsProps{
		Theme:     woxcomponent.ControlTheme{Background: woxui.Color{R: 20, G: 20, B: 20, A: 255}, Text: woxui.Color{A: 255}},
		Wallpaper: wallpaper, WallpaperBlurred: blurred,
	}, ThemeCatalogItem{IsAuto: true, LightPreviewTheme: woxcomponent.Theme{Background: woxui.Color{R: 255, G: 255, B: 255, A: 255}}, DarkPreviewTheme: woxcomponent.Theme{Background: woxui.Color{A: 255}}}, 600, 700).(woxwidget.Container)
	children := preview.Child.(woxwidget.Flex).Children
	if len(children) != 1 {
		t.Fatal("AUTO preview must not repeat the theme description")
	}
	stage := children[0].(woxwidget.Align).Child.(woxwidget.Stack)
	autoPreview := stage.Children[2].Child.(woxwidget.Stack)

	if len(stage.Children) != 4 || len(autoPreview.Children) != 3 {
		t.Fatal("AUTO preview must retain its split background and content")
	}
	if stageWallpaper := stage.Children[1].Child.(woxwidget.Image); stageWallpaper.Source != wallpaper || stageWallpaper.Radius != 29*stage.Width/1440 {
		t.Fatal("theme preview wallpaper should clip to the stage rounded corners")
	}
	if autoWallpaper := autoPreview.Children[0].Child.(woxwidget.Image); autoWallpaper.Source != blurred || autoWallpaper.Radius != 8 {
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
	props := ThemeSettingsProps{
		Mode: "installed", SystemLabel: "系统",
		Theme: woxcomponent.ControlTheme{TextSecondary: tagColor, SelectionText: woxui.Color{R: 240, G: 244, B: 248, A: 255}},
		Items: []ThemeCatalogItem{{ID: "light", Name: "Wox Light", IsSystem: true, Selected: true}},
	}
	trailing, _ := themeListTrailing(props, props.Items[0])
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
	rowTag := tagSlot.Child.(woxwidget.Container)
	if rowTag.BorderColor != tagColor || rowTag.Child.(woxwidget.Text).Color != tagColor {
		t.Fatalf("selected System tag = border %#v text %#v, want secondary %#v", rowTag.BorderColor, rowTag.Child.(woxwidget.Text).Color, tagColor)
	}
}

func TestThemeListUsesSharedScrollbarWhenOverflowing(t *testing.T) {
	items := make([]ThemeCatalogItem, 10)
	for index := range items {
		items[index] = ThemeCatalogItem{ID: fmt.Sprint(index), Name: fmt.Sprint(index)}
	}
	list := themeList(ThemeSettingsProps{Items: items, Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}}}, 260, 300).(woxwidget.Flex)
	scrollbar := list.Children[1].(woxwidget.Stateful)
	props := scrollbar.Widget.(woxcomponent.ScrollViewProps)

	if props.ContentHeight != 0 || props.ThumbColor.A != 255 {
		t.Fatalf("theme scrollbar hint = %.0f color alpha %d, want measured shared scrollbar", props.ContentHeight, props.ThumbColor.A)
	}
}

// TestActiveThemeExplainsDisabledApply makes the current theme state visible.
func TestActiveThemeExplainsDisabledApply(t *testing.T) {
	actions := themeActions(ThemeSettingsProps{ApplyLabel: "Apply", AppliedLabel: "Applied"}, ThemeCatalogItem{IsInstalled: true, IsSystem: true, Active: true})
	semantics := actions[0].(woxwidget.Semantics)
	if semantics.Label != "Applied" || !semantics.Disabled {
		t.Fatalf("active theme action = %+v", semantics)
	}
}

func TestThemeDetailShowsDescriptionAndPreviewTogether(t *testing.T) {
	for _, mode := range []string{"store", "installed"} {
		detail := ThemeCatalogItem{Name: "Jade", Description: "A jade theme."}
		view := themeDetail(ThemeSettingsProps{Mode: mode, Detail: &detail, ActiveDetailTab: "description"}, 600, 700).(woxwidget.Flex)
		if len(view.Children) != 2 {
			t.Fatal("theme detail must not include tabs")
		}
		scroll := view.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
		rows := scroll.Content.(woxwidget.Flex).Children
		description := rows[0].(woxwidget.Container).Child.(woxwidget.TextBlock)
		if description.Value != detail.Description || description.MaxLines != 0 || len(rows) != 2 {
			t.Fatal("description and preview must share one scroll surface")
		}
	}
}
