package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formTableListRowHeight = float32(48)

const (
	formTableDefaultMaxHeight    = float32(300)
	formTableOperationWidth      = float32(120)
	formTableColumnSpacing       = float32(10)
	formTableColumnTooltipWidth  = float32(20)
	formTableHorizontalMargin    = float32(5)
	formTableFlexibleColumnWidth = float32(100)
)

// FormTableColumn describes one visible inline table column.
type FormTableColumn struct {
	Label   string
	Tooltip string
	Width   float32
}

// FormTableCell contains one prepared inline table value.
type FormTableCell struct {
	Text           string
	Icon           *woxui.Image
	IndicatorColor *woxui.Color
}

// FormTableRowAction describes a table-specific icon action appended after the standard actions.
type FormTableRowAction struct {
	ID    string
	Label string
	Icon  *woxui.Image
	OnTap func()
}

// FormTableRow keeps display ordering tied to the source row used by the editor.
type FormTableRow struct {
	Index           int
	Cells           []FormTableCell
	TrailingActions []FormTableRowAction
}

// FormTableFieldProps contains the full inline table presentation and actions.
type FormTableFieldProps struct {
	ID              string
	Title           string
	Description     string
	Width           float32
	Height          float32
	LabelWidth      float32
	MaxHeight       int
	InlineTitle     bool
	Invalid         bool
	Columns         []FormTableColumn
	Rows            []FormTableRow
	SecondaryLabel  string
	HideEditAction  bool
	HideCloneAction bool
	AddLabel        string
	EditLabel       string
	CloneLabel      string
	DeleteLabel     string
	OperationLabel  string
	EmptyLabel      string
	InfoIcon        *woxui.Image
	DemoIcon        *woxui.Image
	DemoKind        string
	SecondaryIcon   *woxui.Image
	AddIcon         *woxui.Image
	EditIcon        *woxui.Image
	CloneIcon       *woxui.Image
	DeleteIcon      *woxui.Image
	EmptyIcon       *woxui.Image
	Theme           woxcomponent.Theme
	OnSecondary     func()
	OnAdd           func()
	OnOpenRow       func(int)
	OnCloneRow      func(int)
	OnDeleteRow     func(int)
	OnTooltip       func(bool, string, woxui.Rect)
	OnDemoHover     func(string, bool, woxui.Rect)
}

// FormTableFieldHeight returns the content height used by form scrolling and rendering.
func FormTableFieldHeight(inlineTitle bool, description string, rowCount, maximumHeight int) float32 {
	gridHeight := formTableGridHeight(rowCount, maximumHeight)
	if inlineTitle {
		headerHeight := float32(30)
		if description != "" {
			headerHeight = 60
		}
		return 6 + headerHeight + 8 + gridHeight + 34
	}
	descriptionHeight := float32(0)
	if description != "" {
		descriptionHeight = 4 + 48
	}
	return 6 + 36 + 6 + gridHeight + descriptionHeight + 10
}

func formTableGridHeight(rowCount, maximumHeight int) float32 {
	bodyHeight := tableSurfaceEmptyHeight
	if rowCount > 0 {
		bodyHeight = float32(rowCount) * tableSurfaceRowHeight
	}
	maximum := float32(maximumHeight)
	if maximum <= 0 {
		maximum = formTableDefaultMaxHeight
	}
	maximum = max(float32(120), maximum)
	return min(maximum, tableSurfaceHeaderHeight+bodyHeight)
}

// FormTableField builds the Flutter-parity title, action, grid, and empty state.
func FormTableField(props FormTableFieldProps) woxwidget.Widget {
	gridHeight := formTableGridHeight(len(props.Rows), props.MaxHeight)
	if props.InlineTitle {
		headerHeight := float32(30)
		if props.Description != "" {
			headerHeight = 60
		}
		header := formTableInlineHeader(props, props.Width, headerHeight)
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{header, formTableGrid(props, props.Width, gridHeight)},
		}}
	}

	labelWidth := props.LabelWidth
	if labelWidth <= 0 {
		labelWidth = 132
	}
	const labelGap = float32(12)
	fieldWidth := max(float32(0), props.Width-labelWidth-labelGap)
	labelChildren := []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
	}
	tableChildren := []woxwidget.Widget{formTableGrid(props, fieldWidth, gridHeight)}
	if props.Description != "" {
		tableChildren = append(tableChildren, woxwidget.TextBlock{
			Value: props.Description, Width: min(fieldWidth, float32(620)), Height: 48, MaxLines: 3, LineHeight: 16,
			Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		})
	}
	label := woxwidget.Container{Width: labelWidth, Height: max(float32(0), props.Height-16), Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: labelChildren}}
	actions := woxwidget.Container{Width: fieldWidth, Height: 36, Padding: woxwidget.Insets{Left: max(float32(0), fieldWidth-74)}, Child: formTableAddButton(props)}
	table := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: tableChildren}
	field := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{actions, table}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: labelGap, Children: []woxwidget.Widget{label, field}}}
}

