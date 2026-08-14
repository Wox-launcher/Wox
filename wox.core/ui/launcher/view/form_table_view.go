package view

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formTableListRowHeight = float32(48)

const (
	formTableDefaultMaxHeight          = float32(300)
	formTableOperationWidth            = float32(120)
	formTableColumnSpacing             = float32(10)
	formTableColumnTooltipWidth        = float32(20)
	formTableHorizontalMargin          = float32(5)
	formTableFlexibleColumnWidth       = float32(100)
	formTableRowFieldGap               = float32(10)
	formTableMarkdownDescriptionGap    = float32(4)
	formTableMarkdownDescriptionLine   = float32(15)
	formTableMarkdownDescriptionRunGap = float32(3)
)

var formTableMarkdownLinkPattern = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

// FormTableColumn describes one visible inline table column.
type FormTableColumn struct {
	Label   string
	Tooltip string
	Width   float32
}

// FormTableCell contains one prepared inline table value.
type FormTableCell struct {
	Text           string
	Tooltip        string
	Icon           *woxui.Image
	IconSize       float32
	IndicatorColor *woxui.Color
	Child          woxwidget.Widget
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
	ReadOnly        bool
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
			headerHeight = 54
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
		children := make([]woxwidget.Widget, 0, 2)
		if props.Title != "" || props.Description != "" || props.SecondaryLabel != "" || !props.ReadOnly {
			children = append(children, formTableInlineHeader(props, props.Width))
		}
		children = append(children, formTableGrid(props, props.Width, gridHeight))
		padding := woxwidget.Insets{Top: 6}
		if props.Height <= 0 {
			padding.Bottom = 10
		}
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: padding, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 8, Children: children,
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
			Value: props.Description, Width: min(fieldWidth, float32(620)), MaxLines: 3, LineHeight: 16,
			Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		})
	}
	labelHeight := float32(0)
	if props.Height > 0 {
		labelHeight = max(float32(0), props.Height-16)
	}
	label := woxwidget.Container{Width: labelWidth, Height: labelHeight, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: labelChildren}}
	table := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: tableChildren}
	fieldChildren := []woxwidget.Widget{table}
	if !props.ReadOnly {
		actions := woxwidget.Container{Width: fieldWidth, Height: 36, Child: woxwidget.Align{
			Width: fieldWidth, Height: 36, Horizontal: 1, Vertical: 0.5, Child: formTableAddButton(props),
		}}
		fieldChildren = append([]woxwidget.Widget{actions}, fieldChildren...)
	}
	field := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: fieldChildren}
	padding := woxwidget.Insets{Top: 6}
	if props.Height <= 0 {
		padding.Bottom = 10
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: padding, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: labelGap, Children: []woxwidget.Widget{label, field}}}
}

func formTableInlineHeader(props FormTableFieldProps, width float32) woxwidget.Widget {
	actionsWidth := float32(0)
	if props.SecondaryLabel != "" {
		actionsWidth += 130
	}
	if !props.ReadOnly {
		actionsWidth += 74
	}
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
	leftChildren := make([]woxwidget.Widget, 0, 2)
	if props.Title != "" || (props.DemoKind != "" && props.DemoIcon != nil) {
		leftChildren = append(leftChildren, woxwidget.Container{Height: 22, Child: title})
	}
	if props.Description != "" {
		leftChildren = append(leftChildren, woxwidget.TextBlock{
			Value: props.Description, MaxLines: 2, LineHeight: 16,
			Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		})
	}
	children := []woxwidget.Widget{woxwidget.Expanded{Child: woxwidget.Container{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: leftChildren}}}}
	gap := float32(0)
	if actionsWidth > 0 {
		gap = 16
		children = append(children, woxwidget.Align{
			Width: actionsWidth, Height: 30, Horizontal: 1, Child: formTableHeaderActions(props),
		})
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisEnd, Children: children}
}

// formTableHeaderActions keeps specialized secondary actions aligned with the
// table's standard Add control.
func formTableHeaderActions(props FormTableFieldProps) woxwidget.Widget {
	actions := make([]woxwidget.Widget, 0, 2)
	if props.SecondaryLabel != "" {
		actions = append(actions, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: props.ID + "-secondary", Label: props.SecondaryLabel, Icon: props.SecondaryIcon, IconSize: 15, IconGap: 5,
			Size: woxcomponent.ButtonCompact, Variant: woxcomponent.ButtonOutline,
			Disabled: props.Invalid, OnTap: props.OnSecondary, Theme: props.Theme,
		}))
	}
	if !props.ReadOnly {
		actions = append(actions, formTableAddButton(props))
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: actions}
}

