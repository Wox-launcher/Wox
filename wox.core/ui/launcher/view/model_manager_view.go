package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const ModelManagerRowHeight = float32(82)

// ModelManagerOption contains prepared status and actions for one model.
type ModelManagerOption struct {
	Name          string
	Detail        string
	Status        string
	SelectedRow   bool
	PrimaryAction bool
	ActionLabel   string
	ActionEnabled bool
	OnAction      func()
	OnDelete      func()
	OnSelect      func()
}

// ModelManagerProps contains the model manager overlay state.
type ModelManagerProps struct {
	Width             float32
	Height            float32
	Theme             woxcomponent.Theme
	Title             string
	Loading           bool
	Busy              bool
	Error             string
	Scroll            float32
	EngineLabel       string
	EngineButtonLabel string
	EngineEnabled     bool
	Options           []ModelManagerOption
	OnEngine          func()
	OnRefresh         func()
	OnClose           func()
	OnScroll          func(float32)
	OnSetViewport     func(float32)
}

// ModelManagerView builds the modal engine and model manager.
func ModelManagerView(props ModelManagerProps) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(780), props.Width-28))
	panelHeight := max(float32(0), min(float32(660), props.Height-28))
	return modelManagerPanel(props, panelWidth, panelHeight)
}

func modelManagerPanel(props ModelManagerProps, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	headerHeight := float32(54)
	engineHeight := float32(72)
	footerHeight := float32(58)
	statusHeight := float32(28)
	viewportHeight := max(float32(82), height-headerHeight-engineHeight-footerHeight-statusHeight-32)
	if props.OnSetViewport != nil {
		props.OnSetViewport(viewportHeight)
	}
	header := woxwidget.Container{Width: innerWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
		woxwidget.Text{Value: "Core owns model files and downloads; this portable page owns selection and progress state.", Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ActionHeader},
	}}}
	engine := woxwidget.Container{Width: innerWidth, Height: engineHeight - 8, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(120), innerWidth-156), Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Runtime engine", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.TextBlock{Value: props.EngineLabel, Width: max(float32(100), innerWidth-156), Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
		}}},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "model-manager-engine", Label: props.EngineButtonLabel, Width: 132, Disabled: !props.EngineEnabled, OnTap: props.OnEngine, Theme: props.Theme}),
	}}}
	rows := make([]woxwidget.Widget, 0, len(props.Options))
	for index, option := range props.Options {
		index := index
		option := option
		background := props.Theme.QueryBackground
		foreground := props.Theme.ActionText
		if option.SelectedRow {
			background = props.Theme.SelectedBackground
			foreground = props.Theme.SelectedTitle
		}
		buttons := make([]woxwidget.Widget, 0, 2)
		deleteWidth := float32(0)
		if option.OnDelete != nil {
			deleteWidth = 76
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: fmt.Sprintf("model-delete-%d", index), Label: "Delete", Width: deleteWidth, Disabled: props.Busy || props.Loading, OnTap: option.OnDelete, Theme: props.Theme}))
		}
		variant := woxcomponent.ButtonSecondary
		if option.PrimaryAction {
			variant = woxcomponent.ButtonPrimary
		}
		buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: fmt.Sprintf("model-action-%d", index), Label: option.ActionLabel, Width: 96, Disabled: !option.ActionEnabled, Variant: variant, OnTap: option.OnAction, Theme: props.Theme}))
		buttonWidth := float32(96) + deleteWidth
		if deleteWidth > 0 {
			buttonWidth += 8
		}
		detailWidth := max(float32(120), innerWidth-buttonWidth-42)
		rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("model-row-%d", index), OnTap: option.OnSelect, Child: woxwidget.Container{
			Width: innerWidth, Height: ModelManagerRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Container{Width: detailWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: option.Name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
					woxwidget.TextBlock{Value: option.Detail, Width: detailWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
					woxwidget.TextBlock{Value: option.Status, Width: detailWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
				}}},
				woxwidget.Container{Width: buttonWidth, Height: 48, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
			}},
		}})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: innerWidth, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No model options were returned by the plugin.", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "model-manager-list", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: innerWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*ModelManagerRowHeight), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	status := props.Error
	if status == "" {
		if props.Loading {
			status = "Refreshing model and engine status…"
		} else {
			status = "↑↓ select · Enter download/select · Delete removes a dictation model · Ctrl+R refresh"
		}
	}
	statusColor := props.Theme.ActionHeader
	if props.Error != "" {
		statusColor = props.Theme.ErrorText
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-216), Height: 40},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "model-manager-refresh", Label: "Refresh", Width: 104, Disabled: props.Loading || props.Busy, OnTap: props.OnRefresh, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "model-manager-close", Label: "Close", Width: 104, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnClose, Theme: props.Theme}),
	}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "model-manager-dialog", Label: props.Title, Width: width, Height: height,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "model-manager-shade",
		Padding: woxwidget.UniformInsets(16), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			header,
			engine,
			list,
			woxwidget.Container{Width: innerWidth, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{Value: status, Width: innerWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
			woxwidget.Container{Width: innerWidth, Height: footerHeight, Padding: woxwidget.Insets{Top: 10}, Child: footer},
		}},
	})
}
