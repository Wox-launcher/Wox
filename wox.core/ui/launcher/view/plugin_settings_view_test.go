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
		List:   PluginListProps{Width: 260, Height: 660, Theme: woxcomponent.Theme{}},
		Detail: PluginDetailProps{Width: 659, Height: 660, Theme: woxcomponent.Theme{}},
		Theme:  woxcomponent.Theme{},
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
		List:        PluginListProps{Width: 250, Height: 660, Theme: woxcomponent.Theme{}},
		Detail:      PluginDetailProps{Width: 689, Height: 660, Theme: woxcomponent.Theme{}},
		FilterPanel: &PluginFilterPanelProps{Width: 360, Theme: woxcomponent.Theme{}},
		Theme:       woxcomponent.Theme{},
	}).(woxwidget.Stack)

	positioned := page.Children[2]
	if positioned.Left != 206 || positioned.Top != 64 {
		t.Fatalf("filter panel position = (%v, %v), want (206, 64) at 8px below the filter action", positioned.Left, positioned.Top)
	}
}

func TestPluginSettingsFilterPanelUsesAvailableFlutterWidth(t *testing.T) {
	page := PluginSettingsPage(PluginSettingsPageProps{
		Width: 600, Height: 700,
		List:        PluginListProps{Width: 250, Height: 660, Theme: woxcomponent.Theme{}},
		Detail:      PluginDetailProps{Width: 289, Height: 660, Theme: woxcomponent.Theme{}},
		FilterPanel: &PluginFilterPanelProps{Width: 660, Theme: woxcomponent.Theme{}},
		Theme:       woxcomponent.Theme{},
	}).(woxwidget.Stack)

	positioned := page.Children[2]
	panel := positioned.Child.(woxwidget.FocusScope).Child.(woxwidget.Container)
	if positioned.Left != 206 || panel.Width != 382 {
		t.Fatalf("filter panel geometry = left %v width %v, want trigger-aligned width clamped to the 12px edge", positioned.Left, panel.Width)
	}
}

func TestPluginListMessageKeepsSettingsBackground(t *testing.T) {
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, Message: "Loading", Theme: woxcomponent.Theme{QueryBackground: woxui.Color{R: 255, G: 255, B: 255, A: 255}},
	}).(woxwidget.Container)

	if list.Color.A != 0 {
		t.Fatalf("plugin list message background = %#v, want transparent settings background", list.Color)
	}
}

func TestPluginFilterPanelMatchesFlutterLayout(t *testing.T) {
	panel := PluginFilterPanel(PluginFilterPanelProps{
		Width: 360, LabelWidth: 80, RuntimeTitle: "Runtime",
		Options: []PluginFilterOption{
			{ID: "disabled", Label: "Disabled"},
			{ID: "enabled", Label: "Enabled"},
			{ID: "upgradable", Label: "Upgradable"},
			{ID: "third-party", Label: "Third party"},
		},
		Runtimes: []PluginFilterOption{{ID: "nodejs", Label: "Node.js"}, {ID: "python", Label: "Python"}},
		Theme:    woxcomponent.Theme{Background: woxui.Color{R: 10, G: 20, B: 30, A: 120}},
		OnToggle: func(string) {},
	}).(woxwidget.FocusScope).Child.(woxwidget.Container)

	if panel.Color.A != 255 || panel.Height != 154 {
		t.Fatalf("filter panel surface = alpha %d height %v, want opaque Flutter surface at 154px", panel.Color.A, panel.Height)
	}
	rows := panel.Child.(woxwidget.Flex)
	if len(rows.Children) != 5 || rows.Gap != 10 {
		t.Fatalf("filter panel rows = %d gap %v, want four status rows and one runtime row with 10px gaps", len(rows.Children), rows.Gap)
	}
	status := rows.Children[0].(woxwidget.Flex)
	if _, ok := status.Children[1].(woxwidget.Semantics); !ok {
		t.Fatalf("status control = %T, want checkbox semantics", status.Children[1])
	}
	runtime := rows.Children[4].(woxwidget.Flex)
	if _, ok := runtime.Children[1].(woxwidget.ScrollView); !ok {
		t.Fatalf("runtime options = %T, want one horizontal row", runtime.Children[1])
	}
}

