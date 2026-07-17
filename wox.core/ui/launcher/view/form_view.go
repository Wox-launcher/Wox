package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// FormPanelProps contains the prepared rows and actions rendered by a launcher form.
type FormPanelProps struct {
	Width         float32
	Height        float32
	Title         string
	Rows          []woxwidget.Widget
	ContentHeight float32
	KeepVisible   *woxwidget.ScrollRange
	CancelLabel   string
	SaveLabel     string
	Theme         woxcomponent.Theme
	OnCancel      func()
	OnSave        func()
}

// FormPanel builds the shared launcher form shell.
func FormPanel(props FormPanelProps) woxwidget.Widget {
	bodyHeight := props.Height - 100
	body := woxwidget.ScrollView{
		Key: "form-scroll", ID: "form-scroll", Width: props.Width - 28, Height: bodyHeight,
		ContentHeight: max(bodyHeight, props.ContentHeight), KeepVisible: props.KeepVisible,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), props.Width-28-210), Height: 36},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-cancel", Label: props.CancelLabel, Width: 86, Height: 36, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-save", Label: props.SaveLabel, Width: 104, Height: 36, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSave, Theme: props.Theme}),
	}}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Radius: 12, Color: props.Theme.ActionBackground,
		Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14, Bottom: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Container{Width: props.Width - 28, Height: 28, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}},
			body,
			buttons,
		}},
	}
}

// FormStaticFieldProps contains one non-interactive form row.
type FormStaticFieldProps struct {
	Width  float32
	Height float32
	Value  string
	Kind   string
	Theme  woxcomponent.Theme
}

// FormStaticField builds a heading, label, spacer, or unsupported field row.
func FormStaticField(props FormStaticFieldProps) woxwidget.Widget {
	if props.Kind == "newline" {
		return woxwidget.Painter{Width: props.Width, Height: props.Height}
	}
	style := woxui.TextStyle{Size: 12}
	color := props.Theme.ActionHeader
	padding := woxwidget.Insets{Top: 8}
	if props.Kind == "head" {
		style = woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
		color = props.Theme.ActionText
	}
	if props.Kind == "unsupported" {
		style = woxui.TextStyle{Size: 11}
		padding.Top = 10
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: padding, Child: woxwidget.Text{Value: props.Value, Style: style, Color: color}}
}

// FormModelFieldProps contains one model selector row.
type FormModelFieldProps struct {
	ID      string
	Label   string
	Value   string
	Status  string
	Width   float32
	Height  float32
	Focused bool
	Theme   woxcomponent.Theme
	OnTap   func()
}

// FormModelField builds a model selector row.
func FormModelField(props FormModelFieldProps) woxwidget.Widget {
	background := formFieldBackground(props.Focused, props.Theme)
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		formFieldLabel(props.Label, 132, 56, 16, props.Theme),
		woxwidget.Container{Width: props.Width - 142, Height: 56, Radius: 8, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Value, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
		}}},
	}}}}
}

// FormAppFieldProps contains one application selector row.
type FormAppFieldProps struct {
	ID      string
	Label   string
	Name    string
	Detail  string
	Width   float32
	Height  float32
	Focused bool
	Theme   woxcomponent.Theme
	OnTap   func()
}

// FormAppField builds an application selector row.
func FormAppField(props FormAppFieldProps) woxwidget.Widget {
	fieldWidth := props.Width - 142
	value := woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{
		Width: fieldWidth, Height: 42, Radius: 8, Color: formFieldBackground(props.Focused, props.Theme), Padding: woxwidget.Insets{Left: 12, Top: 7, Right: 12},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.Text{Value: props.Detail, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
		}},
	}}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		formFieldLabel(props.Label, 132, 42, 11, props.Theme), value,
	}}}
}

// FormHotkeyFieldProps contains one Flutter-parity hotkey recorder row.
type FormHotkeyFieldProps struct {
	ID          string
	Label       string
	Description string
	Labels      []string
	Placeholder string
	Status      string
	Width       float32
	Height      float32
	Focused     bool
	Recording   bool
	Window      *woxui.Window
	Theme       woxcomponent.Theme
	OnTap       func()
}