func formTableInlineHeader(props FormTableFieldProps, width, height float32) woxwidget.Widget {
	actionsWidth := float32(74)
	if props.SecondaryLabel != "" {
		actionsWidth += 130
	}
	titleWidth := max(float32(0), width-actionsWidth-16)
	var title woxwidget.Widget = woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}
	if props.DemoKind != "" && props.DemoIcon != nil {
		title = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			title,
			woxwidget.Semantics{
				Key: woxwidget.Key("settings-demo-trigger-" + props.DemoKind), AutomationID: "settings-demo-" + props.DemoKind, Role: woxui.AccessibilityRoleButton, Label: props.Title,
				Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
				Child: woxwidget.Gesture{
					ID: "settings-demo-hover-" + props.DemoKind,
					OnHoverAt: func(inside bool, bounds woxui.Rect) {
						if props.OnDemoHover != nil {
							props.OnDemoHover(props.DemoKind, inside, bounds)
						}
					},
					Child: woxwidget.Image{Source: props.DemoIcon, Width: 18, Height: 18},
				},
			},
		}}
	}
	children := []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: titleWidth, Height: 22, Child: title}},
		{Left: max(float32(0), width-actionsWidth), Top: max(float32(0), height-30), Child: formTableHeaderActions(props)},
	}
	if props.Description != "" {
		children = append(children, woxwidget.StackChild{Top: 22, Child: woxwidget.TextBlock{
			Value: props.Description, Width: titleWidth, Height: 34, MaxLines: 2, LineHeight: 16,
			Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		}})
	}
	return woxwidget.Stack{Width: width, Height: height, Children: children}
}

// formTableHeaderActions keeps specialized secondary actions aligned with the
// table's standard Add control.
func formTableHeaderActions(props FormTableFieldProps) woxwidget.Widget {
	actions := make([]woxwidget.Widget, 0, 2)
	if props.SecondaryLabel != "" {
		actions = append(actions, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: props.ID + "-secondary", Label: props.SecondaryLabel, Icon: props.SecondaryIcon, IconSize: 15, IconGap: 5, Width: 122,
			Size: woxcomponent.ButtonCompact, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 9, Right: 7},
			Disabled: props.Invalid, OnTap: props.OnSecondary, Theme: props.Theme,
		}))
	}
	actions = append(actions, formTableAddButton(props))
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: actions}
}

func formTableAddButton(props FormTableFieldProps) woxwidget.Widget {
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: props.ID + "-add", Label: props.AddLabel, Icon: props.AddIcon, IconSize: 15, IconGap: 5, Width: 74,
		Size: woxcomponent.ButtonCompact, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 9, Right: 7},
		Disabled: props.Invalid, OnTap: props.OnAdd, Theme: props.Theme,
	})
}

type formTableGridProps struct {
	field  FormTableFieldProps
	width  float32
	height float32
}

type formTableGridState struct {
	horizontalHeader *woxwidget.ScrollController
	horizontalBody   *woxwidget.ScrollController
	verticalBody     *woxwidget.ScrollController
}

func newFormTableGridState() *formTableGridState {
	return &formTableGridState{
		horizontalHeader: woxwidget.NewScrollController(0),
		horizontalBody:   woxwidget.NewScrollController(0),
		verticalBody:     woxwidget.NewScrollController(0),
	}
}

// InitState creates the synchronized header, body, and vertical scroll positions.
func (s *formTableGridState) InitState(_ woxwidget.StateContext, _ any) {
	*s = *newFormTableGridState()
}

// DidUpdateWidget preserves table scroll positions while its rows and geometry update.
func (s *formTableGridState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build renders the table using retained scroll controllers.
func (s *formTableGridState) Build(_ woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(formTableGridProps)
	return buildFormTableGrid(props.field, props.width, props.height, s)
}

// Dispose releases no external resources; child scroll views detach themselves.
func (s *formTableGridState) Dispose() {}

func formTableGrid(props FormTableFieldProps, width, height float32) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID + "-grid"), Type: (*formTableGridState)(nil),
		Widget: formTableGridProps{field: props, width: width, height: height},
		CreateState: func() woxwidget.State {
			return newFormTableGridState()
		},
	}
}