func TestPluginListBadgeUsesFlutterTagGeometry(t *testing.T) {
	activeColor := woxui.Color{R: 90, G: 100, B: 110, A: 255}
	inactiveColor := woxui.Color{R: 120, G: 130, B: 140, A: 255}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Items: []PluginListItem{
			{ID: "clipboard", Name: "Clipboard", Status: "1.0.0", Badge: "System", Selected: true},
			{ID: "shell", Name: "Shell", Status: "1.0.0", Badge: "System"},
		},
		Theme: woxcomponent.Theme{ActionSelectedText: activeColor, ResultSubtitle: inactiveColor},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.Flex)
	row := rows.Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	rowContent := row.Child.(woxwidget.Flex)
	status := rowContent.Children[1].(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Text)
	if status.Color != activeColor {
		t.Fatalf("selected plugin subtitle color = %#v, want %#v", status.Color, activeColor)
	}
	inactiveRow := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
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
	wantPadding := woxwidget.Insets{Left: 4, Top: 1, Right: 4, Bottom: 1}
	if badge.Padding != wantPadding {
		t.Fatalf("badge padding = %+v, want %+v", badge.Padding, wantPadding)
	}
	if badge.BorderWidth != 0.5 {
		t.Fatalf("badge border width = %v, want 0.5", badge.BorderWidth)
	}
	label := badge.Child.(woxwidget.Text)
	if label.Style.Size != 11 {
		t.Fatalf("badge font size = %v, want 11", label.Style.Size)
	}
}

func TestPluginStoreInstalledIconUsesSelectionColor(t *testing.T) {
	installedIcon := &woxui.Image{}
	selectedInstalledIcon := &woxui.Image{}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660, InstalledIcon: installedIcon, InstalledSelectedIcon: selectedInstalledIcon,
		Items: []PluginListItem{
			{ID: "awake", Name: "Awake", ShowInstalledIcon: true, Selected: true},
			{ID: "arc", Name: "Arc", ShowInstalledIcon: true},
		},
		Theme: woxcomponent.Theme{},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rows := props.Content.(woxwidget.Flex)
	selectedRow := rows.Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
	inactiveRow := rows.Children[1].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container).Child.(woxwidget.Flex)
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
		Items: []PluginListItem{{ID: "clipboard", Name: "Clipboard", Selected: true, Highlighted: true}},
		Theme: woxcomponent.Theme{SelectedBackground: selected},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	props := column.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := props.Content.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
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
	list := PluginList(PluginListProps{Width: 260, Height: 300, Items: items, Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}}})
	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	scrollbar := column.Children[1].(woxwidget.Stateful)
	props := scrollbar.Widget.(woxcomponent.ScrollViewProps)

	if props.ContentHeight != 0 || props.ThumbColor.A != 255 {
		t.Fatalf("plugin scrollbar hint = %.0f color alpha %d, want measured shared scrollbar", props.ContentHeight, props.ThumbColor.A)
	}
}

func TestPluginManagementButtonsUseIntrinsicWidth(t *testing.T) {
	actions := pluginOutlineActions([]PluginAction{{ID: "plugin-uninstall", Label: "Uninstall", Width: 124, Enabled: true}}, woxcomponent.Theme{})
	button := actions.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)

	if button.Width != 0 {
		t.Fatalf("plugin management button width = %v, want intrinsic width", button.Width)
	}
}

func TestPluginStoreChipCentersContentVertically(t *testing.T) {
	chip := pluginStoreChip("v0.2.3", nil, nil, woxcomponent.Theme{}).(woxwidget.Gesture).Child.(woxwidget.Container)
	content := chip.Child.(woxwidget.Align)

	if content.Vertical != 0.5 {
		t.Fatalf("plugin store chip vertical alignment = %v, want centered", content.Vertical)
	}
}

