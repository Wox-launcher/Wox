package component

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const textFieldLineHeight = float32(20)

type textFieldLine struct {
	start int
	end   int
	text  string
}

// TextFieldProps describes a controlled Wox text field.
type TextFieldProps struct {
	ID          string
	Label       string
	Hint        string
	Width       float32
	Height      float32
	Radius      float32
	Padding     woxwidget.Insets
	Background  woxui.Color
	Transparent bool
	BorderColor woxui.Color
	BorderWidth float32
	Style       woxui.TextStyle
	TextColor   woxui.Color
	// TextAlignmentY optically positions measured glyph bounds within each line without moving the caret.
	TextAlignmentY float32
	State          woxui.TextEditingState
	Focused        bool
	Autofocus      bool
	// ControllerManagedFocus preserves a surface's existing shared Tab and keyboard routing.
	ControllerManagedFocus bool
	Disabled               bool
	ReadOnly               bool
	Protected              bool
	MaxLines               int
	Window                 *woxui.Window
	Theme                  Theme
	OnCaret                func(int)
	OnKey                  func(woxui.KeyEvent) bool
	OnTextInput            func(woxui.TextInputEvent) bool
	OnFocusChange          func(bool)
	OnSetValue             func(string) error
}

// WoxTextField builds a controlled text field with shared IME, selection, and accessibility behavior.
func WoxTextField(props TextFieldProps) woxwidget.Widget {
	height := props.Height
	if height <= 0 {
		height = 40
	}
	radius := props.Radius
	if radius <= 0 {
		radius = 8
	}
	padding := props.Padding
	if padding == (woxwidget.Insets{}) {
		padding = woxwidget.Insets{Left: 12, Top: 9, Right: 12, Bottom: 7}
	}
	background := props.Background
	if background.A == 0 && !props.Transparent {
		background = props.Theme.QueryBackground
	}
	style := props.Style
	if style.Size <= 0 {
		style = woxui.TextStyle{Size: 13}
	}
	textColor := props.TextColor
	if textColor.A == 0 {
		textColor = props.Theme.ActionText
	}
	maxLines := max(1, props.MaxLines)
	innerWidth := max(float32(0), props.Width-padding.Left-padding.Right)
	innerHeight := max(float32(0), height-padding.Top-padding.Bottom)
	state := props.State
	content := woxwidget.Gesture{ID: props.ID, OnTapAt: func(position woxui.Point) {
		if props.Disabled || props.Window == nil || props.OnCaret == nil {
			return
		}
		point := woxui.Point{X: max(float32(0), position.X-padding.Left), Y: max(float32(0), position.Y-padding.Top)}
		props.OnCaret(textFieldOffsetAt(state, props.Window, style, maxLines, innerWidth, point))
	}, Child: woxwidget.Container{
		Width: props.Width, Height: height, Radius: radius, Color: background, BorderColor: props.BorderColor, BorderWidth: props.BorderWidth, Padding: padding,
		Child: woxwidget.Clip{Width: innerWidth, Height: innerHeight, Child: woxwidget.CaretPainter{Width: innerWidth, Height: innerHeight, Active: props.Focused, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect, caretVisible bool) {
			if state.Text == "" && state.Composition == "" && props.Hint != "" {
				displayList.DrawText(props.Hint, textFieldAlignedTextBounds(bounds, props.Hint, style, props.TextAlignmentY, props.Window), style, props.Theme.ResultSubtitle)
			}
			if props.Window != nil {
				drawTextField(displayList, bounds, state, style, textColor, props.Theme, props.Focused, caretVisible, maxLines, props.TextAlignmentY, props.Window)
				if props.ControllerManagedFocus && props.Focused {
					_ = props.Window.SetTextInputState(woxui.TextInputState{Enabled: true, CursorRect: textFieldCursorRect(state, style, maxLines, bounds, props.Window)})
				}
			}
		}},
		}}}
	if props.ControllerManagedFocus {
		actions := []woxui.AccessibilityAction{}
		if !props.Disabled && !props.ReadOnly && props.OnSetValue != nil {
			actions = append(actions, woxui.AccessibilityActionSetValue)
		}
		return woxwidget.Semantics{
			Key: woxwidget.Key(props.ID), AutomationID: props.ID, Role: woxui.AccessibilityRoleTextField, Label: props.Label,
			Value: state.Text, Actions: actions, Disabled: props.Disabled, ReadOnly: props.ReadOnly, Protected: props.Protected,
			OnAction: func(action woxui.AccessibilityAction, value string) error {
				if action == woxui.AccessibilityActionSetValue && props.OnSetValue != nil {
					return props.OnSetValue(value)
				}
				return nil
			},
			Child: content,
		}
	}
	key := woxwidget.Key(props.ID)
	return woxwidget.EditableText{
		Key: key, AutomationID: props.ID, Label: props.Label, Value: state.Text, ReadOnly: props.ReadOnly, Protected: props.Protected,
		Autofocus: props.Autofocus, Disabled: props.Disabled, OnKey: props.OnKey, OnTextInput: props.OnTextInput,
		OnFocusChange: props.OnFocusChange, OnSetValue: props.OnSetValue,
		TextInput: func(bounds woxui.Rect) woxui.TextInputState {
			if !props.Focused || props.Window == nil {
				return woxui.TextInputState{}
			}
			innerBounds := woxui.Rect{X: bounds.X + padding.Left, Y: bounds.Y + padding.Top, Width: innerWidth, Height: innerHeight}
			return woxui.TextInputState{Enabled: true, CursorRect: textFieldCursorRect(state, style, maxLines, innerBounds, props.Window)}
		},
		Child: content,
	}
}