func buildFormTableGrid(props FormTableFieldProps, width, height float32, state *formTableGridState) woxwidget.Widget {
	widths := formTableColumnWidths(props.Columns, width)
	operationWidth := min(width, widths[len(widths)-1])
	leftViewportWidth := max(float32(0), width-operationWidth)
	leftContentWidth := float32(0)
	for index := range props.Columns {
		leftContentWidth += widths[index]
	}
	// Flutter expands the last data column when the declared columns are narrower
	// than the viewport, keeping the pinned operation column directly adjacent.
	if len(props.Columns) > 0 && leftContentWidth < leftViewportWidth {
		widths[len(props.Columns)-1] += leftViewportWidth - leftContentWidth
		leftContentWidth = leftViewportWidth
	}
	headerCells := make([]woxwidget.Widget, 0, len(props.Columns))
	for index, column := range props.Columns {
		headerCells = append(headerCells, formTableHeaderCell(props, column, widths[index], index))
	}
	leftContentWidth = max(leftViewportWidth, leftContentWidth)
	leftHeader := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: headerCells}
	operationHeader := formTableHeaderCell(props, FormTableColumn{Label: props.OperationLabel}, operationWidth, len(props.Columns))
	bodyHeight := max(float32(0), height-tableSurfaceHeaderHeight)
	if len(props.Rows) == 0 {
		header := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.ScrollView{
				Key: woxwidget.Key(props.ID + "-columns"), ID: props.ID + "-columns", Width: leftViewportWidth, Height: tableSurfaceHeaderHeight,
				ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalHeader, Child: leftHeader,
			},
			operationHeader,
		}}
		return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, formTableEmptyState(props, width, bodyHeight)},
		}}
	}

	contentHeight := float32(len(props.Rows)) * tableSurfaceRowHeight
	leftRows := make([]woxwidget.Widget, 0, len(props.Rows))
	operationRows := make([]woxwidget.Widget, 0, len(props.Rows))
	for _, row := range props.Rows {
		leftRows = append(leftRows, formTableDataRowCells(props, row, widths[:len(props.Columns)], leftContentWidth))
		operationRows = append(operationRows, formTableOperationCell(props, row, operationWidth))
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.ScrollView{
			Key: woxwidget.Key(props.ID + "-columns-header"), ID: props.ID + "-columns-header", Width: leftViewportWidth, Height: tableSurfaceHeaderHeight,
			ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalHeader, OnOffsetChanged: func(offset float32) {
				state.horizontalBody.JumpTo(offset)
			}, Child: leftHeader,
		},
		operationHeader,
	}}
	bodyContent := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.ScrollView{
			Key: woxwidget.Key(props.ID + "-columns-body"), ID: props.ID + "-columns-body", Width: leftViewportWidth, Height: contentHeight,
			ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalBody, OnOffsetChanged: func(offset float32) {
				state.horizontalHeader.JumpTo(offset)
			}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: leftRows},
		},
		woxwidget.Flex{Axis: woxwidget.Vertical, Children: operationRows},
	}}
	body := woxwidget.ScrollView{
		Key: woxwidget.Key(props.ID + "-rows"), ID: props.ID + "-rows", Width: width, Height: bodyHeight,
		ContentHeight: contentHeight, Controller: state.verticalBody, Child: bodyContent,
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, body}}}
}

// formTableColumnWidths preserves Flutter's declared widths and leaves overflow
// to the horizontally scrolling content area instead of shrinking every column.
func formTableColumnWidths(columns []FormTableColumn, tableWidth float32) []float32 {
	widths := make([]float32, len(columns)+1)
	widths[len(widths)-1] = formTableOperationWidth + formTableHorizontalMargin*2
	zeroWidthColumns := 0
	totalDeclaredWidth := float32(0)
	totalTooltipWidth := float32(0)
	for _, column := range columns {
		totalDeclaredWidth += column.Width + formTableColumnSpacing
		if column.Width == 0 {
			zeroWidthColumns++
		}
		if column.Tooltip != "" {
			totalTooltipWidth += formTableColumnTooltipWidth
		}
	}
	flexibleWidth := formTableFlexibleColumnWidth
	if zeroWidthColumns == 1 {
		availableWidth := tableWidth - totalDeclaredWidth - (formTableOperationWidth + formTableColumnSpacing) - totalTooltipWidth
		if availableWidth > 0 {
			flexibleWidth = availableWidth
		}
	}

	for index, column := range columns {
		columnWidth := column.Width
		if columnWidth == 0 {
			columnWidth = flexibleWidth
		}
		if column.Tooltip != "" {
			columnWidth += formTableColumnTooltipWidth
		}
		widths[index] = columnWidth + formTableColumnSpacing
	}
	return widths
}

func formTableHeaderCell(props FormTableFieldProps, column FormTableColumn, width float32, index int) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	contentWidth := max(float32(0), width-16)
	children := []woxwidget.Widget{woxwidget.TextBlock{
		Value: column.Label, Width: contentWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: tableSurfaceHeaderFontSize, Weight: woxui.FontWeightSemibold}, Color: style.headerText,
	}}
	if column.Tooltip != "" {
		contentWidth = max(float32(0), contentWidth-20)
		children[0] = woxwidget.TextBlock{Value: column.Label, Width: contentWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: tableSurfaceHeaderFontSize, Weight: woxui.FontWeightSemibold}, Color: style.headerText}
		var icon woxwidget.Widget = woxwidget.Container{Width: 14, Height: 14}
		if props.InfoIcon != nil {
			icon = woxwidget.Image{Source: props.InfoIcon, Width: 14, Height: 14}
		}
		children = append(children, woxwidget.Gesture{ID: fmt.Sprintf("%s-column-tooltip-%d", props.ID, index), OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if props.OnTooltip != nil {
				props.OnTooltip(inside, column.Tooltip, bounds)
			}
		}, Child: icon})
	}
	return woxwidget.Container{Width: width, Height: tableSurfaceHeaderHeight, Color: style.headerBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
		Padding: woxwidget.Insets{Left: 8, Top: 9, Right: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: children}}
}

