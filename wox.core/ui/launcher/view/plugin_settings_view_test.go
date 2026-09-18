package view

import (
	"fmt"
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestPluginSettingsPageUsesFlutterPaneSpacing(t *testing.T) {
	page := PluginSettingsPage(PluginSettingsPageProps{
		Width: 1000, Height: 700,
		List:   PluginListProps{Width: 260, Height: 660, Theme: woxcomponent.ControlTheme{}},
		Detail: PluginDetailProps{Width: 659, Height: 660, Theme: woxcomponent.ControlTheme{}},
		Theme:  woxcomponent.ControlTheme{},
	})

	container, ok := page.(woxwidget.Container)
	if !ok {
		t.Fatalf("page type = %T, want woxwidget.Container", page)
	}
	if container.Padding != woxwidget.UniformInsets(20) {
		t.Fatalf("page padding = %+v, want 20 on every edge", container.Padding)
	}
	panes := container.Child.(woxwidget.Flex)
	if len(panes.Children) != 5 {
		t.Fatalf("pane child count = %d, want list, gap, divider, gap, detail", len(panes.Children))
	}
	if gap := panes.Children[1].(woxwidget.Container).Width; gap != 10 {
		t.Fatalf("left divider gap = %.0f, want 10", gap)
	}
	if divider := panes.Children[2].(woxwidget.Container).Width; divider != 1 {
		t.Fatalf("divider width = %.0f, want 1", divider)
	}
	if gap := panes.Children[3].(woxwidget.Container).Width; gap != 10 {
		t.Fatalf("right divider gap = %.0f, want 10", gap)
	}
}

func TestPluginSettingsFilterPanelAlignsWithFilterButton(t *testing.T) {
	page := PluginSettingsPage(PluginSettingsPageProps{
		Width: 1000, Height: 700,
		List:        PluginListProps{Width: 250, Height: 660, Theme: woxcomponent.ControlTheme{}},
		Detail:      PluginDetailProps{Width: 689, Height: 660, Theme: woxcomponent.ControlTheme{}},
		FilterPanel: &PluginFilterPanelProps{Width: 360, Theme: woxcomponent.ControlTheme{}},
		Theme:       woxcomponent.ControlTheme{},
	}).(woxwidget.Stack)

	positioned := page.Children[2]
	if positioned.Left != 236 || positioned.Top != 64 {
		t.Fatalf("filter panel position = (%v, %v), want (236, 64) aligned to the trailing filter action", positioned.Left, positioned.Top)
	}
}

func TestPluginSettingsFilterPanelUsesAvailableWidth(t *testing.T) {
	page := PluginSettingsPage(PluginSettingsPageProps{
		Width: 600, Height: 700,
		List:        PluginListProps{Width: 250, Height: 660, Theme: woxcomponent.ControlTheme{}},
		Detail:      PluginDetailProps{Width: 289, Height: 660, Theme: woxcomponent.ControlTheme{}},
		FilterPanel: &PluginFilterPanelProps{Width: 360, Theme: woxcomponent.ControlTheme{}},
		Theme:       woxcomponent.ControlTheme{},
	}).(woxwidget.Stack)

	positioned := page.Children[2]
	panel := positioned.Child.(woxwidget.FocusScope).Child.(woxwidget.Container)
	if positioned.Left != 228 || panel.Width != 360 {
		t.Fatalf("filter panel geometry = left %v width %v, want a 360-wide panel shifted to the 12px edge", positioned.Left, panel.Width)
	}
}

func TestPluginListLoadingUsesCenteredIndicator(t *testing.T) {
	loading := PluginList(PluginListProps{Width: 260, Height: 660, Message: "Loading"}).(woxwidget.Align)
	if loading.Width != 260 || loading.Height != 660 || loading.Horizontal != 0.5 || loading.Vertical != 0.5 {
		t.Fatalf("plugin list loading alignment = %#v, want centered", loading)
	}
	if indicator := loading.Child.(woxwidget.LoopAnimation); indicator.Key != "wox-loading-indicator" {
		t.Fatalf("plugin list loading child = %#v, want WoxLoadingIndicator", indicator)
	}
}

func TestPluginDetailUsesCatalogEmptyState(t *testing.T) {
	empty := PluginDetail(PluginDetailProps{Width: 600, Height: 700, EmptyTitle: "No plugins", EmptyDescription: "Refresh to load plugins", EmptyIcon: &woxui.Image{}}).(woxwidget.Align)
	if empty.Width != 600 || empty.Height != 700 || empty.Horizontal != 0.5 || empty.Vertical != 0.42 {
		t.Fatalf("plugin detail empty state = %#v, want the centered catalog empty state", empty)
	}
}

func TestPluginFilterPanelUsesLabeledDropdowns(t *testing.T) {
	panel := PluginFilterPanel(PluginFilterPanelProps{
		Width: 360, LabelWidth: 80,
		Fields: []PluginFilterField{
			{ID: "enabled", Label: "Enabled Status", Value: "All"},
			{ID: "upgrade", Label: "Upgrade Status", Value: "All"},
			{ID: "type", Label: "Plugin Type", Value: "All"},
			{ID: "runtime", Label: "Plugin Runtime", Value: "All"},
		},
		ResetLabel: "Reset", ResetEnabled: true,
		Theme:  woxcomponent.ControlTheme{Surface: woxui.Color{R: 10, G: 20, B: 30, A: 120}, Border: woxui.Color{R: 90, G: 90, B: 90, A: 255}},
		OnOpen: func(string, woxui.Rect) {}, OnReset: func() {},
	}).(woxwidget.FocusScope).Child.(woxwidget.Container)

	if !panel.Floating || panel.Color != (woxui.Color{R: 10, G: 20, B: 30, A: 120}) || panel.BorderWidth != 1 || panel.Height != 240 {
		t.Fatalf("filter panel surface = floating %v color %#v border %v height %v, want a floating Surface surface with a hairline at 240px", panel.Floating, panel.Color, panel.BorderWidth, panel.Height)
	}
	rows := panel.Child.(woxwidget.Flex)
	if len(rows.Children) != 5 || rows.Gap != 12 {
		t.Fatalf("filter panel rows = %d gap %v, want four labeled dropdowns and a reset action with 12px gaps", len(rows.Children), rows.Gap)
	}
	row := rows.Children[0].(woxwidget.Flex)
	if row.CrossAxisAlignment != woxwidget.CrossAxisCenter || row.Gap != 12 {
		t.Fatalf("filter row alignment/gap = %v/%v, want centered 12px label/control pairing", row.CrossAxisAlignment, row.Gap)
	}
	if _, ok := row.Children[1].(woxwidget.Keyed); !ok {
		t.Fatalf("filter control = %T, want a keyed dropdown anchor", row.Children[1])
	}
	reset := rows.Children[4].(woxwidget.Align)
	if reset.Horizontal != 1 || reset.Vertical != 0.5 {
		t.Fatalf("reset alignment = (%v, %v), want trailing and vertically centered", reset.Horizontal, reset.Vertical)
	}
	if focusedControlGesture(reset.Child).ID != "plugin-filter-reset" {
		t.Fatal("filter panel must expose a trailing reset action")
	}
}

func TestPluginListSearchHasFilterActionOnly(t *testing.T) {
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, Placeholder: "Search 67 plugins", FilterLabel: "Filter", FilterIcon: &woxui.Image{},
		OnFilter: func() {}, Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}},
	})
	search := list.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	overlay := search.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Flex)
	if len(overlay.Children) != 3 {
		t.Fatalf("search overlay children = %d, want spacer, filter action, and trailing inset", len(overlay.Children))
	}
	action := overlay.Children[1].(woxwidget.Align).Child.(woxwidget.Stateful)
	if action.Key != "plugin-filter" {
		t.Fatalf("search action = %q, want only the filter button", action.Key)
	}
}

