package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formAIModelControlHeight = float32(34)

// FormPanelProps contains the prepared rows and actions rendered by a launcher form.
type FormPanelProps struct {
	Width          float32
	MaximumHeight  float32
	Padding        woxwidget.Insets
	Rows           []woxwidget.Widget
	KeepVisibleKey woxwidget.Key
	CancelLabel    string
	SaveLabel      string
	Theme          woxcomponent.Theme
	OnCancel       func()
	OnSave         func()
}

// FormPanel builds the shared launcher form shell.
func FormPanel(props FormPanelProps) woxwidget.Widget {
	padding := props.Padding
	if padding == (woxwidget.Insets{}) {
		padding = woxwidget.Insets{Left: 14, Top: 10, Right: 14, Bottom: 10}
	}
	contentWidth := props.Width - padding.Left - padding.Right
	bodyMaximumHeight := max(float32(1), props.MaximumHeight-padding.Top-padding.Bottom-46)
	body := woxwidget.ScrollView{
		Key: "form-scroll", ID: "form-scroll", Width: contentWidth, MaxHeight: bodyMaximumHeight,
		KeepVisibleKey: props.KeepVisibleKey,
		Child:          woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), contentWidth-286), Height: 36},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-cancel", Label: props.CancelLabel, Width: 108, Height: 36, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-save", Label: props.SaveLabel, Width: 166, Height: 36, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSave, Theme: props.Theme}),
	}}
	return woxwidget.Container{
		Width: props.Width, Radius: 12, Color: props.Theme.ActionBackground,
		Padding: padding,
		Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{body, buttons}},
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
		height := props.Height
		if height <= 0 {
			height = 12
		}
		return woxwidget.Painter{Width: props.Width, Height: height}
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
	height := props.Height
	if height <= 0 {
		height = 34
	}
	return woxwidget.Container{Width: props.Width, Height: height, Padding: padding, Child: woxwidget.Text{Value: props.Value, Style: style, Color: color}}
}

// FormModelFieldProps contains one model selector row.
type FormModelFieldProps struct {
	ID          string
	Label       string
	Description string
	Value       string
	Width       float32
	Height      float32
	LabelWidth  float32
	Focused     bool
	Theme       woxcomponent.Theme
	OnTap       func(woxui.Rect)
}

// FormModelField builds the same compact outlined dropdown trigger used by Flutter.
func FormModelField(props FormModelFieldProps) woxwidget.Widget {
	controlWidth := formFieldControlWidth(props.Width, props.LabelWidth)
	control := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: props.ID, Label: props.Label, Value: props.Value, Width: controlWidth, Height: 34, Outline: formFieldOutline(props.Focused, props.Theme),
		Foreground: props.Theme.ActionText, Secondary: props.Theme.ActionHeader, Theme: props.Theme, OnTapBounds: props.OnTap,
	})
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, control, 34, props.Theme)
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
	ID             string
	Label          string
	Description    string
	Labels         []string
	Placeholder    string
	Status         string
	Width          float32
	Height         float32
	LabelWidth     float32
	SettingsLayout bool
	Recording      bool
	Error          bool
	Hold           bool
	HoldPrefix     string
	Window         *woxui.Window
	Theme          woxcomponent.Theme
	OnTap          func()
	OnFocusChange  func(bool)
}