func formTableAddButton(props FormTableFieldProps) woxwidget.Widget {
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: props.ID + "-add", Label: props.AddLabel, Icon: props.AddIcon, IconSize: 15, IconGap: 5,
		Size: woxcomponent.ButtonCompact, Variant: woxcomponent.ButtonOutline,
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
	widths := formTableColumnWidthsWithOperation(props.Columns, width, !props.ReadOnly)
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
	var operationHeader woxwidget.Widget
	if !props.ReadOnly {
		operationHeader = formTableHeaderCell(props, FormTableColumn{Label: props.OperationLabel}, operationWidth, len(props.Columns))
	}
	bodyHeight := max(float32(0), height-tableSurfaceHeaderHeight)
	if len(props.Rows) == 0 {
		headerChildren := []woxwidget.Widget{
			woxwidget.ScrollView{
				Key: woxwidget.Key(props.ID + "-columns"), ID: props.ID + "-columns", Width: leftViewportWidth, Height: tableSurfaceHeaderHeight,
				ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalHeader, Child: leftHeader,
			},
		}
		if operationHeader != nil {
			headerChildren = append(headerChildren, operationHeader)
		}
		header := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: headerChildren}
		return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, formTableEmptyState(props, width, bodyHeight)},
		}}
	}

	contentHeight := float32(len(props.Rows)) * tableSurfaceRowHeight
	leftRows := make([]woxwidget.Widget, 0, len(props.Rows))
	operationRows := make([]woxwidget.Widget, 0, len(props.Rows))
	for _, row := range props.Rows {
		leftRows = append(leftRows, formTableDataRowCells(props, row, widths[:len(props.Columns)], leftContentWidth))
		if !props.ReadOnly {
			operationRows = append(operationRows, formTableOperationCell(props, row, operationWidth))
		}
	}
	headerChildren := []woxwidget.Widget{
		woxwidget.ScrollView{
			Key: woxwidget.Key(props.ID + "-columns-header"), ID: props.ID + "-columns-header", Width: leftViewportWidth, Height: tableSurfaceHeaderHeight,
			ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalHeader, OnOffsetChanged: func(offset float32) {
				state.horizontalBody.JumpTo(offset)
			}, Child: leftHeader,
		},
	}
	if operationHeader != nil {
		headerChildren = append(headerChildren, operationHeader)
	}
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: headerChildren}
	bodyChildren := []woxwidget.Widget{
		woxwidget.ScrollView{
			Key: woxwidget.Key(props.ID + "-columns-body"), ID: props.ID + "-columns-body", Width: leftViewportWidth, Height: contentHeight,
			ContentWidth: leftContentWidth, Horizontal: true, Controller: state.horizontalBody, OnOffsetChanged: func(offset float32) {
				state.horizontalHeader.JumpTo(offset)
			}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: leftRows},
		},
	}
	if len(operationRows) > 0 {
		bodyChildren = append(bodyChildren, woxwidget.Flex{Axis: woxwidget.Vertical, Children: operationRows})
	}
	bodyContent := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: bodyChildren}
	body := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key(props.ID + "-rows"), Width: width, Height: bodyHeight,
		ContentHeight: contentHeight, Controller: state.verticalBody, Content: bodyContent, ThumbColor: props.Theme.ResultSubtitle,
	})
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, body}}}
}

// formTableColumnWidths preserves Flutter's declared widths and leaves overflow
// to the horizontally scrolling content area instead of shrinking every column.
func formTableColumnWidths(columns []FormTableColumn, tableWidth float32) []float32 {
	return formTableColumnWidthsWithOperation(columns, tableWidth, true)
}

// formTableColumnWidthsWithOperation lets read-only tables use the full surface without an empty action column.
func formTableColumnWidthsWithOperation(columns []FormTableColumn, tableWidth float32, includeOperation bool) []float32 {
	widths := make([]float32, len(columns)+1)
	operationWidth := float32(0)
	if includeOperation {
		operationWidth = formTableOperationWidth + formTableHorizontalMargin*2
	}
	widths[len(widths)-1] = operationWidth
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
		availableWidth := tableWidth - totalDeclaredWidth - operationWidth - totalTooltipWidth
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
		Value: column.Label, Width: contentWidth, Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: woxcomponent.TableHeaderFontSize, Weight: woxui.FontWeightSemibold}, Color: style.headerText,
	}}
	if column.Tooltip != "" {
		contentWidth = max(float32(0), contentWidth-20)
		children[0] = woxwidget.TextBlock{Value: column.Label, Width: contentWidth, Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: woxcomponent.TableHeaderFontSize, Weight: woxui.FontWeightSemibold}, Color: style.headerText}
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
		woxwidget.Align{Width: contentWidth, Height: 18, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: woxcomponent.TableEmptyFontSize}, Color: props.Theme.ResultSubtitle}},
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
		cells = append(cells, formTableDataCellAt(props, row.Index, index, cell, widths[index]))
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
	if icon != nil {
		hoverBackground := props.Theme.ResultSubtitle
		hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: id, Label: label, Icon: woxwidget.Image{Source: icon, Width: 16, Height: 16}, Width: 26, Height: 24, Radius: 4,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: onTap,
		})
	}
	theme := props.Theme
	theme.ResultTitle = props.Theme.ResultSubtitle
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: id, Label: label, Height: 24,
		Variant: woxcomponent.ButtonText, FontSize: 10, OnTap: onTap, Theme: theme,
	})
}

