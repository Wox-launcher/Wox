package preview

import (
	"strings"

	"wox/plugin/system/shell/interactive"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type ShellSelection struct {
	Anchor, End int
}

type ShellPreviewProps struct {
	Width, Height, Padding, BarHeight float32
	CellWidth, CellHeight             float32
	Style                             woxui.TextStyle
	Theme                             woxcomponent.Theme
	Frame                             *interactive.Frame
	Selection                         ShellSelection
	Output                            *ShellOutputProps
	Footer                            woxwidget.Widget
	FocusKey                          woxwidget.Key
	Open, Reopen, Help                string
	Error, MarkedText                 string
	OnOpen, OnFocus, OnBlur           func()
	OnKey                             func(woxui.KeyEvent) bool
	OnText                            func(woxui.TextInputEvent) bool
	OnSelect                          func(index int, extend bool)
	OnScroll                          func(rows int)
}

// ShellPreviewView renders the controls and a fixed-cell viewport from a single frame snapshot.
func ShellPreviewView(p ShellPreviewProps) woxwidget.Widget {
	if p.Frame == nil && p.Error == "" {
		// Keep the pane in place while switching sessions without flashing a startup label.
		return woxwidget.Container{Width: p.Width, Height: p.Height}
	}
	button := woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: "shell-interactive-open", Label: p.Reopen, Variant: woxcomponent.ButtonOutlinedSurface,
		Theme: p.Theme.Controls, OnTap: p.OnOpen,
	})
	status := func(label string) woxwidget.Widget {
		return woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: p.Theme.Controls.Scaled(woxcomponent.ResultSubtitleFontSize)}, Color: p.Theme.Controls.TextSecondary}
	}
	if p.Frame == nil {
		return woxwidget.Align{
			Width: p.Width, Height: p.Height, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: p.Padding, Children: []woxwidget.Widget{status(p.Error), button}},
		}
	}

	gridWidth := max(0, p.Width-2*p.Padding)
	gridHeight := max(0, p.Height-p.BarHeight-p.Padding)
	footer := p.Footer
	if footer == nil {
		footer = status(p.Help)
		if p.Frame.Exited || p.Error != "" {
			footer = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: p.Padding, Children: []woxwidget.Widget{button, status(p.Error)}}
		}
	}
	return woxwidget.Stack{Width: p.Width, Height: p.Height, Children: []woxwidget.StackChild{
		{Top: p.Padding, Left: p.Padding, Child: shellGrid(p, gridWidth, gridHeight)},
		{Top: p.Padding + gridHeight, Left: p.Padding, Child: woxwidget.Align{Width: gridWidth, Height: p.BarHeight, Vertical: 0.5, Child: footer}},
	}}
}

func shellGrid(p ShellPreviewProps, width, height float32) woxwidget.Widget {
	frame := p.Frame
	cellAt := func(point woxui.Point) int {
		column := min(frame.Size.Columns-1, max(0, int(point.X/p.CellWidth)))
		row := min(frame.Size.Rows-1, max(0, int(point.Y/p.CellHeight)))
		index := row*frame.Size.Columns + column
		if index > 0 && index < len(frame.Cells) && column > 0 && frame.Cells[index].Continuation && frame.Cells[index-1].Wide {
			index--
		}
		return index
	}
	painter := woxwidget.CaretPainter{
		Width: width, Height: height, Active: p.Output == nil && !frame.Exited,
		Paint: func(list *woxui.DisplayList, bounds woxui.Rect, focused, caretVisible bool) {
			paintShellGrid(list, bounds, p, focused, caretVisible)
		},
	}
	if p.Output != nil {
		return shellOutputGrid(*p.Output, painter, cellAt)
	}
	gesture := woxwidget.Gesture{
		Child: painter, Cursor: woxui.PointerCursorText,
		OnTapAt: func(point woxui.Point) {
			p.OnFocus()
			p.OnSelect(cellAt(point), false)
		},
		OnPanStart: func(point woxui.Point) {
			p.OnFocus()
			p.OnSelect(cellAt(point), false)
		},
		OnPanUpdate: func(point woxui.Point) { p.OnSelect(cellAt(point), true) },
		OnScroll: func(delta woxui.Point) {
			if delta.Y > 0 {
				p.OnScroll(-3)
			} else if delta.Y < 0 {
				p.OnScroll(3)
			}
		},
	}
	return woxwidget.Focusable{
		Key: p.FocusKey, SkipTraversal: true, UnfocusOnPointerOutside: true, FocusRingColor: p.Theme.Controls.Focus,
		OnKey: p.OnKey, OnTextInput: p.OnText,
		OnFocusChange: func(focused bool) {
			if !focused {
				p.OnBlur()
			}
		},
		TextInput: func(bounds woxui.Rect) woxui.TextInputState {
			return woxui.TextInputState{Enabled: !frame.Exited && p.Error == "", CursorRect: shellCellRect(bounds, p, frame.CursorColumn, frame.CursorRow)}
		},
		Child: woxwidget.Semantics{
			AutomationID: "shell-interactive-screen", Role: woxui.AccessibilityRoleTextField,
			Label: p.Open, Value: shellTextRange(frame, 0, len(frame.Cells)), ReadOnly: true,
			Child: gesture,
		},
	}
}