// FormHotkeyField keeps the recorder right-aligned without moving it when a recording hint appears.
func FormHotkeyField(props FormHotkeyFieldProps) woxwidget.Widget {
	labelWidth := float32(132)
	gap := float32(10)
	if props.Width >= 842 {
		labelWidth = min(float32(550), max(float32(132), props.Width-292))
		gap = 32
	}
	controlWidth := max(float32(0), props.Width-labelWidth-gap)
	recorder, recorderWidth := woxcomponent.WoxHotkeyRecorder(woxcomponent.HotkeyRecorderProps{
		Labels: props.Labels, Placeholder: props.Placeholder, Focused: props.Focused, Window: props.Window, Theme: props.Theme,
	})
	recorder = woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: recorder}
	recorderLeft := max(float32(0), controlWidth-recorderWidth)
	controlChildren := []woxwidget.StackChild{{Left: recorderLeft, Top: 8, Child: recorder}}
	if props.Recording && props.Status != "" && recorderLeft > 8 {
		controlChildren = append(controlChildren, woxwidget.StackChild{Top: 14, Child: woxwidget.Align{
			Width: recorderLeft - 8, Height: 18, Horizontal: 1, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		}})
	}
	control := woxwidget.Stack{Width: controlWidth, Height: 46, Children: controlChildren}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Label, Description: props.Description, Width: props.Width, Height: props.Height, LabelWidth: labelWidth, Gap: gap,
		Padding: woxwidget.Insets{Top: 5, Bottom: 5}, Child: control, Theme: props.Theme,
	})
}

// FormValueFieldProps contains one compact tappable form value.
type FormValueFieldProps struct {
	ID      string
	Label   string
	Value   string
	Width   float32
	Height  float32
	Focused bool
	Theme   woxcomponent.Theme
	OnTap   func()
}

// FormValueField builds a checkbox or selector row.
func FormValueField(props FormValueFieldProps) woxwidget.Widget {
	return woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		formFieldLabel(props.Label, 132, 42, 11, props.Theme),
		woxwidget.Container{Width: props.Width - 142, Height: 42, Radius: 8, Color: formFieldBackground(props.Focused, props.Theme), Padding: woxwidget.Insets{Left: 12, Top: 12}, Child: woxwidget.Text{
			Value: props.Value, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}},
	}}}}
}

// FormTextFieldProps contains one editable form row.
type FormTextFieldProps struct {
	ID        string
	Label     string
	Width     float32
	Height    float32
	State     woxui.TextEditingState
	Focused   bool
	Protected bool
	MaxLines  int
	Window    *woxui.Window
	Theme     woxcomponent.Theme
	OnFocus   func()
	OnChanged func(string)
	OnKey     func(woxui.KeyEvent) bool
	OnBrowse  func()
}

// FormTextField builds a shared text input row with an optional directory picker.
func FormTextField(props FormTextFieldProps) woxwidget.Widget {
	fieldWidth := props.Width - 142
	inputWidth := fieldWidth
	if props.OnBrowse != nil {
		inputWidth = max(float32(80), fieldWidth-92)
	}
	fieldHeight := max(float32(42), props.Height-14)
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: props.ID, Label: props.Label, Width: inputWidth, Height: fieldHeight, Radius: 8,
		Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 10, Bottom: 8}, Background: formFieldBackground(props.Focused, props.Theme),
		Style: woxui.TextStyle{Size: 13}, Value: props.State.Text, Focused: props.Focused, Protected: props.Protected,
		MaxLines: props.MaxLines, Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	var valueField woxwidget.Widget = input
	if props.OnBrowse != nil {
		valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			input,
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-browse", Label: "Browse", Width: 84, Height: fieldHeight, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnBrowse, Theme: props.Theme}),
		}}
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		formFieldLabel(props.Label, 132, fieldHeight, 11, props.Theme), valueField,
	}}}
}

func formFieldLabel(label string, width, height, top float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: top}, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ActionText,
	}}
}

func formFieldBackground(focused bool, theme woxcomponent.Theme) woxui.Color {
	if focused {
		return theme.SelectedBackground
	}
	return theme.QueryBackground
}
