package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formTableListRowHeight = float32(48)

// FormTableOverlayProps contains the prepared body rendered by the shared table editor.
type FormTableOverlayProps struct {
	Width    float32
	Height   float32
	Title    string
	Subtitle string
	Body     woxwidget.Widget
	Theme    woxcomponent.Theme
}

// FormTableOverlay builds the modal table editor shell.
func FormTableOverlay(props FormTableOverlayProps) woxwidget.Widget {
	panelWidth := max(float32(0), min(float32(760), props.Width-28))
	panelHeight := max(float32(0), min(float32(640), props.Height-28))
	innerWidth := max(float32(0), panelWidth-32)
	header := woxwidget.Container{Width: innerWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
		woxwidget.Text{Value: props.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ActionHeader},
	}}}
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-modal-shade", BackdropAlpha: 205,
		Padding: woxwidget.UniformInsets(16), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{header, props.Body}},
	})
}

// FormTableListProps contains the prepared rows and actions rendered by a table editor.
type FormTableListProps struct {
	Width         float32
	Height        float32
	Rows          []string
	Selected      int
	Scroll        float32
	Status        string
	StatusError   bool
	AddLabel      string
	DeleteLabel   string
	CloseLabel    string
	CanAdd        bool
	CanEdit       bool
	CanDelete     bool
	ShowClone     bool
	Theme         woxcomponent.Theme
	OnSetViewport func(float32)
	OnScroll      func(float32)
	OnSelect      func(int)
	OnAdd         func()
	OnEdit        func()
	OnDelete      func()
	OnClone       func()
	OnClose       func()
}

// FormTableList builds the row list and editor actions.
func FormTableList(props FormTableListProps) woxwidget.Widget {
	footerHeight := float32(54)
	statusHeight := float32(28)
	viewportHeight := max(float32(48), props.Height-footerHeight-statusHeight)
	if props.OnSetViewport != nil {
		props.OnSetViewport(viewportHeight)
	}
	rows := make([]woxwidget.Widget, 0, len(props.Rows))
	for index, value := range props.Rows {
		index := index
		background := props.Theme.QueryBackground
		foreground := props.Theme.ActionText
		if index == props.Selected {
			background = props.Theme.SelectedBackground
			foreground = props.Theme.SelectedTitle
		}
		rows = append(rows, woxwidget.Gesture{
			ID: fmt.Sprintf("form-table-row-%d", index),
			OnTap: func() {
				if props.OnSelect != nil {
					props.OnSelect(index)
				}
			},
			Child: woxwidget.Container{Width: props.Width, Height: formTableListRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 12, Top: 15, Right: 10}, Child: woxwidget.Text{
				Value: value, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: foreground,
			}},
		})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: props.Width, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No rows yet. Choose Add row to create one.", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "form-table-list-scroll", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: props.Width, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*formTableListRowHeight), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	status := props.Status
	if status == "" {
		status = "↑↓ select · Enter edit · Delete remove · Ctrl+N add · Esc close"
	}
	statusColor := props.Theme.ActionHeader
	if props.StatusError {
		statusColor = props.Theme.ErrorText
	}
	leftButtons := []woxwidget.Widget{
		formTableButton("form-table-add", props.AddLabel, 104, props.CanAdd, false, props.OnAdd, props.Theme),
		formTableButton("form-table-edit", "Edit", 86, props.CanEdit, false, props.OnEdit, props.Theme),
		formTableButton("form-table-delete", props.DeleteLabel, 86, props.CanDelete, false, props.OnDelete, props.Theme),
	}
	fixedWidth := float32(104 + 86 + 86 + 104)
	if props.ShowClone {
		leftButtons = append(leftButtons, formTableButton("form-table-clone", "Clone remote", 112, props.CanAdd, false, props.OnClone, props.Theme))
		fixedWidth += 112
	}
	buttonChildren := append([]woxwidget.Widget(nil), leftButtons...)
	buttonChildren = append(buttonChildren, woxwidget.Painter{Width: max(float32(0), props.Width-fixedWidth-float32(len(leftButtons)+1)*8), Height: 38})
	buttonChildren = append(buttonChildren, formTableButton("form-table-close", props.CloseLabel, 104, true, true, props.OnClose, props.Theme))
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		list,
		woxwidget.Container{Width: props.Width, Height: statusHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 10}, Color: statusColor}},
		woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttonChildren}},
	}}
}

// FormTableRowEditorProps contains a prepared table row form.
type FormTableRowEditorProps struct {
	Width         float32
	Height        float32
	Title         string
	Rows          []woxwidget.Widget
	ContentHeight float32
	Scroll        float32
	Status        string
	CancelLabel   string
	SaveLabel     string
	Theme         woxcomponent.Theme
	OnSetViewport func(float32)
	OnScroll      func(float32)
	OnCancel      func()
	OnSave        func()
}

// FormTableRowEditor builds the add, edit, or clone row form.
func FormTableRowEditor(props FormTableRowEditorProps) woxwidget.Widget {
	footerHeight := float32(54)
	titleHeight := float32(32)
	statusHeight := float32(0)
	if props.Status != "" {
		statusHeight = 28
	}
	bodyHeight := max(float32(48), props.Height-titleHeight-footerHeight-statusHeight)
	if props.OnSetViewport != nil {
		props.OnSetViewport(bodyHeight)
	}
	body := woxwidget.Gesture{ID: "form-table-row-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: props.Width, Height: bodyHeight, ContentHeight: max(bodyHeight, props.ContentHeight), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
	}}
	children := []woxwidget.Widget{
		woxwidget.Container{Width: props.Width, Height: titleHeight, Child: woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}},
		body,
	}
	if statusHeight > 0 {
		children = append(children, woxwidget.Container{Width: props.Width, Height: statusHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: props.Status, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ErrorText,
		}})
	}
	buttons := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), props.Width-210), Height: 38},
		formTableButton("form-table-row-cancel", props.CancelLabel, 96, true, false, props.OnCancel, props.Theme),
		formTableButton("form-table-row-save", props.SaveLabel, 104, true, true, props.OnSave, props.Theme),
	}}
	children = append(children, woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: buttons})
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}
}

func formTableButton(id, label string, width float32, enabled, primary bool, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	variant := woxcomponent.ButtonSecondary
	if primary {
		variant = woxcomponent.ButtonPrimary
	}
	return woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: id, Label: label, Width: width, Disabled: !enabled, Variant: variant, OnTap: onTap, Theme: theme})
}