func formTableDataCell(props FormTableFieldProps, cell FormTableCell, width float32) woxwidget.Widget {
	return formTableDataCellAt(props, 0, 0, cell, width)
}

// formTableDataCellAt gives row-specific tooltip triggers stable table coordinates.
func formTableDataCellAt(props FormTableFieldProps, rowIndex, columnIndex int, cell FormTableCell, width float32) woxwidget.Widget {
	style := newTableSurfaceStyle(props.Theme)
	contentWidth := max(float32(0), width-14)
	if cell.Tooltip != "" && props.InfoIcon != nil {
		contentWidth = max(float32(0), contentWidth-20)
	}
	var content woxwidget.Widget = woxwidget.TextBlock{
		Value: cell.Text, Width: contentWidth, Height: 18, MaxLines: 1, ShrinkWrap: cell.Tooltip != "",
		Style: woxui.TextStyle{Size: woxcomponent.TableBodyFontSize}, Color: props.Theme.ResultTitle,
	}
	paddingTop := float32(10)
	if cell.Child != nil {
		content = cell.Child
		paddingTop = 6
	} else if cell.IndicatorColor != nil {
		content = woxwidget.Container{Width: 16, Height: 16, Radius: 8, Color: *cell.IndicatorColor}
	} else if cell.Icon != nil {
		iconSize := cell.IconSize
		if iconSize <= 0 {
			iconSize = 16
		}
		children := []woxwidget.Widget{woxwidget.Image{Source: cell.Icon, Width: iconSize, Height: iconSize}}
		if cell.Text != "" {
			children = append(children, woxwidget.TextBlock{Value: cell.Text, Width: max(float32(0), contentWidth-iconSize-8), Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: woxcomponent.TableBodyFontSize}, Color: props.Theme.ResultTitle})
		}
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: children}
		paddingTop = max(float32(0), (tableSurfaceRowHeight-iconSize)/2)
	}
	if cell.Tooltip != "" && props.InfoIcon != nil {
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			content,
			woxwidget.Gesture{ID: fmt.Sprintf("%s-row-%d-cell-%d-tooltip", props.ID, rowIndex, columnIndex), OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnTooltip != nil {
					props.OnTooltip(inside, cell.Tooltip, bounds)
				}
			}, Child: woxwidget.Image{Source: props.InfoIcon, Width: 14, Height: 14}},
		}}
	}
	return woxwidget.Container{Width: width, Height: tableSurfaceRowHeight, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth,
		Padding: woxwidget.Insets{Left: 8, Top: paddingTop, Right: 6}, Child: content}
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
	panelHeight := min(float32(120), max(float32(0), props.Height-56))
	innerWidth := max(float32(0), panelWidth-48)
	actions := settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "form-table-delete-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-delete-confirm", Label: props.DeleteLabel, OnTap: props.OnDelete},
	)
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-delete-dialog", Label: props.Message, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-delete-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.Insets{Left: 24, Top: 20, Right: 24, Bottom: 22}, BorderColor: border, BorderWidth: 0.75,
		InitialFocus: "form-table-delete-cancel", OnEscape: props.OnCancel, Theme: props.Theme,
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
	Theme       woxcomponent.Theme
	OnSelect    func(int)
	OnAdd       func()
	OnEdit      func()
	OnDelete    func()
	OnClose     func()
}

// FormTableList builds the row list and editor actions.
func FormTableList(props FormTableListProps) woxwidget.Widget {
	footerHeight := float32(54)
	statusHeight := float32(28)
	viewportHeight := max(float32(48), props.Height-footerHeight-statusHeight)
	rows := make([]woxwidget.Widget, 0, len(props.Rows))
	for index, value := range props.Rows {
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
		list = woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "form-table-list-scroll", Width: props.Width, Height: viewportHeight,
			KeepVisible: keepVisible,
			Content:     woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle,
		})
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
		formTableButton("form-table-add", props.AddLabel, props.CanAdd, false, props.OnAdd, props.Theme),
		formTableButton("form-table-edit", "Edit", props.CanEdit, false, props.OnEdit, props.Theme),
		formTableButton("form-table-delete", props.DeleteLabel, props.CanDelete, false, props.OnDelete, props.Theme),
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		list,
		woxwidget.Container{Width: props.Width, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
		woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Stack{Width: props.Width, Height: 38, Children: []woxwidget.StackChild{
			{Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: leftButtons}},
			{AnchorRight: true, Right: 0, Child: formTableButton("form-table-close", props.CloseLabel, true, true, props.OnClose, props.Theme)},
		}}},
	}}
}