func textFieldLines(value string) []textFieldLine {
	runes := []rune(value)
	lines := make([]textFieldLine, 0, strings.Count(value, "\n")+1)
	start := 0
	for index, current := range runes {
		if current == '\n' {
			lines = append(lines, textFieldLine{start: start, end: index, text: string(runes[start:index])})
			start = index + 1
		}
	}
	lines = append(lines, textFieldLine{start: start, end: len(runes), text: string(runes[start:])})
	return lines
}

func textFieldLineIndex(lines []textFieldLine, offset int) int {
	for index, line := range lines {
		if offset <= line.end || index == len(lines)-1 {
			return index
		}
	}
	return 0
}

func textFieldOffsetAt(state woxui.TextEditingState, window *woxui.Window, style woxui.TextStyle, maxLines int, width float32, point woxui.Point) int {
	lines := textFieldLines(state.Text)
	caretLine := textFieldLineIndex(lines, state.Selection.Focus)
	firstLine := max(0, caretLine-maxLines+1)
	lineIndex := min(len(lines)-1, firstLine+max(0, int(point.Y/textFieldLineHeight)))
	line := lines[lineIndex]
	runes := []rune(line.text)
	if maxLines == 1 {
		point.X += textFieldHorizontalOffset([]rune(state.Text), state.Selection.Focus, style, width, window)
	}
	offset := len(runes)
	previousWidth := float32(0)
	for candidate := 1; candidate <= len(runes); candidate++ {
		metrics, _ := window.MeasureText(string(runes[:candidate]), style)
		if point.X < (previousWidth+metrics.Size.Width)*0.5 {
			offset = candidate - 1
			break
		}
		previousWidth = metrics.Size.Width
	}
	return line.start + offset
}

func textFieldHorizontalOffset(runes []rune, focus int, style woxui.TextStyle, width float32, window *woxui.Window) float32 {
	focus = max(0, min(len(runes), focus))
	metrics, _ := window.MeasureText(string(runes[:focus]), style)
	return max(float32(0), metrics.Size.Width-max(float32(0), width-4))
}

// textFieldAlignedTextBounds aligns measured glyphs while preserving the line box used by editing geometry.
func textFieldAlignedTextBounds(bounds woxui.Rect, value string, style woxui.TextStyle, alignment float32, window *woxui.Window) woxui.Rect {
	if alignment <= 0 || value == "" || window == nil {
		return bounds
	}
	metrics, err := window.MeasureText(value, style)
	if err != nil || metrics.Size.Height <= 0 {
		return bounds
	}
	height := min(bounds.Height, metrics.Size.Height)
	bounds.Y += max(float32(0), bounds.Height-height) * min(alignment, float32(1))
	bounds.Height = height
	return bounds
}