func TestPluginListSearchUsesValueText(t *testing.T) {
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, Placeholder: "Search 67 plugins",
		Theme: woxcomponent.ControlTheme{Text: title, TextSecondary: woxui.Color{R: 255, A: 255}},
	})
	search := list.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	wantBorder := title
	wantBorder.A = 100
	if search.BorderColor != wantBorder {
		t.Fatalf("plugin search border = %#v, want Text %#v", search.BorderColor, wantBorder)
	}
	input := search.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
	if input.Theme.TextSecondary != title {
		t.Fatalf("plugin search hint token = %#v, want Text so TextSecondary cannot restyle it", input.Theme.TextSecondary)
	}
}

func TestPluginListBadgeUsesFlutterTagGeometry(t *testing.T) {
	activeColor := woxui.Color{R: 90, G: 100, B: 110, A: 255}
	inactiveColor := woxui.Color{R: 120, G: 130, B: 140, A: 255}
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Entries: pluginListEntries(
			PluginListItem{ID: "clipboard", Name: "Clipboard", Status: "1.0.0", Badge: "System", Selected: true},
			PluginListItem{ID: "shell", Name: "Shell", Status: "1.0.0", Badge: "System"},
		),
		Theme: woxcomponent.ControlTheme{SelectionText: activeColor, AccentText: woxui.Color{A: 255}, TextSecondary: inactiveColor, Text: title},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.LazyList)
	row := focusedControlGesture(rows.ItemBuilder(0)).Child.(woxwidget.Container)
	rowContent := row.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	status := rowContent.Children[1].(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Text)
	if status.Color != activeColor {
		t.Fatalf("selected plugin subtitle color = %#v, want %#v", status.Color, activeColor)
	}
	inactiveRow := focusedControlGesture(rows.ItemBuilder(1)).Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	inactiveStatus := inactiveRow.Children[1].(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Text)
	if inactiveStatus.Color != inactiveColor {
		t.Fatalf("unselected plugin subtitle color = %#v, want %#v", inactiveStatus.Color, inactiveColor)
	}
	badgeSlot := rowContent.Children[2].(woxwidget.Align)
	if badgeSlot.Horizontal != 1 || badgeSlot.Vertical != 0.5 {
		t.Fatalf("badge slot alignment = (%v, %v), want trailing and vertically centered", badgeSlot.Horizontal, badgeSlot.Vertical)
	}
	textWidth := rowContent.Children[1].(woxwidget.Container).Width
	if contentWidth := float32(32+10) + textWidth + float32(10) + badgeSlot.Width; contentWidth != row.Width-row.Padding.Left-row.Padding.Right {
		t.Fatalf("plugin row content width = %v, want inner width %v so the tag keeps the 6px trailing padding", contentWidth, row.Width-row.Padding.Left-row.Padding.Right)
	}
	badge := badgeSlot.Child.(woxwidget.Container)
	wantPadding := woxwidget.Insets{Left: 4, Top: 2, Right: 4, Bottom: 2}
	if badge.Padding != wantPadding {
		t.Fatalf("badge padding = %+v, want %+v", badge.Padding, wantPadding)
	}
	if badge.BorderWidth != 1 {
		t.Fatalf("badge border width = %v, want 1", badge.BorderWidth)
	}
	label := badge.Child.(woxwidget.Text)
	if label.Color != inactiveColor || badge.BorderColor != inactiveColor {
		t.Fatal("selected badge must retain the secondary text color")
	}
	if label.Style.Size != 11 {
		t.Fatalf("badge font size = %v, want 11", label.Style.Size)
	}
	inactiveBadge := inactiveRow.Children[2].(woxwidget.Align).Child.(woxwidget.Container)
	if inactiveBadge.BorderColor != inactiveColor || inactiveBadge.Child.(woxwidget.Text).Color != inactiveColor {
		t.Fatalf("unselected System badge = border %#v text %#v, want Text", inactiveBadge.BorderColor, inactiveBadge.Child.(woxwidget.Text).Color)
	}
}

func TestPluginStoreInstalledIconUsesSelectionColor(t *testing.T) {
	installedIcon := &woxui.Image{}
	selectedInstalledIcon := &woxui.Image{}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, InstalledIcon: installedIcon, InstalledSelectedIcon: selectedInstalledIcon,
		Entries: pluginListEntries(
			PluginListItem{ID: "awake", Name: "Awake", ShowInstalledIcon: true, Selected: true},
			PluginListItem{ID: "arc", Name: "Arc", ShowInstalledIcon: true},
		),
		Theme: woxcomponent.ControlTheme{},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.LazyList)
	selectedRow := focusedControlGesture(rows.ItemBuilder(0)).Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	inactiveRow := focusedControlGesture(rows.ItemBuilder(1)).Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	selected := selectedRow.Children[2].(woxwidget.Align).Child.(woxwidget.Image)
	inactive := inactiveRow.Children[2].(woxwidget.Align).Child.(woxwidget.Image)

	if selected.Source != selectedInstalledIcon || inactive.Source != installedIcon || selected.Width != 20 || selected.Height != 20 {
		t.Fatalf("installed icons = selected %p inactive %p size %.0fx%.0f", selected.Source, inactive.Source, selected.Width, selected.Height)
	}
}

