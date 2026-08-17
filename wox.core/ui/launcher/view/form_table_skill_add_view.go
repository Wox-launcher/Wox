package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// FormTableSkillAddDialogProps contains the Flutter-aligned local/remote add-skill surface.
type FormTableSkillAddDialogProps struct {
	Width        float32
	Height       float32
	Title        string
	LocalLabel   string
	RemoteLabel  string
	LocalHint    string
	RemoteHint   string
	Tab          int
	Error        string
	Cloning      bool
	CloningLabel string
	CancelLabel  string
	AddLabel     string
	CancelWidth  float32
	AddWidth     float32
	Field        woxwidget.Widget
	FieldHeight  float32
	Theme        woxcomponent.Theme
	OnTab        func(int)
	OnCancel     func()
	OnAdd        func()
}

// FormTableSkillAddDialog builds the modal dialog that adds a local directory or a
// remote Git repository, mirroring Flutter's custom create dialog for AI skills.
func FormTableSkillAddDialog(props FormTableSkillAddDialogProps) woxwidget.Widget {
	panelWidth := min(float32(560), max(float32(0), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	statusHeight := float32(0)
	if props.Error != "" || props.Cloning {
		statusHeight = 28
	}
	hintHeight := float32(40)
	actionsHeight := SettingsDialogActionsHeight
	childCount := 5
	if statusHeight > 0 {
		childCount++
	}
	contentHeight := 28 + 32 + hintHeight + props.FieldHeight + statusHeight + actionsHeight + float32(childCount-1)*12
	panelHeight := max(float32(260), min(float32(360), contentHeight+48))
	panelHeight = max(float32(0), min(panelHeight, props.Height-56))

	title := woxwidget.Container{Width: innerWidth, Height: 28, Child: woxwidget.Text{
		Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
	}}

	tabs := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		formTableSkillAddTab(0, props.LocalLabel, props.Tab == 0, props.Theme, func() {
			if props.OnTab != nil {
				props.OnTab(0)
			}
		}),
		formTableSkillAddTab(1, props.RemoteLabel, props.Tab == 1, props.Theme, func() {
			if props.OnTab != nil {
				props.OnTab(1)
			}
		}),
	}}

	hint := props.LocalHint
	if props.Tab == 1 {
		hint = props.RemoteHint
	}
	hintText := woxwidget.TextBlock{Value: hint, Width: innerWidth, Height: hintHeight, MaxLines: 2, LineHeight: 19,
		Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle}

	children := []woxwidget.Widget{title, tabs, hintText, props.Field}
	if statusHeight > 0 {
		var status woxwidget.Widget
		if props.Cloning {
			status = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxcomponent.WoxLoadingIndicator(14, props.Theme.ResultSubtitle),
				woxwidget.Text{Value: props.CloningLabel, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
			}}
		} else {
			status = woxwidget.TextBlock{Value: props.Error, Width: innerWidth, Height: 22, MaxLines: 1,
				Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ErrorText}
		}
		children = append(children, status)
	}

	children = append(children, settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "form-table-skill-add-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-skill-add-confirm", Label: props.AddLabel, Disabled: props.Cloning, OnTap: props.OnAdd},
	))

	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}
	border := formTableAlpha(props.Theme.ResultSubtitle, 104)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "form-table-skill-add-dialog", Label: props.Title, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-skill-add-backdrop", BackdropAlpha: 210,
		Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: border, BorderWidth: 1,
		InitialFocus: woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", props.Tab)), OnEscape: props.OnCancel, Theme: props.Theme,
		Child: body,
	})
}

// formTableSkillAddTab mirrors Flutter's _buildAddSkillTab pill selector.
func formTableSkillAddTab(index int, label string, selected bool, theme woxcomponent.Theme, onTap func()) woxwidget.Widget {
	background := woxui.Color{}
	borderColor := formTableAlpha(theme.ResultSubtitle, 60)
	foreground := theme.ResultSubtitle
	weight := woxui.FontWeightRegular
	if selected {
		background = theme.ActionSelected
		background.A = 24
		borderColor = formTableAlpha(theme.ActionText, 90)
		foreground = theme.ActionText
		weight = woxui.FontWeightSemibold
	}
	return woxwidget.Gesture{ID: fmt.Sprintf("form-table-skill-add-tab-%d", index), OnTap: onTap, Child: woxwidget.Container{
		Width: 132, Height: 32, Radius: 6, Color: background, Padding: woxwidget.UniformInsets(6),
		BorderColor: borderColor, BorderWidth: 1, Child: woxwidget.Align{Width: 120, Height: 20, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: label, Style: woxui.TextStyle{Size: 13, Weight: weight}, Color: foreground,
		}},
	}}
}