func formTableEmptyState(props FormTableFieldProps, width, height float32) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	label := props.EmptyLabel
	if props.Invalid {
		label = "Invalid table data"
	}
	var icon woxwidget.Widget = woxwidget.Container{Width: 24, Height: 24}
	if props.EmptyIcon != nil {
		icon = woxwidget.Image{Source: props.EmptyIcon, Width: 24, Height: 24}
	}
	contentWidth := float32(110)
	content := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
		woxwidget.Align{Width: contentWidth, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon},
		woxwidget.Align{Width: contentWidth, Height: 18, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}},
	}}
	return woxwidget.Container{Width: width, Height: height, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
		Padding: woxwidget.Insets{Left: max(float32(0), (width-contentWidth)/2), Top: max(float32(0), (height-46)/2)}, Child: content}
}

// formTableDataRowCells builds the horizontally scrolling portion of one row.
func formTableDataRowCells(props FormTableFieldProps, row FormTableRow, widths []float32, width float32) woxwidget.Widget {
	cells := make([]woxwidget.Widget, 0, len(widths))
	for index := range props.Columns {
		cell := FormTableCell{}
		if index < len(row.Cells) {
			cell = row.Cells[index]
		}
		cells = append(cells, formTableDataCell(props, cell, widths[index]))
	}
	return woxwidget.Container{Width: width, Height: tableSurfaceRowHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}}
}

// formTableOperationCell builds the pinned action portion of one row.
func formTableOperationCell(props FormTableFieldProps, row FormTableRow, width float32) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	actions := make([]woxwidget.Widget, 0, 3+len(row.TrailingActions))
	if !props.HideEditAction {
		actions = append(actions, formTableIconButton(props, fmt.Sprintf("%s-row-%d-edit", props.ID, row.Index), props.EditLabel, props.EditIcon, func() {
			if props.OnOpenRow != nil {
				props.OnOpenRow(row.Index)
			}
		}))
	}
	if !props.HideCloneAction {
		actions = append(actions, formTableIconButton(props, fmt.Sprintf("%s-row-%d-clone", props.ID, row.Index), props.CloneLabel, props.CloneIcon, func() {
			if props.OnCloneRow != nil {
				props.OnCloneRow(row.Index)
			}
		}))
	}
	actions = append(actions,
		formTableIconButton(props, fmt.Sprintf("%s-row-%d-delete", props.ID, row.Index), props.DeleteLabel, props.DeleteIcon, func() {
			if props.OnDeleteRow != nil {
				props.OnDeleteRow(row.Index)
			}
		}),
	)
	for index, action := range row.TrailingActions {
		action := action
		actionID := action.ID
		if actionID == "" {
			actionID = fmt.Sprintf("trailing-%d", index)
		}
		actions = append(actions, formTableIconButton(props, fmt.Sprintf("%s-row-%d-%s", props.ID, row.Index, actionID), action.Label, action.Icon, action.OnTap))
	}
	operation := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, Children: actions}
	return woxwidget.Container{Width: width, Height: tableSurfaceRowHeight, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
		Padding: woxwidget.Insets{Left: 4, Top: 6, Right: 4}, Child: operation}
}

func formTableIconButton(props FormTableFieldProps, id, label string, icon *woxui.Image, onTap func()) woxwidget.Widget {
	var content woxwidget.Widget = woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle}
	if icon != nil {
		content = woxwidget.Image{Source: icon, Width: 16, Height: 16}
	}
	key := woxwidget.Key(id)
	return woxwidget.Semantics{Key: key, AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: label, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, Child: woxwidget.Focusable{Key: key, OnKey: func(event woxui.KeyEvent) bool {
		if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
			return false
		}
		if event.Down && onTap != nil {
			onTap()
		}
		return true
	}, Child: woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Align{Width: 26, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: content}}}}
}

func formTableDataCell(props FormTableFieldProps, cell FormTableCell, width float32) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	contentWidth := max(float32(0), width-14)
	var content woxwidget.Widget = woxwidget.TextBlock{Value: cell.Text, Width: contentWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle}
	if cell.IndicatorColor != nil {
		content = woxwidget.Container{Width: 16, Height: 16, Radius: 8, Color: *cell.IndicatorColor}
	} else if cell.Icon != nil {
		children := []woxwidget.Widget{woxwidget.Image{Source: cell.Icon, Width: 16, Height: 16}}
		if cell.Text != "" {
			children = append(children, woxwidget.TextBlock{Value: cell.Text, Width: max(float32(0), contentWidth-22), Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle})
		}
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: children}
	}
	return woxwidget.Container{Width: width, Height: tableSurfaceRowHeight, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
		Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 6}, Child: content}
}

func formTableAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// FormTableOverlayProps contains the prepared body rendered by the shared table editor.
type FormTableOverlayProps struct {
	Width       float32
	Height      float32
	PanelWidth  float32
	PanelHeight float32
	Title       string
	Subtitle    string
	RowEditor   bool
	Body        woxwidget.Widget
	Theme       woxcomponent.Theme
}

// FormTableOverlay builds the modal table editor shell.
func FormTableOverlay(props FormTableOverlayProps) woxwidget.Widget {
	panelWidth := props.PanelWidth
	if panelWidth <= 0 {
		panelWidth = min(float32(760), props.Width-28)
	}
	panelWidth = max(float32(0), min(panelWidth, props.Width-28))
	panelHeight := props.PanelHeight
	if panelHeight <= 0 {
		panelHeight = min(float32(640), props.Height-28)
	}
	panelHeight = max(float32(0), min(panelHeight, props.Height-28))
	padding := woxwidget.UniformInsets(16)
	radius := float32(12)
	borderColor := woxui.Color{}
	borderWidth := float32(0)
	child := props.Body
	if props.RowEditor {
		padding = woxwidget.UniformInsets(24)
		radius = 20
		borderColor = formTableAlpha(props.Theme.ResultSubtitle, 104)
		borderWidth = 0.75
	} else {
		innerWidth := max(float32(0), panelWidth-32)
		header := woxwidget.Container{Width: innerWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.Text{Value: props.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ActionHeader},
		}}}
		child = woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, props.Body}}
	}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-modal-shade", BackdropAlpha: 205,
		Padding: padding, Radius: radius, BorderColor: borderColor, BorderWidth: borderWidth, Theme: props.Theme, Child: child,
	})
}

// FormTableDeleteDialogProps contains the Flutter-aligned row deletion confirmation.
type FormTableDeleteDialogProps struct {
	Width       float32
	Height      float32
	Message     string
	CancelLabel string
	DeleteLabel string
	Theme       woxcomponent.Theme
	OnCancel    func()
	OnDelete    func()
}

// FormTableDeleteDialog builds the compact confirmation shown before deleting one row.
func FormTableDeleteDialog(props FormTableDeleteDialogProps) woxwidget.Widget {
	panelWidth := min(float32(270), max(float32(0), props.Width-56))
	panelHeight := min(float32(110), max(float32(0), props.Height-56))
	innerWidth := max(float32(0), panelWidth-48)
	const buttonWidth = float32(64)
	const buttonGap = float32(16)
	actionsWidth := buttonWidth*2 + buttonGap
	actions := woxwidget.Container{
		Width: innerWidth, Height: 34, Padding: woxwidget.Insets{Left: max(float32(0), innerWidth-actionsWidth)},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: buttonGap, Children: []woxwidget.Widget{
			woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "form-table-delete-cancel", Label: props.CancelLabel, Width: buttonWidth, Height: 34,
				Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme,
			}),
			woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "form-table-delete-confirm", Label: props.DeleteLabel, Width: buttonWidth, Height: 34,
				Variant: woxcomponent.ButtonPrimary, OnTap: props.OnDelete, Theme: props.Theme,
			}),
		}},
	}
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-delete-dialog", Label: props.Message, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-delete-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.Insets{Left: 24, Top: 20, Right: 24, Bottom: 22}, BorderColor: border, BorderWidth: 0.75,
		InitialFocus: "form-table-delete-cancel", OnDismiss: props.OnCancel, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: props.Message, Width: innerWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ActionText},
			actions,
		}},
	})
}

// FormTableListProps contains the prepared rows and actions rendered by a table editor.
type FormTableListProps struct {
	Width       float32
	Height      float32
	Rows        []string
	Selected    int
	Status      string
	StatusError bool
	AddLabel    string
	DeleteLabel string
	CloseLabel  string
	CanAdd      bool
	CanEdit     bool
	CanDelete   bool
	ShowClone   bool
	Theme       woxcomponent.Theme
	OnSelect    func(int)
	OnAdd       func()
	OnEdit      func()
	OnDelete    func()
	OnClone     func()
	OnClose     func()
}

