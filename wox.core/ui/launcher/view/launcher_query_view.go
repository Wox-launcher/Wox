package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherQueryProps contains the prepared text and callbacks for the launcher query editor.
type LauncherQueryProps struct {
	Width            float32
	Height           float32
	Style            woxui.TextStyle
	State            woxui.TextEditingState
	DisplayValue     string
	Selected         string
	CompletionSuffix string
	PrefixWidth      float32
	SelectedWidth    float32
	CaretWidth       float32
	CompositionWidth float32
	FocusWidth       float32
	TextWidth        float32
	CaretHeight      float32
	Focused          bool
	Theme            woxcomponent.Theme
	OnTapAt          func(float32)
	OnTapEnd         func()
	OnDragStart      func()
	OnKey            func(woxui.KeyEvent) bool
	OnTextInput      func(woxui.TextInputEvent) bool
	OnSetValue       func(string) error
	OnTextInputState func(woxui.TextInputState)
}

// LauncherHeaderProps contains the query box and its optional accessories.
type LauncherHeaderProps struct {
	Width             float32
	Height            float32
	QueryBoxHeight    float32
	QueryEditorHeight float32
	QueryWidth        float32
	QueryRadius       float32
	AppPadding        woxwidget.Insets
	Theme             woxcomponent.Theme
	Query             LauncherQueryProps
	Refinement        woxwidget.Widget
	RefinementWidth   float32
	Glance            woxwidget.Widget
	GlanceWidth       float32
	Icon              *woxui.Image
}

// LauncherHeaderView builds the query box and prepared accessory views.
func LauncherHeaderView(props LauncherHeaderProps) woxwidget.Widget {
	const queryLeftPadding = float32(8)
	const accessoryGap = float32(12)
	queryVerticalPadding := (props.QueryBoxHeight - props.QueryEditorHeight) / 2
	children := []woxwidget.Widget{woxwidget.Container{
		Width: props.QueryWidth, Height: props.QueryBoxHeight, Padding: woxwidget.Insets{Top: queryVerticalPadding, Bottom: queryVerticalPadding},
		Child: LauncherQueryView(props.Query),
	}}
	if props.Refinement != nil {
		children = append(children, woxwidget.Container{
			Width: props.RefinementWidth, Height: props.QueryBoxHeight, Padding: woxwidget.Insets{Top: 10.5, Bottom: 10.5}, Child: props.Refinement,
		})
	}
	if props.Glance != nil {
		children = append(children, woxwidget.Container{
			Width: props.GlanceWidth, Height: props.QueryBoxHeight, Padding: woxwidget.Insets{Top: 12.5, Bottom: 12.5}, Child: props.Glance,
		})
	}
	if props.Icon != nil {
		children = append(children, woxwidget.Container{
			Width: 30, Height: props.QueryBoxHeight, Padding: woxwidget.Insets{Top: 10.5, Bottom: 10.5},
			Child: woxwidget.Container{Width: 30, Height: 34, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Image{Source: props.Icon, Width: 30, Height: 30}},
		})
	}
	horizontalPadding := props.AppPadding.Left + props.AppPadding.Right
	return woxwidget.Container{
		Width: props.Width, Height: props.Height,
		Padding: woxwidget.Insets{Left: props.AppPadding.Left, Top: props.AppPadding.Top, Right: props.AppPadding.Right},
		Child: woxwidget.Container{
			Width: props.Width - horizontalPadding, Height: props.QueryBoxHeight, Radius: props.QueryRadius, Color: props.Theme.QueryBackground,
			Padding: woxwidget.Insets{Left: queryLeftPadding, Right: 6},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: accessoryGap, Children: children},
		},
	}
}

// LauncherQueryView builds the query editor from adapter-prepared text metrics.
func LauncherQueryView(props LauncherQueryProps) woxwidget.Widget {
	var editor woxwidget.Widget = woxwidget.Gesture{
		ID: "query-editor",
		OnTapAt: func(position woxui.Point) {
			if props.OnTapAt != nil {
				props.OnTapAt(position.X)
			}
		},
		Child: woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			caretY := bounds.Y + (bounds.Height-props.CaretHeight)/2
			if props.Focused && props.State.Composition == "" && props.CompletionSuffix != "" {
				hintColor := props.Theme.QueryText
				hintColor.A = 96
				displayList.DrawText(props.CompletionSuffix, woxui.Rect{X: bounds.X + props.TextWidth, Y: bounds.Y, Width: max(float32(0), bounds.Width-props.TextWidth), Height: bounds.Height}, props.Style, hintColor)
			}
			if props.Focused && props.State.Composition == "" && props.Selected != "" {
				displayList.FillRoundedRect(woxui.Rect{X: bounds.X + props.PrefixWidth, Y: caretY, Width: props.SelectedWidth, Height: props.CaretHeight}, 3, props.Theme.SelectionBackground)
			}
			displayList.DrawText(props.DisplayValue, bounds, props.Style, props.Theme.QueryText)
			if props.Focused && props.State.Composition == "" && props.Selected != "" {
				displayList.DrawText(props.Selected, woxui.Rect{X: bounds.X + props.PrefixWidth, Y: bounds.Y, Width: props.SelectedWidth, Height: bounds.Height}, props.Style, props.Theme.SelectionText)
			}
			if !props.Focused {
				return
			}

			cursorX := bounds.X + props.CaretWidth
			displayList.FillRect(woxui.Rect{X: cursorX, Y: caretY, Width: 1, Height: props.CaretHeight}, props.Theme.Cursor)
			if props.OnTextInputState != nil {
				props.OnTextInputState(woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: cursorX, Y: caretY, Width: 1, Height: props.CaretHeight}})
			}
			if props.State.Composition != "" {
				displayList.FillRect(woxui.Rect{X: bounds.X + props.PrefixWidth, Y: caretY + props.CaretHeight - 1, Width: props.CompositionWidth, Height: 1}, props.Theme.Cursor)
			}
		}},
	}
	editor = woxwidget.EditableText{
		Key:          "launcher-query-input-key",
		AutomationID: "launcher.query.input",
		Label:        "Search Wox",
		Value:        props.State.Text,
		Autofocus:    true,
		Disabled:     !props.Focused,
		OnKey:        props.OnKey,
		OnTextInput:  props.OnTextInput,
		OnSetValue:   props.OnSetValue,
		TextInput: func(bounds woxui.Rect) woxui.TextInputState {
			return woxui.TextInputState{Enabled: true, CursorRect: woxui.Rect{X: bounds.X + props.FocusWidth, Y: bounds.Y, Width: 1, Height: bounds.Height}}
		},
		Child: editor,
	}
	dragLeft := min(props.Width, props.TextWidth+6)
	if dragLeft >= props.Width {
		return editor
	}
	return woxwidget.Stack{
		Width: props.Width, Height: props.Height,
		Children: []woxwidget.StackChild{
			{Child: editor},
			{Left: dragLeft, Child: woxwidget.Gesture{
				ID:          "query-drag-area",
				OnTap:       props.OnTapEnd,
				OnDragStart: props.OnDragStart,
				Child:       woxwidget.Container{Width: props.Width - dragLeft, Height: props.Height},
			}},
		},
	}
}
