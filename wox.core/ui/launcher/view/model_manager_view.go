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
	State         string
	Progress      int
	Languages     string
	Description   string
	SizeMB        int
	Recommended   bool
	SelectedRow   bool
	PrimaryAction bool
	ActionLabel   string
	ActionEnabled bool
	OnAction      func()
	OnDelete      func()
	OnSelect      func()
	OnChoose      func()
}

// ModelManagerProps contains the model manager overlay state.
type ModelManagerProps struct {
	Width             float32
	Height            float32
	Theme             woxcomponent.Theme
	Title             string
	Anchor            woxui.Rect
	Anchored          bool
	Loading           bool
	Busy              bool
	Error             string
	EngineLabel       string
	EngineButtonLabel string
	EngineEnabled     bool
	EngineKnown       bool
	EngineReady       bool
	RecommendedLabel  string
	DeleteLabel       string
	DownloadIcon      *woxui.Image
	DeleteIcon        *woxui.Image
	ErrorIcon         *woxui.Image
	Options           []ModelManagerOption
	OnEngine          func()
	OnRefresh         func()
	OnClose           func()
}

// ModelManagerView builds the modal engine and model manager.
func ModelManagerView(props ModelManagerProps) woxwidget.Widget {
	if props.Anchored {
		return modelManagerDropdown(props)
	}
	panelWidth := max(float32(0), min(float32(780), props.Width-28))
	panelHeight := max(float32(0), min(float32(660), props.Height-28))
	return modelManagerPanel(props, panelWidth, panelHeight)
}

// modelManagerDropdown mirrors Flutter's field-width model menu without replacing the settings page with a dialog.
func modelManagerDropdown(props ModelManagerProps) woxwidget.Widget {
	const margin = float32(12)
	anchor := props.Anchor
	if anchor.Width <= 0 || anchor.Height <= 0 {
		anchor = woxui.Rect{X: max(margin, props.Width-620-margin), Y: 160, Width: min(float32(620), props.Width-margin*2), Height: 34}
	}
	menuWidth := min(anchor.Width, max(float32(1), props.Width-margin*2))
	menuLeft := min(max(margin, anchor.X), max(margin, props.Width-menuWidth-margin))
	engineHeight := float32(0)
	showEngine := props.EngineKnown && !props.EngineReady
	if showEngine {
		engineHeight = 54
	}
	errorHeight := float32(0)
	if props.Error != "" {
		errorHeight = 34
	}
	listHeight := min(float32(len(props.Options))*ModelManagerRowHeight, max(ModelManagerRowHeight, float32(300)-engineHeight-errorHeight))
	menuHeight := engineHeight + listHeight + errorHeight
	menuTop := anchor.Y + anchor.Height
	if menuTop+menuHeight > props.Height-margin {
		menuTop = anchor.Y - menuHeight
	}
	menuTop = min(max(margin, menuTop), max(margin, props.Height-menuHeight-margin))

	children := make([]woxwidget.Widget, 0, 3)
	if showEngine {
		engineTextWidth := max(float32(80), menuWidth-174)
		engineChildren := []woxwidget.Widget{
			woxwidget.TextBlock{Value: props.EngineLabel, Width: engineTextWidth, Height: 34, MaxLines: 2, LineHeight: 16, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
		}
		if props.EngineEnabled {
			engineChildren = append(engineChildren, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "model-manager-engine", Label: props.EngineButtonLabel, Width: 132, Height: 34, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnEngine, Theme: props.Theme,
			}))
		}
		children = append(children, woxwidget.Container{Width: menuWidth, Height: engineHeight, Padding: woxwidget.Insets{Left: 12, Top: 10, Right: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: engineChildren}})
	}

	rows := make([]woxwidget.Widget, 0, len(props.Options))
	for index, option := range props.Options {
		index := index
		option := option
		background := props.Theme.ActionBackground
		if option.SelectedRow {
			background = props.Theme.SelectedBackground
			background.A = min(uint8(80), background.A)
		}
		trailingWidth := float32(96)
		if option.OnDelete != nil {
			trailingWidth = 34
		}
		contentWidth := max(float32(100), menuWidth-trailingWidth-36)
		titleColor := props.Theme.ResultSubtitle
		if option.OnDelete != nil {
			titleColor = props.Theme.ActionText
		}
		titleChildren := []woxwidget.Widget{
			woxwidget.Text{Value: option.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: titleColor},
		}
		if option.Recommended {
			titleChildren = append(titleChildren, woxwidget.Container{Height: 18, Radius: 3, Color: modelManagerAlpha(props.Theme.Cursor, 38), Padding: woxwidget.Insets{Left: 5, Top: 2, Right: 5}, Child: woxwidget.Text{
				Value: props.RecommendedLabel, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor,
			}})
		}
		if option.SizeMB > 0 {
			titleChildren = append(titleChildren, woxwidget.Text{Value: fmt.Sprintf("~%dMB", option.SizeMB), Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle})
		}
		var trailing woxwidget.Widget
		if option.OnDelete != nil {
			buttonTheme := props.Theme
			buttonTheme.ResultTitle = props.Theme.ResultSubtitle
			trailing = woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: fmt.Sprintf("model-delete-%d", index), Label: props.DeleteLabel, Icon: props.DeleteIcon, IconSize: 16, IconOnly: props.DeleteIcon != nil,
				Width: 34, Height: 34, Variant: woxcomponent.ButtonText, FontSize: 10, Disabled: props.Busy || props.Loading, OnTap: option.OnDelete, Theme: buttonTheme,
			})
		} else if option.State == "downloading" {
			trailing = modelManagerProgress(fmt.Sprintf("model-progress-%d", index), option.ActionLabel, option.Progress, trailingWidth, props.Theme)
		} else {
			icon := (*woxui.Image)(nil)
			if option.State == "" || option.State == "not_downloaded" {
				icon = props.DownloadIcon
			} else if option.State == "failed" {
				icon = props.ErrorIcon
			}
			trailing = woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: fmt.Sprintf("model-action-%d", index), Label: option.ActionLabel, Icon: icon, IconSize: 14, IconGap: 6, Width: trailingWidth, Height: 34, Padding: woxwidget.Insets{Left: 10, Right: 10}, FontSize: 11, Disabled: !option.ActionEnabled, Variant: woxcomponent.ButtonOutline, OnTap: option.OnAction, Theme: props.Theme})
		}
		activate := option.OnSelect
		if option.OnChoose != nil {
			activate = option.OnChoose
		}
		detail := woxwidget.Container{Width: contentWidth, Height: 64, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: titleChildren},
			woxwidget.TextBlock{Value: option.Languages, Width: contentWidth, Height: 16, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
			woxwidget.TextBlock{Value: option.Description, Width: contentWidth, Height: 32, MaxLines: 2, LineHeight: 15, Style: woxui.TextStyle{Size: 11}, Color: modelManagerAlpha(props.Theme.ResultSubtitle, 204)},
		}}}
		rowContent := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{detail, trailing}}
		radius := float32(0)
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: fmt.Sprintf("model-row-%d", index), Label: option.Name, Width: menuWidth, Height: ModelManagerRowHeight, Radius: &radius,
			Background: &background, Selected: option.SelectedRow, Padding: woxwidget.Insets{Left: 12, Top: 9, Right: 12}, OnTap: activate, Child: rowContent, Theme: props.Theme,
		}))
	}
	children = append(children, woxwidget.ScrollView{
		Key: "model-manager-list", ID: "model-manager-list", Width: menuWidth, Height: listHeight, ContentHeight: max(listHeight, float32(len(rows))*ModelManagerRowHeight),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	})
	if props.Error != "" {
		children = append(children, woxwidget.Container{Width: menuWidth, Height: errorHeight, Padding: woxwidget.Insets{Left: 12, Top: 8, Right: 12}, Child: woxwidget.TextBlock{
			Value: props.Error, Width: menuWidth - 24, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ErrorText,
		}})
	}
	menuContent := woxwidget.Container{Width: menuWidth, Height: menuHeight, Radius: 4, Color: props.Theme.ActionBackground, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	menuBorder := woxwidget.Container{Width: menuWidth, Height: menuHeight, Radius: 4, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1}
	menu := woxwidget.FocusScope{Key: "model-manager-scope", Modal: true, Child: woxwidget.Stack{Width: menuWidth, Height: menuHeight, Children: []woxwidget.StackChild{{Child: menuContent}, {Child: menuBorder}}}}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "model-manager-backdrop", OnTap: props.OnClose, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: menuLeft, Top: menuTop, Child: menu},
	}}
}