// FormTableRowFieldProps contains one Flutter-parity field in a table row editor.
type FormTableRowFieldProps struct {
	ID                  string
	Kind                string
	Label               string
	Description         string
	DescriptionMarkdown bool
	Error               string
	Value               string
	Detail              string
	HotkeyLabels        []string
	Placeholder         string
	RecordingStatus     string
	Width               float32
	Height              float32
	LabelWidth          float32
	State               woxui.TextEditingState
	Controller          *woxwidget.TextEditingController
	Focused             bool
	Recording           bool
	RecordingError      bool
	Hold                bool
	HoldPrefix          string
	Checked             bool
	Protected           bool
	MaxLines            int
	Image               *woxui.Image
	SelectIcon          *woxui.Image
	ImageEmoji          string
	EmojiLabel          string
	UploadLabel         string
	BrowseLabel         string
	SelectLabel         string
	EmojiWidth          float32
	UploadWidth         float32
	SelectWidth         float32
	EmojiIcon           *woxui.Image
	UploadIcon          *woxui.Image
	ActionIcon          *woxui.Image
	ActionLabel         string
	TrailingLabel       string
	Window              *woxui.Window
	Theme               woxcomponent.Theme
	OnTap               func()
	OnFocusChange       func(bool)
	OnChoiceTap         func(woxui.Rect)
	OnTrailingTap       func(woxui.Rect)
	OnActionTap         func()
	OnActionHover       func(bool, woxui.Rect)
	OnFocus             func()
	OnChanged           func(string)
	OnSelectionChanged  func(woxui.TextSelection)
	OnKey               func(woxui.KeyEvent) bool
	OnBrowse            func()
	OnEmoji             func()
	OnUpload            func()
	OnOpenLink          func(string)
}

// FormTableRowFieldHeight returns the compact split-row height used by Flutter's table editor.
func FormTableRowFieldHeight(kind, description string, maxLines int) float32 {
	return FormTableRowFieldHeightWithError(kind, description, "", maxLines)
}

// FormTableRowFieldHeightWithError includes inline validation text under a field.
func FormTableRowFieldHeightWithError(kind, description, errorMessage string, maxLines int) float32 {
	return FormTableRowFieldHeightFor(kind, description, errorMessage, maxLines, false, 0)
}

// FormTableRowFieldHeightFor sizes one row field, including wrapped markdown help text.
func FormTableRowFieldHeightFor(kind, description, errorMessage string, maxLines int, markdown bool, controlWidth float32) float32 {
	descriptionHeight := formTableRowDescriptionHeight(description, markdown, controlWidth)
	errorHeight := float32(0)
	if errorMessage != "" {
		if description != "" {
			errorHeight = 22
		} else {
			errorHeight = 20
		}
	}
	switch kind {
	case "label":
		return 34
	case "woxImage":
		return 88 + descriptionHeight + errorHeight
	case "checkbox":
		return 32 + descriptionHeight + errorHeight
	case "app":
		return 46 + descriptionHeight + errorHeight
	case "textbox", "password", "dirPath":
		controlHeight := float32(34)
		if maxLines > 1 {
			controlHeight = 14 + float32(min(maxLines, 8))*20
		}
		return controlHeight + 4 + descriptionHeight + errorHeight
	default:
		return 38 + descriptionHeight + errorHeight
	}
}

// formTableRowDescriptionHeight mirrors Flutter's intrinsic help-text sizing for plain and markdown tips.
func formTableRowDescriptionHeight(description string, markdown bool, controlWidth float32) float32 {
	if description == "" {
		return 0
	}
	if !markdown {
		return 22 + float32(strings.Count(description, "\n"))*18
	}
	plain := formTableMarkdownPlainText(description)
	paragraphs := strings.Split(plain, "\n\n")
	height := float32(0)
	for index, paragraph := range paragraphs {
		if index > 0 {
			height += formTableMarkdownDescriptionGap
		}
		lines := formTableEstimateWrappedLines(strings.TrimSpace(paragraph), controlWidth)
		height += float32(lines)*formTableMarkdownDescriptionLine + float32(max(0, lines-1))*formTableMarkdownDescriptionRunGap
	}
	// Keep the same trailing slack plain TextBlock descriptions reserve under the control gap.
	return height + 4
}

// formTableMarkdownPlainText keeps link labels so wrap estimates match the rendered tip.
func formTableMarkdownPlainText(value string) string {
	return formTableMarkdownLinkPattern.ReplaceAllString(value, "$1")
}