func TestPluginListSearchHighlightKeepsSelectedFillAndAddsBorder(t *testing.T) {
	selected := woxui.Color{R: 60, G: 80, B: 100, A: 255}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Entries: pluginListEntries(PluginListItem{ID: "clipboard", Name: "Clipboard", Selected: true, Highlighted: true}),
		Theme:   woxcomponent.ControlTheme{SelectionBackground: selected},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := focusedControlGesture(props.Content.(woxwidget.LazyList).ItemBuilder(0)).Child.(woxwidget.Container)
	if row.Color != selected {
		t.Fatalf("selected plugin fill = %#v, want selected color %#v", row.Color, selected)
	}
	if row.BorderWidth != 1 || row.BorderColor.A != 122 {
		t.Fatalf("plugin search highlight border = %#v at %v, want Flutter 0.48 alpha border", row.BorderColor, row.BorderWidth)
	}
}

func TestPluginListUsesSharedScrollbarWhenOverflowing(t *testing.T) {
	items := make([]PluginListItem, 10)
	for index := range items {
		items[index] = PluginListItem{ID: fmt.Sprint(index), Name: fmt.Sprint(index)}
	}
	list := PluginList(PluginListProps{Width: 260, Height: 300, Entries: pluginListEntries(items...), Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}}})
	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	scrollbar := column.Children[1].(woxwidget.Stateful)
	props := scrollbar.Widget.(woxcomponent.ScrollViewProps)

	if props.ContentHeight != 0 || props.ThumbColor.A != 255 {
		t.Fatalf("plugin scrollbar hint = %.0f color alpha %d, want measured shared scrollbar", props.ContentHeight, props.ThumbColor.A)
	}
}

func TestPluginDetailHeaderAlignsTitleWithIcon(t *testing.T) {
	header := pluginDetailHeader(PluginHeaderProps{
		Name: "Wox Query History", Version: "1.0.0", Author: "Wox Launcher",
	}, 600, 80, woxcomponent.ControlTheme{}).(woxwidget.Container)
	if len(header.Child.(woxwidget.Flex).Children) != 2 {
		t.Fatal("plugin header must use two compact rows")
	}
	identity := header.Child.(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
	row := identity.Child.(woxwidget.Flex)
	if len(row.Children) != 2 {
		t.Fatal("identity must contain only icon and title")
	}
	if identity.Height != 40 || row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("identity row = height %v alignment %v, want a 40-high centered icon/title row", identity.Height, row.CrossAxisAlignment)
	}
	if _, ok := row.Children[0].(woxwidget.Container); !ok {
		t.Fatalf("identity leading child = %T, want the plugin icon", row.Children[0])
	}
	title := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	if title.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("title alignment = %v, want the same cross-axis center as the icon", title.CrossAxisAlignment)
	}
	if name := title.Children[0].(woxwidget.Text); name.Value != "Wox Query History" {
		t.Fatalf("title = %q, want the plugin name beside the icon", name.Value)
	}
}

func TestPluginManagementButtonsUseIntrinsicWidth(t *testing.T) {
	actions := pluginOutlineActions([]PluginAction{{ID: "plugin-uninstall", Label: "Uninstall", Width: 124, Enabled: true}}, woxcomponent.ControlTheme{})
	button := focusedControlGesture(actions.(woxwidget.Flex).Children[0]).Child.(woxwidget.Container)

	if button.Width != 0 {
		t.Fatalf("plugin management button width = %v, want intrinsic width", button.Width)
	}
}

func TestPluginStoreChipCentersContent(t *testing.T) {
	chip := pluginStoreChip("v0.2.3", nil, nil, woxcomponent.ControlTheme{}).(woxwidget.Gesture).Child.(woxwidget.Container)
	content := chip.Child.(woxwidget.Align)

	if content.Horizontal != 0.5 || content.Vertical != 0.5 {
		t.Fatalf("plugin store chip alignment = (%v, %v), want centered", content.Horizontal, content.Vertical)
	}
}

func TestPluginStoreHeaderCentersActionsWithoutDuplicateWebsite(t *testing.T) {
	store := pluginStoreDetail(PluginStoreDetailProps{WebsiteLabel: "Website", OnWebsite: func() {}, Management: []PluginAction{{ID: "disable", Label: "Disable", Enabled: true}}}, 800, 600, woxcomponent.ControlTheme{})
	header := store.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if header.Axis != woxwidget.Horizontal || header.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(header.Children) != 2 {
		t.Fatal("header must center actions alongside identity")
	}
	identity := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Flex)
	if len(identity.Children) != 2 {
		t.Fatal("identity must contain name and author only")
	}
	actions := header.Children[1].(woxwidget.Flex)
	if len(actions.Children) != 1 || focusedControlGesture(actions.Children[0]).ID != "disable" {
		t.Fatal("header must only expose management actions")
	}
}

func TestPluginDetailsShareDescriptionFirstLayout(t *testing.T) {
	form := &PluginFormProps{SectionLabel: "Settings", Rows: []woxwidget.Widget{woxwidget.Keyed{Key: "plugin-setting-row-1", Child: woxwidget.Container{Height: 800}}}, KeepVisibleKey: "plugin-setting-row-1"}
	keywords := &PluginFormProps{Rows: []woxwidget.Widget{woxwidget.Text{Value: "Keywords"}}}
	commands := &PluginFormProps{Rows: []woxwidget.Widget{woxwidget.Text{Value: "Commands"}}}
	metadata := &PluginMetadataProps{EmptyTitle: "No data access", EmptyDescription: "Redundant explanation"}
	description := PluginStoreDetailProps{ScrollID: "plugin-detail-example", Description: "Run shell commands", Runtime: "Go", Keywords: keywords, Commands: commands, Metadata: metadata, Error: "Operation failed"}
	for _, width := range []float32{340, 800} {
		for _, installed := range []bool{false, true} {
			props := PluginDetailProps{Width: width, Height: 500, Store: &description}
			if installed {
				props.Store = nil
				props.Editor = &PluginEditorProps{ScrollID: description.ScrollID, Form: form, DescriptionDetail: &description, Keywords: keywords, Commands: commands, Metadata: metadata, Error: description.Error}
			}
			page := PluginDetail(props).(woxwidget.Container)
			children := page.Child.(woxwidget.Flex).Children
			if len(children) != 2 {
				t.Fatal("want header and shared scroll without tabs")
			}
			scroll := children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
			if scroll.Key != "plugin-detail-example" || scroll.Height != 420 || scroll.Width != width-16 {
				t.Fatal("incorrect shared viewport")
			}
			if installed && scroll.KeepVisibleKey != form.KeepVisibleKey {
				t.Fatal("lost settings focus target")
			}
			if scroll.Width-scroll.Content.(woxwidget.Container).Width != 16 {
				t.Fatal("scrollbar must have a 16-unit gutter outside the content")
			}
			content := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children
			privacy := content[len(content)-2].(woxwidget.Flex)
			if len(privacy.Children) != 1 {
				t.Fatal("no-access privacy copy must be a single sentence")
			}
			intro := content[0].(woxwidget.Flex)
			if intro.Children[0].(woxwidget.Flex).CrossAxisAlignment != woxwidget.CrossAxisCenter {
				t.Fatal("description and tags must be vertically centered")
			}
			if intro.Children[0].(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 120}).(woxwidget.TextBlock).Value != description.Description {
				t.Fatal("description must appear first without a group heading")
			}
			want := 8
			if installed {
				want = 10
			}
			if len(content) != want {
				t.Fatalf("got %d content items, want %d", len(content), want)
			}
			if installed {
				header := content[1].(woxwidget.Container)
				if header.Height != 43 {
					t.Fatal("plugin groups must use the shared settings section header")
				}
				if content[2].(woxwidget.Flex).Children[0].(woxwidget.Keyed).Key != "plugin-setting-row-1" {
					t.Fatal("settings must immediately follow description")
				}
			}
			if content[len(content)-1].(woxwidget.TextBlock).Value != description.Error {
				t.Fatal("operation error must remain visible")
			}
		}
	}
}

