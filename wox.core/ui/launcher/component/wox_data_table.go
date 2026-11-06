package component

import (
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
)

// DataTableColumn supplies stable column identity and an optional logical width.
type DataTableColumn struct {
	ID, Label string
	Width     float32
}

// DataTableRow keeps action callbacks local to an immutable row snapshot.
type DataTableRow struct {
	ID      string
	Cells   []string
	Actions []MenuItem
}

// DataTableProps composes bounded rows into a header and scrollable body.
type DataTableProps struct {
	ID, Label     string
	Columns       []DataTableColumn
	Rows          []DataTableRow
	Width, Height float32
	Theme         ControlTheme
	Paint         *ControlPaint
}

// WoxDataTable shares collapsed grid borders, fixed headers, and native scrolling.
func WoxDataTable(p DataTableProps) widget.Widget {
	return widget.LayoutBuilder{Build: func(available ui.Size) widget.Widget {
		width := available.Width
		if p.Width > 0 {
			width = min(width, p.Width)
		}
		rowHeight, padding := p.Theme.Scaled(40), p.Theme.Scaled(8)
		actionSize := p.Theme.Scaled(28)
		actionCount := 0
		for _, row := range p.Rows {
			actionCount = max(actionCount, len(row.Actions))
		}
		actionsWidth := float32(0)
		if actionCount > 0 {
			actionsWidth = float32(actionCount)*actionSize + 2*padding
		}
		widths := make([]float32, len(p.Columns))
		fixed, flexible := actionsWidth, 0
		for i, column := range p.Columns {
			widths[i] = column.Width
			if column.Width > 0 {
				fixed += column.Width
			} else {
				flexible++
			}
		}
		contentWidth := fixed
		for i := range widths {
			if widths[i] == 0 {
				widths[i] = max(p.Theme.Scaled(120), (width-fixed)/float32(max(1, flexible)))
				contentWidth += widths[i]
			}
		}
		height := min(rowHeight*float32(len(p.Rows)+1), available.Height)
		if p.Height > 0 {
			height = min(p.Height, available.Height)
		}
		cell := func(id, label string, w float32, trailing, bottom, header bool) widget.Widget {
			style := ui.TextStyle{Size: p.Theme.Scaled(SettingsControlFontSize)}
			if header {
				style.Weight = ui.FontWeightSemibold
			}
			text := widget.TextBlock{Value: label, MaxLines: 1, Style: style, Color: p.Paint.foreground(p.Theme.Text), ShrinkWrap: true}
			return widget.Semantics{AutomationID: id, Role: ui.AccessibilityRoleText, Label: label, Child: WoxTableGridCell(TableGridCellProps{
				Width: w, Height: rowHeight, Border: p.Theme.Border, Trailing: trailing, Bottom: bottom,
				Padding: widget.Insets{Left: padding, Right: padding}, Child: widget.Align{Vertical: 0.5, Child: text},
			})}
		}
		headers := make([]widget.Widget, 0, len(p.Columns)+1)
		for i, column := range p.Columns {
			headers = append(headers, cell(p.ID+"/header/"+column.ID, column.Label, widths[i], i+1 < len(p.Columns) || actionCount > 0, true, true))
		}
		if actionCount > 0 {
			headers = append(headers, WoxTableGridCell(TableGridCellProps{Width: actionsWidth, Height: rowHeight, Border: p.Theme.Border, Bottom: true}))
		}
		rows := make([]widget.Widget, 0, len(p.Rows))
		for rowIndex, row := range p.Rows {
			cells := make([]widget.Widget, 0, len(p.Columns)+1)
			for i, column := range p.Columns {
				text := ""
				if i < len(row.Cells) {
					text = row.Cells[i]
				}
				cells = append(cells, cell(row.ID+"/cell/"+column.ID, text, widths[i], i+1 < len(p.Columns) || actionCount > 0, rowIndex+1 < len(p.Rows), false))
			}
			if actionCount > 0 {
				actions := make([]widget.Widget, 0, len(row.Actions))
				for _, action := range row.Actions {
					icon := action.Icon
					if icon == "" {
						icon = "action.execute"
					}
					foreground := p.Theme.Text
					if action.Disabled {
						foreground = p.Theme.TextSecondary
					}
					foreground = p.Paint.foreground(foreground)
					button := WoxIconButton(IconButtonProps{ID: action.ID, Label: action.Label, Disabled: action.Disabled, Width: actionSize, Height: actionSize,
						Icon: IconGlyph(icon, p.Theme.Scaled(16), foreground), Radius: p.Theme.Scaled(4), FocusRingColor: p.Theme.Focus,
						HoverBackground: controlHoverColor(p.Theme.Surface, p.Theme.Text), OnTap: action.OnSelect})
					actions = append(actions, WoxTooltip(TooltipProps{Key: widget.Key(action.ID + "/hint"), Message: action.Label, Theme: p.Theme, Child: button}))
				}
				cells = append(cells, WoxTableGridCell(TableGridCellProps{Width: actionsWidth, Height: rowHeight, Border: p.Theme.Border, Bottom: rowIndex+1 < len(p.Rows),
					Padding: widget.Insets{Left: padding, Right: padding}, Child: widget.Align{Vertical: 0.5, Child: widget.Flex{Axis: widget.Horizontal, Children: actions}},
				}))
			}
			rows = append(rows, widget.Semantics{AutomationID: row.ID, Role: ui.AccessibilityRoleGroup, Child: widget.Flex{Axis: widget.Horizontal, Children: cells}})
		}
		body := WoxScrollView(ScrollViewProps{Key: widget.Key(p.ID + "/body-scroll"), Width: contentWidth, Height: max(float32(0), height-rowHeight),
			Content: widget.Flex{Axis: widget.Vertical, Children: rows}, RetainState: true, Theme: p.Theme})
		headerColor := p.Theme.Surface
		if p.Paint != nil && p.Paint.Background != nil {
			headerColor = ui.Color{}
		}
		// Paint the surface once behind cells and the frame once above them.
		// Internal collapsed separators keep their own theme and thickness.
		frame := p.Paint.applyContainer(widget.Container{Width: contentWidth, Height: height, BorderColor: p.Theme.Border, BorderWidth: TableGridBorderWidth})
		surface := frame
		surface.BorderWidth = 0
		frame.Color = ui.Color{}
		content := widget.Stack{Width: contentWidth, Height: height, Children: []widget.StackChild{
			{Child: surface},
			{Child: widget.Flex{Axis: widget.Vertical, Children: []widget.Widget{
				widget.Container{Color: headerColor, Child: widget.Flex{Axis: widget.Horizontal, Children: headers}}, body,
			}}},
			{Child: frame},
		}}
		if p.Paint != nil && p.Paint.BackgroundTransition != nil {
			// Header and body have different native fills. Paint disjoint regions
			// of one rounded surface so translucent overrides never composite twice.
			background := p.Paint.animateBackground(p.ID+"/body", ui.Color{}, func(bodyColor ui.Color) widget.Widget {
				return p.Paint.animateBackground(p.ID+"/header", p.Theme.Surface, func(headerColor ui.Color) widget.Widget {
					return widget.Painter{Width: contentWidth, Height: height, Paint: func(list *ui.DisplayList, bounds ui.Rect) {
						box := surface
						box.Color = headerColor
						list.PushClipRect(ui.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: min(rowHeight, bounds.Height)})
						box.PaintBackground(list, bounds)
						list.PopClipRect()
						box.Color = bodyColor
						list.PushClipRect(ui.Rect{X: bounds.X, Y: bounds.Y + rowHeight, Width: bounds.Width, Height: max(float32(0), bounds.Height-rowHeight)})
						box.PaintBackground(list, bounds)
						list.PopClipRect()
					}}
				})
			})
			content.Children[0].Child = background
			content.Children[1].Child = widget.Flex{Axis: widget.Vertical, Children: []widget.Widget{
				widget.Flex{Axis: widget.Horizontal, Children: headers}, body,
			}}
		}
		return widget.Semantics{AutomationID: p.ID, Role: ui.AccessibilityRoleGroup, Label: p.Label, Child: WoxScrollView(ScrollViewProps{
			Key: widget.Key(p.ID + "/horizontal-scroll"), Width: width, Height: height, ContentWidth: contentWidth, Horizontal: true, RetainState: true, Theme: p.Theme, Content: content,
		})}
	}}
}