func TestPluginStoreDetailTabsMatchInstalledEditorMetrics(t *testing.T) {
	tabs := []PluginTab{{ID: "description", Label: "Description", Width: 96}, {ID: "keywords", Label: "Keywords", Width: 88}}
	store := pluginStoreDetail(PluginStoreDetailProps{
		Name: "Shell", Version: "1.0.0", Author: "Wox", Runtime: "Go", ActiveTab: "description", Tabs: tabs,
	}, 800, 600, woxcomponent.Theme{})

	container := store.(woxwidget.Container)
	if container.Padding.Left != 16 || container.Padding.Right != 16 {
		t.Fatalf("store detail padding = %+v, want the installed editor's 16px inset", container.Padding)
	}
	storeTabs := container.Child.(woxwidget.Flex).Children[1].(woxwidget.Container)

	editor := pluginEditor(PluginEditorProps{
		ActiveTab: "settings",
		Tabs:      []PluginTab{{ID: "settings", Label: "Settings", Width: 80}, {ID: "keywords", Label: "Keywords", Width: 88}},
	}, 800, 600, woxcomponent.Theme{})
	editorTabs := editor.(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Container)

	if storeTabs.Height != editorTabs.Height || storeTabs.Width != editorTabs.Width {
		t.Fatalf("store tab strip %vx%v != installed %vx%v", storeTabs.Width, storeTabs.Height, editorTabs.Width, editorTabs.Height)
	}
	if storeTabs.Height != 44 {
		t.Fatalf("store detail tab strip height = %v, want the installed editor's 44", storeTabs.Height)
	}
}

func TestPluginStoreKeywordsUseSharedFormTabBody(t *testing.T) {
	accent := woxui.Color{R: 33, G: 150, B: 243, A: 255}
	table := FormTableField(FormTableFieldProps{
		ID: "plugin-keywords", Width: 720, MaxHeight: 300, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: "Keyword", Tooltip: "The keyword that triggers this plugin."}},
		Rows:    []FormTableRow{{Index: 0, Cells: []FormTableCell{{Text: "awake"}}}},
		Theme:   woxcomponent.Theme{},
	})
	store := pluginStoreDetail(PluginStoreDetailProps{
		Name: "Awake", Version: "0.0.4", Author: "qianlifeng", Runtime: "NodeJS", ActiveTab: "keywords",
		Tabs: []PluginTab{{ID: "keywords", Label: "Keywords", Width: 88}},
		TabForm: &PluginFormProps{
			Intro:       "Trigger keywords are prefixes you type in Wox to activate this plugin.",
			Rows:        []woxwidget.Widget{table},
			IntroAccent: accent,
		},
	}, 800, 600, woxcomponent.Theme{Background: woxui.Color{R: 30, G: 30, B: 30, A: 255}})

	body := store.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.ScrollView)
	rows := body.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(rows) != 3 {
		t.Fatalf("store keyword body rows = %d, want hint box, spacer, and keyword table", len(rows))
	}
	if _, ok := rows[0].(woxwidget.Container); !ok {
		t.Fatalf("store keyword intro = %T, want hint box container", rows[0])
	}
	if _, ok := rows[2].(woxwidget.Container); !ok {
		t.Fatalf("store keyword table = %T, want readonly form table", rows[2])
	}
}

func TestPluginStoreCommandsUseSharedFormTabBody(t *testing.T) {
	table := FormTableField(FormTableFieldProps{
		ID: "plugin-commands", Width: 720, MaxHeight: 300, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: "Name", Width: 120}, {Label: "Description"}},
		Rows:    []FormTableRow{{Index: 0, Cells: []FormTableCell{{Text: "fix"}, {Text: "Fix selection"}}}},
		Theme:   woxcomponent.Theme{},
	})
	store := pluginStoreDetail(PluginStoreDetailProps{
		Name: "Example", Version: "1.0.0", Author: "Wox", Runtime: "Go", ActiveTab: "commands",
		Tabs:    []PluginTab{{ID: "commands", Label: "Commands", Width: 96}},
		TabForm: &PluginFormProps{Intro: "Commands are subcommands after the trigger keyword.", Rows: []woxwidget.Widget{table}},
	}, 800, 600, woxcomponent.Theme{})

	body := store.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.ScrollView)
	if got := body.ID; got != "plugin-detail-commands" {
		t.Fatalf("store commands scroll id = %q, want shared plugin detail body id", got)
	}
}

