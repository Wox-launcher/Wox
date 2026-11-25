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
	Theme        woxcomponent.ControlTheme
	OnTab        func(int)
	OnCancel     func()
	OnAdd        func()
}

// FormTableSkillAddDialog builds the modal dialog that adds a local directory or a
// remote Git repository, mirroring Flutter's custom create dialog for AI skills.
func FormTableSkillAddDialog(props FormTableSkillAddDialogProps) woxwidget.Widget {
	panelWidth := min(float32(560), max(float32(0), props.Width-56))
	innerWidth := max(float32(0), panelWidth-48)
	title := woxwidget.TextBlock{Value: props.Title, MinLineHeight: props.Theme.Scaled(28), MaxLines: 1,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(18), Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text}

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
	hintText := woxwidget.TextBlock{Value: hint, MinLineHeight: props.Theme.Scaled(19), MaxLines: 2,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.TextSecondary}

	children := []woxwidget.Widget{title, tabs, hintText, props.Field}
	statusLabel := woxwidget.TextBlock{Value: props.Error, MinLineHeight: props.Theme.Scaled(20), MaxLines: 2,
		Style: woxui.TextStyle{Size: props.Theme.Scaled(13)}, Color: props.Theme.Error}
	if props.Cloning {
		statusLabel.Value, statusLabel.Color = props.CloningLabel, props.Theme.TextSecondary
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxcomponent.WoxLoadingIndicator(14, props.Theme.TextSecondary), woxwidget.Expanded{Child: statusLabel},
		}})
	} else if props.Error != "" {
		children = append(children, statusLabel)
	}

	actions := settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "form-table-skill-add-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "form-table-skill-add-confirm", Label: props.AddLabel, Disabled: props.Cloning, OnTap: props.OnAdd},
	)

	body := woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children}
	return woxwidget.TextLayoutBuilder{Build: func(_ woxui.Size, measure woxwidget.TextMeasurer) woxwidget.Widget {
		bodyHeight := props.FieldHeight + float32(len(children)-1)*body.Gap
		for _, child := range children[:3] {
			bodyHeight += woxwidget.MeasureStateless(measure, child, innerWidth).Height
		}
		if props.Cloning {
			bodyHeight += max(14, woxwidget.MeasureStateless(measure, statusLabel, max(1, innerWidth-22)).Height)
		} else if props.Error != "" {
			bodyHeight += woxwidget.MeasureStateless(measure, statusLabel, innerWidth).Height
		}
		return settingsScrollableDialog(woxcomponent.DialogProps{
			ID: "form-table-skill-add-dialog", Label: props.Title, Width: panelWidth,
			OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "form-table-skill-add-backdrop", BackdropAlpha: 210,
			Radius: 20, Padding: woxwidget.UniformInsets(24), BorderColor: formTableAlpha(props.Theme.TextSecondary, 104), BorderWidth: 1,
			InitialFocus: woxwidget.Key(fmt.Sprintf("form-table-row-field-%d", props.Tab)), OnEscape: props.OnCancel, Theme: props.Theme,
		}, body, actions, bodyHeight, 260, 360)
	}}
}

// formTableSkillAddTab mirrors Flutter's _buildAddSkillTab pill selector.
func formTableSkillAddTab(index int, label string, selected bool, theme woxcomponent.ControlTheme, onTap func()) woxwidget.Widget {
	background := woxui.Color{}
	borderColor := formTableAlpha(theme.TextSecondary, 60)
	foreground := theme.TextSecondary
	weight := woxui.FontWeightRegular
	if selected {
		background = theme.Accent
		background.A = 24
		borderColor = formTableAlpha(theme.Text, 90)
		foreground = theme.Text
		weight = woxui.FontWeightSemibold
	}
	return woxwidget.Gesture{ID: fmt.Sprintf("form-table-skill-add-tab-%d", index), OnTap: onTap, Child: woxwidget.Container{
		Width: 132, Radius: 6, Color: background, Padding: woxwidget.UniformInsets(6),
		BorderColor: borderColor, BorderWidth: 1, Child: woxwidget.TextBlock{
			Value: label, MinLineHeight: theme.Scaled(20), MaxLines: 1, Centered: true, AlignmentY: 0.5,
			Style: woxui.TextStyle{Size: theme.Scaled(13), Weight: weight}, Color: foreground,
		},
	}}
}
