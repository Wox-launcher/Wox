package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// RuntimeSettingsLabels contains final user-facing copy for the runtime page.
type RuntimeSettingsLabels struct {
	Title             string
	Description       string
	StatusSection     string
	ExecutableSection string
	Browse            string
	Clear             string
	Loading           string
	Empty             string
}

// RuntimeStatus contains prepared runtime host presentation data.
type RuntimeStatus struct {
	Runtime      string
	DisplayName  string
	Mark         string
	Icon         *woxui.Image
	Version      string
	StatusCode   string
	StatusLabel  string
	Detail       string
	PluginLabel  string
	Actionable   bool
	InstallLabel string
	InstallIcon  *woxui.Image
	RestartLabel string
	RestartIcon  *woxui.Image
	OnInstall    func()
	OnRestart    func()
}

// RuntimeSettingRow contains one executable path editor and its actions.
type RuntimeSettingRow struct {
	ID          string
	Title       string
	Description string
	Placeholder string
	State       woxui.TextEditingState
	Focused     bool
	Disabled    bool
	Window      *woxui.Window
	OnHover     func()
	OnFocus     func()
	OnChanged   func(string)
	OnKey       func(woxui.KeyEvent) bool
	OnBrowse    func()
	OnClear     func()
}

// RuntimeSettingsProps contains runtime inventory and executable settings.
type RuntimeSettingsProps struct {
	Width            float32
	Height           float32
	SettingRowHeight float32
	Theme            woxcomponent.Theme
	Labels           RuntimeSettingsLabels
	Loading          bool
	Restarting       bool
	Error            string
	Note             string
	Selected         int
	Statuses         []RuntimeStatus
	Settings         []RuntimeSettingRow
}

// RuntimeSettingsView mirrors Flutter's full-width runtime summary and executable path form.
func RuntimeSettingsView(props RuntimeSettingsProps) woxwidget.Widget {
	return buildRuntimeSettingsView(props)
}

// buildRuntimeSettingsView composes immutable runtime data beneath the retained SettingsPage scroll owner.
func buildRuntimeSettingsView(props RuntimeSettingsProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	settingRowHeight := props.SettingRowHeight
	if settingRowHeight <= 0 {
		settingRowHeight = 72
	}
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{Title: props.Labels.Title, Description: props.Labels.Description, Width: contentWidth, Theme: props.Theme}),
		woxwidget.Container{Width: contentWidth, Height: 32, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Text{
			Value: props.Labels.StatusSection, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle,
		}},
	}
	messageHeight := float32(0)
	if props.Loading || props.Error != "" {
		messageHeight = 24
		message := props.Labels.Loading
		color := props.Theme.ResultSubtitle
		if props.Error != "" {
			message = props.Error
			color = props.Theme.ErrorText
		}
		children = append(children, woxwidget.Container{Width: contentWidth, Height: messageHeight, Padding: woxwidget.Insets{Bottom: 6}, Child: woxwidget.Text{
			Value: message, Style: woxui.TextStyle{Size: 11}, Color: color,
		}})
	}
	statusHeight := runtimeStatusGridHeight(props.Statuses, contentWidth)
	children = append(children,
		runtimeStatusGrid(props, contentWidth, statusHeight),
		woxwidget.Container{Width: contentWidth, Height: 20},
		woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Labels.ExecutableSection, Width: contentWidth, Theme: props.Theme}),
	)
	rowsTop := woxcomponent.PageHeaderHeight + 32 + messageHeight + statusHeight + 20 + 43
	for _, row := range props.Settings {
		children = append(children, runtimeExecutableSettingRow(props, row, contentWidth, settingRowHeight))
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 16})
	contentHeight := rowsTop + float32(len(props.Settings))*settingRowHeight + 16
	if props.Note != "" {
		children = append(children, SettingsNote(props.Note, contentWidth, props.Theme))
		contentHeight += 34
	}
	var keepVisible *woxwidget.ScrollRange
	if props.Selected >= 0 && props.Selected < len(props.Settings) {
		top := rowsTop + float32(props.Selected)*settingRowHeight
		keepVisible = &woxwidget.ScrollRange{Start: top, End: top + settingRowHeight}
	}
	return SettingsPage(SettingsPageProps{
		ID: "runtime-page-scroll", Width: props.Width, Height: props.Height, Children: children, ContentHeight: contentHeight, KeepVisible: keepVisible,
	})
}