// FormHotkeyField builds the shared recorder row for form and built-in settings layouts.
func FormHotkeyField(props FormHotkeyFieldProps) woxwidget.Widget {
	recorder, recorderWidth := woxcomponent.WoxHotkeyRecorder(woxcomponent.HotkeyRecorderProps{
		ID: props.ID, Labels: props.Labels, Placeholder: props.Placeholder, Focused: props.Recording, Error: props.Error, Hold: props.Hold, HoldPrefix: props.HoldPrefix,
		Window: props.Window, Theme: props.Theme, OnFocusChange: props.OnFocusChange,
	})
	recorder = woxwidget.Semantics{
		Key: woxwidget.Key(props.ID), AutomationID: props.ID, Role: woxui.AccessibilityRoleButton, Label: props.Label,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && props.OnTap != nil {
				props.OnTap()
			}
			return nil
		},
		Child: woxwidget.Gesture{ID: props.ID, OnTap: props.OnTap, Child: recorder},
	}
	controlWidth := formFieldControlWidth(props.Width, props.LabelWidth)
	statusGap := float32(12)
	controlChildren := []woxwidget.StackChild{{Top: 2, Child: recorder}}
	if props.SettingsLayout {
		const (
			gap       = float32(32)
			edgeInset = float32(2)
		)
		controlWidth = max(float32(0), props.Width-props.LabelWidth-gap)
		controlChildren[0] = woxwidget.StackChild{Top: 2, Right: edgeInset, AnchorRight: true, Child: recorder}
	}
	if props.Recording && props.Status != "" && controlWidth > recorderWidth+statusGap {
		statusColor := props.Theme.ResultSubtitle
		if props.Error {
			statusColor = props.Theme.ErrorText
		}
		if props.SettingsLayout {
			const labelGap = float32(32)
			recorderLeft := controlWidth - 2 - recorderWidth
			hintLeft := -props.LabelWidth - labelGap
			hintWidth := max(float32(0), recorderLeft-statusGap-hintLeft)
			// Flutter positions the hint from its actual render box. Clip Go's wider overflow area before the recorder so fallback glyphs cannot outpaint measured text bounds.
			controlChildren = append(controlChildren, woxwidget.StackChild{Left: hintLeft, Top: 6, Child: woxwidget.Clip{
				Width: hintWidth, Height: 22, Child: woxwidget.Align{Width: hintWidth, Height: 22, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{
					Value: props.Status, Style: woxui.TextStyle{Size: 12}, Color: statusColor,
				}},
			}})
		} else {
			controlChildren = append(controlChildren, woxwidget.StackChild{Left: recorderWidth + statusGap, Top: 8, Child: woxwidget.Text{
				Value: props.Status, Style: woxui.TextStyle{Size: 12}, Color: statusColor,
			}})
		}
	}
	control := woxwidget.Stack{Width: controlWidth, Height: 34, Children: controlChildren}
	if props.SettingsLayout {
		const gap = float32(32)
		return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
			Label: props.Label, Description: props.Description, Width: props.Width, Height: 62, LabelWidth: props.LabelWidth, Gap: gap,
			Padding: woxwidget.Insets{Left: 2, Top: 5, Right: 2, Bottom: 5},
			Child:   control, Theme: props.Theme,
		})
	}
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, control, 34, props.Theme)
}

// FormSwitchFieldProps contains one Flutter-style plugin boolean row.
type FormSwitchFieldProps struct {
	ID          string
	Label       string
	Description string
	Width       float32
	Height      float32
	LabelWidth  float32
	Checked     bool
	Theme       woxcomponent.Theme
	OnChange    func(bool)
}

// FormSwitchField builds a real switch instead of exposing the boolean as text.
func FormSwitchField(props FormSwitchFieldProps) woxwidget.Widget {
	control := woxcomponent.WoxSwitch(woxcomponent.SwitchProps{ID: props.ID, Label: props.Label, Value: props.Checked, OnChange: props.OnChange, Theme: props.Theme})
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, control, 22, props.Theme)
}

// FormSelectFieldProps contains one outlined form dropdown.
type FormSelectFieldProps struct {
	ID          string
	Label       string
	Description string
	Value       string
	Width       float32
	Height      float32
	LabelWidth  float32
	Focused     bool
	Theme       woxcomponent.Theme
	OnTap       func()
	OnChoiceTap func(woxui.Rect)
}

// FormSelectField builds an expanded dropdown with the same value and indicator split as Flutter.
func FormSelectField(props FormSelectFieldProps) woxwidget.Widget {
	controlWidth := formFieldControlWidth(props.Width, props.LabelWidth)
	control := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: props.ID, Label: props.Label, Value: props.Value, Width: controlWidth, Height: 34, Outline: formFieldOutline(props.Focused, props.Theme),
		Foreground: props.Theme.ActionText, Secondary: props.Theme.ActionHeader, Theme: props.Theme, OnTap: props.OnTap, OnTapBounds: props.OnChoiceTap,
	})
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, control, 34, props.Theme)
}