func TestPluginStorePrivacyUsesSharedMetadataTabBody(t *testing.T) {
	metadata := PluginMetadataProps{
		Header: "Data Access",
		Items: []PluginMetadataItem{
			{Title: "Active window name", Description: "Reads the active window title."},
		},
	}
	store := pluginStoreDetail(PluginStoreDetailProps{
		Name: "Example", Version: "1.0.0", Author: "Wox", Runtime: "Go", ActiveTab: "privacy",
		Tabs: []PluginTab{{ID: "privacy", Label: "Privacy", Width: 80}}, Metadata: &metadata,
	}, 800, 600, woxcomponent.Theme{})

	body := store.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Container)
	if body.Padding.Top != 18 {
		t.Fatalf("store privacy body padding = %+v, want metadata tab top inset", body.Padding)
	}
}

func TestPluginDetailEmptyStateUsesCenteredTitleAndSubtitle(t *testing.T) {
	body := pluginMetadataTab(PluginMetadataProps{
		EmptyTitle:       "This plugin requires no data access",
		EmptyDescription: "This plugin does not request sensitive data such as the active window, browser URL, or AI model access.",
	}, 600, 400, "plugin-detail-privacy", woxcomponent.Theme{}).(woxwidget.Align)

	content := body.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	title := content.Children[0].(woxwidget.Align).Child.(woxwidget.Text)
	description := content.Children[1].(woxwidget.TextBlock)
	if body.Vertical != 0.45 || title.Style.Size != 18 || description.Style.Size != 12 || !description.Centered {
		t.Fatalf("empty privacy body = vertical %v title %v subtitle %v centered %v, want centered 18px/12px copy", body.Vertical, title.Style.Size, description.Style.Size, description.Centered)
	}
}

func TestPluginStoreCommandsEmptyStateUsesCenteredCopy(t *testing.T) {
	store := pluginStoreDetail(PluginStoreDetailProps{
		Name: "Example", Version: "1.0.0", Author: "Wox", Runtime: "Go", ActiveTab: "commands",
		Tabs: []PluginTab{{ID: "commands", Label: "Commands", Width: 96}},
		TabForm: &PluginFormProps{
			EmptyTitle:       "This plugin has no command",
			EmptyDescription: "This plugin does not provide subcommands after its trigger keyword. Use the trigger keyword directly.",
		},
	}, 800, 600, woxcomponent.Theme{})

	body := store.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Align)
	title := body.Child.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Align).Child.(woxwidget.Text)
	if title.Value != "This plugin has no command" {
		t.Fatalf("empty commands title = %q", title.Value)
	}
}

func TestPluginEditorAutoSavingFormHasNoFooter(t *testing.T) {
	editor := pluginEditor(PluginEditorProps{
		Header:    PluginHeaderProps{},
		ActiveTab: "settings",
		Form: &PluginFormProps{
			Rows: []woxwidget.Widget{woxwidget.Container{Width: 400, Height: 40}},
		},
	}, 600, 500, woxcomponent.Theme{})

	children := editor.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(children) != 3 {
		t.Fatalf("plugin editor child count = %d, want header, tabs, and form only", len(children))
	}
	if _, ok := children[2].(woxwidget.ScrollView); !ok {
		t.Fatalf("plugin editor body type = %T, want scroll view without a save footer", children[2])
	}
	scroll := children[2].(woxwidget.ScrollView)
	content := scroll.Child.(woxwidget.Container)
	if scroll.ContentHeight != 0 || content.Height != 0 {
		t.Fatalf("plugin form height hints = scroll %.0f content %.0f, want intrinsic child measurement", scroll.ContentHeight, content.Height)
	}
}

