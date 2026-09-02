package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingsPageProps contains prepared settings rows and scroll geometry.
type SettingsPageProps struct {
	ID             string
	Width          float32
	Height         float32
	Children       []woxwidget.Widget
	Gap            float32
	KeepVisible    *woxwidget.ScrollRange
	KeepVisibleKey woxwidget.Key
}

// SettingsPageContentWidth returns the content width inside the shared page insets.
func SettingsPageContentWidth(width float32) float32 {
	return max(float32(0), width-80)
}

// SettingsPage builds the common scrollable settings page.
func SettingsPage(props SettingsPageProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	viewportHeight := max(float32(1), props.Height-58)
	id := props.ID
	if id == "" {
		id = "settings-page-scroll"
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 40, Top: 34, Right: 40, Bottom: 24}, Child: woxwidget.ScrollView{
		Key: woxwidget.Key(id), ID: id, KeepVisible: props.KeepVisible, KeepVisibleKey: props.KeepVisibleKey,
		Width: contentWidth, Height: viewportHeight,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: props.Gap, Children: props.Children},
	}}
}

// SettingsMessage builds a neutral page-level loading or error message.
func SettingsMessage(value string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 24}, Child: woxwidget.TextBlock{
		Value: value, Width: width, Height: 80, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: theme.ResultSubtitle,
	}}
}

// SettingRowProps contains one built-in setting and its editing actions.
type SettingRowProps struct {
	ID            string
	Title         string
	Description   string
	Value         string
	ValueTrailing string
	ValueLeading  *woxui.Image
	Width         float32
	Background    woxui.Color
	Disabled      bool
	Kind          string
	ControlWidth  float32
	BrowseFile    bool
	Editing       woxui.TextEditingState
	Focused       bool
	Window        *woxui.Window
	Theme         woxcomponent.Theme
	OnTap         func()
	OnChoiceTap   func(woxui.Rect)
	OnFocus       func()
	OnChanged     func(string)
	OnKey         func(woxui.KeyEvent) bool
	OnBrowse      func()
}

func SettingChoiceAnchorKey(id string) woxwidget.Key {
	return woxwidget.Key("setting-choice-anchor-" + id)
}

// SettingRow builds a text, switch, or choice setting row.
func SettingRow(props SettingRowProps) woxwidget.Widget {
	fieldTheme := props.Theme
	valueColor := props.Theme.ResultTitle
	if props.Disabled {
		fieldTheme.ResultTitle = props.Theme.ResultSubtitle
		valueColor = props.Theme.ResultSubtitle
	}
	valueWidth := min(float32(280), max(float32(190), props.Width*0.32))
	if props.Kind == "text" {
		valueWidth = min(float32(440), max(float32(280), props.Width*0.46))
		if props.ControlWidth > 0 {
			valueWidth = props.ControlWidth
		}
	}
	labelWidth := max(float32(180), props.Width-valueWidth-32)
	var valueField woxwidget.Widget
	switch props.Kind {
	case "text":
		inputWidth := valueWidth
		if props.BrowseFile {
			inputWidth = max(float32(120), valueWidth-82)
		}
		input := woxcomponent.WoxSettingTextField(woxcomponent.TextFieldProps{
			ID: "setting-text-" + props.ID, Label: props.Title, Width: inputWidth, Value: props.Editing.Text, Focused: props.Focused,
			Window: props.Window, Theme: props.Theme, Disabled: props.Disabled, OnChanged: props.OnChanged, OnKey: props.OnKey,
			OnFocusChange: func(focused bool) {
				if focused && props.OnFocus != nil {
					props.OnFocus()
				}
			},
		})
		valueField = input
		if props.BrowseFile {
			valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{input, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "setting-browse-" + props.ID, Label: "Browse", Disabled: props.Disabled, Variant: woxcomponent.ButtonSurface, OnTap: props.OnBrowse, Theme: props.Theme,
			})}}
		}
	case "bool":
		valueField = woxwidget.Align{Width: valueWidth, Height: woxcomponent.SettingsControlHeight, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
			ID: "setting-switch-" + props.ID, Label: props.Title, Value: props.Value == "true", Disabled: props.Disabled, Theme: props.Theme,
			OnChange: func(bool) {
				if props.OnTap != nil {
					props.OnTap()
				}
			},
		})}
	default:
		onTap := props.OnTap
		onTapBounds := props.OnChoiceTap
		if onTapBounds != nil {
			onTap = nil
		}
		if props.Disabled {
			onTap = nil
			onTapBounds = nil
		}
		valueField = woxwidget.Keyed{Key: SettingChoiceAnchorKey(props.ID), Child: woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
			ID: "setting-choice-" + props.ID, Label: props.Title, Value: props.Value, Trailing: props.ValueTrailing, Leading: props.ValueLeading,
			Width: valueWidth, Height: woxcomponent.SettingsControlHeight,
			Foreground: valueColor, Secondary: valueColor, Theme: props.Theme, OnTap: onTap, OnTapBounds: onTapBounds,
		})}
	}
	return woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Title, Description: props.Description, Width: props.Width, Height: woxcomponent.SettingsRowHeight, LabelWidth: labelWidth, Gap: 28,
		Radius: 6, Background: props.Background, Padding: woxwidget.Insets{Left: 2, Right: 2, Bottom: 5}, Child: valueField, Theme: fieldTheme,
	})
}