// FormAIModelFieldProps contains Flutter's two-part provider/model selector state.
type FormAIModelFieldProps struct {
	ID                 string
	Label              string
	Description        string
	Provider           string
	Model              string
	ModelNameHint      string
	ProviderIcon       *woxui.Image
	ModelIcon          *woxui.Image
	EditIcon           *woxui.Image
	ListIcon           *woxui.Image
	ModelsAvailable    bool
	Width              float32
	Height             float32
	LabelWidth         float32
	Focused            bool
	Window             *woxui.Window
	Theme              woxcomponent.Theme
	OnProviderTap      func(woxui.Rect)
	OnModelTap         func(woxui.Rect)
	OnModelNameChanged func(string)
	OnFinishEdit       func(string)
	OnEditModeChanged  func(bool)
}

// FormAIModelField retains only edit-mode and text interaction state; the committed model stays in the form.
func FormAIModelField(props FormAIModelFieldProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID + "-ai-model"), Type: (*formAIModelFieldState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &formAIModelFieldState{} },
	}
}

type formAIModelFieldState struct {
	editing    bool
	controller *woxwidget.TextEditingController
	focusNode  *woxwidget.FocusNode
}

func (s *formAIModelFieldState) InitState(_ woxwidget.StateContext, widget any) {
	props := widget.(FormAIModelFieldProps)
	s.controller = woxwidget.NewTextEditingController(props.Model)
	s.focusNode = woxwidget.NewFocusNode()
}

func (s *formAIModelFieldState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(FormAIModelFieldProps)
	props := newWidget.(FormAIModelFieldProps)
	if !s.editing && oldProps.Model != props.Model {
		s.controller.SetText(props.Model, false)
	}
}

func (s *formAIModelFieldState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(FormAIModelFieldProps)
	controlWidth := formFieldControlWidth(props.Width, props.LabelWidth)
	editWidth := float32(34)
	gap := float32(8)
	selectorsWidth := max(float32(0), controlWidth-editWidth-gap)
	providerWidth := max(float32(90), (selectorsWidth-gap)/3)
	modelWidth := max(float32(120), selectorsWidth-gap-providerWidth)
	outline := formFieldOutline(props.Focused, props.Theme)

	provider := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: props.ID + "-provider", Label: props.Label + " provider", Value: props.Provider, Leading: props.ProviderIcon,
		Width: providerWidth, Height: formAIModelControlHeight, Outline: outline, Foreground: props.Theme.ActionText, Secondary: props.Theme.ActionHeader,
		Theme: props.Theme, OnTapBounds: props.OnProviderTap,
	})
	var model woxwidget.Widget
	if s.editing {
		model = woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: props.ID + "-name", Label: props.Label, Hint: props.ModelNameHint, Width: modelWidth, Height: formAIModelControlHeight, Radius: 4,
			Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 9, Bottom: 6}, Transparent: true, BorderColor: outline, BorderWidth: 1,
			Style: woxui.TextStyle{Size: 13}, Controller: s.controller, FocusNode: s.focusNode, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
			OnChanged: props.OnModelNameChanged,
		})
	} else {
		model = woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
			ID: props.ID + "-model", Label: props.Label + " model", Value: props.Model, Leading: props.ModelIcon,
			Width: modelWidth, Height: formAIModelControlHeight, Outline: outline, Foreground: props.Theme.ActionText, Secondary: props.Theme.ActionHeader,
			Theme: props.Theme, OnTapBounds: props.OnModelTap,
		})
	}

	icon := props.EditIcon
	buttonLabel := "Edit model name"
	if s.editing {
		icon = props.ListIcon
		buttonLabel = "Choose a configured model"
	}
	toggleEditing := func() {
		if s.editing && props.OnFinishEdit != nil {
			props.OnFinishEdit(s.controller.Text())
		}
		if props.OnEditModeChanged != nil {
			props.OnEditModeChanged(!s.editing)
		}
		context.SetState(func() { s.editing = !s.editing })
	}
	hoverBackground := props.Theme.ResultSubtitle
	hoverBackground.A = 26
	toggle := woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
		ID: props.ID + "-edit", Label: buttonLabel, Icon: woxwidget.Image{Source: icon, Width: 18, Height: 18},
		Width: editWidth, Height: formAIModelControlHeight, Radius: 4, HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor,
		Disabled: !props.ModelsAvailable || props.Model == "", OnTap: toggleEditing,
	})
	control := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{provider, model, toggle}}
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, control, formAIModelControlHeight, props.Theme)
}

