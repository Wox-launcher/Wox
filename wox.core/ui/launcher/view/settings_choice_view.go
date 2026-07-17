package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const settingsChoiceRowHeight = float32(46)

// SettingsChoice is one value displayed by the settings choice picker.
type SettingsChoice struct {
	Value string
	Label string
}

// SettingsChoiceProps contains the immutable state and actions rendered by the choice picker.
type SettingsChoiceProps struct {
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	Title         string
	CurrentValue  string
	Query         woxui.TextEditingState
	Choices       []SettingsChoice
	Selected      int
	Scroll        float32
	OnCaret       func(int)
	OnSetQuery    func(string) error
	OnKey         func(woxui.KeyEvent) bool
	OnTextInput   func(woxui.TextInputEvent) bool
	OnChoose      func(int)
	OnCancel      func()
	OnScroll      func(float32)
	OnSetViewport func(float32)
}

// SettingsChoiceView builds the modal settings choice picker.
func SettingsChoiceView(props SettingsChoiceProps) woxwidget.Widget {
	panelWidth := min(float32(620), props.Width-40)
	panelHeight := min(float32(650), props.Height-40)
	return settingsChoiceDialog(props, panelWidth, panelHeight)
}

func settingsChoiceDialog(props SettingsChoiceProps, width, height float32) woxwidget.Widget {
	innerWidth := width - 32
	headerHeight := float32(46)
	searchHeight := float32(48)
	footerHeight := float32(52)
	viewportHeight := max(float32(46), height-headerHeight-searchHeight-footerHeight-32)
	if props.OnSetViewport != nil {
		props.OnSetViewport(viewportHeight)
	}
	search := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: "setting-choice-search", Label: "Filter choices", Hint: "Filter choices…", Width: innerWidth, Height: 40,
		State: props.Query, Focused: true, Autofocus: true, MaxLines: 1, Window: props.Window, Theme: props.Theme,
		OnCaret: props.OnCaret, OnSetValue: props.OnSetQuery, OnKey: props.OnKey, OnTextInput: props.OnTextInput,
	})
	rows := make([]woxwidget.Widget, 0, len(props.Choices))
	for index, choice := range props.Choices {
		index := index
		foreground := props.Theme.ActionText
		if index == props.Selected {
			foreground = props.Theme.SelectedTitle
		}
		mark := ""
		if choice.Value == props.CurrentValue {
			mark = "  ✓"
		}
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: fmt.Sprintf("setting-choice-%d", index), Label: choice.Label, Width: innerWidth, Height: settingsChoiceRowHeight,
			Selected: index == props.Selected, Padding: woxwidget.Insets{Left: 14, Top: 14, Right: 12}, Theme: props.Theme,
			OnTap: func() {
				if props.OnChoose != nil {
					props.OnChoose(index)
				}
			},
			Child: woxwidget.Text{Value: choice.Label + mark, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
		}))
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: innerWidth, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No matching choices", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
		}}
	} else {
		list = woxwidget.Gesture{ID: "setting-choice-list", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*settingsChoiceRowHeight), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), innerWidth-112), Height: 40, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{
			Value: fmt.Sprintf("%d choices · type to filter · ↑↓ move · Enter select", len(props.Choices)), Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle,
		}},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "setting-choice-cancel", Label: "Cancel", Width: 104, OnTap: props.OnCancel, Theme: props.Theme}),
	}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "setting-choice-dialog", Label: props.Title, Width: width, Height: height,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "setting-choice-shade",
		Padding: woxwidget.UniformInsets(16), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}},
			woxwidget.Container{Width: innerWidth, Height: searchHeight, Child: search},
			list,
			woxwidget.Container{Width: innerWidth, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: footer},
		}},
	})
}
