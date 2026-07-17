package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingsPageProps contains prepared settings rows and scroll geometry.
type SettingsPageProps struct {
	ID            string
	Width         float32
	Height        float32
	Children      []woxwidget.Widget
	ContentHeight float32
	Gap           float32
	Scroll        float32
	OnScroll      func(float32)
	OnSetGeometry func(float32, float32)
}

// SettingsPageContentWidth returns the content width inside the shared page insets.
func SettingsPageContentWidth(width float32) float32 {
	return max(float32(0), width-82)
}

// SettingsPage builds the common scrollable settings page.
func SettingsPage(props SettingsPageProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	viewportHeight := max(float32(1), props.Height-58)
	if props.OnSetGeometry != nil {
		props.OnSetGeometry(viewportHeight, props.ContentHeight)
	}
	id := props.ID
	if id == "" {
		id = "settings-page-scroll"
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 24}, Child: woxwidget.Gesture{ID: id, OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, props.ContentHeight), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: props.Gap, Children: props.Children},
	}}}
}

// SettingsMessage builds a neutral page-level loading or error message.
func SettingsMessage(value string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 24}, Child: woxwidget.TextBlock{
		Value: value, Width: width, Height: 80, Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: theme.ResultSubtitle,
	}}
}

// SettingsNote builds the compact note shown below a settings form.
func SettingsNote(value string, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 34, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{Value: value, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}}
}

// SettingRowProps contains one built-in setting and its editing actions.
type SettingRowProps struct {
	ID            string
	Title         string
	Description   string
	Value         string
	ValueTrailing string
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
	OnScroll      func(float32)
	OnCaret       func(int)
	OnBrowse      func()
}

func SettingChoiceAnchorKey(id string) woxwidget.Key {
	return woxwidget.Key("setting-choice-anchor-" + id)
}

// SettingRow builds a text, switch, or choice setting row.
func SettingRow(props SettingRowProps) woxwidget.Widget {
	fieldTheme := props.Theme
	subtitle := props.Theme.ResultSubtitle
	valueColor := props.Theme.Cursor
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
	if props.Kind == "bool" {
		valueWidth = 42
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
			ID: "setting-text-" + props.ID, Label: props.Title, Width: inputWidth, State: props.Editing, Focused: props.Focused,
			Window: props.Window, Theme: props.Theme, Disabled: props.Disabled, OnCaret: props.OnCaret,
		})
		valueField = input
		if props.BrowseFile {
			valueField = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{input, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "setting-browse-" + props.ID, Label: "Browse", Width: 74, Height: 38, Disabled: props.Disabled, Variant: woxcomponent.ButtonSurface, OnTap: props.OnBrowse, Theme: props.Theme,
			})}}
		}
	case "bool":
		valueField = woxwidget.Container{Width: valueWidth, Height: 44, Padding: woxwidget.Insets{Top: 10}, Child: woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
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
		const indicatorWidth = float32(24)
		contentWidth := max(float32(0), valueWidth-16-indicatorWidth)
		trailingWidth := float32(0)
		trailingGap := float32(0)
		if props.ValueTrailing != "" {
			trailingWidth = min(float32(68), max(float32(0), contentWidth-60))
			trailingGap = min(float32(10), max(float32(0), contentWidth-trailingWidth-60))
		}
		valueChildren := []woxwidget.Widget{
			woxwidget.Align{Width: max(float32(0), contentWidth-trailingWidth-trailingGap), Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: props.Value, Style: woxui.TextStyle{Size: 13}, Color: valueColor}},
		}
		if trailingGap > 0 {
			valueChildren = append(valueChildren, woxwidget.Container{Width: trailingGap, Height: 24})
		}
		if trailingWidth > 0 {
			valueChildren = append(valueChildren, woxwidget.Align{Width: trailingWidth, Height: 24, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{Value: props.ValueTrailing, Style: woxui.TextStyle{Size: 12}, Color: subtitle}})
		}
		valueChildren = append(valueChildren, dropdownIndicator(indicatorWidth, 24, valueColor))
		valueField = woxwidget.Gesture{ID: "setting-choice-" + props.ID, OnTap: onTap, OnTapBounds: onTapBounds, Child: woxwidget.Keyed{Key: SettingChoiceAnchorKey(props.ID), Child: woxwidget.Container{
			Width: valueWidth, Height: 34, Radius: 4, BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 140), BorderWidth: 1, Padding: woxwidget.Insets{Left: 8, Top: 5, Right: 8, Bottom: 5},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: valueChildren},
		}}}
	}
	row := woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: props.Title, Description: props.Description, Width: props.Width, Height: 62, LabelWidth: labelWidth, Gap: 28,
		Radius: 6, Background: props.Background, Padding: woxwidget.Insets{Left: 2, Top: 5, Right: 2, Bottom: 5}, Child: valueField, Theme: fieldTheme,
	})
	return woxwidget.Gesture{ID: "setting-" + props.ID, OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: row}
}