// runtimeStatusGridHeight keeps every card in one responsive row aligned to the tallest status.
func runtimeStatusGridHeight(statuses []RuntimeStatus, width float32) float32 {
	if len(statuses) == 0 {
		return 36
	}
	columns := runtimeStatusColumns(width)
	height := float32(0)
	for start := 0; start < len(statuses); start += columns {
		rowHeight := float32(168)
		for index := start; index < min(start+columns, len(statuses)); index++ {
			if statuses[index].Actionable {
				rowHeight = 224
			}
		}
		height += rowHeight
		if start+columns < len(statuses) {
			height += 12
		}
	}
	return height
}

// runtimeStatusColumns matches Flutter's one, two, and three-column breakpoints.
func runtimeStatusColumns(width float32) int {
	if width >= 860 {
		return 3
	}
	if width >= 560 {
		return 2
	}
	return 1
}

// runtimeStatusGrid builds the full-width status summary or its empty state.
func runtimeStatusGrid(props RuntimeSettingsProps, width, height float32) woxwidget.Widget {
	if len(props.Statuses) == 0 {
		message := props.Labels.Empty
		if props.Loading {
			message = ""
		}
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{
			Value: message, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
		}}
	}
	columns := runtimeStatusColumns(width)
	cardWidth := (width - float32(columns-1)*12) / float32(columns)
	rows := make([]woxwidget.Widget, 0, (len(props.Statuses)+columns-1)/columns)
	for start := 0; start < len(props.Statuses); start += columns {
		end := min(start+columns, len(props.Statuses))
		rowHeight := float32(168)
		for _, status := range props.Statuses[start:end] {
			if status.Actionable {
				rowHeight = 224
			}
		}
		cards := make([]woxwidget.Widget, 0, end-start)
		for _, status := range props.Statuses[start:end] {
			cards = append(cards, runtimeStatusCard(props, status, cardWidth, rowHeight))
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: cards})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: rows}}
}

// runtimeStatusCard preserves Flutter's reserved detail and plugin-count alignment.
func runtimeStatusCard(props RuntimeSettingsProps, status RuntimeStatus, width, height float32) woxwidget.Widget {
	theme := props.Theme
	statusColor := runtimeStatusColor(status.StatusCode, theme)
	innerWidth := max(float32(0), width-28)
	titleWidth := max(float32(60), innerWidth-46)
	var icon woxwidget.Widget = woxwidget.Text{Value: status.Mark, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}
	if status.Icon != nil {
		icon = woxwidget.Image{Source: status.Icon, Width: 22, Height: 22}
	}
	pillWidth := runtimeLabelWidth(status.StatusLabel, 40, 150)
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 34, Height: 34, Radius: 8, Color: runtimeWithAlpha(theme.ResultTitle, 26), Child: woxwidget.Align{
			Width: 34, Height: 34, Horizontal: 0.5, Vertical: 0.5, Child: icon,
		}},
		woxwidget.Container{Width: titleWidth, Height: 48, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(20), titleWidth-62), Height: 20, Child: woxwidget.Text{Value: status.DisplayName, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
				woxwidget.Container{Width: 62, Height: 20, Child: woxwidget.Text{Value: status.Version, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle}},
			}},
			woxwidget.Container{Width: pillWidth, Height: 22, Radius: 11, Color: runtimeStatusBackground(status.StatusCode, theme), Padding: woxwidget.Insets{Left: 8, Top: 4}, Child: woxwidget.Text{
				Value: status.StatusLabel, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: statusColor,
			}},
		}}},
	}}
	children := []woxwidget.Widget{
		header,
		woxwidget.Container{Width: innerWidth, Height: 12},
		woxwidget.Container{Width: innerWidth, Height: 40, Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.TextBlock{
			Value: status.Detail, Width: max(float32(0), innerWidth-46), Height: 40, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 17, Color: theme.ResultSubtitle,
		}},
		woxwidget.Container{Width: innerWidth, Height: 14},
		woxwidget.Container{Width: innerWidth, Height: 18, Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.Text{
			Value: status.PluginLabel, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultSubtitle,
		}},
	}
	if status.Actionable {
		buttons := make([]woxwidget.Widget, 0, 2)
		if status.OnInstall != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "runtime-install-" + status.Runtime, Label: status.InstallLabel, Icon: status.InstallIcon, IconSize: 14,
				Width: runtimeLabelWidth(status.InstallLabel, 82, 132), Height: 38, Radius: 4, Disabled: props.Restarting, Variant: woxcomponent.ButtonOutline, OnTap: status.OnInstall, Theme: theme,
			}))
		}
		if status.OnRestart != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "runtime-restart-" + status.Runtime, Label: status.RestartLabel, Icon: status.RestartIcon, IconSize: 14,
				Width: runtimeLabelWidth(status.RestartLabel, 92, 132), Height: 38, Radius: 4, Disabled: props.Restarting, Variant: woxcomponent.ButtonOutline, OnTap: status.OnRestart, Theme: theme,
			}))
		}
		children = append(children,
			woxwidget.Container{Width: innerWidth, Height: 10},
			woxwidget.Container{Width: innerWidth, Height: 38, Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
		)
	}
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: height, Padding: woxwidget.UniformInsets(14), BorderColor: runtimeOutlineColor(theme), Theme: theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	})
}

