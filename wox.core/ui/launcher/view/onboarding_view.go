package view

import (
	"strconv"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	OnboardingSidebarWidth = float32(256)
	OnboardingFooterHeight = float32(72)
)

// OnboardingStep describes one item in the first-run progress rail.
type OnboardingStep struct {
	ID     string
	Title  string
	Accent woxui.Color
}

// OnboardingChoice describes one language or Glance option.
type OnboardingChoice struct {
	Value    string
	Label    string
	Leading  *woxui.Image
	Trailing string
}

// OnboardingPermission describes one passive permission check and action.
type OnboardingPermission struct {
	ID          string
	Title       string
	Description string
	Ready       bool
}

// OnboardingProps contains the prepared first-run state and actions.
type OnboardingProps struct {
	Width              float32
	Height             float32
	AppIcon            *woxui.Image
	Wallpaper          *woxui.Image
	WallpaperBlurred   *woxui.Image
	Steps              []OnboardingStep
	ActiveStep         int
	Labels             map[string]string
	Language           string
	GlanceEnabled      bool
	GlanceLabel        string
	GlanceValue        string
	GlanceIcon         *woxui.Image
	MainHotkeyLabels   []string
	SelectHotkeyLabels []string
	Hotkey             woxwidget.Widget
	Permissions        []OnboardingPermission
	PermissionLoading  bool
	ChoiceKind         string
	ChoiceValue        string
	ChoiceAnchor       woxui.Rect
	Choices            []OnboardingChoice
	Window             *woxui.Window
	Theme              woxcomponent.Theme
	OnStep             func(int)
	OnBack             func()
	OnNext             func()
	OnSkip             func()
	OnFinish           func()
	OnToggleGlance     func(bool)
	OnOpenChoice       func(string)
	OnSelectChoice     func(string)
	OnPermission       func(string)
}

// OnboardingView builds the ten-step management surface used on first run.
func OnboardingView(props OnboardingProps) woxwidget.Widget {
	if len(props.Steps) == 0 {
		return woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background}
	}
	active := min(max(0, props.ActiveStep), len(props.Steps)-1)
	step := props.Steps[active]
	bodyHeight := max(float32(0), props.Height-OnboardingFooterHeight)
	rail := onboardingRail(props, active, bodyHeight)
	page := onboardingPage(props, step, bodyHeight)
	footer := onboardingFooter(props, active)
	layers := []woxwidget.StackChild{{Child: woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.Background,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{rail, page}},
			footer,
		}},
	}}}
	if props.ChoiceKind != "" {
		choices := make([]SettingsChoice, len(props.Choices))
		for index, choice := range props.Choices {
			choices[index] = SettingsChoice{
				Value: choice.Value, Label: choice.Label, Leading: choice.Leading, Trailing: choice.Trailing,
			}
		}
		title := props.Labels["language"]
		if props.ChoiceKind == "glance" {
			title = props.Labels["glance.primary"]
		}
		kind := props.ChoiceKind
		layers = append(layers, woxwidget.StackChild{Child: SettingsChoiceView(SettingsChoiceProps{
			ID: "onboarding-choice-picker", Width: props.Width, Height: props.Height, Anchor: props.ChoiceAnchor,
			Theme: props.Theme, Window: props.Window, Title: title, CurrentValue: props.ChoiceValue, Choices: choices,
			OnChoose: func(index int) {
				if props.OnSelectChoice != nil && index >= 0 && index < len(props.Choices) {
					props.OnSelectChoice(props.Choices[index].Value)
				}
			},
			OnCancel: func() {
				if props.OnOpenChoice != nil {
					props.OnOpenChoice(kind)
				}
			},
		})})
	}
	return woxwidget.Semantics{
		Key: "onboarding-window", AutomationID: "onboarding.window", Role: woxui.AccessibilityRoleWindow, Label: props.Labels["title"],
		Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers},
	}
}

