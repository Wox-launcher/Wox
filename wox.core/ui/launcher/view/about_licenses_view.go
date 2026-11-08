package view

import (
	"strconv"
	"strings"

	"wox/resource/licenses"
	"wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	"wox/ui/widget"
)

// AboutLicensesProps describes the list or the selected read-only license.
type AboutLicensesProps struct {
	Width, Height      float32
	Theme              component.ControlTheme
	Window             *woxui.Window
	Title, Back, Empty string
	Entries            []licenses.Entry
	Selected           *licenses.Entry
	OnBack             func()
	OnSelect           func(licenses.Entry)
}

// AboutLicensesView gives license text its own scrollable viewport within About.
func AboutLicensesView(props AboutLicensesProps) widget.Widget {
	theme := props.Theme
	scale := theme.Scaled
	inset, gap := scale(24), scale(12)
	width := max(float32(0), props.Width-2*inset)
	buttonHeight, titleHeight := scale(32), scale(30)
	bodyHeight := max(float32(0), props.Height-2*inset-buttonHeight-titleHeight-2*gap)
	title := props.Title
	var body widget.Widget
	if entry := props.Selected; entry != nil {
		title = entry.Name
		if entry.Version != "" {
			title += " " + entry.Version
		}
		value := strings.ReplaceAll(entry.Text, "\r\n", "\n")
		body = component.WoxTextField(component.TextFieldProps{
			ID: "about-license-text", Label: entry.Name, Width: width, Height: bodyHeight,
			Value: value, ReadOnly: true, MaxLines: max(2, len(value)),
			Transparent: true, DisableHover: true, Padding: widget.Insets{Right: scale(12)},
			Style: woxui.TextStyle{Size: scale(component.SettingsLabelFontSize)}, LineHeight: scale(20),
			TextColor: theme.Text, Window: props.Window, Theme: theme,
		})
	} else {
		body = aboutLicenseList(props, width, bodyHeight)
	}
	return widget.Container{
		Width: props.Width, Height: props.Height,
		Padding: widget.Insets{Left: inset, Top: inset, Right: inset, Bottom: inset},
		Child: widget.Flex{Axis: widget.Vertical, Gap: gap, Children: []widget.Widget{
			component.WoxButton(component.ButtonProps{
				ID: "about-licenses-back", Label: props.Back, Height: buttonHeight,
				Variant: component.ButtonSecondary, OnTap: props.OnBack, Theme: theme,
			}),
			widget.Align{Width: width, Height: titleHeight, Vertical: 0.5, Child: widget.Text{
				Value: title, Style: woxui.TextStyle{Size: scale(component.SettingsPageTitleFontSize), Weight: woxui.FontWeightSemibold},
				Color: theme.Text,
			}},
			body,
		}},
	}
}

func aboutLicenseList(props AboutLicensesProps, width, height float32) widget.Widget {
	theme := props.Theme
	if len(props.Entries) == 0 {
		return widget.Text{Value: props.Empty, Style: woxui.TextStyle{Size: theme.Scaled(component.SettingsHelpFontSize)}, Color: theme.TextSecondary}
	}
	rows := make([]widget.Widget, 0, len(props.Entries))
	for i, entry := range props.Entries {
		rows = append(rows, component.WoxButton(component.ButtonProps{
			ID: "about-license-" + strconv.Itoa(i), Label: entry.Name, FontSize: component.SettingsControlFontSize,
			Width: width, Height: theme.Scaled(32), AlignLeading: true, Variant: component.ButtonText,
			OnTap: func() { props.OnSelect(entry) }, Theme: theme,
		}))
	}
	return widget.ScrollView{
		Key: "about-licenses-list", ID: "about-licenses-list", Width: width, Height: height,
		Child: widget.Flex{Axis: widget.Vertical, Children: rows},
	}
}