// FormTableList builds the row list and editor actions.
func FormTableList(props FormTableListProps) woxwidget.Widget {
	footerHeight := float32(54)
	statusHeight := float32(28)
	viewportHeight := max(float32(48), props.Height-footerHeight-statusHeight)
	rows := make([]woxwidget.Widget, 0, len(props.Rows))
	for index, value := range props.Rows {
		index := index
		background := props.Theme.QueryBackground
		foreground := props.Theme.ActionText
		if index == props.Selected {
			background = props.Theme.SelectedBackground
			foreground = props.Theme.SelectedTitle
		}
		rows = append(rows, woxwidget.Gesture{
			ID: fmt.Sprintf("form-table-row-%d", index),
			OnTap: func() {
				if props.OnSelect != nil {
					props.OnSelect(index)
				}
			},
			Child: woxwidget.Container{Width: props.Width, Height: formTableListRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 15, Right: 10}, Child: woxwidget.Text{
				Value: value, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground,
			}},
		})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: props.Width, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No rows yet. Choose Add row to create one.", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader,
		}}
	} else {
		var keepVisible *woxwidget.ScrollRange
		if props.Selected >= 0 && props.Selected < len(rows) {
			start := float32(props.Selected) * formTableListRowHeight
			keepVisible = &woxwidget.ScrollRange{Start: start, End: start + formTableListRowHeight}
		}
		list = woxwidget.ScrollView{
			Key: "form-table-list-scroll", ID: "form-table-list-scroll", Width: props.Width, Height: viewportHeight,
			ContentHeight: max(viewportHeight, float32(len(rows))*formTableListRowHeight), KeepVisible: keepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}
	}
	status := props.Status
	if status == "" {
		status = "↑↓ select · Enter edit · Delete remove · Ctrl+N add · Esc close"
	}
	statusColor := props.Theme.ActionHeader
	if props.StatusError {
		statusColor = props.Theme.ErrorText
	}
	leftButtons := []woxwidget.Widget{
		formTableButton("form-table-add", props.AddLabel, 104, props.CanAdd, false, props.OnAdd, props.Theme),
		formTableButton("form-table-edit", "Edit", 86, props.CanEdit, false, props.OnEdit, props.Theme),
		formTableButton("form-table-delete", props.DeleteLabel, 86, props.CanDelete, false, props.OnDelete, props.Theme),
	}
	fixedWidth := float32(104 + 86 + 86 + 104)
	if props.ShowClone {
		leftButtons = append(leftButtons, formTableButton("form-table-clone", "Clone remote", 112, props.CanAdd, false, props.OnClone, props.Theme))
		fixedWidth += 112
	}
	buttonChildren := append([]woxwidget.Widget(nil), leftButtons...)
	buttonChildren = append(buttonChildren, woxwidget.Painter{Width: max(float32(0), props.Width-fixedWidth-float32(len(leftButtons)+1)*8), Height: 38})
	buttonChildren = append(buttonChildren, formTableButton("form-table-close", props.CloseLabel, 104, true, true, props.OnClose, props.Theme))
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		list,
		woxwidget.Container{Width: props.Width, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
		woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttonChildren}},
	}}
}

// FormTableRowFieldProps contains one Flutter-parity field in a table row editor.
type FormTableRowFieldProps struct {
	ID              string
	Kind            string
	Label           string
	Description     string
	Value           string
	Detail          string
	HotkeyLabels    []string
	Placeholder     string
	RecordingStatus string
	Width           float32
	Height          float32
	LabelWidth      float32
	State           woxui.TextEditingState
	Focused         bool
	Recording       bool
	RecordingError  bool
	Hold            bool
	HoldPrefix      string
	Checked         bool
	Protected       bool
	MaxLines        int
	Image           *woxui.Image
	ImageEmoji      string
	EmojiLabel      string
	UploadLabel     string
	BrowseLabel     string
	EmojiWidth      float32
	UploadWidth     float32
	EmojiIcon       *woxui.Image
	UploadIcon      *woxui.Image
	Window          *woxui.Window
	Theme           woxcomponent.Theme
	OnTap           func()
	OnChoiceTap     func(woxui.Rect)
	OnFocus         func()
	OnChanged       func(string)
	OnKey           func(woxui.KeyEvent) bool
	OnBrowse        func()
	OnEmoji         func()
	OnUpload        func()
}

// FormTableRowFieldHeight returns the compact split-row height used by Flutter's table editor.
func FormTableRowFieldHeight(kind, description string, maxLines int) float32 {
	descriptionHeight := float32(0)
	if description != "" {
		descriptionHeight = 22
	}
	switch kind {
	case "label":
		return 34
	case "woxImage":
		return 88 + descriptionHeight
	case "checkbox":
		return 32 + descriptionHeight
	case "app":
		return 46 + descriptionHeight
	case "textbox", "password", "dirPath":
		controlHeight := float32(34)
		if maxLines > 1 {
			controlHeight = 14 + float32(min(maxLines, 8))*20
		}
		return controlHeight + 4 + descriptionHeight
	default:
		return 38 + descriptionHeight
	}
}