// formTableEstimateWrappedLines approximates Flutter markdown wrapping without a native text measurer.
func formTableEstimateWrappedLines(value string, width float32) int {
	if value == "" {
		return 1
	}
	if width <= 0 {
		return strings.Count(value, "\n") + 1
	}
	charWidth := float32(7)
	maxChars := max(1, int(width/charWidth))
	lines := 0
	for _, paragraph := range strings.Split(value, "\n") {
		if strings.TrimSpace(paragraph) == "" {
			lines++
			continue
		}
		current := 0
		lines++
		for _, word := range strings.Fields(paragraph) {
			need := utf8.RuneCountInString(word)
			if current == 0 {
				current = need
			} else if current+1+need <= maxChars {
				current += 1 + need
				continue
			} else {
				lines++
				current = need
			}
			for current > maxChars {
				lines++
				current -= maxChars
			}
		}
	}
	return max(1, lines)
}

// FormTableRowField renders labels, controls, and help text with the same split layout as Flutter.
func FormTableRowField(props FormTableRowFieldProps) woxwidget.Widget {
	labelWidth := min(max(float32(60), props.LabelWidth), max(float32(60), props.Width-120))
	controlWidth := max(float32(0), props.Width-labelWidth-10)
	controlHeight := formTableRowControlHeight(props)
	control := formTableRowControl(props, controlWidth, controlHeight)
	rightChildren := []woxwidget.Widget{control}
	if props.Description != "" {
		descriptionHeight := formTableRowDescriptionHeight(props.Description, props.DescriptionMarkdown, controlWidth)
		// DescriptionHeight includes trailing slack used by the row height formula; the widget itself should not.
		widgetHeight := max(float32(18), descriptionHeight-4)
		var description woxwidget.Widget = woxwidget.TextBlock{
			Value: props.Description, Width: controlWidth, Height: widgetHeight, MaxLines: max(1, strings.Count(props.Description, "\n")+1), LineHeight: 18,
			Style: woxui.TextStyle{Size: 12}, Color: formTableAlpha(props.Theme.ActionText, 154),
		}
		if props.DescriptionMarkdown {
			description = woxcomponent.WoxMarkdown(woxcomponent.MarkdownProps{
				ID: props.ID + "-description", Document: woxcomponent.ParseMarkdown(props.Description), Width: controlWidth,
				FontSize: 12, BlockGap: formTableMarkdownDescriptionGap, ExcludeLinkFocus: true, Theme: props.Theme, Window: props.Window, OnOpenLink: props.OnOpenLink,
			})
			description = woxwidget.Container{Width: controlWidth, Height: widgetHeight, Child: description}
		}
		rightChildren = append(rightChildren, description)
	}
	if props.Error != "" {
		rightChildren = append(rightChildren, woxwidget.TextBlock{
			Value: props.Error, Width: controlWidth, Height: 16, MaxLines: 1, LineHeight: 16,
			Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText,
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
			ID: props.ID, Labels: props.HotkeyLabels, Placeholder: props.Placeholder, Focused: props.Focused, Error: props.RecordingError, Hold: props.Hold, HoldPrefix: props.HoldPrefix,
			Window: props.Window, Theme: props.Theme, OnFocusChange: props.OnFocusChange,
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
			woxwidget.Expanded{Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Text{
				Value: props.RecordingStatus, Style: woxui.TextStyle{Size: 12}, Color: statusColor,
			}}},
		}}
	case "label":
		return woxwidget.TextBlock{Value: props.Value, Width: width, Height: height, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader}
	default:
		return formTableRowValueControl(props, width, height)
	}
}

