package view

import (
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

func TestPluginListBadgeUsesFlutterTagGeometry(t *testing.T) {
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Items: []PluginListItem{{ID: "clipboard", Name: "Clipboard", Status: "1.0.0", Badge: "System"}},
		Theme: woxcomponent.Theme{},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	scroll := column.Children[1].(woxwidget.ScrollView)
	rows := scroll.Child.(woxwidget.Flex)
	row := rows.Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	rowContent := row.Child.(woxwidget.Flex)
	badgeSlot := rowContent.Children[2].(woxwidget.Align)
	if badgeSlot.Horizontal != 1 || badgeSlot.Vertical != 0.5 {
		t.Fatalf("badge slot alignment = (%v, %v), want trailing and vertically centered", badgeSlot.Horizontal, badgeSlot.Vertical)
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

func TestPluginListSearchHighlightKeepsSelectedFillAndAddsBorder(t *testing.T) {
	selected := woxui.Color{R: 60, G: 80, B: 100, A: 255}
	list := PluginList(PluginListProps{
		Width: 260, Height: 660,
		Items: []PluginListItem{{ID: "clipboard", Name: "Clipboard", Selected: true, Highlighted: true}},
		Theme: woxcomponent.Theme{SelectedBackground: selected},
	})

	column := list.(woxwidget.Container).Child.(woxwidget.Flex)
	row := column.Children[1].(woxwidget.ScrollView).Child.(woxwidget.Flex).Children[0].(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if row.Color != selected {
		t.Fatalf("selected plugin fill = %#v, want selected color %#v", row.Color, selected)
	}
	if row.BorderWidth != 1 || row.BorderColor.A != 122 {
		t.Fatalf("plugin search highlight border = %#v at %v, want Flutter 0.48 alpha border", row.BorderColor, row.BorderWidth)
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

func TestFormTableInlineHeaderShowsTemplateAndAddActions(t *testing.T) {
	field := FormTableField(FormTableFieldProps{
		ID: "commands", Title: "Commands", Width: 720, Height: 220, InlineTitle: true,
		SecondaryLabel: "From Templates", AddLabel: "Add", Theme: woxcomponent.Theme{},
	})

	container := field.(woxwidget.Container)
	column := container.Child.(woxwidget.Flex)
	header := column.Children[0].(woxwidget.Stack)
	actions := header.Children[1].Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("header action count = %d, want template and add", len(actions.Children))
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

	header := field.(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Stack)
	title := header.Children[0].Child.(woxwidget.Container).Child.(woxwidget.Flex)
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
	body := grid.Children[1].(woxwidget.ScrollView)
	if body.Height != tableSurfaceRowHeight*3 || body.ContentHeight != tableSurfaceRowHeight*8 {
		t.Fatalf("vertical body geometry = viewport %v, content %v; want %v and %v", body.Height, body.ContentHeight, tableSurfaceRowHeight*3, tableSurfaceRowHeight*8)
	}
	bodyRow := body.Child.(woxwidget.Flex)
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
	actions := content.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	if len(actions.Children) != 2 {
		t.Fatalf("delete dialog action count = %d, want cancel and delete", len(actions.Children))
	}
}