func onboardingRail(props OnboardingProps, active int, height float32) woxwidget.Widget {
	innerWidth := OnboardingSidebarWidth - 42
	title := woxwidget.Text{Value: props.Labels["title"], Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}
	var logo woxwidget.Widget = woxwidget.Container{
		Width: 28, Height: 28, Radius: 6, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Child: woxwidget.Align{Width: 28, Height: 28, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "W", Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{A: 255},
		}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 28, Height: 28}
	}
	rows := make([]woxwidget.Widget, 0, len(props.Steps))
	for index, step := range props.Steps {
		rows = append(rows, onboardingRailStep(step, index, active, innerWidth, func() {
			if props.OnStep != nil {
				props.OnStep(index)
			}
		}, props.Theme))
	}
	listHeight := max(float32(1), height-130)
	content := woxwidget.Container{
		Width: OnboardingSidebarWidth, Height: height, Color: settingsColorAlpha(props.Theme.ToolbarText, 12),
		Padding: woxwidget.Insets{Left: 24, Top: 30, Right: 18, Bottom: 16},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 11, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{logo, title}},
			woxwidget.TextBlock{
				Value: props.Labels["subtitle"], Width: innerWidth, Height: 38, MaxLines: 2,
				Style: woxui.TextStyle{Size: 13}, LineHeight: 19, Color: props.Theme.ResultSubtitle,
			},
			woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
				Key: "onboarding-rail-scroll", Width: innerWidth, Height: listHeight, ContentHeight: max(listHeight, float32(len(rows))*58),
				Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, ThumbColor: props.Theme.ResultSubtitle,
			}),
		}},
	}
	return woxwidget.Stack{Width: OnboardingSidebarWidth, Height: height, Children: []woxwidget.StackChild{
		{Child: content},
		{Left: OnboardingSidebarWidth - 1, Child: woxwidget.Container{Width: 1, Height: height, Color: settingsColorAlpha(props.Theme.PreviewSplit, 128)}},
	}}
}

func onboardingRailStep(step OnboardingStep, index, active int, width float32, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	selected := index == active
	done := index < active
	nodeSize := float32(18)
	if selected {
		nodeSize = 24
	}
	nodeColor := theme.ResultSubtitle
	nodeText := strconv.Itoa(index + 1)
	if selected {
		nodeColor = step.Accent
	}
	if done {
		nodeColor = theme.ResultTitle
		nodeText = "✓"
	}
	rowColor := woxui.Color{}
	border := woxui.Color{}
	if selected {
		rowColor = settingsColorAlpha(theme.ResultTitle, 30)
		border = settingsColorAlpha(theme.ResultTitle, 46)
	}
	labelColor := theme.ResultSubtitle
	weight := woxui.FontWeightRegular
	if selected {
		labelColor = theme.ResultTitle
		weight = woxui.FontWeightSemibold
	}
	node := woxwidget.Container{
		Width: nodeSize, Height: nodeSize, Radius: nodeSize / 2, Color: settingsColorAlpha(nodeColor, 36),
		BorderColor: settingsColorAlpha(nodeColor, 150), BorderWidth: 1,
		Child: woxwidget.Align{Width: nodeSize, Height: nodeSize, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: nodeText, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: nodeColor,
		}},
	}
	content := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Align{Width: 30, Height: 58, Horizontal: 0.5, Vertical: 0.5, Child: node},
		woxwidget.Container{
			Width: width - 40, Height: 38, Radius: 8, Color: rowColor, BorderColor: border, BorderWidth: 1,
			Padding: woxwidget.Insets{Left: 10, Top: 11, Right: 8},
			Child:   woxwidget.TextBlock{Value: step.Title, Width: width - 58, Height: 18, MaxLines: 1, Style: woxui.TextStyle{Size: 13, Weight: weight}, Color: labelColor},
		},
	}}
	id := "onboarding-step-" + step.ID
	return woxwidget.Semantics{
		Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleButton, Label: step.Title,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		Child:   woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{Width: width, Height: 58, Child: content}},
	}
}

func onboardingPage(props OnboardingProps, step OnboardingStep, height float32) woxwidget.Widget {
	width := max(float32(0), props.Width-OnboardingSidebarWidth)
	innerWidth := max(float32(0), width-76)
	contentHeight := onboardingPageContentHeight(props, step, innerWidth)
	content := onboardingStepContent(props, step, innerWidth, contentHeight)
	previewHeight := max(float32(120), height-30-44-16-contentHeight-18-20)
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 38, Top: 30, Right: 38, Bottom: 20},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: step.Title, Width: innerWidth, Height: 44, MaxLines: 1, Style: woxui.TextStyle{Size: 32, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
			woxwidget.Container{Width: innerWidth, Height: 16},
			content,
			woxwidget.Container{Width: innerWidth, Height: 18},
			woxwidget.Container{
				Width: innerWidth, Height: previewHeight, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 13),
				BorderColor: settingsColorAlpha(props.Theme.ResultTitle, 28), BorderWidth: 1, Padding: woxwidget.UniformInsets(10),
				Child: onboardingPreview(props, step, innerWidth-20, previewHeight-20),
			},
		}},
	}
}

// onboardingPageContentHeight keeps control-heavy steps stable while matching Flutter's intrinsic info-panel height for text-only steps.
func onboardingPageContentHeight(props OnboardingProps, step OnboardingStep, width float32) float32 {
	switch step.ID {
	case "welcome":
		return 138
	case "permissions":
		return 172
	case "glance":
		return 154
	case "mainHotkey", "selectionHotkey":
		return 90
	}

	textHeight := float32(21)
	if props.Window != nil {
		layout := woxwidget.LayoutTextBlock(props.Window, props.Labels[step.ID+".body"], woxui.TextStyle{Size: 14}, max(float32(0), width-44), 4, 21)
		textHeight = layout.Size.Height
	}
	return 22 + textHeight + 22
}