// paintShellGrid draws backgrounds first so the second cell of a wide glyph cannot cover its text.
func paintShellGrid(list *woxui.DisplayList, bounds woxui.Rect, p ShellPreviewProps, focused, caretVisible bool) {
	list.PushClipRect(bounds)
	defer list.PopClipRect()
	themeFG := p.Theme.PreviewText
	themeBG := woxui.Color{}
	if p.Theme.PreviewBackgroundColor != nil {
		themeBG = *p.Theme.PreviewBackgroundColor
	}
	frame := p.Frame
	first, last := shellSelectionRange(p.Selection)
	selectedColor := p.Theme.Controls.TextSelectionBackground
	for index, cell := range frame.Cells {
		bg := shellRGBA(cell.Background, themeBG)
		if cell.Inverse {
			bg = shellRGBA(cell.Foreground, themeFG)
		}
		if index >= first && index < last {
			bg = selectedColor
		}
		if bg != themeBG || index >= first && index < last {
			list.FillRect(shellCellRect(bounds, p, index%frame.Size.Columns, index/frame.Size.Columns), bg)
		}
	}
	for index, cell := range frame.Cells {
		if cell.Text == "" || cell.Continuation {
			continue
		}
		rect := shellCellRect(bounds, p, index%frame.Size.Columns, index/frame.Size.Columns)
		if cell.Wide {
			rect.Width *= 2
		}
		fg := shellRGBA(cell.Foreground, themeFG)
		if cell.Inverse {
			fg = shellRGBA(cell.Background, p.Theme.Controls.Surface)
		}
		if index >= first && index < last {
			fg = p.Theme.Controls.TextSelectionText
		}
		style := p.Style
		if cell.Bold {
			style.Weight = woxui.FontWeightSemibold
		}
		style.Italic = cell.Italic
		list.DrawText(cell.Text, rect, style, fg)
		if cell.Underline {
			list.FillRect(woxui.Rect{X: rect.X, Y: rect.Y + rect.Height - 2, Width: rect.Width, Height: 1}, fg)
		}
	}
	if !frame.CursorVisible || frame.Exited || !focused {
		return
	}
	cursor := shellCellRect(bounds, p, frame.CursorColumn, frame.CursorRow)
	if p.MarkedText != "" {
		cursor.Width = max(0, bounds.X+bounds.Width-cursor.X)
		list.DrawText(p.MarkedText, cursor, p.Style, themeFG)
	} else if caretVisible {
		cursor.Width = 2
		list.FillRect(cursor, themeFG)
	}
}

func shellCellRect(bounds woxui.Rect, p ShellPreviewProps, column, row int) woxui.Rect {
	return woxui.Rect{X: bounds.X + float32(column)*p.CellWidth, Y: bounds.Y + float32(row)*p.CellHeight, Width: p.CellWidth, Height: p.CellHeight}
}

func shellRGBA(value uint32, fallback woxui.Color) woxui.Color {
	if value == 0 {
		return fallback
	}
	return woxui.Color{R: uint8(value >> 24), G: uint8(value >> 16), B: uint8(value >> 8), A: uint8(value)}
}

func shellSelectionRange(selection ShellSelection) (first, end int) {
	if selection.Anchor == selection.End {
		return 0, 0
	}
	return min(selection.Anchor, selection.End), max(selection.Anchor, selection.End) + 1
}

func ShellSelectedText(frame *interactive.Frame, selection ShellSelection) string {
	first, end := shellSelectionRange(selection)
	return shellTextRange(frame, first, end)
}

// shellTextRange skips the second cell of wide glyphs and trims padding at each row boundary.
func shellTextRange(frame *interactive.Frame, first, end int) string {
	if frame == nil || frame.Size.Columns == 0 || first >= end || first >= len(frame.Cells) {
		return ""
	}
	var text, line strings.Builder
	for index := max(0, first); index < min(end, len(frame.Cells)); index++ {
		if index > first && index%frame.Size.Columns == 0 {
			text.WriteString(strings.TrimRight(line.String(), " "))
			text.WriteByte('\n')
			line.Reset()
		}
		cell := frame.Cells[index]
		if cell.Continuation {
			continue
		}
		if cell.Text == "" {
			line.WriteByte(' ')
		} else {
			line.WriteString(cell.Text)
		}
	}
	text.WriteString(strings.TrimRight(line.String(), " "))
	return strings.TrimRight(text.String(), "\n")
}