func TestPluginDetailOmitsEmptySettingsAndCommands(t *testing.T) {
	page := pluginEditor(PluginEditorProps{Form: &PluginFormProps{EmptyTitle: "No settings"}, Commands: &PluginFormProps{EmptyTitle: "No commands"}, DescriptionDetail: &PluginStoreDetailProps{Description: "Description"}}, 600, 500, woxcomponent.ControlTheme{}).(woxwidget.Container)
	scroll := page.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if len(scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children) != 1 {
		t.Fatal("empty settings and commands must not create placeholder groups")
	}
}

func TestPluginMetadataDescriptionWrapsInsteadOfClipping(t *testing.T) {
	row := pluginMetadataRow(PluginMetadataItem{
		Title:       "Active window process ID",
		Description: "For example, when browsing a webpage this plugin reads the active window process ID.",
	}, 600, woxcomponent.ControlTheme{}).(woxwidget.Container)
	content := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	title := content.Children[0].(woxwidget.TextBlock)
	description := content.Children[1].(woxwidget.TextBlock)
	if row.Height != 0 || content.Axis != woxwidget.Vertical || title.MaxLines != 0 || description.MaxLines != 0 || title.Width != 600 || description.Width != 600 {
		t.Fatal("privacy title and description must wrap at full width with intrinsic height")
	}

}

func TestFormTableInlineTitleMatchesFormLabelWeight(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "roots", Title: "Search Roots", Width: 720, Height: 220, InlineTitle: true,
		AddLabel: "Add", Theme: woxcomponent.ControlTheme{Text: woxui.Color{R: 240, G: 240, B: 240, A: 255}},
	})
	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	title := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightRegular || title.Color.R != 240 {
		t.Fatalf("inline table title = size %.0f weight %v color %#v, want the shared 13 regular form label", title.Style.Size, title.Style.Weight, title.Color)
	}
}

func TestFormTableInlineTitleUsesHeaderWeight(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "query-hotkeys", Title: "Query Hotkeys", Width: 720, Height: 220, InlineTitle: true,
		HeaderWeight: woxui.FontWeightSemibold, AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})
	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	title := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightSemibold {
		t.Fatalf("settings table title = size %.0f weight %v, want 13 semibold", title.Style.Size, title.Style.Weight)
	}
}

func TestFormTableInlineHeaderShowsTemplateAndAddActions(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Width: 720, Height: 220, InlineTitle: true,
		SecondaryLabel: "From Templates", AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})

	container := field.(woxwidget.Container)
	column := container.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Flex)
	actions := header.Children[1].(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("header action count = %d, want template and add", len(actions.Children))
	}
	for index, action := range actions.Children {
		button := focusedControlGesture(action).Child.(woxwidget.Container)
		if button.Padding.Left != 12 || button.Padding.Right != 12 {
			t.Fatalf("header action %d horizontal padding = %+v, want shared compact padding", index, button.Padding)
		}
	}
}

func TestFormTableInlineHeaderAlignsAddButtonWithTableRightEdge(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "query-hotkeys", Title: "Query Hotkeys", Width: 720, Height: 220, InlineTitle: true,
		AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})

	// Expanded on the title consumes the leftover width, so a content-sized action
	// row lands on the table's right edge. A fixed-width slot would align the same
	// way but caps the buttons, which clipped every non-English label.
	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	if _, expanded := header.Children[0].(woxwidget.Expanded); !expanded {
		t.Fatalf("header title = %T, want Expanded so the actions sit on the table right edge", header.Children[0])
	}
	actions, ok := header.Children[1].(woxwidget.Flex)
	if !ok {
		t.Fatalf("header actions = %T, want an intrinsically sized row", header.Children[1])
	}
	if len(actions.Children) != 1 {
		t.Fatalf("header action count = %d, want only add", len(actions.Children))
	}
}

func TestFormTableInlineHeaderAlignsActionsWithTitleWhenDescriptionIsPresent(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "tray-queries", Title: "Tray Queries", Description: "Open a configured query from the tray.",
		Width: 720, Height: 220, InlineTitle: true, AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})

	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	if header.CrossAxisAlignment != woxwidget.CrossAxisStart || header.Gap != 16 {
		t.Fatalf("header alignment/gap = %v/%v, want top alignment with 16px action gap", header.CrossAxisAlignment, header.Gap)
	}
	left := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	description := left.Children[1].(woxwidget.TextBlock)
	if description.Height != 0 || description.MaxLines != 2 || description.LineHeight != 16 {
		t.Fatalf("description geometry = height %v, lines %d, line height %v; want intrinsic height up to two 16px lines", description.Height, description.MaxLines, description.LineHeight)
	}
	withoutDescription := FormTableFieldHeight(true, "", 1, 0)
	withDescription := FormTableFieldHeight(true, "Open a configured query from the tray.", 1, 0)
	if withDescription-withoutDescription != 24 {
		t.Fatalf("inline description height delta = %v, want 24px for the two-line header maximum", withDescription-withoutDescription)
	}
}

func TestReadonlyInlineTableOmitsEmptyHeader(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "plugin-commands", Width: 720, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: "Name"}, {Label: "Description"}},
		Rows:    []FormTableRow{{Index: 0, Cells: []FormTableCell{{Text: "fix"}, {Text: "Fix selection"}}}},
		Theme:   woxcomponent.ControlTheme{},
	}).(woxwidget.Container)

	children := field.Child.(woxwidget.Flex).Children
	if len(children) != 1 {
		t.Fatalf("headerless readonly table children = %d, want only the grid", len(children))
	}
	if _, ok := children[0].(woxwidget.Stateful); !ok {
		t.Fatalf("headerless readonly table child = %T, want shared table grid", children[0])
	}
}