func drawTextField(displayList *woxui.DisplayList, bounds woxui.Rect, state woxui.TextEditingState, style woxui.TextStyle, textColor woxui.Color, theme Theme, focused, caretVisible bool, maxLines int, textAlignmentY float32, window *woxui.Window) {
	displayRunes, start, end, focus, compositionStart, compositionEnd := textFieldDisplayState(state)
	lines := textFieldLines(string(displayRunes))
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/textFieldLineHeight)))
	firstLine := max(0, caretLine-visibleLines+1)
	lastLine := min(len(lines), firstLine+visibleLines)
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(displayRunes, focus, style, bounds.Width, window)
	}
	for lineIndex := firstLine; lineIndex < lastLine; lineIndex++ {
		line := lines[lineIndex]
		y := bounds.Y + float32(lineIndex-firstLine)*textFieldLineHeight
		textBounds := textFieldAlignedTextBounds(woxui.Rect{X: bounds.X - horizontalOffset, Y: y, Width: bounds.Width + horizontalOffset, Height: textFieldLineHeight}, line.text, style, textAlignmentY, window)
		selectionStart := max(start, line.start)
		selectionEnd := min(end, line.end)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedMetrics, _ := window.MeasureText(string(displayRunes[selectionStart:selectionEnd]), style)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: y, Width: selectedMetrics.Size.Width, Height: textFieldLineHeight}, 3, theme.SelectionBackground)
		}
		displayList.DrawText(line.text, textBounds, style, textColor)
		if focused && selectionStart < selectionEnd {
			prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:selectionStart]), style)
			selectedText := string(displayRunes[selectionStart:selectionEnd])
			selectedMetrics, _ := window.MeasureText(selectedText, style)
			selectedBounds := textBounds
			selectedBounds.X = bounds.X - horizontalOffset + prefixMetrics.Size.Width
			selectedBounds.Width = selectedMetrics.Size.Width
			displayList.DrawText(selectedText, selectedBounds, style, theme.SelectionText)
		}
	}
	if !focused {
		return
	}
	line := lines[caretLine]
	caretMetrics, _ := window.MeasureText(string(displayRunes[line.start:focus]), style)
	cursorX := bounds.X - horizontalOffset + caretMetrics.Size.Width
	cursorY := bounds.Y + float32(caretLine-firstLine)*textFieldLineHeight
	if compositionStart >= line.start && compositionEnd <= line.end {
		prefixMetrics, _ := window.MeasureText(string(displayRunes[line.start:compositionStart]), style)
		compositionMetrics, _ := window.MeasureText(string(displayRunes[compositionStart:compositionEnd]), style)
		displayList.FillRect(woxui.Rect{X: bounds.X - horizontalOffset + prefixMetrics.Size.Width, Y: cursorY + 19, Width: compositionMetrics.Size.Width, Height: 1}, theme.Cursor)
	}
	if caretVisible {
		displayList.FillRect(woxui.Rect{X: cursorX, Y: cursorY, Width: 1, Height: textFieldLineHeight}, theme.Cursor)
	}
}

func textFieldCursorRect(state woxui.TextEditingState, style woxui.TextStyle, maxLines int, bounds woxui.Rect, window *woxui.Window) woxui.Rect {
	displayRunes, _, _, focus, _, _ := textFieldDisplayState(state)
	lines := textFieldLines(string(displayRunes))
	caretLine := textFieldLineIndex(lines, focus)
	visibleLines := max(1, min(maxLines, int(bounds.Height/textFieldLineHeight)))
	firstLine := max(0, caretLine-visibleLines+1)
	horizontalOffset := float32(0)
	if visibleLines == 1 {
		horizontalOffset = textFieldHorizontalOffset(displayRunes, focus, style, bounds.Width, window)
	}
	line := lines[caretLine]
	metrics, _ := window.MeasureText(string(displayRunes[line.start:focus]), style)
	return woxui.Rect{
		X:     bounds.X - horizontalOffset + metrics.Size.Width,
		Y:     bounds.Y + float32(caretLine-firstLine)*textFieldLineHeight,
		Width: 1, Height: 22,
	}
}

func textFieldDisplayState(state woxui.TextEditingState) ([]rune, int, int, int, int, int) {
	runes := []rune(state.Text)
	start := max(0, min(len(runes), state.Selection.Start()))
	end := max(start, min(len(runes), state.Selection.End()))
	focus := max(0, min(len(runes), state.Selection.Focus))
	displayValue := state.Text
	compositionStart := -1
	compositionEnd := -1
	if state.Composition != "" {
		displayValue = string(runes[:start]) + state.Composition + string(runes[end:])
		compositionStart = start
		compositionEnd = start + len([]rune(state.Composition))
		start = compositionEnd
		end = compositionEnd
		focus = compositionEnd
	}
	return []rune(displayValue), start, end, focus, compositionStart, compositionEnd
}