func TestPluginEditorIntroUsesFlutterHintBoxStyle(t *testing.T) {
	accent := woxui.Color{R: 33, G: 150, B: 243, A: 255}
	icon := &woxui.Image{}
	editor := pluginEditor(PluginEditorProps{
		Form: &PluginFormProps{Intro: "Trigger keyword help", IntroIcon: icon, IntroAccent: accent, Rows: []woxwidget.Widget{woxwidget.Container{Height: 40}}},
	}, 600, 500, woxcomponent.Theme{Background: woxui.Color{R: 250, G: 250, B: 250, A: 255}})

	scroll := editor.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.ScrollView)
	rows := scroll.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	hint := rows.Children[0].(woxwidget.Container)
	content := hint.Child.(woxwidget.Flex)
	if hint.Radius != 10 || hint.BorderWidth != 1 || hint.Color.A != 26 || hint.BorderColor.A != 77 {
		t.Fatalf("hint box style = radius %v border %v colors %#v/%#v", hint.Radius, hint.BorderWidth, hint.Color, hint.BorderColor)
	}
	if len(content.Children) != 2 || content.Children[0].(woxwidget.Image).Source != icon {
		t.Fatal("hint box should show the tinted info icon before its text")
	}
	if text := content.Children[1].(woxwidget.Expanded).Child.(woxwidget.TextBlock); text.Style.Size != 13 {
		t.Fatalf("hint text size = %v, want Flutter 13px", text.Style.Size)
	}
}

func TestPluginEditorDescriptionUsesSharedDetailView(t *testing.T) {
	editor := pluginEditor(PluginEditorProps{
		ActiveTab: "description",
		DescriptionDetail: &PluginStoreDetailProps{
			Name: "Shell", Description: "Run shell commands", Author: "Wox Launcher", Version: "1.0.0", Runtime: "Go", WebsiteChipLabel: "Website ↗",
		},
	}, 800, 600, woxcomponent.Theme{})

	body := editor.(woxwidget.Container).Child.(woxwidget.Flex).Children[2].(woxwidget.Container)
	if body.Padding.Left != 0 || body.Padding.Right != 0 {
		t.Fatalf("description padding = %+v, want flush alignment with trigger-keyword/form tabs", body.Padding)
	}
	if body.Width != 768 {
		t.Fatalf("description width = %v, want the shared inner detail width without an extra inset", body.Width)
	}
	props := body.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: body.Width, Height: body.Height - body.Padding.Top}).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	detail := props.Content.(woxwidget.Flex)
	name := detail.Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	metadata := detail.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	if name.Value != "Shell" || len(metadata.Children) != 3 {
		t.Fatalf("description detail = name %q metadata %d, want identity and version/runtime/website chips", name.Value, len(metadata.Children))
	}
}

func TestPluginMetadataDescriptionWrapsInsteadOfClipping(t *testing.T) {
	row := pluginMetadataRow(PluginMetadataItem{
		Title:       "Active window process ID",
		Description: "For example, when browsing a webpage this plugin reads the active window process ID.",
	}, 600, woxcomponent.Theme{}).(woxwidget.Container)
	description := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.TextBlock)

	if description.MaxLines != 2 || description.LineHeight != 16 {
		t.Fatalf("metadata description wrapping = %d lines at %vpx, want two 16px lines", description.MaxLines, description.LineHeight)
	}
}

func TestFormTableInlineHeaderShowsTemplateAndAddActions(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Width: 720, Height: 220, InlineTitle: true,
		SecondaryLabel: "From Templates", AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	container := field.(woxwidget.Container)
	column := container.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Flex)
	actionSlot := header.Children[1].(woxwidget.Align)
	if actionSlot.Width != 204 || actionSlot.Height != 30 || actionSlot.Horizontal != 1 {
		t.Fatalf("header action slot = %vx%v alignment %v, want a compact slot right-aligned to the table edge", actionSlot.Width, actionSlot.Height, actionSlot.Horizontal)
	}
	actions := actionSlot.Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("header action count = %d, want template and add", len(actions.Children))
	}
	for index, action := range actions.Children {
		button := action.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
		if button.Padding.Left != 12 || button.Padding.Right != 12 {
			t.Fatalf("header action %d horizontal padding = %+v, want shared compact padding", index, button.Padding)
		}
	}
}