func TestFormTableInlineHeaderForwardsDemoHover(t *testing.T) {
	var gotKind string
	var gotInside bool
	var gotBounds woxui.Rect
	field := FormTableField(FormTableFieldProps{
		ID: "query-hotkeys", Title: "Query Hotkeys", Width: 720, Height: 220, InlineTitle: true,
		DemoKind: "query-hotkeys", DemoIcon: &woxui.Image{}, AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
		OnDemoHover: func(kind string, inside bool, bounds woxui.Rect) {
			gotKind = kind
			gotInside = inside
			gotBounds = bounds
		},
	})

	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	title := header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	trigger := title.Children[1].(woxwidget.Semantics)
	if trigger.AutomationID != "settings-demo-query-hotkeys" {
		t.Fatalf("demo automation ID = %q", trigger.AutomationID)
	}
	bounds := woxui.Rect{X: 310, Y: 142, Width: 18, Height: 18}
	trigger.Child.(woxwidget.Gesture).OnHoverAt(true, bounds)
	if gotKind != "query-hotkeys" || !gotInside || gotBounds != bounds {
		t.Fatalf("demo hover = (%q, %v, %+v), want query-hotkeys at %+v", gotKind, gotInside, gotBounds, bounds)
	}
}

func TestFormTableMixedLayoutUsesMeasuredLabelWidth(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Width: 720, Height: 220, LabelWidth: 84,
		AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})

	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	if row.Gap != 12 {
		t.Fatalf("label gap = %v, want Flutter's 12", row.Gap)
	}
	label := row.Children[0].(woxwidget.Container)
	if label.Width != 84 {
		t.Fatalf("label width = %v, want measured width 84", label.Width)
	}
	fieldColumn := row.Children[1].(woxwidget.Flex)
	actions := fieldColumn.Children[0].(woxwidget.Container)
	if actions.Width != 624 {
		t.Fatalf("field width = %v, want 720 - 84 - 12 = 624", actions.Width)
	}
	actionAlignment := actions.Child.(woxwidget.Align)
	if actionAlignment.Width != 624 || actionAlignment.Horizontal != 1 {
		t.Fatalf("mixed-layout add alignment = width %v alignment %v, want the table's right edge", actionAlignment.Width, actionAlignment.Horizontal)
	}
}

func TestFormTableMixedLayoutPlacesDescriptionBelowTable(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Description: "Configure custom commands.",
		Width: 720, Height: 272, LabelWidth: 84, AddLabel: "Add", Theme: woxcomponent.ControlTheme{},
	})

	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	title := row.Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if title.Value != "Commands" || title.Style.Size != 13 || title.Style.Weight != woxui.FontWeightRegular {
		t.Fatalf("mixed-layout title = %+v, want the shared form label", title)
	}
	fieldColumn := row.Children[1].(woxwidget.Flex)
	table := fieldColumn.Children[1].(woxwidget.Flex)
	if table.Gap != 4 {
		t.Fatalf("table tooltip gap = %v, want Flutter's 4", table.Gap)
	}
	if len(table.Children) != 1 {
		t.Fatalf("table child count = %d, want only the tooltip when the list is empty", len(table.Children))
	}
	description := table.Children[0].(woxwidget.TextBlock)
	if description.Value != "Configure custom commands." {
		t.Fatalf("table tooltip = %q", description.Value)
	}

	withoutDescription := FormTableFieldHeight(false, "", 0, 0)
	withDescription := FormTableFieldHeight(false, "Configure custom commands.", 0, 0)
	if withDescription-withoutDescription != 52 {
		t.Fatalf("description height delta = %v, want 52", withDescription-withoutDescription)
	}
}

func TestFormTableColumnWidthsMatchFlutterAndDoNotScale(t *testing.T) {
	columns := []FormTableColumn{
		{Label: "Alias", Width: 100},
		{Label: "Command", Tooltip: "Command help"},
		{Label: "Interpreter", Tooltip: "Interpreter help", Width: 120},
		{Label: "Working directory", Tooltip: "Directory help", Width: 180},
		{Label: "Enabled", Width: 60},
		{Label: "Silent", Tooltip: "Silent help", Width: 60},
	}

	widths := formTableColumnWidths(columns, 626)
	want := []float32{110, 110, 130, 190, 70, 70, 130}
	if len(widths) != len(want) {
		t.Fatalf("column width count = %d, want %d", len(widths), len(want))
	}
	for index := range want {
		if widths[index] != want[index] {
			t.Fatalf("column width %d = %v, want %v", index, widths[index], want[index])
		}
	}
}

func TestFormTablePinsOperationColumnBesideScrollableContent(t *testing.T) {
	props := FormTableFieldProps{
		ID: "commands", Width: 626, Height: 118, OperationLabel: "Operation", Theme: woxcomponent.ControlTheme{},
		Columns: []FormTableColumn{
			{Label: "Alias", Width: 100},
			{Label: "Command", Tooltip: "Command help"},
			{Label: "Interpreter", Tooltip: "Interpreter help", Width: 120},
			{Label: "Working directory", Tooltip: "Directory help", Width: 180},
			{Label: "Enabled", Width: 60},
			{Label: "Silent", Tooltip: "Silent help", Width: 60},
		},
		Rows: []FormTableRow{{Index: 0, Cells: make([]FormTableCell, 6)}},
	}

	grid := formTableGridFlex(t, buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()))
	header := grid.Children[0].(woxwidget.Flex)
	left := header.Children[0].(woxwidget.ScrollView)
	if !left.Horizontal {
		t.Fatal("table content should scroll horizontally")
	}
	if left.Width != 496 || left.ContentWidth != 680 {
		t.Fatalf("left table geometry = viewport %v, content %v; want 496 and 680", left.Width, left.ContentWidth)
	}
	operationHeader := header.Children[1].(woxwidget.Container)
	if operationHeader.Width != 130 {
		t.Fatalf("pinned operation width = %v, want 130", operationHeader.Width)
	}
}

func TestFormTableExpandsLastColumnBeforePinnedOperation(t *testing.T) {
	props := FormTableFieldProps{
		ID: "ignored-apps", Width: 626, Height: 118, OperationLabel: "Operation", Theme: woxcomponent.ControlTheme{},
		Columns: []FormTableColumn{{Label: "Application", Tooltip: "Application help"}},
		Rows:    []FormTableRow{{Index: 0, Cells: []FormTableCell{{Text: "Notes"}}}},
	}

	grid := formTableGridFlex(t, buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()))
	header := grid.Children[0].(woxwidget.Flex)
	left := header.Children[0].(woxwidget.ScrollView)
	leftHeader := left.Child.(woxwidget.Flex)
	column := leftHeader.Children[0].(woxwidget.Container)
	if column.Width != 496 || left.ContentWidth != 496 {
		t.Fatalf("expanded column geometry = column %v, content %v; want 496 and 496", column.Width, left.ContentWidth)
	}
	operation := header.Children[1].(woxwidget.Container)
	if operation.Width != 130 {
		t.Fatalf("operation column width = %v, want 130", operation.Width)
	}
}