// FormTableRowField renders labels, controls, and help text with the same split layout as Flutter.
func FormTableRowField(props FormTableRowFieldProps) woxwidget.Widget {
	labelWidth := min(max(float32(60), props.LabelWidth), max(float32(60), props.Width-120))
	controlWidth := max(float32(0), props.Width-labelWidth-10)
	controlHeight := formTableRowControlHeight(props)
	control := formTableRowControl(props, controlWidth, controlHeight)
	rightChildren := []woxwidget.Widget{control}
	if props.Description != "" {
		rightChildren = append(rightChildren, woxwidget.TextBlock{
			Value: props.Description, Width: controlWidth, Height: 18, MaxLines: 1, LineHeight: 18,
			Style: woxui.TextStyle{Size: 12}, Color: formTableAlpha(props.Theme.ActionText, 154),
		})
	}
	labelTop := float32(8)
	if props.Kind == "checkbox" {
		labelTop = 1
	} else if props.Kind == "woxImage" {
		labelTop = 31
	}
	label := woxwidget.Container{Width: labelWidth, Height: props.Height, Padding: woxwidget.Insets{Top: labelTop}, Child: woxwidget.TextBlock{
		Value: props.Label, Width: labelWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: formTableAlpha(props.Theme.ActionText, 235),
	}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		label,
		woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: rightChildren},
	}}}
}

func formTableRowControlHeight(props FormTableRowFieldProps) float32 {
	switch props.Kind {
	case "woxImage":
		return 80
	case "checkbox":
		return 20
	case "app":
		return 42
	case "textbox", "password", "dirPath":
		if props.MaxLines > 1 {
			return 14 + float32(min(props.MaxLines, 8))*20
		}
		return 34
	case "label":
		return 24
	default:
		return 34
	}
}

func formTableRowControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	switch props.Kind {
	case "textbox", "password", "dirPath":
		return formTableRowTextControl(props, width, height)
	case "checkbox":
		return formTableRowCheckboxControl(props)
	case "woxImage":
		return formTableRowImageControl(props, height)
	case "app":
		return formTableRowAppControl(props, width, height)
	case "select", "selectAIModel":
		return formTableRowSelectControl(props, width, height)
	case "hotkey", "dictationHotkey":
		recorder, recorderWidth := woxcomponent.WoxHotkeyRecorder(woxcomponent.HotkeyRecorderProps{
			Labels: props.HotkeyLabels, Placeholder: props.Placeholder, Focused: props.Focused, Error: props.RecordingError, Hold: props.Hold, HoldPrefix: props.HoldPrefix,
			Window: props.Window, Theme: props.Theme,
		})
		recorder = woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: recorder}
		if !props.Recording || props.RecordingStatus == "" || width-recorderWidth <= 8 {
			return recorder
		}
		statusColor := props.Theme.ResultSubtitle
		if props.RecordingError {
			statusColor = props.Theme.ErrorText
		}
		return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			recorder,
			woxwidget.Align{Width: max(float32(0), width-recorderWidth-8), Height: height, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.RecordingStatus, Style: woxui.TextStyle{Size: 12}, Color: statusColor,
			}},
		}}
	case "label":
		return woxwidget.TextBlock{Value: props.Value, Width: width, Height: height, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader}
	default:
		return formTableRowValueControl(props, width, height)
	}
}

// formTableRowTextControl keeps directory browsing beside the same outlined text control.
func formTableRowTextControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	inputWidth := width
	if props.OnBrowse != nil {
		inputWidth = max(float32(100), width-90)
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: props.ID, Label: props.Label, Width: inputWidth, Height: height, Radius: 4,
		Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 9, Bottom: 6}, Transparent: true,
		BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1,
		Style: woxui.TextStyle{Size: 13}, Value: props.State.Text, Focused: props.Focused, Protected: props.Protected,
		MaxLines: max(1, props.MaxLines), Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	if props.OnBrowse == nil {
		return input
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		input,
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-browse", Label: props.BrowseLabel, Width: 82, Height: height, Radius: 4, Variant: woxcomponent.ButtonOutline, OnTap: props.OnBrowse, Theme: props.Theme}),
	}}
}

func formTableRowCheckboxControl(props FormTableRowFieldProps) woxwidget.Widget {
	var mark woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if props.Checked {
		mark = woxwidget.Align{Width: 16, Height: 16, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "✓", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}}
	}
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: 18, Height: 18, Radius: 3, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Padding: woxwidget.UniformInsets(1), Child: mark,
	}}
}

