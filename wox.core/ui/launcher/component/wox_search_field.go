package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SearchFieldAction describes one centered trailing search-field action.
type SearchFieldAction struct {
	ID       string
	Label    string
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
	Controller    *woxwidget.TextEditingController
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
	trailingInset := float32(0)
	if clearWidth > 0 || actionsWidth > 0 {
		trailingInset = 4
	}
	inputWidth := max(float32(40), props.Width)
	leftPadding := float32(12)
	if leadingWidth > 0 {
		leftPadding = leadingWidth + 2
	}
	rightPadding := float32(6) + clearWidth + actionsWidth + trailingInset
	border := withAlpha(props.Theme.ResultSubtitle, 170)
	onFocusChange := props.OnFocusChange
	if props.OnFocus != nil {
		// Preserve the search entry hook now that the full-width field also receives icon clicks.
		originalOnFocusChange := onFocusChange
		onFocusChange = func(focused bool) {
			if focused {
				props.OnFocus()
			}
			if originalOnFocusChange != nil {
				originalOnFocusChange(focused)
			}
		}
	}
	input := WoxTextField(TextFieldProps{
		ID: props.ID, Label: props.Label, Hint: props.Label, Width: inputWidth, Height: height, Radius: 4,
		Padding: woxwidget.Insets{Left: leftPadding, Top: 11, Right: rightPadding, Bottom: 11}, Transparent: true,
		BorderColor: border, BorderWidth: 1, FocusRingColor: props.Theme.Cursor,
		Style: woxui.TextStyle{Size: SettingsControlFontSize}, TextColor: props.Theme.ResultTitle, TextAlignmentY: 0.5,
		Value: props.Value, Focused: props.Focused, Autofocus: props.Autofocus, Controller: props.Controller, MaxLines: 1, Window: props.Window, Theme: props.Theme,
		OnKey: props.OnKey, OnFocusChange: onFocusChange, OnChanged: props.OnChanged, OnSetValue: props.OnSetValue,
	})
	overlayChildren := make([]woxwidget.Widget, 0, len(props.Actions)+4)
	if props.SearchIcon != nil {
		// Keep the icon non-interactive so pointer hit-testing falls through to the full-width text field.
		overlayChildren = append(overlayChildren, woxwidget.Align{
			Width: leadingWidth, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Image{Source: props.SearchIcon, Width: 18, Height: 18},
		})
	}
	overlayChildren = append(overlayChildren, woxwidget.Expanded{Child: woxwidget.Container{Height: height}})
	if clearWidth > 0 {
		hoverBackground := withAlpha(props.Theme.ResultSubtitle, 25)
		overlayChildren = append(overlayChildren, woxwidget.Align{Width: clearWidth, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: WoxIconButton(IconButtonProps{
			ID: props.ID + "-clear", Label: "Clear search", Icon: CloseGlyph(16, props.Theme.ResultSubtitle), Width: 28, Height: 28, Radius: 14,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, OnTap: props.OnClear,
		})})
	}
	for _, action := range props.Actions {
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
		hoverBackground := withAlpha(props.Theme.ResultSubtitle, 25)
		if action.Active {
			hoverBackground = props.Theme.SelectedBackground
		}
		label := action.Label
		if label == "" {
			label = action.ID
		}
		buttonSize := min(width, float32(30))
		overlayChildren = append(overlayChildren, woxwidget.Align{Width: width, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: WoxIconButton(IconButtonProps{
			ID: action.ID, Label: label, Icon: woxwidget.Image{Source: action.Icon, Width: iconSize, Height: iconSize}, Width: buttonSize, Height: buttonSize, Radius: buttonSize / 2,
			Background: background, HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, Disabled: action.Disabled, OnTap: action.OnTap,
		})})
	}
	if trailingInset > 0 {
		overlayChildren = append(overlayChildren, woxwidget.Container{Width: trailingInset, Height: height})
	}
	return woxwidget.Container{Width: props.Width, Height: height, Radius: 4, BorderColor: border, BorderWidth: 1, Child: woxwidget.Stack{Width: props.Width, Height: height, Children: []woxwidget.StackChild{
		{Child: input},
		{Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: overlayChildren}},
	}}}
}