func TestFormTableInlineHeaderAlignsAddButtonWithTableRightEdge(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "query-hotkeys", Title: "Query Hotkeys", Width: 720, Height: 220, InlineTitle: true,
		AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	actionSlot := header.Children[1].(woxwidget.Align)
	if actionSlot.Width != 74 || actionSlot.Height != 30 || actionSlot.Horizontal != 1 {
		t.Fatalf("add action slot = %vx%v alignment %v, want compact height and its right edge aligned with the table", actionSlot.Width, actionSlot.Height, actionSlot.Horizontal)
	}
}

func TestFormTableInlineHeaderKeepsActionsNearTableWhenDescriptionIsPresent(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "tray-queries", Title: "Tray Queries", Description: "Open a configured query from the tray.",
		Width: 720, Height: 220, InlineTitle: true, AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Flex)
	if header.CrossAxisAlignment != woxwidget.CrossAxisEnd || header.Gap != 16 {
		t.Fatalf("header alignment/gap = %v/%v, want bottom alignment with 16px action gap", header.CrossAxisAlignment, header.Gap)
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
		Theme:   woxcomponent.Theme{},
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
		DemoKind: "query-hotkeys", DemoIcon: &woxui.Image{}, AddLabel: "Add", Theme: woxcomponent.Theme{},
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
		AddLabel: "Add", Theme: woxcomponent.Theme{},
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
		Width: 720, Height: 272, LabelWidth: 84, AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	row := field.(woxwidget.Container).Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(label.Children) != 1 {
		t.Fatalf("label child count = %d, want title only", len(label.Children))
	}
	fieldColumn := row.Children[1].(woxwidget.Flex)
	table := fieldColumn.Children[1].(woxwidget.Flex)
	if table.Gap != 4 {
		t.Fatalf("table tooltip gap = %v, want Flutter's 4", table.Gap)
	}
	if len(table.Children) != 2 {
		t.Fatalf("table child count = %d, want grid and tooltip", len(table.Children))
	}
	description := table.Children[1].(woxwidget.TextBlock)
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
	want := []float32{110, 130, 150, 210, 70, 90, 130}
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
		ID: "commands", Width: 626, Height: 118, OperationLabel: "Operation", Theme: woxcomponent.Theme{},
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

	grid := buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()).(woxwidget.Container).Child.(woxwidget.Flex)
	header := grid.Children[0].(woxwidget.Flex)
	left := header.Children[0].(woxwidget.ScrollView)
	if !left.Horizontal {
		t.Fatal("table content should scroll horizontally")
	}
	if left.Width != 496 || left.ContentWidth != 760 {
		t.Fatalf("left table geometry = viewport %v, content %v; want 496 and 760", left.Width, left.ContentWidth)
	}
	operationHeader := header.Children[1].(woxwidget.Container)
	if operationHeader.Width != 130 {
		t.Fatalf("pinned operation width = %v, want 130", operationHeader.Width)
	}
}

func TestFormTableExpandsLastColumnBeforePinnedOperation(t *testing.T) {
	props := FormTableFieldProps{
		ID: "ignored-apps", Width: 626, Height: 118, OperationLabel: "Operation", Theme: woxcomponent.Theme{},
		Columns: []FormTableColumn{{Label: "Application", Tooltip: "Application help"}},
	}

	grid := buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()).(woxwidget.Container).Child.(woxwidget.Flex)
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
		Columns: []FormTableColumn{{Label: "Name", Width: 180}}, Rows: rows, Theme: woxcomponent.Theme{},
	}

	grid := buildFormTableGrid(props, props.Width, props.Height, newFormTableGridState()).(woxwidget.Container).Child.(woxwidget.Flex)
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
	props := FormTableFieldProps{
		ID: "ai-skills", HideEditAction: true, HideCloneAction: true,
		DeleteLabel: "Delete", Theme: woxcomponent.Theme{},
	}
	row := FormTableRow{
		Index: 4,
		TrailingActions: []FormTableRowAction{{
			ID: "open-folder", Label: "Open folder",
		}},
	}

	cell := formTableOperationCell(props, row, 130).(woxwidget.Container)
	actions := cell.Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("skills operation count = %d, want delete and open-folder", len(actions.Children))
	}
}