// formTableRowImageControl restores Flutter's preview plus emoji and upload actions.
func formTableRowImageControl(props FormTableRowFieldProps, height float32) woxwidget.Widget {
	var preview woxwidget.Widget
	if props.Image != nil {
		preview = woxwidget.Align{Width: 80, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.Image, Width: 64, Height: 64}}
	} else if props.ImageEmoji != "" {
		if props.Focused {
			preview = woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
				ID: props.ID + "-emoji-input", Label: props.Label, Width: 78, Height: height - 2, Radius: 7,
				Padding: woxwidget.Insets{Left: 27, Top: 28, Right: 20, Bottom: 28}, Transparent: true,
				Style: woxui.TextStyle{Size: 18}, Value: props.State.Text, Focused: true, MaxLines: 1,
				Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
				OnFocusChange: func(focused bool) {
					if focused && props.OnFocus != nil {
						props.OnFocus()
					}
				},
			})
		} else {
			preview = woxwidget.Align{Width: 80, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.ImageEmoji, Style: woxui.TextStyle{Size: 58}, Color: props.Theme.ActionText,
			}}
		}
	} else {
		preview = woxwidget.Container{Width: 80, Height: height}
	}
	previewBox := woxwidget.Gesture{ID: props.ID + "-preview", OnTap: props.OnEmoji, Child: woxwidget.Container{
		Width: 80, Height: height, Radius: 8, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Child: preview,
	}}
	buttonWidth := max(float32(98), props.EmojiWidth)
	uploadWidth := max(float32(98), props.UploadWidth)
	buttonsWidth := buttonWidth + 8 + uploadWidth
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
		previewBox,
		woxwidget.Container{
			Width: buttonsWidth, Height: height, Padding: woxwidget.Insets{Top: 27},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-emoji", Label: props.EmojiLabel, Icon: props.EmojiIcon, IconSize: 14, IconGap: 6, Width: buttonWidth, Height: 28, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 11, Right: 7}, OnTap: props.OnEmoji, Theme: props.Theme}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-upload", Label: props.UploadLabel, Icon: props.UploadIcon, IconSize: 14, IconGap: 6, Width: uploadWidth, Height: 28, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 11, Right: 7}, OnTap: props.OnUpload, Theme: props.Theme}),
			}},
		},
	}}
}

func formTableRowAppControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: width, Height: height, Radius: 4, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Top: 5, Right: 9},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: props.Value, Width: width - 20, Height: 17, MaxLines: 1, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.TextBlock{Value: props.Detail, Width: width - 20, Height: 14, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
		}},
	}}
}

// formTableRowSelectControl keeps the selected value and dropdown indicator in separate aligned slots, matching Flutter's expanded dropdown button.
func formTableRowSelectControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	foreground := props.Theme.ActionText
	if props.OnChoiceTap == nil {
		foreground = formTableAlpha(foreground, 128)
	}
	return woxDropdown(dropdownTriggerProps{
		ID: props.ID, Label: props.Label, Value: props.Value, Width: width, Height: height, Outline: formTableRowOutline(props.Theme, props.Focused),
		Foreground: foreground, Secondary: props.Theme.ActionHeader, OnTap: props.OnTap, OnTapBounds: props.OnChoiceTap,
	})
}

func formTableRowValueControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: width, Height: height, Radius: 4, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 9},
		Child: woxwidget.TextBlock{Value: props.Value, Width: width - 19, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionText},
	}}
}

func formTableRowOutline(theme woxcomponent.Theme, focused bool) woxui.Color {
	if focused {
		return formTableAlpha(theme.ActionText, 220)
	}
	return formTableAlpha(theme.ResultSubtitle, 190)
}

// FormTableRowEditorProps contains a prepared table row form.
type FormTableRowEditorProps struct {
	Width         float32
	Height        float32
	Title         string
	Rows          []woxwidget.Widget
	ContentHeight float32
	KeepVisible   *woxwidget.ScrollRange
	Status        string
	CancelLabel   string
	SaveLabel     string
	Theme         woxcomponent.Theme
	OnCancel      func()
	OnSave        func()
}

// FormTableRowEditor builds the add, edit, or clone row form.
func FormTableRowEditor(props FormTableRowEditorProps) woxwidget.Widget {
	footerHeight := float32(62)
	titleHeight := float32(0)
	if props.Title != "" {
		titleHeight = 32
	}
	statusHeight := float32(0)
	if props.Status != "" {
		statusHeight = 28
	}
	bodyHeight := max(float32(48), props.Height-titleHeight-footerHeight-statusHeight)
	body := woxwidget.ScrollView{
		Key: "form-table-row-scroll", ID: "form-table-row-scroll", Width: props.Width, Height: bodyHeight,
		ContentHeight: max(bodyHeight, props.ContentHeight), KeepVisible: props.KeepVisible,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
	}
	children := make([]woxwidget.Widget, 0, 4)
	if titleHeight > 0 {
		children = append(children, woxwidget.Container{Width: props.Width, Height: titleHeight, Child: woxwidget.Text{
			Value: props.Title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}})
	}
	children = append(children, body)
	if statusHeight > 0 {
		children = append(children, woxwidget.Container{Width: props.Width, Height: statusHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: props.Status, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ErrorText,
		}})
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 28, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), props.Width-208), Height: 36},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-row-cancel", Label: props.CancelLabel, Width: 70, Height: 36, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-row-save", Label: props.SaveLabel, Width: 82, Height: 36, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSave, Theme: props.Theme}),
	}}
	children = append(children, woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: buttons})
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

func formTableButton(id, label string, width float32, enabled, primary bool, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	variant := woxcomponent.ButtonSecondary
	if primary {
		variant = woxcomponent.ButtonPrimary
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Width: width, Disabled: !enabled, Variant: variant, OnTap: onTap, Theme: theme})
}