func (s *formAIModelFieldState) Dispose() {}

// FormTextFieldProps contains one editable form row.
type FormTextFieldProps struct {
	ID          string
	Label       string
	Description string
	Suffix      string
	Width       float32
	Height      float32
	LabelWidth  float32
	State       woxui.TextEditingState
	Focused     bool
	Protected   bool
	MaxLines    int
	Window      *woxui.Window
	Theme       woxcomponent.Theme
	OnFocus     func()
	OnChanged   func(string)
	OnKey       func(woxui.KeyEvent) bool
	OnBrowse    func()
}

// FormTextField builds a shared text input row with an optional directory picker.
func FormTextField(props FormTextFieldProps) woxwidget.Widget {
	fieldWidth := formFieldControlWidth(props.Width, props.LabelWidth)
	inputWidth := fieldWidth
	if props.OnBrowse != nil {
		inputWidth = max(float32(80), fieldWidth-92)
	}
	suffixWidth := float32(0)
	if props.Suffix != "" {
		suffixWidth = 28
		inputWidth = max(float32(60), inputWidth-suffixWidth)
	}
	fieldHeight := float32(34)
	if props.MaxLines > 1 {
		fieldHeight = 14 + float32(min(props.MaxLines, 8))*20
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: props.ID, Label: props.Label, Width: inputWidth, Height: fieldHeight, Radius: 4,
		Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 9, Bottom: 6}, Transparent: true,
		BorderColor: formFieldOutline(props.Focused, props.Theme), BorderWidth: 1,
		Style: woxui.TextStyle{Size: 13}, Value: props.State.Text, Focused: props.Focused, Protected: props.Protected,
		MaxLines: props.MaxLines, Window: props.Window, Theme: props.Theme, OnChanged: props.OnChanged, OnKey: props.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && props.OnFocus != nil {
				props.OnFocus()
			}
		},
	})
	var valueField woxwidget.Widget = input
	if props.Suffix != "" {
		valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			input,
			woxwidget.Align{Width: 20, Height: fieldHeight, Vertical: 0.5, Child: woxwidget.Text{Value: props.Suffix, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ActionText}},
		}}
	}
	if props.OnBrowse != nil {
		valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			input,
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: props.ID + "-browse", Label: "Browse", Width: 84, Height: fieldHeight, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnBrowse, Theme: props.Theme}),
		}}
	}
	return formFieldLayout(props.Label, props.Description, props.Width, props.Height, props.LabelWidth, valueField, fieldHeight, props.Theme)
}

func formFieldLayout(label, description string, width, height, labelWidth float32, control woxwidget.Widget, controlHeight float32, theme woxcomponent.Theme) woxwidget.Widget {
	if labelWidth <= 0 {
		labelWidth = 132
	}
	const gap = float32(12)
	controlWidth := max(float32(0), width-labelWidth-gap)
	rightChildren := []woxwidget.Widget{control}
	if description != "" {
		rightChildren = append(rightChildren, woxwidget.TextBlock{
			Value: description, Width: controlWidth, LineHeight: 18,
			Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle,
		})
	}
	padding := woxwidget.Insets{}
	if height <= 0 {
		padding.Bottom = 10
	}
	return woxwidget.Container{Width: width, Height: height, Padding: padding, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		formFieldLabel(label, labelWidth, controlHeight, 6, theme),
		woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: rightChildren},
	}}}
}

func formFieldControlWidth(width, labelWidth float32) float32 {
	if labelWidth <= 0 {
		labelWidth = 132
	}
	return max(float32(0), width-labelWidth-12)
}

func formFieldLabel(label string, width, height, top float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: top}, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: 13}, Color: theme.ActionText,
	}}
}

func formFieldOutline(focused bool, theme woxcomponent.Theme) woxui.Color {
	if focused {
		return settingsColorAlpha(theme.ActionText, 220)
	}
	return settingsColorAlpha(theme.ResultSubtitle, 190)
}

func formFieldBackground(focused bool, theme woxcomponent.Theme) woxui.Color {
	if focused {
		return theme.SelectedBackground
	}
	return theme.QueryBackground
}
