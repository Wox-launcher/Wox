package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SearchFieldAction describes one centered trailing search-field action.
type SearchFieldAction struct {
	ID       string
	Icon     *woxui.Image
	Width    float32
	IconSize float32
	Active   bool
	Disabled bool
	OnTap    func()
}

// SearchFieldProps describes the shared settings and catalog search control.
type SearchFieldProps struct {
	ID            string
	Label         string
	Width         float32
	Value         string
	Focused       bool
	Autofocus     bool
	SearchIcon    *woxui.Image
	Actions       []SearchFieldAction
	Window        *woxui.Window
	Theme         Theme
	OnFocus       func()
	OnClear       func()
	OnKey         func(woxui.KeyEvent) bool
	OnFocusChange func(bool)
	OnChanged     func(string)
	OnSetValue    func(string) error
}

// WoxSearchField keeps compact search geometry and native text focus consistent across settings surfaces.
func WoxSearchField(props SearchFieldProps) woxwidget.Widget {
	const height = float32(42)
	leadingWidth := float32(0)
	if props.SearchIcon != nil {
		leadingWidth = 36
	}
	clearWidth := float32(0)
	if props.Value != "" && props.OnClear != nil {
		clearWidth = 34
	}
	actionsWidth := float32(0)
	for _, action := range props.Actions {
		width := action.Width
		if width <= 0 {
			width = 30
		}
		actionsWidth += width
	}
	inputWidth := max(float32(40), props.Width-leadingWidth-clearWidth-actionsWidth)
	leftPadding := float32(12)
	if leadingWidth > 0 {
		leftPadding = 2
	}
	input := WoxTextField(TextFieldProps{
		ID: props.ID, Label: props.Label, Hint: props.Label, Width: inputWidth, Height: height,
		Padding: woxwidget.Insets{Left: leftPadding, Top: 11, Right: 6, Bottom: 11}, Transparent: true,
		Style: woxui.TextStyle{Size: 13}, TextColor: props.Theme.ResultTitle, TextAlignmentY: 0.5,
		Value: props.Value, Focused: props.Focused, Autofocus: props.Autofocus, MaxLines: 1, Window: props.Window, Theme: props.Theme,
		OnKey: props.OnKey, OnFocusChange: props.OnFocusChange, OnChanged: props.OnChanged, OnSetValue: props.OnSetValue,
	})
	children := make([]woxwidget.Widget, 0, len(props.Actions)+3)
	if props.SearchIcon != nil {
		children = append(children, woxwidget.Gesture{ID: props.ID + "-icon", OnTap: props.OnFocus, Child: woxwidget.Align{
			Width: leadingWidth, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.SearchIcon, Width: 18, Height: 18},
		}})
	}
	children = append(children, input)
	if clearWidth > 0 {
		children = append(children, woxwidget.Gesture{ID: props.ID + "-clear", OnTap: props.OnClear, Child: woxwidget.Align{
			Width: clearWidth, Height: height, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 17}, Color: props.Theme.ResultSubtitle},
		}})
	}
	for _, action := range props.Actions {
		action := action
		width := action.Width
		if width <= 0 {
			width = 30
		}
		iconSize := action.IconSize
		if iconSize <= 0 {
			iconSize = 16
		}
		background := woxui.Color{}
		if action.Active {
			background = props.Theme.SelectedBackground
		}
		onTap := action.OnTap
		if action.Disabled {
			onTap = nil
		}
		children = append(children, woxwidget.Gesture{ID: action.ID, OnTap: onTap, Child: woxwidget.Container{
			Width: width, Height: height, Radius: 4, Color: background,
			Child: woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: action.Icon, Width: iconSize, Height: iconSize}},
		}})
	}
	border := withAlpha(props.Theme.ResultSubtitle, 170)
	if props.Focused {
		border = props.Theme.Cursor
	}
	return woxwidget.Container{Width: props.Width, Height: height, Radius: 4, BorderColor: border, BorderWidth: 1, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}}
}