// formTableRowTextControl keeps directory browsing and query-test actions beside the same outlined text control.
func formTableRowTextControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	inputWidth := width
	sideActions := make([]woxwidget.Widget, 0, 2)
	if props.OnBrowse != nil {
		inputWidth = max(float32(100), inputWidth-90)
		sideActions = append(sideActions, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-browse", Label: props.BrowseLabel, Height: height, Radius: 4, Variant: woxcomponent.ButtonOutline, OnTap: props.OnBrowse, Theme: props.Theme}))
	}
	if props.ActionIcon != nil && props.OnActionTap != nil {
		inputWidth = max(float32(100), inputWidth-42)
		action := woxwidget.Semantics{
			AutomationID: props.ID + "-action", Role: woxui.AccessibilityRoleButton, Label: props.ActionLabel,
			Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
			OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action != woxui.AccessibilityActionActivate {
					return fmt.Errorf("unsupported action %q", action)
				}
				props.OnActionTap()
				return nil
			},
			Child: woxwidget.Gesture{
				ID: props.ID + "-action", OnTap: props.OnActionTap,
				OnHoverAt: func(inside bool, bounds woxui.Rect) {
					if props.OnActionHover != nil {
						props.OnActionHover(inside, bounds)
					}
				},
				Child: woxwidget.Align{Width: 34, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.ActionIcon, Width: 18, Height: 18}},
			},
		}
		sideActions = append(sideActions, action)
	}
	padding := woxwidget.Insets{Left: 10, Top: 7, Right: 9, Bottom: 6}
	if props.TrailingLabel != "" {
		padding.Right = 38
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: props.ID, Label: props.Label, Width: inputWidth, Height: height, Radius: 4,
		Padding: padding, Transparent: true,
		BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1,
		Style: woxui.TextStyle{Size: 13}, Value: props.State.Text, Controller: props.Controller, Protected: props.Protected,
		MaxLines: max(1, props.MaxLines), Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
		OnSelectionChanged: props.OnSelectionChanged,
		OnFocusChange: func(focused bool) {
			if props.OnFocusChange != nil {
				props.OnFocusChange(focused)
			}
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	if props.TrailingLabel != "" {
		trailing := woxwidget.Gesture{ID: props.ID + "-trailing", OnTapBounds: props.OnTrailingTap, Child: woxwidget.Container{Width: 34, Height: height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: props.TrailingLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle,
		}}}
		input = woxwidget.Stack{Width: inputWidth, Height: height, Children: []woxwidget.StackChild{
			{Child: input},
			{Left: max(float32(0), inputWidth-34), Child: trailing},
		}}
	}
	if len(sideActions) == 0 {
		return input
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: append([]woxwidget.Widget{input}, sideActions...)}
}

func formTableRowCheckboxControl(props FormTableRowFieldProps) woxwidget.Widget {
	var mark woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if props.Checked {
		mark = woxwidget.Align{Width: 16, Height: 16, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "✓", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}}
	}
	control := woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: 18, Height: 18, Radius: 3, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Padding: woxwidget.UniformInsets(1), Child: mark,
	}}
	return woxwidget.Semantics{
		AutomationID: props.ID, Role: woxui.AccessibilityRoleCheckBox, Label: props.Label,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionToggle}, Checked: props.Checked,
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action != woxui.AccessibilityActionToggle && action != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported checkbox action %q", action)
			}
			if props.OnTap != nil {
				props.OnTap()
			}
			return nil
		},
		Child: formTableRowFocusableControl(props, control),
	}
}

// formTableRowImageControl restores Flutter's preview plus emoji and upload actions.
// The icon is a display surface (like Flutter's WoxImageSelector preview); it never
// becomes a text field, so no caret can appear inside it.
func formTableRowImageControl(props FormTableRowFieldProps, height float32) woxwidget.Widget {
	var preview woxwidget.Widget
	if props.Image != nil {
		preview = woxwidget.Align{Width: 80, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.Image, Width: 64, Height: 64}}
	} else if props.ImageEmoji != "" {
		preview = woxwidget.Align{Width: 80, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: props.ImageEmoji, Style: woxui.TextStyle{Size: 58}, Color: props.Theme.ActionText,
		}}
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
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-emoji", Label: props.EmojiLabel, Icon: props.EmojiIcon, IconSize: 14, IconGap: 6, Height: 28, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 11, Right: 7}, OnTap: props.OnEmoji, Theme: props.Theme}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-upload", Label: props.UploadLabel, Icon: props.UploadIcon, IconSize: 14, IconGap: 6, Height: 28, Radius: 4, FontSize: 12, Variant: woxcomponent.ButtonOutline, Padding: woxwidget.Insets{Left: 11, Right: 7}, OnTap: props.OnUpload, Theme: props.Theme}),
			}},
		},
	}}
}

func formTableRowAppControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	buttonWidth := max(float32(98), props.SelectWidth)
	previewWidth := max(float32(0), width-buttonWidth-10)
	previewChildren := make([]woxwidget.Widget, 0, 2)
	textWidth := previewWidth - 24
	if props.Image != nil {
		previewChildren = append(previewChildren, woxwidget.Image{Source: props.Image, Width: 24, Height: 24, Fit: woxwidget.ImageFitContain})
		textWidth -= 34
	}
	textColor := props.Theme.ActionText
	if props.Detail == "" {
		textColor = props.Theme.ResultSubtitle
	}
	previewChildren = append(previewChildren, woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.TextBlock{
		Value: props.Value, Width: max(float32(0), textWidth), Height: 18, LineHeight: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 13}, Color: textColor,
	}})
	preview := woxwidget.Container{
		Width: previewWidth, Height: height, Radius: 4, BorderColor: formTableAlpha(props.Theme.ResultSubtitle, 115), BorderWidth: 1,
		Padding: woxwidget.Insets{Left: 12, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: previewChildren},
	}
	selectButton := woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: props.ID, Label: props.SelectLabel, Height: height, Radius: 4, FontSize: 13,
		Variant: woxcomponent.ButtonPrimary, OnTap: props.OnTap, Theme: props.Theme,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{preview, selectButton}}
}