func onboardingStepContent(props OnboardingProps, step OnboardingStep, width, height float32) woxwidget.Widget {
	if step.ID == "permissions" {
		return onboardingPermissions(props, width, height)
	}
	if step.ID == "mainHotkey" || step.ID == "selectionHotkey" {
		return woxcomponent.WoxPanel(woxcomponent.PanelProps{
			Width: width, Height: height, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 18, Bottom: 12},
			Color: settingsColorAlpha(props.Theme.ResultTitle, 10), BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 46), Theme: props.Theme,
			Child: props.Hotkey,
		})
	}
	if step.ID == "glance" {
		return onboardingGlance(props, width, height)
	}
	description := props.Labels[step.ID+".body"]
	if step.ID == "welcome" {
		return onboardingWelcome(props, width, height, description)
	}
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: height, Padding: woxwidget.UniformInsets(22),
		Color: settingsColorAlpha(props.Theme.ResultTitle, 14), BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 40), Theme: props.Theme,
		Child: woxwidget.TextBlock{
			Value: description, Width: width - 44, MaxLines: 4,
			Style: woxui.TextStyle{Size: 14}, LineHeight: 21, Color: props.Theme.ResultTitle,
		},
	})
}

func onboardingWelcome(props OnboardingProps, width, height float32, description string) woxwidget.Widget {
	choiceWidth := min(float32(320), max(float32(0), width-44))
	contentWidth := max(float32(0), width-44)
	openChoice := func() {
		if props.OnOpenChoice != nil {
			props.OnOpenChoice("language")
		}
	}
	dropdown := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
		ID: "onboarding-language", Label: props.Labels["language"], Value: props.Language, Width: choiceWidth, Height: 34,
		Outline: settingsColorAlpha(props.Theme.ResultSubtitle, 140), Foreground: props.Theme.ResultTitle,
		Secondary: props.Theme.ResultSubtitle, Theme: props.Theme, OnTap: openChoice,
	})
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: height, Padding: woxwidget.UniformInsets(22),
		Color: settingsColorAlpha(props.Theme.ResultTitle, 14), BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 40), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: description, Width: contentWidth, Height: 21, MaxLines: 1, Style: woxui.TextStyle{Size: 14}, LineHeight: 21, Color: props.Theme.ResultTitle},
			woxwidget.Container{Width: contentWidth, Height: 20},
			woxwidget.Container{Width: contentWidth, Height: 1, Color: settingsColorAlpha(props.Theme.ResultTitle, 28)},
			woxwidget.Container{Width: contentWidth, Height: 18},
			woxwidget.Stack{Width: contentWidth, Height: 34, Children: []woxwidget.StackChild{
				{Top: 7, Child: woxwidget.Text{Value: props.Labels["language"], Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
				{Left: contentWidth - choiceWidth, Child: dropdown},
			}},
		}},
	})
}

func onboardingPermissions(props OnboardingProps, width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Permissions))
	rowHeight := height / max(float32(1), float32(len(props.Permissions)))
	for _, permission := range props.Permissions {
		status := props.Labels["permission.authorize"]
		var action woxwidget.Widget
		if permission.Ready {
			status = "✓"
			action = woxwidget.Container{
				Width: 32, Height: 32, Radius: 16, Color: settingsColorAlpha(woxui.Color{R: 34, G: 197, B: 94, A: 255}, 36),
				Child: woxwidget.Align{Width: 32, Height: 32, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 18}, Color: woxui.Color{R: 34, G: 197, B: 94, A: 255}}},
			}
		} else {
			if props.PermissionLoading {
				status = props.Labels["permission.checking"]
			}
			action = woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "onboarding-permission-" + permission.ID, Label: status, Height: 30, Size: woxcomponent.ButtonCompact,
				Variant: woxcomponent.ButtonOutline, Disabled: props.PermissionLoading, Theme: props.Theme,
				OnTap: func() {
					if props.OnPermission != nil {
						props.OnPermission(permission.ID)
					}
				},
			})
		}
		rows = append(rows, woxwidget.Container{
			Width: width, Height: rowHeight, Padding: woxwidget.Insets{Left: 18, Top: 14, Right: 18, Bottom: 12},
			BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 30), BorderWidth: 1,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Container{
					Width: 38, Height: 38, Radius: 8, Color: settingsColorAlpha(permissionColor(permission.Ready), 30),
					Child: woxwidget.Align{Width: 38, Height: 38, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
						Value: permissionGlyph(permission.ID), Style: woxui.TextStyle{Size: 19}, Color: permissionColor(permission.Ready),
					}},
				},
				woxwidget.Container{Width: max(float32(0), width-230), Height: rowHeight - 26, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
					woxwidget.Text{Value: permission.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
					woxwidget.TextBlock{Value: permission.Description, Width: max(float32(0), width-230), Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle},
				}}},
				woxwidget.Align{Width: 110, Height: rowHeight - 26, Horizontal: 1, Vertical: 0.5, Child: action},
			}},
		})
	}
	return woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: settingsColorAlpha(props.Theme.ResultTitle, 10),
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}
}