func TestFormTableDataCellDoesNotOpenEditor(t *testing.T) {
	cell := formTableDataCell(FormTableFieldProps{Theme: woxcomponent.Theme{}}, FormTableCell{Text: "value"}, 120)
	if _, interactive := cell.(woxwidget.Gesture); interactive {
		t.Fatal("plain table cells must not open the row editor")
	}
}

func TestFormTableTypographyMatchesSharedTokens(t *testing.T) {
	props := FormTableFieldProps{ID: "commands", EmptyLabel: "No rows", Theme: woxcomponent.Theme{}}
	header := formTableHeaderCell(props, FormTableColumn{Label: "Name"}, 120, 0).(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.TextBlock)
	body := formTableDataCell(props, FormTableCell{Text: "Translate"}, 120).(woxwidget.Container).Child.(woxwidget.TextBlock)
	empty := formTableEmptyState(props, 240, tableSurfaceEmptyHeight).(woxwidget.Container).Child.(woxwidget.Flex).Children[1].(woxwidget.Align).Child.(woxwidget.Text)

	if header.Style.Size != woxcomponent.TableHeaderFontSize || body.Style.Size != woxcomponent.TableBodyFontSize || empty.Style.Size != woxcomponent.TableEmptyFontSize {
		t.Fatalf("table typography = %v/%v/%v, want %v/%v/%v", header.Style.Size, body.Style.Size, empty.Style.Size, woxcomponent.TableHeaderFontSize, woxcomponent.TableBodyFontSize, woxcomponent.TableEmptyFontSize)
	}
	if header.Height != 18 || header.LineHeight != 18 {
		t.Fatalf("table header slot = height %v line height %v, want 18/18", header.Height, header.LineHeight)
	}
}

func TestFormTableOperationIncludesEditCloneAndDelete(t *testing.T) {
	icon := &woxui.Image{}
	props := FormTableFieldProps{
		ID: "commands", EditLabel: "Edit", CloneLabel: "Clone", DeleteLabel: "Delete",
		EditIcon: icon, CloneIcon: icon, DeleteIcon: icon, Theme: woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}},
	}
	cell := formTableOperationCell(props, FormTableRow{Index: 3}, 130).(woxwidget.Container)
	actions := cell.Child.(woxwidget.Flex)
	if len(actions.Children) != 3 {
		t.Fatalf("operation action count = %d, want edit, clone, and delete", len(actions.Children))
	}
	for index, action := range actions.Children {
		button := action.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if button.Width != 26 || button.Height != 24 || button.HoverBackground.A == 0 {
			t.Fatalf("operation action %d = %+v, want hoverable 26x24 icon button", index, button)
		}
		if button.OnHoverAt != nil {
			t.Fatalf("operation action %d unexpectedly exposes a tooltip hover callback", index)
		}
	}
}

func TestFormTableDeleteDialogMatchesFlutterActions(t *testing.T) {
	dialog := FormTableDeleteDialog(FormTableDeleteDialogProps{
		Width: 912, Height: 768, Message: "Are you sure?", CancelLabel: "Cancel", DeleteLabel: "Delete", Theme: woxcomponent.Theme{},
	}).(woxwidget.Stateful)
	state := dialog.CreateState()
	state.InitState(woxwidget.StateContext{}, dialog.Widget)
	stack := state.Build(woxwidget.StateContext{}, dialog.Widget).(woxwidget.Stack)
	panel := stack.Children[1].Child.(woxwidget.FocusScope).Child.(woxwidget.Semantics).Child.(woxwidget.Container)
	if panel.Width != 270 || panel.Height != 110 || panel.Radius != 20 {
		t.Fatalf("delete dialog geometry = %vx%v radius %v, want 270x110 radius 20", panel.Width, panel.Height, panel.Radius)
	}
	content := panel.Child.(woxwidget.Flex)
	actions := content.Children[1].(woxwidget.Align)
	if actions.Horizontal != 1 {
		t.Fatal("delete actions should stay right-aligned")
	}
	buttons := actions.Child.(woxwidget.Flex)
	if len(buttons.Children) != 2 {
		t.Fatalf("delete dialog action count = %d, want cancel and delete", len(buttons.Children))
	}
	for _, child := range buttons.Children {
		container := child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
		if container.Width != 0 {
			t.Fatalf("delete action width = %v, want content-sized Cancel/Delete labels", container.Width)
		}
	}
}

