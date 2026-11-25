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
	ExecutableHelp    string
	Browse            string
	Clear             string
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
	RefreshLabel string
	RefreshIcon  *woxui.Image
	RestartLabel string
	RestartIcon  *woxui.Image
	OnInstall    func()
	OnRefresh    func()
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
	Highlighted bool
	Window      *woxui.Window
	OnHover     func()
	OnFocus     func()
	OnChanged   func(string)
	OnKey       func(woxui.KeyEvent) bool
	OnBrowse    func()
	OnSave      func()
	OnClear     func()
}

// RuntimeSettingsProps contains runtime inventory and executable settings.
type RuntimeSettingsProps struct {
	Width            float32
	Height           float32
	SettingRowHeight float32
	Theme            woxcomponent.ControlTheme
	Labels           RuntimeSettingsLabels
	Loading          bool
	Restarting       bool
	Refreshing       bool
	OnTooltip        func(bool, string, woxui.Rect)
	Editing          bool
	Saving           bool
	Error            string
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
			Value: props.Labels.StatusSection, Style: woxui.TextStyle{Size: props.Theme.Scaled(13), Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text,
		}},
	}
	if props.Error != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Padding: woxwidget.Insets{Bottom: 6}, Child: woxwidget.TextBlock{
			Value: props.Error, MaxLines: 1, MinLineHeight: props.Theme.Scaled(18), Style: woxui.TextStyle{Size: props.Theme.Scaled(11)}, Color: props.Theme.Error,
		}})
	}
	children = append(children,
		runtimeStatusGrid(props, contentWidth),
		woxwidget.Container{Width: contentWidth, Height: woxcomponent.SectionHeaderLead},
		woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Labels.ExecutableSection, Width: contentWidth, Theme: props.Theme}),
	)
	if props.Labels.ExecutableHelp != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Padding: woxwidget.Insets{Bottom: 8}, Child: woxwidget.TextBlock{
			Value: props.Labels.ExecutableHelp, Width: contentWidth, MaxLines: 2, MinLineHeight: props.Theme.Scaled(18),
			Style: woxui.TextStyle{Size: props.Theme.Scaled(woxcomponent.SettingsHelpFontSize)}, Color: props.Theme.TextSecondary,
		}})
	}
	var selectedKey woxwidget.Key
	for index, row := range props.Settings {
		key := woxwidget.Key(row.ID)
		children = append(children, woxwidget.Keyed{Key: key, Child: runtimeExecutableSettingRow(props, row, contentWidth, settingRowHeight)})
		if index == props.Selected {
			selectedKey = key
		}
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 16})
	return SettingsPage(SettingsPageProps{Theme: props.Theme,
		ID: "runtime-page-scroll", Width: props.Width, Height: props.Height, Children: children, KeepVisibleKey: selectedKey,
	})
}

// runtimeStatusThreeColumnMinWidth is the default Settings content width so
// Node.js, Python, and Script stay on one row in the default window.
func runtimeStatusThreeColumnMinWidth() float32 {
	return SettingsPageContentWidth(woxcomponent.SettingsWindowWidth - woxcomponent.SettingsRailMaxWidth)
}

// runtimeStatusColumns prefers three cards in one row at the default page width.
func runtimeStatusColumns(width float32) int {
	if width >= runtimeStatusThreeColumnMinWidth() {
		return 3
	}
	if width >= 560 {
		return 2
	}
	return 1
}

// runtimeStatusGrid builds the full-width status summary or its empty state.
func runtimeStatusGrid(props RuntimeSettingsProps, width float32) woxwidget.Widget {
	if len(props.Statuses) == 0 {
		message := props.Labels.Empty
		if props.Loading {
			message = ""
		}
		return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: message, MaxLines: 1, MinLineHeight: props.Theme.Scaled(28), Style: woxui.TextStyle{Size: props.Theme.Scaled(12)}, Color: props.Theme.TextSecondary,
		}}
	}
	columns := runtimeStatusColumns(width)
	cardWidth := (width - float32(columns-1)*12) / float32(columns)
	cards := make([]woxwidget.Widget, 0, len(props.Statuses))
	for _, status := range props.Statuses {
		cardHeight := float32(168)
		if status.Actionable {
			cardHeight = 224
		}
		cards = append(cards, runtimeStatusCard(props, status, cardWidth, cardHeight))
	}
	return woxwidget.Grid{
		Width: width, Columns: columns, CellWidth: cardWidth, ColumnGap: 12, RowGap: 12,
		CrossAxisAlignment: woxwidget.CrossAxisStretch, Children: cards,
	}
}