func onboardingGlance(props OnboardingProps, width, height float32) woxwidget.Widget {
	switchControl := woxcomponent.WoxSwitch(woxcomponent.SwitchProps{
		ID: "onboarding-glance-enable", Label: props.Labels["glance.enable"], Value: props.GlanceEnabled, Theme: props.Theme, OnChange: props.OnToggleGlance,
	})
	children := []woxwidget.Widget{
		woxwidget.Flex{Axis: woxwidget.Horizontal, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 86, Height: 64, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.Labels["glance.enable"], Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
				woxwidget.TextBlock{Value: props.Labels["glance.enable.body"], Width: width - 86, Height: 38, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle},
			}}},
			switchControl,
		}},
	}
	if props.GlanceEnabled {
		contentWidth := max(float32(0), width-44)
		choiceWidth := min(float32(300), max(float32(0), contentWidth-140))
		openChoice := func() {
			if props.OnOpenChoice != nil {
				props.OnOpenChoice("glance")
			}
		}
		dropdown := woxcomponent.WoxDropdown(woxcomponent.DropdownProps{
			ID: "onboarding-glance-choice", Label: props.Labels["glance.primary"], Value: props.GlanceLabel, Trailing: props.GlanceValue, Leading: props.GlanceIcon,
			Width: choiceWidth, Height: 34, Outline: settingsColorAlpha(props.Theme.ResultSubtitle, 140),
			Foreground: props.Theme.ResultTitle, Secondary: props.Theme.ResultSubtitle, Theme: props.Theme, OnTap: openChoice,
		})
		children = append(children, woxwidget.Stack{Width: contentWidth, Height: 34, Children: []woxwidget.StackChild{
			{Top: 7, Child: woxwidget.Text{Value: props.Labels["glance.primary"], Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}},
			{Left: contentWidth - choiceWidth, Child: dropdown},
		}})
	}
	return woxcomponent.WoxPanel(woxcomponent.PanelProps{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 22, Top: 18, Right: 22, Bottom: 14},
		Color: settingsColorAlpha(props.Theme.ResultTitle, 14), BorderColor: settingsColorAlpha(props.Theme.ResultSubtitle, 40), Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	})
}

func onboardingFooter(props OnboardingProps, active int) woxwidget.Widget {
	last := active == len(props.Steps)-1
	nextLabel := props.Labels["next"]
	nextAction := props.OnNext
	nextID := "onboarding-next"
	if last {
		nextLabel = props.Labels["finish"]
		nextAction = props.OnFinish
		nextID = "onboarding-finish"
	}
	content := woxwidget.Container{
		Width: props.Width, Height: OnboardingFooterHeight, Color: settingsColorAlpha(props.Theme.Background, 245),
		Padding: woxwidget.Insets{Left: 28, Top: 17, Right: 28, Bottom: 17},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisSpaceBetween, Children: []woxwidget.Widget{
			woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "onboarding-skip", Label: props.Labels["skip"], Height: 38, Padding: woxwidget.Insets{Left: 8, Right: 8},
				Variant: woxcomponent.ButtonText, Theme: props.Theme, OnTap: props.OnSkip,
			}),
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				woxcomponent.WoxButton(woxcomponent.ButtonProps{
					ID: "onboarding-back", Label: props.Labels["back"], Height: 38, Disabled: active == 0,
					Variant: woxcomponent.ButtonOutline, Theme: props.Theme, OnTap: props.OnBack,
				}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{
					ID: nextID, Label: nextLabel, Height: 38, Variant: woxcomponent.ButtonPrimary, Theme: props.Theme, OnTap: nextAction,
				}),
			}},
		}},
	}
	return woxwidget.Stack{Width: props.Width, Height: OnboardingFooterHeight, Children: []woxwidget.StackChild{
		{Child: content},
		{Child: woxwidget.Container{Width: props.Width, Height: 1, Color: settingsColorAlpha(props.Theme.PreviewSplit, 128)}},
	}}
}

func permissionColor(ready bool) woxui.Color {
	if ready {
		return woxui.Color{R: 34, G: 197, B: 94, A: 255}
	}
	return woxui.Color{R: 245, G: 158, B: 11, A: 255}
}

func permissionGlyph(id string) string {
	if id == "accessibility" {
		return "♿"
	}
	return "▣"
}