func modelManagerProgress(id, label string, progress int, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	progress = min(100, max(0, progress))
	trackWidth := max(float32(0), width-36)
	track := woxwidget.Container{Width: trackWidth, Height: 4, Radius: 2, Color: modelManagerAlpha(theme.PreviewSplit, 150), Child: woxwidget.Container{Width: trackWidth * float32(progress) / 100, Height: 4, Radius: 2, Color: theme.Cursor}}
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		track,
		woxwidget.Text{Value: fmt.Sprintf("%d%%", progress), Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
	}}
	return woxwidget.Semantics{Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleProgressBar, Label: label, Value: fmt.Sprintf("%d%%", progress), ReadOnly: true, Child: woxwidget.Align{Width: width, Height: 34, Vertical: 0.5, Child: content}}
}

func modelManagerAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

func modelManagerPanel(props ModelManagerProps, width, height float32) woxwidget.Widget {
	innerWidth := max(float32(0), width-32)
	headerHeight := float32(54)
	engineHeight := float32(72)
	footerHeight := float32(58)
	statusHeight := float32(28)
	viewportHeight := max(float32(82), height-headerHeight-engineHeight-footerHeight-statusHeight-32)
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
		radius := float32(7)
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: fmt.Sprintf("model-row-%d", index), Label: option.Name, Width: innerWidth, Height: ModelManagerRowHeight, Radius: &radius,
			Background: &background, Selected: option.SelectedRow, OnTap: option.OnSelect, Theme: props.Theme, Padding: woxwidget.Insets{Left: 14, Top: 10, Right: 10, Bottom: 8},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Container{Width: detailWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: option.Name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
					woxwidget.TextBlock{Value: option.Detail, Width: detailWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ActionHeader},
					woxwidget.TextBlock{Value: option.Status, Width: detailWidth, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 9, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
				}}},
				woxwidget.Container{Width: buttonWidth, Height: 48, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
			}},
		}))
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: innerWidth, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No model options were returned by the plugin.", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ActionHeader,
		}}
	} else {
		var keepVisible *woxwidget.ScrollRange
		for index, option := range props.Options {
			if option.SelectedRow {
				start := float32(index) * ModelManagerRowHeight
				keepVisible = &woxwidget.ScrollRange{Start: start, End: start + ModelManagerRowHeight}
				break
			}
		}
		list = woxwidget.ScrollView{
			Key: "model-manager-list", ID: "model-manager-list", Width: innerWidth, Height: viewportHeight,
			ContentHeight: max(viewportHeight, float32(len(rows))*ModelManagerRowHeight), KeepVisible: keepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}
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