// runtimeStatusCard keeps compact cards aligned while longer text can grow the row.
func runtimeStatusCard(props RuntimeSettingsProps, status RuntimeStatus, width, minHeight float32) woxwidget.Widget {
	theme := props.Theme
	statusColor := runtimeStatusColor(status.StatusCode, theme)
	var icon woxwidget.Widget = woxwidget.Text{Value: status.Mark, Style: woxui.TextStyle{Size: props.Theme.Scaled(11), Weight: woxui.FontWeightSemibold}, Color: theme.Text}
	if status.Icon != nil {
		icon = woxwidget.Image{Source: status.Icon, Width: 22, Height: 22}
	}
	pillWidth := runtimeLabelWidth(status.StatusLabel, 40, 150)
	busy := props.Restarting || props.Refreshing || props.Editing || props.Saving
	statusRow := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Container{Width: pillWidth, Radius: 11, Color: runtimeStatusBackground(status.StatusCode, theme), Padding: woxwidget.Insets{Left: 8, Right: 8}, Child: woxwidget.TextBlock{
			Value: status.StatusLabel, MaxLines: 1, MinLineHeight: theme.Scaled(22), Centered: true, AlignmentY: 0.5,
			Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize), Weight: woxui.FontWeightSemibold}, Color: statusColor,
		}},
	}}
	if status.OnRestart != nil {
		statusRow.Children = append(statusRow.Children, woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: "runtime-restart-" + status.Runtime, Label: status.RestartLabel,
			Icon:  woxwidget.Image{Source: status.RestartIcon, Width: 16, Height: 16},
			Width: 32, Height: 32, Radius: 4, HoverBackground: runtimeWithAlpha(theme.Text, 20), FocusRingColor: theme.Focus,
			Disabled: busy, OnTap: func() {
				if props.OnTooltip != nil {
					props.OnTooltip(false, "", woxui.Rect{})
				}
				status.OnRestart()
			}, OnHoverAt: func(inside bool, bounds woxui.Rect) {
				if props.OnTooltip != nil {
					props.OnTooltip(inside, status.RestartLabel, bounds)
				}
			},
		}))
	}

	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: 34, Height: 34, Radius: 8, Color: runtimeWithAlpha(theme.Text, 26), Child: woxwidget.Align{
			Width: 34, Height: 34, Horizontal: 0.5, Vertical: 0.5, Child: icon,
		}},
		woxwidget.Expanded{Child: woxwidget.Constrained{MinHeight: 56, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Expanded{Child: woxwidget.TextBlock{Value: status.DisplayName, MaxLines: 1, MinLineHeight: theme.Scaled(20), Style: woxui.TextStyle{Size: theme.Scaled(15), Weight: woxui.FontWeightSemibold}, Color: theme.Text}},
				woxwidget.TextBlock{Value: status.Version, Width: 62, MaxLines: 1, MinLineHeight: theme.Scaled(20), Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary},
			}},
			statusRow,
		}}}},
	}}
	children := []woxwidget.Widget{
		header,
		woxwidget.Container{Height: 4},
		woxwidget.Container{Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.Constrained{MinHeight: 40, Child: woxwidget.TextBlock{
			Value: status.Detail, MaxLines: 2, Style: woxui.TextStyle{Size: theme.Scaled(12)}, MinLineHeight: theme.Scaled(17), Color: theme.TextSecondary,
		}}},
		woxwidget.Container{Height: 14},
		woxwidget.Container{Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.TextBlock{
			Value: status.PluginLabel, MaxLines: 1, MinLineHeight: theme.Scaled(18), Style: woxui.TextStyle{Size: theme.Scaled(13)}, Color: theme.TextSecondary,
		}},
	}
	if status.Actionable {
		buttons := make([]woxwidget.Widget, 0, 3)
		if status.OnInstall != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "runtime-install-" + status.Runtime, Label: status.InstallLabel, Icon: status.InstallIcon, IconSize: 14,
				Radius: 4, Disabled: busy, Variant: woxcomponent.ButtonSecondary, OnTap: status.OnInstall, Theme: theme,
			}))
		}
		if status.OnRefresh != nil {
			buttons = append(buttons, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "runtime-refresh-" + status.Runtime, Label: status.RefreshLabel, Icon: status.RefreshIcon, IconSize: 14,
				Radius: 4, Disabled: busy, Variant: woxcomponent.ButtonSecondary, OnTap: status.OnRefresh, Theme: theme,
			}))
		}
		children = append(children,
			woxwidget.Container{Height: 10},
			woxwidget.Container{Height: 38, Padding: woxwidget.Insets{Left: 46}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
		)
	}
	const padding = float32(14)
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Padding: woxwidget.UniformInsets(padding), BorderColor: runtimeOutlineColor(theme), Theme: theme,
		Child: woxwidget.Constrained{MinHeight: max(0, minHeight-2*padding), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}},
	})
}