// runtimeExecutableSettingRow renders one label, path input, browse action, and clear action.
func runtimeExecutableSettingRow(props RuntimeSettingsProps, row RuntimeSettingRow, width, height float32) woxwidget.Widget {
	labelWidth := min(float32(400), max(float32(220), width*0.48))
	controlWidth := max(float32(220), width-labelWidth-32)
	browseWidth := runtimeLabelWidth(props.Labels.Browse, 62, 96)
	clearWidth := runtimeLabelWidth(props.Labels.Clear, 62, 96)
	inputWidth := max(float32(80), controlWidth-browseWidth-clearWidth-20)
	borderColor := runtimeWithAlpha(props.Theme.ResultSubtitle, 164)
	if row.Focused {
		borderColor = props.Theme.Cursor
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: row.ID + "-input", Label: row.Title, Hint: row.Placeholder, Width: inputWidth, Height: 38, Radius: 4,
		Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12, Bottom: 6}, Background: props.Theme.ToolbarBackground, BorderColor: borderColor, BorderWidth: 1,
		Style: woxui.TextStyle{Size: 13}, Value: row.State.Text, Focused: row.Focused, MaxLines: 1, Window: row.Window,
		Theme: props.Theme, Disabled: row.Disabled, OnChanged: row.OnChanged, OnKey: row.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && row.OnFocus != nil {
				row.OnFocus()
			}
		},
	})
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		input,
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: row.ID + "-browse", Label: props.Labels.Browse, Width: browseWidth, Height: 38, Radius: 4, FontSize: 13, Disabled: row.Disabled, Variant: woxcomponent.ButtonPrimary, OnTap: row.OnBrowse, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: row.ID + "-clear", Label: props.Labels.Clear, Width: clearWidth, Height: 38, Radius: 4, FontSize: 13, Disabled: row.Disabled, Variant: woxcomponent.ButtonOutline, OnTap: row.OnClear, Theme: props.Theme}),
	}}
	field := woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: row.Title, Description: row.Description, Width: width, Height: height, LabelWidth: labelWidth, Gap: 32,
		Padding: woxwidget.Insets{Top: 4, Bottom: 4}, DescriptionMaxLines: 2, Child: controls, Theme: props.Theme,
	})
	return woxwidget.Gesture{ID: row.ID, OnHover: func(inside bool) {
		if inside && row.OnHover != nil {
			row.OnHover()
		}
	}, Child: field}
}

// runtimeLabelWidth approximates intrinsic button and pill widths across Latin and CJK labels.
func runtimeLabelWidth(label string, minimum, maximum float32) float32 {
	width := float32(28)
	for _, character := range label {
		if character > 127 {
			width += 12
		} else {
			width += 6.5
		}
	}
	return min(maximum, max(minimum, width))
}

// runtimeStatusColor maps runtime health to the shared success, warning, and error colors.
func runtimeStatusColor(statusCode string, theme woxcomponent.Theme) woxui.Color {
	switch statusCode {
	case "running":
		return woxui.Color{R: 72, G: 190, B: 112, A: 255}
	case "executable_missing", "unsupported_version", "start_failed":
		return theme.ErrorText
	default:
		return woxui.Color{R: 225, G: 166, B: 64, A: 255}
	}
}

func runtimeStatusBackground(statusCode string, theme woxcomponent.Theme) woxui.Color {
	return runtimeWithAlpha(runtimeStatusColor(statusCode, theme), 42)
}

func runtimeOutlineColor(theme woxcomponent.Theme) woxui.Color {
	color := theme.PreviewSplit
	if color.A == 0 {
		color = theme.ResultSubtitle
	}
	return runtimeWithAlpha(color, 34)
}

func runtimeWithAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
