package component

import (
	"strings"
	"unicode"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SectionHeaderLead is the empty space settings pages leave above a section divider.
const SectionHeaderLead = float32(20)

// SectionHeaderProps describes a divider and label between settings groups.
type SectionHeaderProps struct {
	Label       string
	Width       float32
	Action      woxwidget.Widget
	ActionWidth float32
	Theme       ControlTheme
}

// SettingsChromeLabel returns the painted section or navigation group label.
// Cased scripts keep the established 11/uppercase treatment. Scripts without
// case, such as CJK, stay at 13 semibold so the label remains visible.
func SettingsChromeLabel(label string) (string, float32) {
	if settingsChromeLabelHasCase(label) {
		return strings.ToUpper(label), SettingsSectionTitleFontSize
	}
	return label, SettingsLabelFontSize
}

func settingsChromeLabelHasCase(label string) bool {
	for _, r := range label {
		if unicode.ToUpper(r) != unicode.ToLower(r) {
			return true
		}
	}
	return false
}

// WoxSectionHeader builds the shared settings section divider.
func WoxSectionHeader(props SectionHeaderProps) woxwidget.Widget {
	label, size := SettingsChromeLabel(props.Label)
	title := woxwidget.Align{Height: 42, Vertical: 0.5, Child: woxwidget.Text{
		Value: label, Style: woxui.TextStyle{Size: props.Theme.Scaled(size), Weight: woxui.FontWeightSemibold}, Color: props.Theme.TextSecondary,
	}}
	children := []woxwidget.Widget{woxwidget.Expanded{Child: title}}
	if props.Action != nil {
		action := props.Action
		if props.ActionWidth > 0 {
			action = woxwidget.Constrained{MinWidth: props.ActionWidth, MaxWidth: props.ActionWidth, FillWidth: true, Child: action}
		}
		children = append(children, action)
	}
	return woxwidget.Container{Width: props.Width, Height: 43, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.Width, Height: 1, Color: withAlpha(props.Theme.ChromeText, 26)},
		woxwidget.Container{Width: props.Width, Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}},
	}}}
}