func TestFormTableBodyScrollsAllRowsBeforeOuterPage(t *testing.T) {
	rows := make([]FormTableRow, 8)
	for index := range rows {
		rows[index] = FormTableRow{Index: index, Cells: []FormTableCell{{Text: "row"}}}
	}
	props := FormTableFieldProps{
		ID: "commands", Width: 626, Height: tableSurfaceHeaderHeight + tableSurfaceRowHeight*3,
		Columns: []FormTableColumn{{Label: "Name", Width: 180}}, Rows: rows, Theme: woxcomponent.ControlTheme{},
	}

	grid := formTableGridFlex(t, buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()))
	body := grid.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	if body.Height != tableSurfaceRowHeight*3 || body.ContentHeight != tableSurfaceRowHeight*8 {
		t.Fatalf("vertical body geometry = viewport %v, content %v; want %v and %v", body.Height, body.ContentHeight, tableSurfaceRowHeight*3, tableSurfaceRowHeight*8)
	}
	bodyRow := body.Content.(woxwidget.Flex)
	left := bodyRow.Children[0].(woxwidget.ScrollView)
	renderedRows := left.Child.(woxwidget.Flex)
	if len(renderedRows.Children) != len(rows) {
		t.Fatalf("rendered row count = %d, want all %d rows in the inner scroll content", len(renderedRows.Children), len(rows))
	}
}

func TestFormTableOperationCellSupportsSpecializedTrailingActions(t *testing.T) {
	icon := &woxui.Image{}
	props := FormTableFieldProps{
		ID: "ai-skills", HideEditAction: true, HideCloneAction: true,
		DeleteLabel: "Delete", DeleteIcon: icon, Theme: woxcomponent.ControlTheme{},
	}
	row := FormTableRow{
		Index: 4, ReadOnly: true,
		TrailingActions: []FormTableRowAction{{
			ID: "open-folder", Label: "Open folder", Icon: icon, OnTap: func() {},
		}},
	}

	cell := formTableOperationCell(props, row, 130, false).(woxwidget.Container)
	actions := cell.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("skills operation count = %d, want delete and open-folder", len(actions.Children))
	}
	deleteButton := formTableOperationIconButton(actions.Children[0])
	openButton := formTableOperationIconButton(actions.Children[1])
	if !deleteButton.Disabled || deleteButton.OnTap != nil {
		t.Fatal("read-only skill delete action should be disabled")
	}
	if openButton.Disabled || openButton.OnTap == nil {
		t.Fatal("read-only skill open-folder action should remain enabled")
	}
}

func TestFormTableDataCellDoesNotOpenEditor(t *testing.T) {
	cell := formTableDataCell(FormTableFieldProps{Theme: woxcomponent.ControlTheme{}}, FormTableCell{Text: "value"}, 120)
	if _, interactive := cell.(woxwidget.Gesture); interactive {
		t.Fatal("plain table cells must not open the row editor")
	}
}

func TestFormTableTypographyMatchesSharedTokens(t *testing.T) {
	props := FormTableFieldProps{ID: "commands", EmptyLabel: "No rows", Theme: woxcomponent.ControlTheme{}}
	headerCell := formTableHeaderCell(props, FormTableColumn{Label: "Name"}, 120, 0).(woxwidget.Container)
	headerAlign := headerCell.Child.(woxwidget.Align)
	header := headerAlign.Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock)
	body := formTableDataCellContent(t, formTableDataCell(props, FormTableCell{Text: "Translate"}, 120)).(woxwidget.TextBlock)
	empty := formTableEmptyLabel(t, formTableEmptyState(props, 240, woxcomponent.SettingsControlHeight))

	if header.Style.Size != woxcomponent.TableHeaderFontSize || header.Style.Weight != woxui.FontWeightRegular || body.Style.Size != woxcomponent.TableBodyFontSize || empty.Style.Size != woxcomponent.TableEmptyFontSize {
		t.Fatalf("table typography = size %v weight %v / %v / %v, want regular %v/%v/%v", header.Style.Size, header.Style.Weight, body.Style.Size, empty.Style.Size, woxcomponent.TableHeaderFontSize, woxcomponent.TableBodyFontSize, woxcomponent.TableEmptyFontSize)
	}
	if header.Height != 18 || header.LineHeight != 18 || header.AlignmentY != 0.5 {
		t.Fatalf("table header slot = height %v line height %v alignment %v, want an 18px optically centered slot", header.Height, header.LineHeight, header.AlignmentY)
	}
	if headerCell.Padding.Top != 0 || headerAlign.Height != tableSurfaceHeaderHeight || headerAlign.Vertical != 0.5 {
		t.Fatalf("table header alignment = padding %#v slot %#v, want a full-height centered slot", headerCell.Padding, headerAlign)
	}
}

func TestFormTableOperationIncludesEditCloneAndDelete(t *testing.T) {
	icon := &woxui.Image{}
	props := FormTableFieldProps{
		ID: "commands", EditLabel: "Edit", CloneLabel: "Clone", DeleteLabel: "Delete",
		EditIcon: icon, CloneIcon: icon, DeleteIcon: icon, Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}, Error: woxui.Color{R: 210, A: 255}},
	}
	cell := formTableOperationCell(props, FormTableRow{Index: 3}, 130, false).(woxwidget.Container)
	actions := cell.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(actions.Children) != 3 {
		t.Fatalf("operation action count = %d, want edit, clone, and delete", len(actions.Children))
	}
	for index, action := range actions.Children {
		button := formTableOperationIconButton(action)
		if button.Width != woxcomponent.SettingsCompactControlHeight || button.Height != woxcomponent.SettingsCompactControlHeight || button.HoverBackground.A == 0 {
			t.Fatalf("operation action %d = %+v, want hoverable compact icon button", index, button)
		}
		if index < 2 && button.OnHoverAt != nil {
			t.Fatalf("operation action %d unexpectedly exposes a tooltip hover callback", index)
		}
	}
}