func TestPluginStoreScreenshotPreservesAspectRatioFromContentWidth(t *testing.T) {
	screenshot := &woxui.Image{Width: 1600, Height: 900}
	widget := pluginStoreScreenshot(PluginStoreDetailProps{Screenshot: screenshot}, 580, woxcomponent.Theme{})
	frame := widget.(woxwidget.Gesture).Child.(woxwidget.Container)
	image := frame.Child.(woxwidget.Image)
	wantHeight := float32(580) * 900 / 1600

	if frame.Width != 580 || frame.Height != wantHeight {
		t.Fatalf("screenshot frame = %vx%v, want content-width aspect ratio 580x%v", frame.Width, frame.Height, wantHeight)
	}
	if image.Fit != woxwidget.ImageFitContain || image.Width != frame.Width || image.Height != frame.Height {
		t.Fatalf("screenshot image = %#v, want contain fit matching the frame", image)
	}
}

func TestPluginStoreScreenshotShowsLoadingIndicatorBeforeImageArrives(t *testing.T) {
	widget := pluginStoreScreenshot(PluginStoreDetailProps{ScreenshotLoading: true}, 580, woxcomponent.Theme{Cursor: woxui.Color{R: 1, G: 2, B: 3, A: 255}})
	loading := widget.(woxwidget.Align)
	if loading.Width != 580 || loading.Height != 48 || loading.Horizontal != 0.5 || loading.Vertical != 0.5 {
		t.Fatalf("screenshot loading align = %#v, want a compact centered placeholder", loading)
	}
	indicator := loading.Child.(woxwidget.LoopAnimation)
	if indicator.Key != "wox-loading-indicator" {
		t.Fatalf("screenshot loading child = %#v, want WoxLoadingIndicator", indicator)
	}
}

func TestPluginStoreDescriptionUsesLoadingPlaceholderWithoutBlankPanel(t *testing.T) {
	body := pluginStoreDescription(PluginStoreDetailProps{
		Name: "Strava", Description: "Workouts", Author: "Wox-launcher", Version: "0.0.1", Runtime: "Python",
		ScreenshotLoading: true,
	}, 580, 400, woxcomponent.Theme{Cursor: woxui.Color{A: 255}}).(woxwidget.Container)

	scroll := body.Child.(woxwidget.LayoutBuilder).Build(woxui.Size{Width: 580, Height: 400})
	var children []woxwidget.Widget
	switch content := scroll.(type) {
	case woxwidget.Stateful:
		children = content.Widget.(woxcomponent.ScrollViewProps).Content.(woxwidget.Flex).Children
	default:
		// Non-overflowing content may collapse to a plain scroll body without state.
		t.Fatalf("description scroll = %T, want resolved WoxScrollView", scroll)
	}
	if len(children) != 5 {
		t.Fatalf("description children = %d, want title, subtitle, chips, screenshot gap, and loading placeholder", len(children))
	}
	if gap := children[3].(woxwidget.Container); gap.Height != 24 {
		t.Fatalf("screenshot gap height = %v, want Flutter's 24px spacing above the preview", gap.Height)
	}
	if _, ok := children[4].(woxwidget.Align); !ok {
		t.Fatalf("description trailing child = %T, want compact screenshot loading align", children[4])
	}
}