// formTableRowSelectControl keeps the selected value and dropdown indicator in separate aligned slots, matching Flutter's expanded dropdown button.
func formTableRowSelectControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	foreground := props.Theme.ActionText
	if props.OnChoiceTap == nil {
		foreground = formTableAlpha(foreground, 128)
	}
	return woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: props.ID, Label: props.Label, Value: props.Value, Width: width, Height: height, Outline: formTableRowOutline(props.Theme, props.Focused),
		Foreground: foreground, Secondary: props.Theme.ActionHeader, Theme: props.Theme, Focused: props.Focused, OnKey: props.OnKey,
		Leading: props.SelectIcon,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
		OnTap: props.OnTap, OnTapBounds: props.OnChoiceTap,
	})
}

// formTableRowFocusableControl gives non-text row controls the same controlled focus contract as text fields.
func formTableRowFocusableControl(props FormTableRowFieldProps, child woxwidget.Widget) woxwidget.Widget {
	return woxwidget.Focusable{
		Key: woxwidget.Key(props.ID), Autofocus: props.Focused, OnKey: props.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
		Child: child,
	}
}

func formTableRowValueControl(props FormTableRowFieldProps, width, height float32) woxwidget.Widget {
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: width, Height: height, Radius: 4, BorderColor: formTableRowOutline(props.Theme, props.Focused), BorderWidth: 1, Padding: woxwidget.Insets{Left: 10, Top: 8, Right: 9},
		Child: woxwidget.TextBlock{Value: props.Value, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionText},
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
	Header        woxwidget.Widget
	HeaderHeight  float32
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

// FormTableRowEditorFooterHeight reserves the shared action row without duplicating the dialog's bottom padding.
const FormTableRowEditorFooterHeight = SettingsDialogActionsHeight

// FormTableRowEditor builds the add, edit, or clone row form.
func FormTableRowEditor(props FormTableRowEditorProps) woxwidget.Widget {
	titleHeight := float32(0)
	if props.Header != nil {
		titleHeight = props.HeaderHeight
	} else if props.Title != "" {
		titleHeight = 32
	}
	statusHeight := float32(0)
	if props.Status != "" {
		statusHeight = 28
	}
	bodyHeight := max(float32(48), props.Height-titleHeight-FormTableRowEditorFooterHeight-statusHeight)
	body := woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "form-table-row-scroll", Width: props.Width, Height: bodyHeight,
		ContentHeight: max(bodyHeight, props.ContentHeight), KeepVisible: props.KeepVisible,
		// Flutter's table update dialog pads each field with bottom: 10.
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: formTableRowFieldGap, Children: props.Rows}, ThumbColor: props.Theme.ResultSubtitle,
	})
	children := make([]woxwidget.Widget, 0, 4)
	if props.Header != nil {
		children = append(children, props.Header)
	} else if titleHeight > 0 {
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
	children = append(children, settingsDialogActions(props.Width, props.Theme,
		settingsDialogAction{ID: "form-table-row-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-row-save", Label: props.SaveLabel, OnTap: props.OnSave},
	))
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

// QueryHotkeyEditorHeaderProps contains the preset-first header used by Flutter's dedicated query hotkey dialog.
type QueryHotkeyEditorHeaderProps struct {
	Width         float32
	Title         string
	Selected      string
	Description   string
	NormalLabel   string
	WebPanelLabel string
	SilentLabel   string
	CustomLabel   string
	DemoIcon      *woxui.Image
	DemoLabel     string
	Theme         woxcomponent.Theme
	OnSelect      func(string)
	OnDemoHover   func(string, bool, woxui.Rect)
	OnOpenLink    func(string)
}

// QueryHotkeyEditorHeader builds the dedicated title, preset selector, and active preset description.
func QueryHotkeyEditorHeader(props QueryHotkeyEditorHeaderProps) woxwidget.Widget {
	type preset struct{ id, label string }
	presets := []preset{{"normal", props.NormalLabel}, {"web-panel", props.WebPanelLabel}, {"silent", props.SilentLabel}, {"custom", props.CustomLabel}}
	buttonWidth := max(float32(0), (props.Width-24)/4)
	buttons := make([]woxwidget.Widget, 0, len(presets))
	for _, item := range presets {
		item := item
		variant := woxcomponent.ButtonOutlinedSurface
		if item.id == props.Selected {
			variant = woxcomponent.ButtonSelected
		}
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "query-hotkey-preset-" + item.id, Label: item.label, Width: buttonWidth, Height: 38, Radius: 8, FontSize: 12,
			Variant: variant, OnTap: func() { props.OnSelect(item.id) }, Theme: props.Theme,
		}))
	}
	var demo woxwidget.Widget
	if props.Selected != "custom" && props.DemoIcon != nil {
		demo = woxwidget.Semantics{
			AutomationID: "query-hotkey-preset-demo", Role: woxui.AccessibilityRoleButton, Label: props.DemoLabel,
			Child: woxwidget.Gesture{ID: "query-hotkey-preset-demo", OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnDemoHover != nil {
					props.OnDemoHover(props.Selected, inside, bounds)
				}
			}, Child: woxwidget.Container{Padding: woxwidget.Insets{Left: 6}, Child: woxwidget.Image{Source: props.DemoIcon, Width: 18, Height: 18}}},
		}
	}
	return woxwidget.Container{Width: props.Width, Height: 122, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.Width, Height: 44, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: formTableAlpha(props.Theme.ActionText, 240)}},
		woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons},
		woxwidget.Container{Width: props.Width, Height: 40, Padding: woxwidget.Insets{Top: 9}, Child: woxcomponent.WoxMarkdown(woxcomponent.MarkdownProps{
			ID: "query-hotkey-preset-description", Document: woxcomponent.ParseMarkdown(props.Description), Width: props.Width, Theme: props.Theme,
			ExcludeLinkFocus: true, OnOpenLink: props.OnOpenLink, InlineTrailing: demo,
		})},
	}}}
}