func TestFormTableDeleteDialogMatchesFlutterActions(t *testing.T) {
	dialog := FormTableDeleteDialog(FormTableDeleteDialogProps{
		Width: 912, Height: 768, Message: "Are you sure?", CancelLabel: "Cancel", DeleteLabel: "Delete", Theme: woxcomponent.ControlTheme{},
	}).(woxwidget.Stateful)
	state := dialog.CreateState()
	state.InitState(woxwidget.StateContext{}, dialog.Widget)
	stack := state.Build(woxwidget.StateContext{}, dialog.Widget).(woxwidget.Stack)
	panel := stack.Children[1].Child.(woxwidget.FocusScope).Child.(woxwidget.Semantics).Child.(woxwidget.Container)
	if panel.Width != 270 || panel.Height != 116 || panel.Radius != 20 {
		t.Fatalf("delete dialog geometry = %vx%v radius %v, want 270x116 radius 20", panel.Width, panel.Height, panel.Radius)
	}
	content := panel.Child.(woxwidget.Flex)
	actionsFooter := content.Children[1].(woxwidget.Container)
	if actionsFooter.Height != SettingsDialogActionsHeight || actionsFooter.Padding.Top != SettingsDialogActionsHeight-settingsDialogActionHeight {
		t.Fatalf("delete footer = height %v padding %+v, want shared settings actions", actionsFooter.Height, actionsFooter.Padding)
	}
	actions := actionsFooter.Child.(woxwidget.Align)
	if actions.Horizontal != 1 {
		t.Fatal("delete actions should stay right-aligned")
	}
	buttons := actions.Child.(woxwidget.Flex)
	if len(buttons.Children) != 2 {
		t.Fatalf("delete dialog action count = %d, want cancel and delete", len(buttons.Children))
	}
	for _, child := range buttons.Children {
		container := focusedControlGesture(child).Child.(woxwidget.Container)
		if container.Width != 0 {
			t.Fatalf("delete action width = %v, want content-sized Cancel/Delete labels", container.Width)
		}
	}
}

func TestPluginStoreScreenshotPreservesAspectRatioFromContentWidth(t *testing.T) {
	screenshot := &woxui.Image{Width: 1600, Height: 900}
	widget := pluginStoreScreenshot(PluginStoreDetailProps{Screenshot: screenshot}, 580, woxcomponent.ControlTheme{})
	frame := widget.(woxwidget.Gesture).Child.(woxwidget.Container)
	image := frame.Child.(woxwidget.Image)
	wantHeight := float32(580) * 900 / 1600

	if frame.Width != 580 || frame.Height != wantHeight {
		t.Fatalf("screenshot frame = %vx%v, want content-width aspect ratio 580x%v", frame.Width, frame.Height, wantHeight)
	}
	if image.Radius != 8 || image.Fit != woxwidget.ImageFitContain || image.Width != frame.Width || image.Height != frame.Height {
		t.Fatalf("screenshot image = %#v, want contain fit matching the frame", image)
	}
}

func TestPluginStoreScreenshotShowsLoadingIndicatorBeforeImageArrives(t *testing.T) {
	widget := pluginStoreScreenshot(PluginStoreDetailProps{ScreenshotLoading: true}, 580, woxcomponent.ControlTheme{Focus: woxui.Color{R: 1, G: 2, B: 3, A: 255}})
	loading := widget.(woxwidget.Align)
	if loading.Width != 580 || loading.Height != 48 || loading.Horizontal != 0.5 || loading.Vertical != 0.5 {
		t.Fatalf("screenshot loading align = %#v, want a compact centered placeholder", loading)
	}
	indicator := loading.Child.(woxwidget.LoopAnimation)
	if indicator.Key != "wox-loading-indicator" {
		t.Fatalf("screenshot loading child = %#v, want WoxLoadingIndicator", indicator)
	}
}

func TestPluginStoreDescriptionOmitsEmptyRuntimeChip(t *testing.T) {
	body := pluginStoreDescriptionContent(PluginStoreDetailProps{
		Description: "View cloud sync status", WebsiteChipLabel: "GitHub",
	}, 580, woxcomponent.ControlTheme{}).(woxwidget.Flex)
	chips := body.Children[0].(woxwidget.Flex).Children[1].(woxwidget.Flex)
	if len(chips.Children) != 1 {
		t.Fatalf("metadata chips = %#v, want only the website chip for native Go plugins", chips.Children)
	}
}

func TestPluginStoreDescriptionKeepsHostRuntimeChip(t *testing.T) {
	body := pluginStoreDescriptionContent(PluginStoreDetailProps{
		Description: "Workouts", Runtime: "Python", WebsiteChipLabel: "GitHub",
	}, 580, woxcomponent.ControlTheme{}).(woxwidget.Flex)
	chips := body.Children[0].(woxwidget.Flex).Children[1].(woxwidget.Flex)
	if len(chips.Children) != 2 || chips.Children[0].(woxwidget.Gesture).ID != "plugin-runtime" {
		t.Fatalf("metadata chips = %#v, want the Python runtime chip", chips.Children)
	}
}

func TestPluginStoreDescriptionUsesLoadingPlaceholderWithoutBlankPanel(t *testing.T) {
	body := pluginStoreDescriptionContent(PluginStoreDetailProps{
		Name: "Strava", Description: "Workouts", Author: "Wox-launcher", Version: "0.0.1", Runtime: "Python",
		ScreenshotLoading: true,
	}, 580, woxcomponent.ControlTheme{Focus: woxui.Color{A: 255}}).(woxwidget.Flex)
	children := body.Children
	if len(children) != 2 {
		t.Fatalf("description children = %d, want description, metadata, and loading placeholder", len(children))
	}
	if description := children[0].(woxwidget.Flex).Children[0].(woxwidget.Expanded).Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 200}).(woxwidget.TextBlock); description.Value != "Workouts" || description.MaxLines != 0 {
		t.Fatal("description must retain the plugin text without repeating its name or author")
	}
	if _, ok := children[1].(woxwidget.Align); !ok {
		t.Fatal("expected compact loading placeholder")
	}

}

func TestPluginScreenshotFollowsInstalledSettingsButStaysAboveStoreDetails(t *testing.T) {
	for _, loading := range []bool{false, true} {
		for _, installed := range []bool{false, true} {
			description := PluginStoreDetailProps{Description: "Description", ScreenshotLoading: loading}
			if !loading {
				description.Screenshot = &woxui.Image{Width: 640, Height: 480}
			}
			props := PluginDetailProps{Width: 600, Height: 500, Store: &description}
			if installed {
				props.Store = nil
				props.Editor = &PluginEditorProps{DescriptionDetail: &description, Form: &PluginFormProps{Rows: []woxwidget.Widget{woxwidget.Text{Value: "Setting"}}}}
			}
			page := PluginDetail(props).(woxwidget.Container)
			scroll := page.Child.(woxwidget.Flex).Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
			content := scroll.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children
			intro := content[0].(woxwidget.Flex)
			if installed {
				if len(intro.Children) != 1 || len(content) != 4 {
					t.Fatal("installed screenshot must follow the settings")
				}
			} else if len(intro.Children) != 2 || len(content) != 1 {
				t.Fatal("store screenshot must remain with the description")
			}
			if description.ScreenshotLoading != loading || (!loading && description.Screenshot == nil) {
				t.Fatal("layout must not mutate source props")
			}
		}
	}
}