// runtimeExecutableSettingRow renders one label, path input, browse action, and clear action.
func runtimeExecutableSettingRow(props RuntimeSettingsProps, row RuntimeSettingRow, width, height float32) woxwidget.Widget {
	labelWidth := min(float32(400), max(float32(220), width*0.48))
	controlWidth := max(float32(220), width-labelWidth-32)
	browseWidth := runtimeLabelWidth(props.Labels.Browse, 62, 96)
	actionWidth := runtimeLabelWidth(props.Labels.Clear, 62, 96)
	inputWidth := max(float32(80), controlWidth-browseWidth-actionWidth-20)
	input := woxcomponent.WoxSettingTextField(woxcomponent.TextFieldProps{
		ID: row.ID + "-input", Label: row.Title, Hint: row.Placeholder, Width: inputWidth,
		Value: row.State.Text, Focused: row.Focused, Window: row.Window,
		Theme: props.Theme, Disabled: row.Disabled, OnChanged: row.OnChanged, OnKey: row.OnKey,
		OnFocusChange: func(focused bool) {
			if focused && row.OnFocus != nil {
				row.OnFocus()
			}
			if !focused && row.Focused && row.OnSave != nil {
				row.OnSave()
			}
		},
	})
	action := woxcomponent.ButtonProps{ID: row.ID + "-clear", Label: props.Labels.Clear, Width: actionWidth, Disabled: row.Disabled, Variant: woxcomponent.ButtonSecondary, OnTap: row.OnClear, Theme: props.Theme}
	controls := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
		input,
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: row.ID + "-browse", Label: props.Labels.Browse, Width: browseWidth, Radius: 4, FontSize: 13, Disabled: row.Disabled, Variant: woxcomponent.ButtonSecondary, OnTap: row.OnBrowse, Theme: props.Theme}),
		woxcomponent.WoxButton(action),
	}}
	field := woxcomponent.WoxSettingField(woxcomponent.SettingFieldProps{
		Label: row.Title, Description: row.Description, Width: width, Height: height, LabelWidth: labelWidth, Gap: 32,
		Padding: woxwidget.Insets{Top: 4, Bottom: 4}, DescriptionMaxLines: 2, Child: controls, Theme: props.Theme,
	})
	target := woxcomponent.WoxSettingTarget(woxcomponent.SettingTargetProps{Width: width, Height: height, Highlighted: row.Highlighted, Child: field, Theme: props.Theme})
	return woxwidget.Gesture{ID: row.ID, OnHover: func(inside bool) {
		if inside && row.OnHover != nil {
			row.OnHover()
		}
	}, Child: target}
}

// runtimeLabelWidth approximates intrinsic button and pill widths across Latin and CJK labels.
func runtimeLabelWidth(label string, minimum, maximum float32) float32 {
	width := float32(40)
	for _, character := range label {
		if character > 127 {
			width += 13
		} else {
			width += 6.5
		}
	}
	return min(maximum, max(minimum, width))
}

// runtimeStatusColor maps runtime health to the shared success, warning, and error colors.
func runtimeStatusColor(statusCode string, theme woxcomponent.ControlTheme) woxui.Color {
	switch statusCode {
	case "running":
		return woxui.Color{R: 72, G: 190, B: 112, A: 255}
	case "executable_missing", "unsupported_version", "start_failed":
		return theme.Error
	default:
		return woxui.Color{R: 225, G: 166, B: 64, A: 255}
	}
}

func runtimeStatusBackground(statusCode string, theme woxcomponent.ControlTheme) woxui.Color {
	return runtimeWithAlpha(runtimeStatusColor(statusCode, theme), 42)
}

func runtimeOutlineColor(theme woxcomponent.ControlTheme) woxui.Color {
	color := theme.Border
	if color.A == 0 {
		color = theme.TextSecondary
	}
	return runtimeWithAlpha(color, 34)
}

func runtimeWithAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