// QueryVariableChoice is one Flutter-compatible query placeholder row.
type QueryVariableChoice struct {
	Label       string
	Description string
	Icon        *woxui.Image
}

type QueryVariablePickerProps struct {
	Width, Height float32
	Anchor        woxui.Rect
	Choices       []QueryVariableChoice
	Selected      int
	Surface       woxui.Color
	Theme         woxcomponent.Theme
	OnChoose      func(int)
	OnHover       func(int)
	OnCancel      func()
}

// QueryVariablePicker renders the compact two-line picker used by Flutter's query field.
func QueryVariablePicker(props QueryVariablePickerProps) woxwidget.Widget {
	const rowHeight = float32(58)
	menuWidth := min(float32(360), max(float32(1), props.Width-24))
	menuHeight := min(float32(260), float32(len(props.Choices))*rowHeight+12)
	left := min(max(float32(12), props.Anchor.X), max(float32(12), props.Width-menuWidth-12))
	top := props.Anchor.Y + props.Anchor.Height
	if top+menuHeight > props.Height-12 {
		top = props.Anchor.Y - menuHeight
	}
	top = min(max(float32(12), top), max(float32(12), props.Height-menuHeight-12))
	rows := make([]woxwidget.Widget, 0, len(props.Choices))
	for index, choice := range props.Choices {
		index, choice := index, choice
		background := woxui.Color{}
		if index == props.Selected {
			background = formTableAlpha(props.Theme.SelectedBackground, 46)
		}
		activate := func() {
			if props.OnChoose != nil {
				props.OnChoose(index)
			}
		}
		content := woxwidget.Container{Width: menuWidth - 12, Height: rowHeight, Radius: 4, Color: background, Padding: woxwidget.Insets{Left: 14, Top: 8, Right: 10}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			woxwidget.Align{Width: 20, Height: 42, Vertical: .5, Child: woxwidget.Image{Source: choice.Icon, Width: 18, Height: 18}},
			woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
				woxwidget.Text{Value: choice.Label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
				woxwidget.Text{Value: choice.Description, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
			}},
		}}}
		row := woxwidget.Gesture{ID: fmt.Sprintf("query-variable-%d", index), OnTap: activate, OnHover: func(inside bool) {
			if inside && props.OnHover != nil {
				props.OnHover(index)
			}
		}, Child: content}
		rows = append(rows, woxwidget.Semantics{AutomationID: fmt.Sprintf("query-variable-%d", index), Role: woxui.AccessibilityRoleMenuItem, Label: choice.Label, Description: choice.Description, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported variable action %q", action)
			}
			activate()
			return nil
		}, Child: row})
	}
	surface := props.Surface
	if surface.A == 0 {
		surface = props.Theme.Background
		surface.A = 255
	}
	menu := woxwidget.Semantics{AutomationID: "query-variable-picker", Role: woxui.AccessibilityRoleMenu, Child: woxwidget.Container{
		Width: menuWidth, Height: menuHeight, Radius: 4, Color: surface, BorderColor: formTableAlpha(props.Theme.ResultSubtitle, 140), BorderWidth: 1,
		Padding: woxwidget.UniformInsets(6), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "query-variable-backdrop", OnTap: props.OnCancel, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: left, Top: top, Child: menu},
	}}
}

func formTableButton(id, label string, enabled, primary bool, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	variant := woxcomponent.ButtonSecondary
	if primary {
		variant = woxcomponent.ButtonPrimary
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Disabled: !enabled, Variant: variant, OnTap: onTap, Theme: theme})
}