func pluginListEntries(items ...PluginListItem) []PluginListEntry {
	entries := make([]PluginListEntry, len(items))
	for index, item := range items {
		entries[index] = PluginListEntry{ID: item.ID, Item: item}
	}
	return entries
}

func pluginListScroll(list woxwidget.Widget) woxcomponent.ScrollViewProps {
	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	return column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
}

func pluginListLazyList(list woxwidget.Widget) woxwidget.LazyList {
	switch content := pluginListScroll(list).Content.(type) {
	case woxwidget.LazyList:
		return content
	case woxwidget.Stateful:
		return content.Widget.(woxwidget.LazyList)
	default:
		panic(fmt.Sprintf("plugin list content = %T", content))
	}
}

func TestPluginListSectionsUseCompactHeadersAndVariableExtent(t *testing.T) {
	secondary := woxui.Color{R: 120, G: 130, B: 140, A: 255}
	entries := []PluginListEntry{
		{ID: "enabled", Header: "Enabled"},
		{ID: "clipboard", Item: PluginListItem{ID: "clipboard", Name: "Clipboard"}},
		{ID: "disabled", Header: "Disabled"},
		{ID: "shell", Item: PluginListItem{ID: "shell", Name: "Shell", Selected: true}},
	}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, Entries: entries,
		Theme: woxcomponent.ControlTheme{TextSecondary: secondary},
	})
	rows := pluginListLazyList(list)
	if rows.ItemCount != 4 || rows.ItemExtentAt == nil {
		t.Fatalf("grouped list = count %d extentAt %v", rows.ItemCount, rows.ItemExtentAt != nil)
	}
	if got := rows.ItemExtentAt(0); got != woxcomponent.SettingsNavGroupHeight {
		t.Fatalf("first header extent = %v, want %v", got, woxcomponent.SettingsNavGroupHeight)
	}
	if got := rows.ItemExtentAt(1); got != pluginListRowHeight {
		t.Fatalf("plugin row extent = %v, want %v", got, pluginListRowHeight)
	}
	wantDisabled := woxcomponent.SettingsNavGroupHeight + woxcomponent.SettingsNavGroupLead
	if got := rows.ItemExtentAt(2); got != wantDisabled {
		t.Fatalf("following header extent = %v, want %v", got, wantDisabled)
	}

	header := rows.ItemBuilder(0).(woxwidget.Semantics)
	if header.Role != woxui.AccessibilityRoleGroup || header.Label != "Enabled" || header.AutomationID != "plugin-list-section-enabled" {
		t.Fatalf("enabled header = %#v", header)
	}
	label := header.Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text)
	if label.Value != "ENABLED" || label.Style.Size != woxcomponent.SettingsSectionTitleFontSize || label.Style.Weight != woxui.FontWeightSemibold || label.Color != secondary {
		t.Fatalf("enabled header label = %#v", label)
	}
	follow := rows.ItemBuilder(2).(woxwidget.Semantics).Child.(woxwidget.Container)
	if follow.Padding.Top != woxcomponent.SettingsNavGroupLead {
		t.Fatalf("disabled header lead = %v, want %v", follow.Padding.Top, woxcomponent.SettingsNavGroupLead)
	}
}

func TestPluginListKeepVisibleAccountsForSectionHeaders(t *testing.T) {
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Entries: []PluginListEntry{
			{ID: "enabled", Header: "Enabled"},
			{ID: "clipboard", Item: PluginListItem{ID: "clipboard", Name: "Clipboard"}},
			{ID: "disabled", Header: "Disabled"},
			{ID: "shell", Item: PluginListItem{ID: "shell", Name: "Shell", Selected: true}},
		},
		Theme: woxcomponent.ControlTheme{},
	})
	scroll := pluginListScroll(list)
	start := woxcomponent.SettingsNavGroupHeight + pluginListRowHeight + woxcomponent.SettingsNavGroupHeight + woxcomponent.SettingsNavGroupLead
	if scroll.KeepVisible == nil || scroll.KeepVisible.Start != start || scroll.KeepVisible.End != start+pluginListRowHeight {
		t.Fatalf("keep visible = %#v, want [%v, %v]", scroll.KeepVisible, start, start+pluginListRowHeight)
	}
}

func TestPluginListOmitsVariableExtentWithoutSectionHeaders(t *testing.T) {
	rows := pluginListLazyList(PluginList(PluginListProps{
		Width: 260, Height: 660,
		Entries: pluginListEntries(PluginListItem{ID: "store", Name: "Store"}),
		Theme:   woxcomponent.ControlTheme{},
	}))
	if rows.ItemExtentAt != nil {
		t.Fatal("store catalog must keep a fixed-extent LazyList")
	}
}

func TestPluginListDisabledRowsUseSecondaryText(t *testing.T) {
	title := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	secondary := woxui.Color{R: 120, G: 130, B: 140, A: 255}
	selected := woxui.Color{R: 90, G: 100, B: 110, A: 255}
	rows := pluginListLazyList(PluginList(PluginListProps{
		Width: 260, Height: 660,
		Entries: pluginListEntries(
			PluginListItem{ID: "on", Name: "On", Status: "1.0.0", Selected: true},
			PluginListItem{ID: "off", Name: "Off", Status: "1.0.0", Disabled: true, Selected: true},
		),
		Theme: woxcomponent.ControlTheme{Text: title, TextSecondary: secondary, SelectionText: selected},
	}))
	enabledName, enabledStatus := pluginListRowTexts(rows.ItemBuilder(0))
	if enabledName.Color != selected || enabledStatus.Color != selected {
		t.Fatalf("enabled selected colors = %#v %#v, want selection text", enabledName.Color, enabledStatus.Color)
	}
	disabledName, disabledStatus := pluginListRowTexts(rows.ItemBuilder(1))
	if disabledName.Color != secondary || disabledStatus.Color != secondary {
		t.Fatalf("disabled selected colors = %#v %#v, want secondary text so the name stays gray", disabledName.Color, disabledStatus.Color)
	}
}

func pluginListRowTexts(row woxwidget.Widget) (woxwidget.Text, woxwidget.Text) {
	content := focusedControlGesture(row).Child.(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Flex)
	texts := content.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	return texts.Children[0].(woxwidget.Text), texts.Children[1].(woxwidget.Text)
}
