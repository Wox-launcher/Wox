package view

import (
	"fmt"
	"testing"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestThemeListPinsCreateAutoAtBottom(t *testing.T) {
	var created bool
	list := themeList(ThemeSettingsProps{
		Mode: "installed", CreateAutoLabel: "New Auto theme", OnCreateAuto: func() { created = true },
		Items: []ThemeCatalogItem{{ID: "one", Name: "One"}},
		Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, 260, 400).(woxwidget.Flex)
	if len(list.Children) != 3 {
		t.Fatalf("installed list children = %d, want search, catalog, and pinned create row", len(list.Children))
	}
	create := list.Children[2].(woxwidget.Container)
	button := focusedControlGesture(create.Child)
	if create.Height != themeCreateAutoRowHeight+themeCreateAutoBottomGap || create.Padding.Bottom != themeCreateAutoBottomGap || button.ID != "theme-create-auto" {
		t.Fatalf("create row = height %.0f inset %.0f id %q", create.Height, create.Padding.Bottom, button.ID)
	}
	if button.Child.(woxwidget.Container).Height != themeCreateAutoRowHeight || button.Child.(woxwidget.Container).Width != 260 {
		t.Fatalf("create button = %#v, want a full-width %.0f-high action", button.Child, themeCreateAutoRowHeight)
	}
	catalog := list.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	used := woxcomponent.SettingsSearchHeight + list.Gap + catalog.Height + list.Gap + create.Height
	if used != 400 {
		t.Fatalf("installed list used height = %.0f, want 400 so the pinned create row stays inside the catalog", used)
	}
	button.OnTap()
	if !created {
		t.Fatal("create row should start a new Auto theme")
	}
	store := themeList(ThemeSettingsProps{Mode: "store", CreateAutoLabel: "New Auto theme", OnCreateAuto: func() {}}, 260, 400).(woxwidget.Flex)
	if len(store.Children) != 2 {
		t.Fatalf("store list children = %d, want no create row", len(store.Children))
	}
}

func TestThemeAutoSlotFollowsDiagonal(t *testing.T) {
	size := woxui.Size{Width: 100, Height: 60}
	if slot := themeAutoSlotAt(woxui.Point{X: 10, Y: 10}, size); slot != "light" {
		t.Fatalf("top-left slot = %q, want light", slot)
	}
	if slot := themeAutoSlotAt(woxui.Point{X: 90, Y: 50}, size); slot != "dark" {
		t.Fatalf("bottom-right slot = %q, want dark", slot)
	}
}

func TestThemeActionsKeepSystemAutoReadOnly(t *testing.T) {
	var edit bool
	system := themeActions(ThemeSettingsProps{
		ApplyLabel: "Apply", Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, ThemeCatalogItem{IsInstalled: true, IsSystem: true, IsAuto: true})
	if len(system) != 1 || focusedControlGesture(system[0]).ID != "theme-apply" {
		t.Fatalf("system auto actions = %#v, want only apply", system)
	}
	user := themeActions(ThemeSettingsProps{
		ApplyLabel: "Apply", EditAutoLabel: "Edit", UninstallLabel: "Uninstall", OnEditAuto: func() { edit = true },
		Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, ThemeCatalogItem{IsInstalled: true, IsAuto: true})
	if len(user) != 3 {
		t.Fatalf("user auto actions = %d, want apply, edit, uninstall", len(user))
	}
	focusedControlGesture(user[1]).OnTap()
	if !edit {
		t.Fatal("user auto actions should open edit")
	}
}

func TestThemeAutoEditorExposesStableAutomationIDs(t *testing.T) {
	editor := themeAutoEditor(ThemeSettingsProps{
		AutoNamePlaceholder: "Name",
		SaveAutoLabel:       "Save",
		CancelAutoLabel:     "Cancel",
		AutoEditor:          &AutoThemeEditorState{Active: true, Name: woxui.TextEditingState{Text: "Smoke Auto"}},
		Theme:               woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, 640, 400).(woxwidget.Flex)
	header := editor.Children[0].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	name := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Stateful)
	if string(name.Key) != "theme-auto-name" {
		t.Fatalf("auto name field key = %q, want theme-auto-name", name.Key)
	}
	field := name.Widget.(woxcomponent.TextFieldProps)
	if field.Height != woxcomponent.SettingsControlHeight || field.Padding.Top != 6 || field.Padding.Bottom != 6 || field.MaxLines != 1 {
		t.Fatalf("auto name field geometry = height %.0f padding %+v maxLines %d, want WoxSettingTextField", field.Height, field.Padding, field.MaxLines)
	}
	actions := header.Children[1].(woxwidget.Flex)
	if focusedControlGesture(actions.Children[0]).ID != "theme-auto-cancel" {
		t.Fatal("auto editor should expose theme-auto-cancel")
	}
	if focusedControlGesture(actions.Children[1]).ID != "theme-auto-save" {
		t.Fatal("auto editor should expose theme-auto-save")
	}
	user := themeActions(ThemeSettingsProps{
		ApplyLabel: "Apply", EditAutoLabel: "Edit", UninstallLabel: "Uninstall",
		OnEditAuto: func() {}, Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, ThemeCatalogItem{IsInstalled: true, IsAuto: true})
	if focusedControlGesture(user[0]).ID != "theme-apply" || focusedControlGesture(user[2]).ID != "theme-uninstall" {
		t.Fatal("user auto actions should expose theme-apply and theme-uninstall")
	}
}

func TestThemeAutoPreviewShowsEditButtons(t *testing.T) {
	var slot string
	overlay := themeAutoSlotEditOverlay(ThemeSettingsProps{
		ModifyAutoLabel: "Modify",
		OnAutoEditSlot:  func(value string) { slot = value },
		Theme:           woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, woxwidget.Container{Width: 200, Height: 120}, 200, 120,
		woxcomponent.Theme{Background: woxui.Color{R: 245, G: 245, B: 245, A: 120}, ResultTitle: woxui.Color{A: 180}},
		woxcomponent.Theme{Background: woxui.Color{R: 43, G: 43, B: 43, A: 90}, ResultTitle: woxui.Color{R: 255, G: 255, B: 255, A: 160}},
	).(woxwidget.Stack)
	if len(overlay.Children) != 3 {
		t.Fatalf("auto edit overlay children = %d, want preview and two edit actions", len(overlay.Children))
	}
	light := focusedControlGesture(overlay.Children[1].Child.(woxwidget.Align).Child)
	dark := focusedControlGesture(overlay.Children[2].Child.(woxwidget.Align).Child)
	if light.ID != "theme-auto-edit-light" || dark.ID != "theme-auto-edit-dark" {
		t.Fatalf("auto edit ids = %q %q", light.ID, dark.ID)
	}
	lightChip := light.Child.(woxwidget.Container)
	darkChip := dark.Child.(woxwidget.Container)
	if lightChip.Color != (woxui.Color{R: 245, G: 245, B: 245, A: 255}) || darkChip.Color != (woxui.Color{R: 43, G: 43, B: 43, A: 255}) {
		t.Fatalf("auto edit should be an opaque fill for each preview half, light %#v dark %#v", lightChip.Color, darkChip.Color)
	}
	if lightChip.BorderWidth != 1 || lightChip.BorderColor != (woxui.Color{A: 255}) || darkChip.BorderWidth != 1 || darkChip.BorderColor != (woxui.Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("auto edit should keep a hairline, light %#v/%.0f dark %#v/%.0f", lightChip.BorderColor, lightChip.BorderWidth, darkChip.BorderColor, darkChip.BorderWidth)
	}
	light.OnTap()
	if slot != "light" {
		t.Fatalf("light edit slot = %q, want light", slot)
	}
}

func TestThemeAutoEditorIsDialogOverlay(t *testing.T) {
	page := ThemeSettingsView(ThemeSettingsProps{
		Width: 840, Height: 700, CreateAutoLabel: "New Auto theme", EditAutoLabel: "Edit",
		AutoEditor: &AutoThemeEditorState{Active: true, Name: woxui.TextEditingState{Text: "Office"}},
		Theme:      woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}).(woxwidget.Flex)
	if len(page.Children) != 3 {
		t.Fatal("catalog should stay a list and detail pane while the editor overlay is hosted on the window")
	}
	overlay := ThemeAutoEditorOverlay(ThemeSettingsProps{
		Width: 840, Height: 700, OverlayWidth: 1100, OverlayHeight: 760, CreateAutoLabel: "New Auto theme",
		AutoEditor: &AutoThemeEditorState{Active: true, Name: woxui.TextEditingState{Text: "Office"}},
		Theme:      woxcomponent.ControlTheme{Text: woxui.Color{A: 255}, Surface: woxui.Color{R: 32, G: 32, B: 32, A: 180}},
	}).(woxwidget.Stack)
	if overlay.Width != 1100 || overlay.Height != 760 {
		t.Fatalf("auto editor overlay = %.0fx%.0f, want the settings window", overlay.Width, overlay.Height)
	}
	dialog := overlay.Children[0].Child.(woxwidget.Stateful)
	if string(dialog.Key) != "theme-auto-editor-dialog" {
		t.Fatalf("auto editor overlay key = %q, want theme-auto-editor-dialog", dialog.Key)
	}
	props := dialog.Widget.(woxcomponent.DialogProps)
	if !props.Solid || props.OverlayWidth != 1100 || props.OverlayHeight != 760 {
		t.Fatalf("auto editor dialog = solid %v overlay %.0fx%.0f, want a solid window-centered panel", props.Solid, props.OverlayWidth, props.OverlayHeight)
	}
}

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

func TestThemeListLocateActionShowsTooltipOverlay(t *testing.T) {
	var shown bool
	var message string
	anchor := woxui.Rect{X: 220, Y: 8, Width: 30, Height: 30}
	list := themeList(ThemeSettingsProps{
		Mode: "installed", LocateLabel: "Locate current theme", LocateIcon: &woxui.Image{},
		OnTooltip: func(inside bool, text string, bounds woxui.Rect) {
			shown, message = inside, text
			if bounds != anchor {
				t.Fatalf("locate tooltip anchor = %#v, want %#v", bounds, anchor)
			}
		},
	}, 260, 400).(woxwidget.Flex)
	search := list.Children[0].(woxwidget.Container)
	action := search.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Flex).Children[1].(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if action.ID != "theme-locate-current" || action.Label != "Locate current theme" || action.OnHoverAt == nil {
		t.Fatalf("locate action = %#v, want a labeled hover tooltip", action)
	}
	action.OnHoverAt(true, anchor)
	if !shown || message != "Locate current theme" {
		t.Fatalf("locate tooltip = shown %v text %q, want the locate label overlay", shown, message)
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

func TestThemeActionsShareControlHeight(t *testing.T) {
	actions := themeActions(ThemeSettingsProps{
		ApplyLabel: "Apply", EditAutoLabel: "Edit", UninstallLabel: "Uninstall", OnEditAuto: func() {},
		Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, ThemeCatalogItem{IsInstalled: true, IsAuto: true})
	if len(actions) != 3 {
		t.Fatalf("user auto actions = %d, want apply, edit, uninstall", len(actions))
	}
	for _, action := range actions {
		button := focusedControlGesture(action).Child.(woxwidget.Container)
		if button.Height != woxcomponent.SettingsControlHeight {
			t.Fatalf("%s height = %.0f, want %.0f", focusedControlGesture(action).ID, button.Height, woxcomponent.SettingsControlHeight)
		}
	}
	detail := themeDetail(ThemeSettingsProps{
		Detail:     &ThemeCatalogItem{Name: "Auto Theme", Version: "1.0.0", IsInstalled: true, IsAuto: true},
		ApplyLabel: "Apply", EditAutoLabel: "Edit", UninstallLabel: "Uninstall", OnEditAuto: func() {},
		Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	}, 600, 700).(woxwidget.Flex)
	titleRow := detail.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(titleRow.Children) != 2 {
		t.Fatal("theme title row should keep the name beside the actions")
	}
	name := titleRow.Children[0].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 200, Height: 40}).(woxwidget.Clip).Child.(woxwidget.Flex)
	if name.Children[0].(woxwidget.Text).Value != "Auto Theme" {
		t.Fatalf("theme title = %q, want Auto Theme", name.Children[0].(woxwidget.Text).Value)
	}
	if _, ok := titleRow.Children[1].(woxwidget.Flex); !ok {
		t.Fatal("theme actions should shrink-wrap beside the title")
	}
}

func TestThemeCatalogRowsShowOnlyCenteredTitle(t *testing.T) {
	row := themeCatalogRowText(200, "Result", woxui.Color{A: 255}, woxcomponent.ControlTheme{}).(woxwidget.Clip)
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
	titleRow := header.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 200, Height: 40}).(woxwidget.Clip).Child.(woxwidget.Flex)
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
	demo := themeCatalogPreviewWindow(stage.Children[2].Child)

	stageWallpaper := stage.Children[1].Child.(woxwidget.Image)
	windowWallpaper := demo.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Image)
	if stageWallpaper.Source != wallpaper || windowWallpaper.Source != blurred {
		t.Fatal("theme preview did not reuse the loaded wallpaper layers")
	}
	expectedRadius := 29 * stage.Width / 1440
	if stage.Height != stage.Width*620/900 || stage.Children[0].Child.(woxwidget.Container).Radius != expectedRadius || stage.Children[3].Child.(woxwidget.Container).Radius != expectedRadius || stageWallpaper.Radius != expectedRadius || windowWallpaper.Radius != 12 {
		t.Fatal("theme preview wallpaper should preserve the cached image aspect ratio and rounded corners")
	}
	if stage.Children[2].Top != 20 {
		t.Fatalf("theme preview top = %.0f, want a high 20-unit inset instead of a vertically centered card", stage.Children[2].Top)
	}
}

func themeCatalogPreviewWindow(preview woxwidget.Widget) woxwidget.Clip {
	return preview.(woxwidget.Align).Child.(woxwidget.Clip)
}

func TestThemeCatalogPreviewUsesV2WindowChrome(t *testing.T) {
	radius, width, indicator, inset, markerRadius := 28, 3, 3, 10, 2
	color := woxui.Color{R: 79, G: 174, B: 133, A: 255}
	theme := woxcomponent.Theme{
		AppBorderRadius: &radius, AppBorderWidth: &width, AppBorderColor: &color,
		ResultItemActiveIndicatorWidth: &indicator, ResultItemActiveIndicatorInsetTop: &inset,
		ResultItemActiveIndicatorInsetBottom: &inset, ResultItemActiveIndicatorBorderRadius: &markerRadius,
		ResultItemActiveIndicatorColor: &color,
		QueryRadius:                    16, AppPadding: woxwidget.Insets{Left: 12, Top: 12, Right: 12, Bottom: 12},
		Background: woxui.Color{R: 28, G: 35, B: 37, A: 255}, QueryText: woxui.Color{A: 255}, ResultTitle: woxui.Color{A: 255},
	}
	demo := themeCatalogPreviewWindow(themeCatalogPreview(ThemeSettingsProps{
		PreviewTitle: "Wox Theme Preview", PreviewTexts: []string{"One", "Two", "Three"},
		PreviewOpenLabel: "Open", PreviewMoreLabel: "More Actions",
	}, theme, 600, 360))
	children := demo.Child.(woxwidget.Stack).Children
	border := children[len(children)-1].Child.(woxwidget.Container)
	if border.Radius != 28 || border.BorderWidth != 3 || border.BorderColor != color {
		t.Fatalf("catalog preview chrome = radius %.0f width %.0f color %#v, want Jade outline", border.Radius, border.BorderWidth, border.BorderColor)
	}
	query := children[2]
	if query.Left != 12 || query.Top != 12 || query.Child.(woxwidget.Container).Radius != 16 {
		t.Fatalf("catalog preview query = inset %.0f/%.0f radius %.0f, want Jade padding and query corners", query.Left, query.Top, query.Child.(woxwidget.Container).Radius)
	}
	selected := children[4].Child.(woxwidget.Stack)
	if _, ok := selected.Children[0].Child.(woxwidget.Stack); !ok {
		t.Fatal("catalog preview selected row must paint the v2 active indicator")
	}
	queryBox := query.Child.(woxwidget.Container)
	queryText := queryBox.Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if queryBox.Height != 40 || queryText.Style.Size != 14 {
		t.Fatalf("catalog preview query = height %.0f size %.0f, want compact 40/14 type", queryBox.Height, queryText.Style.Size)
	}
	if selected.Height != 48 {
		t.Fatalf("catalog preview row height = %.0f, want compact 48", selected.Height)
	}
	if children[5].Top-children[4].Top != 52 {
		t.Fatalf("catalog preview row pitch = %.0f, want 48-high rows with a 4-unit gap", children[5].Top-children[4].Top)
	}
	toolbarFill := children[len(children)-2].Child.(woxwidget.Container).Child.(woxwidget.Clip).Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Container)
	if !toolbarFill.Floating {
		t.Fatal("catalog preview toolbar must use the live floating material")
	}
}

func TestThemeCatalogToolbarMatchesFlutterGeometry(t *testing.T) {
	toolbar := themeCatalogToolbar(ThemeSettingsProps{PreviewOpenLabel: "打开"}, woxcomponent.Theme{ToolbarText: woxui.Color{A: 255}}, 600, true).(woxwidget.Stack)
	body := toolbar.Children[0].Child.(woxwidget.Container)
	row := body.Child.(woxwidget.Flex)
	action := row.Children[0].(woxwidget.Container)
	keycaps := action.Child.(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	keyText := keycaps.Children[0].(woxwidget.Stack).Children[2].Child.(woxwidget.Align).Child.(woxwidget.Text)
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

	if len(stage.Children) != 4 || len(autoPreview.Children) != 4 {
		t.Fatal("AUTO preview must retain its split background, content, and window chrome")
	}
	if stageWallpaper := stage.Children[1].Child.(woxwidget.Image); stageWallpaper.Source != wallpaper || stageWallpaper.Radius != 29*stage.Width/1440 {
		t.Fatal("theme preview wallpaper should clip to the stage rounded corners")
	}
	if autoWallpaper := autoPreview.Children[0].Child.(woxwidget.Image); autoWallpaper.Source != blurred || autoWallpaper.Radius != 12 {
		t.Fatal("theme preview did not reuse the loaded wallpaper layers")
	}
}

func TestThemeAutoPreviewUsesV2WindowChrome(t *testing.T) {
	radius, width := 28, 3
	color := woxui.Color{R: 79, G: 174, B: 133, A: 255}
	dark := woxcomponent.Theme{AppBorderRadius: &radius, AppBorderWidth: &width, AppBorderColor: &color}
	preview := themeAutoCatalogPreview(ThemeSettingsProps{}, woxcomponent.Theme{}, dark, 400, 240).(woxwidget.Stack)
	border := preview.Children[len(preview.Children)-1].Child.(woxwidget.Container)
	if border.Radius != 28 || border.BorderWidth != 3 || border.BorderColor != color {
		t.Fatalf("AUTO preview chrome = radius %.0f width %.0f color %#v, want authored outline", border.Radius, border.BorderWidth, border.BorderColor)
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
	points := themeRoundedRectPoints(woxui.Rect{Width: common.ThemeSwatchSize, Height: common.ThemeSwatchSize}, common.ThemeSwatchRadius)
	if len(points) != 16 || points[0] == (woxui.Point{X: 32}) || points[15] == (woxui.Point{}) {
		t.Fatalf("rounded swatch points = %#v, want curved corners without square vertices", points)
	}
}

func TestThemeSwatchPaintsAuthoredWindowBorder(t *testing.T) {
	width := 3
	color := woxui.Color{R: 0x4F, G: 0xAE, B: 0x85, A: 255}
	swatch := themeSwatch(woxcomponent.Theme{
		Background:      woxui.Color{R: 28, G: 35, B: 37, A: 255},
		AppWindowChrome: true,
		AppBorderWidth:  &width,
		AppBorderColor:  &color,
	}, common.ThemeSwatchSize).(woxwidget.Container)
	if swatch.BorderWidth != common.ThemeSwatchOutlineWidth(3) || swatch.BorderColor != color {
		t.Fatalf("swatch chrome = width %v color %#v, want Jade's authored outline", swatch.BorderWidth, swatch.BorderColor)
	}
}

func TestThemeSwatchSkipsDefaultWindowChrome(t *testing.T) {
	swatch := themeSwatch(woxcomponent.Theme{Background: woxui.Color{A: 255}, AppWindowChrome: true}, common.ThemeSwatchSize).(woxwidget.Container)
	if swatch.BorderWidth != 0 {
		t.Fatalf("swatch border width = %v, want no default divider outline", swatch.BorderWidth)
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

func TestThemeStoreImageTagMatchesPluginTrailingBadge(t *testing.T) {
	props := ThemeSettingsProps{Mode: "store", ImageLabel: "Image"}
	for _, selected := range []bool{false, true} {
		item := ThemeCatalogItem{ID: "knit", Name: "Knit", ImageTheme: true, IsInstalled: true, Selected: selected}
		slot := themeListRow(props, item, 250).(woxwidget.Container)
		row := focusedControlGesture(slot.Child).Child.(woxwidget.Container)
		children := row.Child.(woxwidget.Align).Child.(woxwidget.Flex).Children
		tagSlot := children[2].(woxwidget.Align)
		tag := tagSlot.Child.(woxwidget.Container)
		label := tag.Child.(woxwidget.Text)
		if len(children) != 4 || tagSlot.Width != 44 || tagSlot.Horizontal != 1 || tagSlot.Vertical != 0.5 || label.Value != "Image" || label.Style.Size != woxcomponent.TagFontSize {
			t.Fatal("Image must use the plugin badge slot before the installed icon")
		}
	}
}

func TestThemeStoreImageDetailKeepsMetadataAndMemoryVisible(t *testing.T) {
	props := ThemeSettingsProps{Mode: "store", ImageLabel: "Image", ImageMemoryLabel: "Image · uses more memory"}
	props.Theme.Warning = woxui.Color{R: 253, G: 186, B: 116, A: 255}
	item := ThemeCatalogItem{Author: "qianlifeng", ImageTheme: true}
	for _, width := range []float32{280, 600} {
		meta := themeDetailMeta(props, item, woxwidget.Container{Width: 104, Height: 32})
		group := meta[0].(woxwidget.Expanded).Child.(woxwidget.Flex)
		group.Children[1] = woxwidget.Semantics{Key: "image-tag", Child: group.Children[1]}
		meta[0] = woxwidget.Expanded{Child: group}
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: meta}
		})
		host.AttachServices(settingsWindowHostServices{})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 32}, Scale: 1.5, PixelSize: woxui.PixelSize{Width: int(width * 1.5), Height: 48}})
		bounds, ok := host.BoundsForKey("image-tag")
		host.Dispose()
		if !ok || bounds.X+bounds.Width > width-104-8 || bounds.Width <= 0 {
			t.Fatalf("image tag overlaps website at width %v: %+v", width, bounds)
		}
		for _, description := range []string{"", "A cozy theme."} {
			item.Description = description
			props.Detail = &item
			detail := themeDetail(props, width, 700).(woxwidget.Flex)
			body := detail.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
			index := 0
			if description != "" {
				index = 1
				text := body.Children[0].(woxwidget.Container).Child.(woxwidget.TextBlock)
				if text.Color != props.Theme.TextSecondary {
					t.Fatal("description should retain its secondary text color")
				}
			}
			text := body.Children[index].(woxwidget.Container).Child.(woxwidget.TextBlock)
			if text.Value != props.ImageMemoryLabel || text.Color != props.Theme.Warning || text.MaxLines != 0 || text.Width != width-40 {
				t.Fatalf("memory hint must wrap within the detail pane: %#v", text)
			}
		}
	}
}

func TestThemeStoreDetailShowsScreenshot(t *testing.T) {
	shot := &woxui.Image{Width: 400, Height: 200}
	view := themeDetail(ThemeSettingsProps{
		Mode: "store", Detail: &ThemeCatalogItem{Name: "Omarchy", Description: "Charcoal", Screenshot: shot},
	}, 600, 700).(woxwidget.Flex)
	body := view.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	built := body.Children[1].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 600, Height: 400}).(woxwidget.Align)
	image := built.Child.(woxwidget.Image)
	if image.Source != shot || image.Fit != woxwidget.ImageFitContain {
		t.Fatalf("store screenshot = %#v", image)
	}
}

func TestThemeDetailShowsDescriptionAndPreviewTogether(t *testing.T) {
	for _, mode := range []string{"store", "installed"} {
		detail := ThemeCatalogItem{Name: "Jade", Description: "A jade theme."}
		view := themeDetail(ThemeSettingsProps{Mode: mode, Detail: &detail, ActiveDetailTab: "description"}, 600, 700).(woxwidget.Flex)
		if len(view.Children) != 2 {
			t.Fatal("theme detail must not include tabs")
		}
		body := view.Children[1].(woxwidget.Container)
		if _, isScroll := body.Child.(woxwidget.Stateful); isScroll {
			t.Fatal("theme detail must not put the description in a scroll view")
		}
		rows := body.Child.(woxwidget.Flex).Children
		description := rows[0].(woxwidget.Container).Child.(woxwidget.TextBlock)
		if description.Value != detail.Description || description.MaxLines != 0 || len(rows) != 2 {
			t.Fatal("description must stay above the preview")
		}
		if _, ok := rows[1].(woxwidget.Expanded); !ok {
			t.Fatal("preview must fill the leftover pane under the description")
		}
	}
}
