package view

import (
	"fmt"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// CloudSettingsPageProps contains cloud settings data and controller callbacks.
type CloudSettingsPageProps struct {
	Width        float32
	Height       float32
	Title        string
	Description  string
	Intro        CloudIntroProps
	Account      CloudAccountProps
	Sync         CloudSyncProps
	Devices      CloudDevicesProps
	Plugins      CloudPluginExclusionsProps
	ConfigNotes  CloudConfigNotesProps
	Message      string
	MessageColor woxui.Color
	ActionMenu   *CloudActionMenuProps
	Theme        woxcomponent.ControlTheme
	OnCloseMenu  func()
}

// CloudIntroProps contains the signed-out product summary and plan comparison.
type CloudIntroProps struct {
	SectionLabel string
	Headline     string
	Description  string
	HeroIcon     *woxui.Image
	HeroFallback string
	Features     []CloudIntroFeatureProps
	FreeLabel    string
	ProLabel     string
	PlanRows     []CloudPlanRowProps
}

// CloudIntroFeatureProps contains one signed-out cloud capability card.
type CloudIntroFeatureProps struct {
	Title        string
	Description  string
	Icon         *woxui.Image
	FallbackIcon string
}

// CloudPlanRowProps contains one Free and Pro comparison row.
type CloudPlanRowProps struct {
	Label     string
	FreeValue string
	ProValue  string
}

// CloudAccountProps contains account presentation and actions.
type CloudAccountProps struct {
	ActionWidth            float32
	SectionLabel           string
	LoggedIn               bool
	LabelWidth             float32
	LoginLabel             string
	RegisterLabel          string
	EmailLabel             string
	Email                  string
	PlanLabel              string
	PlanTips               string
	PlanStatus             string
	BillingLabel           string
	BillingTips            string
	SupportLabel           string
	InfoIcon               *woxui.Image
	SupportIcon            *woxui.Image
	ActionsEnabled         bool
	OnLogin                func()
	OnRegister             func()
	OnOpenAccountMenu      func()
	OnOpenSubscriptionMenu func()
	OnPlanTooltip          func(bool, woxui.Rect)
	OnSupport              func()
}

// CloudSyncProps contains sync status presentation and its primary action.
type CloudSyncProps struct {
	ActionWidth   float32
	SectionLabel  string
	StatusIcon    *woxui.Image
	ButtonIcon    *woxui.Image
	Label         string
	Detail        string
	Color         woxui.Color
	ButtonLabel   string
	ButtonEnabled bool
	OnSync        func()
}

// CloudDevicesProps contains device rows and refresh state.
type CloudDevicesProps struct {
	ActionWidth     float32
	LastActiveLabel string
	SectionLabel    string
	Tips            string
	RefreshLabel    string
	RefreshIcon     *woxui.Image
	RefreshEnabled  bool
	EmptyLabel      string
	Items           []CloudDeviceProps
	OnRefresh       func()
}

// CloudDeviceProps contains one device row and optional revoke action.
type CloudDeviceProps struct {
	Icon          *woxui.Image
	CurrentLabel  string
	ID            string
	Name          string
	Detail        string
	LastSeen      string
	RevokeLabel   string
	ShowRevoke    bool
	RevokeEnabled bool
	OnRevoke      func()
}

// CloudPluginExclusionsProps contains plugin exclusion rows and scrolling state.
type CloudPluginExclusionsProps struct {
	SectionLabel   string
	Tips           string
	ColumnLabel    string
	Items          []CloudPluginExclusionProps
	AddLabel       string
	OperationLabel string
	AddIcon        *woxui.Image
	DeleteIcon     *woxui.Image
	OnAdd          func()
}

// CloudPluginExclusionProps contains one plugin exclusion toggle row.
type CloudPluginExclusionProps struct {
	ID       string
	Name     string
	PluginID string
	Icon     *woxui.Image
	OnDelete func()
}

// CloudConfigNotesProps contains translated configuration caveats.
type CloudConfigNotesProps struct {
	SectionLabel string
	Tips         string
	ItemLabel    string
	ModeLabel    string
	InfoIcon     *woxui.Image
	Items        []CloudConfigNoteProps
	OnTooltip    func(bool, string, woxui.Rect)
}

// CloudConfigNoteProps contains one documented special sync behavior.
type CloudConfigNoteProps struct {
	Item    string
	Mode    string
	Tooltip string
}

// CloudActionMenuProps contains a positioned account or subscription action menu.
type CloudActionMenuProps struct {
	Top   float32
	Modal bool
	Items []CloudActionMenuItemProps
}

// CloudActionMenuItemProps contains one cloud menu action.
type CloudActionMenuItemProps struct {
	ID    string
	Label string
	OnTap func()
}

// CloudPluginExclusionDialogProps contains the temporary row editor used to add one excluded plugin.
type CloudPluginExclusionDialogProps struct {
	Width        float32
	Height       float32
	PanelWidth   float32
	PanelHeight  float32
	FieldLabel   string
	Description  string
	Selected     string
	SelectedName string
	SelectedIcon *woxui.Image
	Choices      []SettingsChoice
	ChoiceAnchor woxui.Rect
	ChoiceOpen   bool
	CancelLabel  string
	SaveLabel    string
	Window       *woxui.Window
	Theme        woxcomponent.ControlTheme
	OnChoiceTap  func(woxui.Rect)
	OnChoose     func(int)
	OnCancel     func()
	OnSave       func()
}

// CloudPluginExclusionDialogHeight fits the field and shared actions inside the dialog padding.
const CloudPluginExclusionDialogHeight = float32(164)

// cloudHelpRowBottom keeps wrapped help copy from sitting on the next title or list row.
const cloudHelpRowBottom = float32(16)

// CloudSettingsPage builds the complete scrollable cloud settings route.
func CloudSettingsPage(props CloudSettingsPageProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	children := make([]woxwidget.Widget, 0, 12)
	appendChild := func(widget woxwidget.Widget) { children = append(children, widget) }

	appendChild(woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
		Title: props.Title, Description: props.Description, Width: contentWidth, Theme: props.Theme,
	}))
	if !props.Account.LoggedIn {
		appendChild(cloudIntro(props.Intro, props.Account, contentWidth, props.Theme))
	} else {
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Account.SectionLabel, Width: contentWidth, Theme: props.Theme}))
		appendChild(cloudAccountCard(props.Account, contentWidth, 0, props.Theme))
	}

	if props.Account.LoggedIn {
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Sync.SectionLabel, Width: contentWidth, Theme: props.Theme}))
		appendChild(cloudSyncCard(props.Sync, contentWidth, props.Theme))
		appendChild(cloudDeviceHeader(props.Devices, contentWidth, props.Theme))
		appendChild(cloudDeviceCard(props.Devices, contentWidth, props.Theme))
		pluginHeight := FormTableFieldHeight(true, props.Plugins.Tips, len(props.Plugins.Items), 260)
		// Flutter wraps built-in setting tables in a 24px outer bottom gap so
		// adjacent tables keep the same breathing room as the settings form.
		appendChild(woxwidget.Container{Width: contentWidth, Padding: woxwidget.Insets{Bottom: 24}, Child: cloudPluginExclusionsCard(props.Plugins, contentWidth, pluginHeight, props.Theme)})
		configHeight := FormTableFieldHeight(true, props.ConfigNotes.Tips, len(props.ConfigNotes.Items), 720)
		appendChild(cloudConfigNotesCard(props.ConfigNotes, contentWidth, configHeight, props.Theme))
	}
	if props.Message != "" {
		appendChild(woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.TextBlock{
			Value: props.Message, Width: contentWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: props.Theme.Scaled(10)}, Color: props.MessageColor,
		}})
	}

	page := SettingsPage(SettingsPageProps{Theme: props.Theme,
		ID: "cloud-page-scroll", Width: props.Width, Height: props.Height, Children: children,
		Gap: 4,
	})
	if props.ActionMenu == nil {
		return page
	}
	menuLeft := max(float32(20), props.Width-236)
	menuTop := props.ActionMenu.Top
	menuWidth := float32(196)
	if props.ActionMenu.Modal {
		menuWidth = 320
		menuLeft = max(float32(20), (props.Width-menuWidth)/2)
		menuTop = max(float32(20), (props.Height-min(float32(420), float32(len(props.ActionMenu.Items))*40+12))/2)
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: page},
		{Child: woxwidget.Gesture{ID: "cloud-action-menu-shade", OnTap: props.OnCloseMenu, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: menuLeft, Top: menuTop, Child: cloudActionMenu(*props.ActionMenu, menuWidth, props.Theme)},
	}}
}

// cloudIntro keeps authentication before optional feature and plan details.
func cloudIntro(props CloudIntroProps, account CloudAccountProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: theme.Scaled(8), Bottom: theme.Scaled(24)}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: theme.Scaled(24), Children: []woxwidget.Widget{
			cloudIntroHero(props, account, width, theme),
			cloudIntroFeatures(props.Features, width, theme),
			cloudPlanComparison(props, width, width < theme.Scaled(620), theme),
		},
	}}
}

// cloudIntroHero groups the explanation and entry points without another Account heading.
func cloudIntroHero(props CloudIntroProps, account CloudAccountProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	actions := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(8), Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-login", Label: account.LoginLabel, Disabled: !account.ActionsEnabled, Variant: woxcomponent.ButtonPrimary, OnTap: account.OnLogin, Theme: theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-register", Label: account.RegisterLabel, Disabled: !account.ActionsEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: account.OnRegister, Theme: theme}),
	}}
	// Match the feature grid's icon slot and text inset so both sections share alignment lines.
	header := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(12), Children: []woxwidget.Widget{
		cloudIntroIcon(props.HeroIcon, props.HeroFallback, 28, 24, theme),
		woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(12), Children: []woxwidget.Widget{
			woxwidget.TextBlock{Value: props.Headline, Style: woxui.TextStyle{Size: theme.Scaled(20), Weight: woxui.FontWeightSemibold}, Color: theme.Text},
			woxwidget.TextBlock{Value: props.Description, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize)}, LineHeight: theme.Scaled(20), Color: theme.TextSecondary},
		}}},
	}}
	if width < theme.Scaled(760) {
		actions.MainAxisAlignment = woxwidget.MainAxisEnd
		return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(16), Children: []woxwidget.Widget{header, actions}}}
	}
	header.Children = append(header.Children, actions)
	return woxwidget.Container{Width: width, Child: header}
}

// cloudIntroFeatures uses an open grid; long translations grow the row instead of being ellipsized.
func cloudIntroFeatures(features []CloudIntroFeatureProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	columns := 3
	if width < theme.Scaled(760) {
		columns = 1
	}
	children := make([]woxwidget.Widget, 0, len(features))
	for _, feature := range features {
		children = append(children, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(12), Children: []woxwidget.Widget{
			cloudIntroIcon(feature.Icon, feature.FallbackIcon, 28, 17, theme),
			woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(6), Children: []woxwidget.Widget{
				woxwidget.TextBlock{Value: feature.Title, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.Text},
				woxwidget.TextBlock{Value: feature.Description, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, LineHeight: theme.Scaled(18), Color: theme.TextSecondary},
			}}},
		}})
	}
	return woxwidget.Grid{Width: width, Columns: columns, ColumnGap: theme.Scaled(24), RowGap: theme.Scaled(20), Children: children}
}

// cloudIntroIcon keeps informational glyphs quiet rather than framing them like controls.
func cloudIntroIcon(icon *woxui.Image, fallback string, size, iconSize float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	size, iconSize = theme.Scaled(size), theme.Scaled(iconSize)
	var mark woxwidget.Widget = woxwidget.Text{Value: fallback, Style: woxui.TextStyle{Size: iconSize}, Color: theme.TextSecondary}
	if icon != nil {
		mark = woxwidget.Image{Source: icon, Width: iconSize, Height: iconSize}
	}
	return woxwidget.Align{Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5, Child: mark}
}

// cloudPlanComparison uses Settings table tokens while retaining intrinsic rows for wrapped copy.
func cloudPlanComparison(props CloudIntroProps, width float32, compact bool, theme woxcomponent.ControlTheme) woxwidget.Widget {
	style := newTableSurfaceStyle(theme)
	children := []woxwidget.Widget{cloudPlanHeader(props, width, compact, theme)}
	for index, row := range props.PlanRows {
		divider := style.rowDivider
		if index == 0 {
			divider = style.border
		}
		children = append(children, woxwidget.Container{Width: width, Height: tableSurfaceBorderWidth, Color: divider}, cloudPlanRow(row, width, compact, theme))
	}
	return woxwidget.Container{Width: width, Radius: woxcomponent.SettingsTableRadius, Color: style.bodyBackground, BorderColor: style.border, BorderWidth: tableSurfaceBorderWidth, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

// cloudPlanHeader uses the same column allocation as the comparison rows.
func cloudPlanHeader(props CloudIntroProps, width float32, compact bool, theme woxcomponent.ControlTheme) woxwidget.Widget {
	style := newTableSurfaceStyle(theme)
	height := theme.Scaled(tableSurfaceHeaderHeight)
	children := []woxwidget.Widget{}
	if !compact {
		children = append(children, woxwidget.Container{Width: theme.Scaled(132)})
	}
	for _, label := range []string{props.FreeLabel, props.ProLabel} {
		// AlignmentY centers within the line box, so the single line must span the header row.
		children = append(children, woxwidget.Expanded{Child: woxwidget.TextBlock{Value: label, Height: height, LineHeight: height, MaxLines: 1, AlignmentY: 0.5, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.TableHeaderFontSize)}, Color: style.headerText}})
	}
	return woxwidget.Container{Width: width, Child: woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Painter{Width: width, Height: height, Paint: func(list *woxui.DisplayList, bounds woxui.Rect) {
			// Clip the lower corners outside the header; only the table's top corners are rounded.
			list.PushClipRect(bounds)
			bounds.Height += woxcomponent.SettingsTableRadius
			list.FillRoundedRect(bounds, woxcomponent.SettingsTableRadius, style.headerBackground)
			list.PopClipRect()
		}}},
		{Child: woxwidget.Container{Width: width, Padding: woxwidget.Insets{Left: theme.Scaled(12), Right: theme.Scaled(12)}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(16), Children: children}}},
	}}}
}

// cloudPlanRow allows full price and entitlement text to wrap, including narrow layouts.
func cloudPlanRow(row CloudPlanRowProps, width float32, compact bool, theme woxcomponent.ControlTheme) woxwidget.Widget {
	label := woxwidget.TextBlock{Value: row.Label, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, LineHeight: theme.Scaled(18), Color: theme.TextSecondary}
	values := []woxwidget.Widget{}
	for _, value := range []string{row.FreeValue, row.ProValue} {
		values = append(values, woxwidget.Expanded{Child: woxwidget.TextBlock{Value: value, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize)}, LineHeight: theme.Scaled(20), Color: theme.Text}})
	}
	var content woxwidget.Widget
	if compact {
		content = woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(8), Children: []woxwidget.Widget{label, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(16), Children: values}}}
	} else {
		content = woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(16), Children: append([]woxwidget.Widget{woxwidget.Container{Width: theme.Scaled(132), Child: label}}, values...)}
	}
	return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Left: theme.Scaled(12), Right: theme.Scaled(12), Top: theme.Scaled(10), Bottom: theme.Scaled(10)}, Child: content}
}

func cloudAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// cloudAccountCard presents signed-in account and subscription controls with wrapping help text.
func cloudAccountCard(props CloudAccountProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	const labelGap = float32(32)
	availableWidth := max(float32(0), width)
	labelWidth := min(props.LabelWidth, max(float32(220), availableWidth-labelGap-220))
	if labelWidth <= 0 {
		labelWidth = max(float32(220), width-390)
	}
	valueWidth := max(float32(220), availableWidth-labelWidth-labelGap)
	planHelp := woxwidget.Semantics{
		Key: "cloud-plan-tooltip-key", AutomationID: "cloud-plan-tooltip", Role: woxui.AccessibilityRoleImage, Label: props.PlanLabel,
		Child: woxwidget.Gesture{ID: "cloud-plan-tooltip-hover", OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if props.OnPlanTooltip != nil {
				props.OnPlanTooltip(inside, bounds)
			}
		}, Child: woxwidget.Image{Source: props.InfoIcon, Width: 14, Height: 14}},
	}
	return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: availableWidth, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: labelGap, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 34, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Text{Value: props.EmailLabel, Style: woxui.TextStyle{Size: theme.Scaled(13), Weight: woxui.FontWeightSemibold}, Color: theme.Text}},
			cloudValueAction("cloud-account-action", props.Email, valueWidth, props.OnOpenAccountMenu, theme),
		}}},
		cloudHelpField(woxcomponent.SettingFieldProps{
			Label: props.PlanLabel, Description: props.PlanTips, Width: availableWidth, LabelWidth: labelWidth, Gap: labelGap,
			LabelAccessory: planHelp, Child: cloudValueAction("cloud-plan-action", props.PlanStatus, valueWidth, props.OnOpenSubscriptionMenu, theme), Theme: theme,
		}),
		cloudHelpField(woxcomponent.SettingFieldProps{
			Label: props.BillingLabel, Description: props.BillingTips, Width: availableWidth, LabelWidth: labelWidth, Gap: labelGap,
			Child: woxwidget.Align{Width: valueWidth, Height: 34, Horizontal: 1, Vertical: 0.5, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: "cloud-support", Label: props.SupportLabel, Icon: props.SupportIcon, IconSize: 16, Width: props.ActionWidth,
				Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnSupport, Theme: theme,
			})}, Theme: theme,
		}),
	}}}
}

// CloudPlanTooltipOverlay renders Flutter's rich Free and Pro comparison and returns its window-relative placement.
func CloudPlanTooltipOverlay(props CloudIntroProps, anchor woxui.Rect, windowWidth, windowHeight float32, theme woxcomponent.ControlTheme) (woxwidget.Widget, float32, float32) {
	const tableWidth = float32(560)
	const tooltipPadding = float32(10)
	const tooltipMargin = float32(12)
	const tooltipGap = float32(24)
	const tableHeight = float32(240)
	tooltipWidth := tableWidth + tooltipPadding*2
	tooltipHeight := tableHeight + tooltipPadding*2
	left := min(max(tooltipMargin, anchor.X+anchor.Width/2-tooltipWidth/2), max(tooltipMargin, windowWidth-tooltipWidth-tooltipMargin))
	top := anchor.Y + anchor.Height + tooltipGap
	if top+tooltipHeight+tooltipMargin > windowHeight {
		top = max(tooltipMargin, anchor.Y-tooltipGap-tooltipHeight)
	}
	panel := woxwidget.Semantics{
		Key: "cloud-plan-tooltip-overlay", AutomationID: "cloud-plan-tooltip-overlay", Role: woxui.AccessibilityRoleGroup, Label: props.FreeLabel + " / " + props.ProLabel,
		Child: woxwidget.Container{
			Width: tooltipWidth, Height: tooltipHeight, Radius: 8, Floating: true, Color: theme.Surface, BorderColor: cloudAlpha(theme.TextSecondary, 112), BorderWidth: 1,
			Padding: woxwidget.UniformInsets(tooltipPadding), Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
				Key: "cloud-plan-comparison-scroll", Width: tableWidth, Height: tableHeight, Theme: theme,
				Content: cloudPlanComparison(props, tableWidth, false, theme),
			}),
		},
	}
	return panel, left, top
}

// cloudValueAction right-aligns an account value and the shared dropdown indicator on one center line.
func cloudValueAction(id, value string, width float32, onTap func(), theme woxcomponent.ControlTheme) woxwidget.Widget {
	hoverBackground := theme.TextSecondary
	hoverBackground.A = uint8(float32(hoverBackground.A) * 0.1)
	return woxwidget.Align{Width: width, Height: 34, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 6, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: value, Style: woxui.TextStyle{Size: theme.Scaled(13), Weight: woxui.FontWeightSemibold}, Color: theme.Text},
			woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
				ID: id, Label: value, Icon: woxcomponent.WoxDropdownIndicator(28, 28, cloudAlpha(theme.Text, 194)), Width: 28, Height: 28, Radius: 6,
				HoverBackground: hoverBackground, FocusRingColor: theme.Focus, OnTap: onTap,
			}),
		},
	}}
}

// cloudSyncCard separates state from detail and lets errors wrap without clipping.
func cloudSyncCard(props CloudSyncProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: theme.Scaled(8), Bottom: theme.Scaled(20)}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: theme.Scaled(12), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Image{Source: props.StatusIcon, Width: theme.Scaled(24), Height: theme.Scaled(24)},
			woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(4), Children: []woxwidget.Widget{
				woxwidget.TextBlock{Value: props.Label, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.Text},
				woxwidget.TextBlock{Value: props.Detail, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, LineHeight: theme.Scaled(18), Color: props.Color},
			}}},
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-sync", Label: props.ButtonLabel, Icon: props.ButtonIcon, IconSize: 16, Width: props.ActionWidth, Disabled: !props.ButtonEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnSync, Theme: theme}),
		},
	}}
}

// cloudDeviceHeader centers refresh against the complete title and help block below the divider.
func cloudDeviceHeader(props CloudDevicesProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	refresh := woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: "cloud-refresh", Label: props.RefreshLabel, Icon: props.RefreshIcon, IconSize: 16, Width: props.ActionWidth,
		Disabled: !props.RefreshEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnRefresh, Theme: theme,
	})
	label, size := woxcomponent.SettingsChromeLabel(fmt.Sprintf("%s · %d", props.SectionLabel, len(props.Items)))
	return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 1, Color: cloudAlpha(theme.ChromeText, 26)},
		woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: theme.Scaled(16), Bottom: theme.Scaled(12)}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: theme.Scaled(16), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
				woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(12), Children: []woxwidget.Widget{
					woxwidget.TextBlock{Value: label, Style: woxui.TextStyle{Size: theme.Scaled(size), Weight: woxui.FontWeightSemibold}, Color: theme.TextSecondary},
					woxwidget.TextBlock{Value: props.Tips, LineHeight: theme.Scaled(18), Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary},
				}}},
				refresh,
			},
		}},
	}}}
}

// cloudHelpField is a settings row whose help text wraps and keeps a gap before the next title.
func cloudHelpField(props woxcomponent.SettingFieldProps) woxwidget.Widget {
	props.Height = woxcomponent.SettingsRowHeight
	props.Padding = woxwidget.Insets{Bottom: cloudHelpRowBottom}
	return woxcomponent.WoxSettingField(props)
}

// cloudDeviceCard stacks activity beneath device details when the settings pane is narrow.
func cloudDeviceCard(props CloudDevicesProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	stacked := width < theme.Scaled(520)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	dateWidth := theme.Scaled(160)
	// Reserve one action column so free-plan revoke buttons cannot shift activity dates.
	actionWidth := float32(0)
	for _, item := range props.Items {
		if item.ShowRevoke {
			actionWidth = max(actionWidth, theme.Scaled(cloudFormButtonWidth(item.RevokeLabel, false)))
		}
	}
	for _, item := range props.Items {
		nameChildren := []woxwidget.Widget{woxwidget.Flexible{Child: woxwidget.TextBlock{Value: item.Name, MaxLines: 1, ShrinkWrap: true, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsLabelFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.Text}}}
		if item.CurrentLabel != "" {
			nameChildren = append(nameChildren, woxcomponent.WoxTag(item.CurrentLabel, theme.TextSecondary, theme))
		}
		labels := []woxwidget.Widget{
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(8), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: nameChildren},
			woxwidget.Text{Value: item.Detail, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary},
		}
		if stacked {
			labels = append(labels, woxwidget.TextBlock{Value: props.LastActiveLabel + " · " + item.LastSeen, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary})
		}
		children := []woxwidget.Widget{
			woxwidget.Image{Source: item.Icon, Width: theme.Scaled(24), Height: theme.Scaled(24)},
			woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: theme.Scaled(4), Children: labels}},
		}
		if !stacked {
			children = append(children, woxwidget.Align{Width: dateWidth, Height: theme.Scaled(40), Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{Value: item.LastSeen, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary}})
		}
		if actionWidth > 0 {
			var action woxwidget.Widget = woxwidget.Container{Width: actionWidth}
			if item.ShowRevoke {
				action = woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: item.ID, Label: item.RevokeLabel, Width: actionWidth, Disabled: !item.RevokeEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: item.OnRevoke, Theme: theme})
			}
			children = append(children, action)
		}
		row := woxwidget.Container{Width: width, Padding: woxwidget.Insets{Top: theme.Scaled(8), Bottom: theme.Scaled(12)}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: theme.Scaled(12), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}}
		rows = append(rows, row)
	}
	if len(props.Items) == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Padding: woxwidget.UniformInsets(theme.Scaled(12)), Child: woxwidget.Text{Value: props.EmptyLabel, Style: woxui.TextStyle{Size: theme.Scaled(woxcomponent.SettingsSecondaryFontSize)}, Color: theme.TextSecondary}})
	}
	return woxwidget.Container{Width: width, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

// cloudPluginExclusionsCard owns the bounded exclusion list and its scroll surface.
func cloudPluginExclusionsCard(props CloudPluginExclusionsProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	rows := make([]FormTableRow, 0, len(props.Items))
	for index, item := range props.Items {
		rows = append(rows, FormTableRow{Index: index, Cells: []FormTableCell{{Text: item.Name, Icon: item.Icon, IconSize: 18}}})
	}
	return FormTableField(FormTableFieldProps{
		ID: "cloud-plugin-exclusions", Title: props.SectionLabel, Description: props.Tips, Width: width, Height: height, MaxHeight: 260, InlineTitle: true,
		Columns: []FormTableColumn{{Label: props.ColumnLabel}}, Rows: rows, HideEditAction: true, HideCloneAction: true,
		AddLabel: props.AddLabel, OperationLabel: props.OperationLabel,
		AddIcon: props.AddIcon, DeleteIcon: props.DeleteIcon,
		HeaderWeight: woxui.FontWeightSemibold, Theme: theme, OnAdd: props.OnAdd,
		OnDeleteRow: func(index int) {
			if index >= 0 && index < len(props.Items) {
				props.Items[index].OnDelete()
			}
		},
	})
}

// cloudConfigNotesCard renders translated platform sync caveats.
func cloudConfigNotesCard(props CloudConfigNotesProps, width, height float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	rows := make([]FormTableRow, 0, len(props.Items))
	for index, note := range props.Items {
		rows = append(rows, FormTableRow{Index: index, Cells: []FormTableCell{{Text: note.Item}, {Text: note.Mode, Tooltip: note.Tooltip}}})
	}
	return FormTableField(FormTableFieldProps{
		ID: "cloud-config-notes", Title: props.SectionLabel, Description: props.Tips, Width: width, Height: height, MaxHeight: 720, InlineTitle: true, ReadOnly: true,
		Columns: []FormTableColumn{{Label: props.ItemLabel}, {Label: props.ModeLabel, Width: 220}}, Rows: rows,
		InfoIcon: props.InfoIcon, HeaderWeight: woxui.FontWeightSemibold, Theme: theme, OnTooltip: props.OnTooltip,
	})
}

// cloudActionMenu renders the active account or subscription menu.
func cloudActionMenu(props CloudActionMenuProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	contentHeight := float32(len(props.Items)) * 40
	height := min(float32(420), contentHeight+12)
	return woxwidget.Container{Width: width, Height: height, Radius: 8, Floating: true, Color: theme.Surface, BorderColor: theme.Border, BorderWidth: 1, Padding: woxwidget.UniformInsets(6), Child: woxwidget.LayoutBuilder{Build: func(size woxui.Size) woxwidget.Widget {
		rows := make([]woxwidget.Widget, 0, len(props.Items))
		for _, item := range props.Items {
			radius := float32(5)
			background := theme.Surface
			hoverBackground := cloudAlpha(theme.TextSecondary, 25)
			rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
				ID: item.ID, Label: item.Label, Width: size.Width, Height: 40, Radius: &radius,
				Background: &background, HoverBackground: &hoverBackground, OnTap: item.OnTap, Theme: theme,
				Padding: woxwidget.Insets{Left: 12, Right: 12},
				Child:   woxwidget.Align{Height: 40, Vertical: 0.5, Child: woxwidget.Text{Value: item.Label, Style: woxui.TextStyle{Size: theme.Scaled(12)}, Color: theme.Text}},
			}))
		}
		return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: "cloud-action-menu-scroll", Width: size.Width, Height: size.Height,
			Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}, Theme: theme, ThumbColor: theme.Text,
		})
	}}}
}

// CloudPluginExclusionDialog mirrors Flutter's generic table-row update dialog for one select column.
func CloudPluginExclusionDialog(props CloudPluginExclusionDialogProps) woxwidget.Widget {
	panelWidth := props.PanelWidth
	if panelWidth <= 0 {
		panelWidth = 648
	}
	panelHeight := props.PanelHeight
	if panelHeight <= 0 {
		panelHeight = CloudPluginExclusionDialogHeight
	}
	contentWidth := max(float32(0), panelWidth-48)
	field := FormTableRowField(FormTableRowFieldProps{
		ID: "cloud-plugin-exclusion-field", Kind: "select", Label: props.FieldLabel, Description: props.Description,
		DescriptionMarkdown: true, Value: props.SelectedName, Width: contentWidth, Height: 60, LabelWidth: 60, Focused: true,
		SelectIcon: props.SelectedIcon, Theme: props.Theme, Window: props.Window,
		OnTap: func() {
			if props.OnChoiceTap != nil {
				props.OnChoiceTap(props.ChoiceAnchor)
			}
		},
		OnChoiceTap: props.OnChoiceTap,
	})
	actions := settingsDialogActions(contentWidth, props.Theme,
		settingsDialogAction{ID: "cloud-plugin-exclusion-cancel", Label: props.CancelLabel, OnTap: props.OnCancel},
		settingsDialogAction{ID: "cloud-plugin-exclusion-save", Label: props.SaveLabel, Disabled: props.Selected == "", OnTap: props.OnSave},
	)
	dialog := woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "cloud-plugin-exclusion-dialog", Label: props.FieldLabel, Width: panelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "cloud-plugin-exclusion-backdrop", BackdropAlpha: 205,
		Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}, Radius: 20,
		BorderColor: cloudAlpha(props.Theme.TextSecondary, 104), BorderWidth: 1, InitialFocus: woxwidget.Key("cloud-plugin-exclusion-field"), OnEscape: props.OnCancel, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: []woxwidget.Widget{field, actions}},
	})
	if !props.ChoiceOpen {
		return dialog
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: dialog},
		{Child: SettingsChoiceView(SettingsChoiceProps{
			ID: "cloud-plugin-exclusion-choice", Width: props.Width, Height: props.Height, Anchor: props.ChoiceAnchor,
			Theme: props.Theme, Window: props.Window, CurrentValue: props.Selected, Choices: props.Choices,
			OnChoose: props.OnChoose, OnCancel: props.OnCancel,
		})},
	}}
}

// CloudFormOverlayProps contains cloud account form data and actions.
type CloudFormOverlayProps struct {
	Width         float32
	Height        float32
	PanelWidth    float32
	Title         string
	Description   string
	Fields        []CloudFormFieldProps
	LinkPrefix    string
	Links         []CloudFormLinkProps
	FieldLink     *CloudFormLinkProps
	Feedback      string
	FeedbackColor woxui.Color
	CancelLabel   string
	SubmitLabel   string
	SubmitEnabled bool
	Saving        bool
	Theme         woxcomponent.ControlTheme
	OnCancel      func()
	OnSubmit      func()
}

// CloudFormFieldProps contains one credential field's render state and controller callback.
type CloudFormFieldProps struct {
	ID            string
	Kind          string
	Label         string
	Checked       bool
	State         woxui.TextEditingState
	Focused       bool
	Autofocus     bool
	Protected     bool
	Window        *woxui.Window
	Controller    *woxwidget.TextEditingController
	FocusNode     *woxwidget.FocusNode
	OnChanged     func(string)
	OnFocusChange func(bool)
	OnTap         func()
}

// CloudFormLinkProps contains one secondary account or legal action.
type CloudFormLinkProps struct {
	ID    string
	Label string
	Width float32
	OnTap func()
}

// CloudFormOverlay builds the cloud credential modal and its field rows.
func CloudFormOverlay(props CloudFormOverlayProps) woxwidget.Widget {
	innerWidth := props.PanelWidth - 48
	rows := make([]woxwidget.Widget, 0, len(props.Fields))
	rowsHeight := float32(0)
	for index, field := range props.Fields {
		if len(rows) > 0 {
			rowsHeight += 12
		}
		if field.Kind == "checkbox" {
			rows = append(rows, cloudFormCheckbox(field, innerWidth, props.Theme))
			rowsHeight += 24
			continue
		}
		var trailingLink *CloudFormLinkProps
		if index == len(props.Fields)-1 {
			trailingLink = props.FieldLink
		}
		rows = append(rows, cloudFormTextField(field, trailingLink, innerWidth, props.Saving, props.Theme))
		rowsHeight += 57
	}

	linkHeight := float32(0)
	var links woxwidget.Widget = woxwidget.Painter{Width: innerWidth}
	if len(props.Links) > 0 {
		linkHeight = 32
		linkChildren := make([]woxwidget.Widget, 0, len(props.Links)+1)
		if props.LinkPrefix != "" && !cloudFormHasCheckbox(props.Fields) {
			linkChildren = append(linkChildren, woxwidget.Text{Value: props.LinkPrefix, Style: woxui.TextStyle{Size: props.Theme.Scaled(11)}, Color: props.Theme.TextSecondary})
		}
		for _, link := range props.Links {
			linkChildren = append(linkChildren, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: link.ID, Label: link.Label, Width: link.Width, Radius: 4, Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: 12,
				Disabled: props.Saving, Variant: woxcomponent.ButtonText, OnTap: link.OnTap, Theme: props.Theme,
			}))
		}
		links = woxwidget.Container{Width: innerWidth, Height: linkHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: linkChildren}}
	}

	content := make([]woxwidget.Widget, 0, 10)
	contentHeight := float32(0)
	appendContent := func(widget woxwidget.Widget, height, gap float32) {
		if len(content) > 0 && gap > 0 {
			content = append(content, woxwidget.Painter{Width: innerWidth, Height: gap})
			contentHeight += gap
		}
		content = append(content, widget)
		contentHeight += height
	}
	appendContent(woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: props.Theme.Scaled(16), Weight: woxui.FontWeightSemibold}, Color: props.Theme.Text}, 20, 0)
	if props.Description != "" {
		appendContent(woxwidget.TextBlock{Value: props.Description, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: props.Theme.Scaled(11)}, LineHeight: 17, Color: props.Theme.TextSecondary}, 34, 12)
	}
	appendContent(woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: rows}, rowsHeight, 24)
	if linkHeight > 0 {
		appendContent(links, linkHeight, 10)
	}
	if props.Feedback != "" {
		appendContent(woxwidget.TextBlock{Value: props.Feedback, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: props.Theme.Scaled(11)}, LineHeight: 17, Color: props.FeedbackColor}, 34, 10)
	}

	actions := settingsDialogActions(innerWidth, props.Theme,
		settingsDialogAction{ID: "cloud-form-cancel", Label: props.CancelLabel, Disabled: props.Saving, OnTap: props.OnCancel},
		settingsDialogAction{ID: "cloud-form-submit", Label: props.SubmitLabel, Disabled: !props.SubmitEnabled, OnTap: props.OnSubmit},
	)
	appendContent(actions, SettingsDialogActionsHeight, 12)

	panelHeight := min(contentHeight+48, props.Height-56)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "cloud-form-dialog", Label: props.Title, Width: props.PanelWidth, Height: panelHeight, InitialFocus: "cloud-form-field-0",
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "cloud-form-backdrop", BackdropColor: woxui.Color{R: 0, G: 0, B: 0, A: 112},
		Radius: 20, Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}, BorderColor: props.Theme.Border, BorderWidth: 1, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: content},
	})
}

// cloudFormTextField renders Flutter's label-above-input account field.
func cloudFormTextField(field CloudFormFieldProps, trailingLink *CloudFormLinkProps, width float32, disabled bool, theme woxcomponent.ControlTheme) woxwidget.Widget {
	focused := field.Focused
	if field.FocusNode != nil {
		focused = field.FocusNode.HasFocus()
	}
	border := theme.TextSecondary
	if focused {
		border = theme.Text
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: field.ID, Label: field.Label, Width: width, Height: 34, Radius: 4, Padding: woxwidget.Insets{Left: 8, Top: 7, Right: 8, Bottom: 6}, Transparent: true,
		BorderColor: border, BorderWidth: 1, Style: woxui.TextStyle{Size: theme.Scaled(13)}, Value: field.State.Text, Focused: focused, Autofocus: field.Autofocus, Protected: field.Protected,
		MaxLines: 1, Window: field.Window, Theme: theme, Controller: field.Controller, FocusNode: field.FocusNode, OnChanged: field.OnChanged, OnFocusChange: field.OnFocusChange,
	})
	var label woxwidget.Widget = woxwidget.Text{Value: field.Label, Style: woxui.TextStyle{Size: theme.Scaled(12), Weight: woxui.FontWeightSemibold}, Color: theme.Text}
	if trailingLink != nil {
		label = woxwidget.Flex{Axis: woxwidget.Horizontal, MainAxisAlignment: woxwidget.MainAxisSpaceBetween, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
			woxwidget.Text{Value: field.Label, Style: woxui.TextStyle{Size: theme.Scaled(12), Weight: woxui.FontWeightSemibold}, Color: theme.Text},
			woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: trailingLink.ID, Label: trailingLink.Label, Radius: 4, Padding: woxwidget.Insets{Left: 1, Right: 1}, FontSize: 11,
				Disabled: disabled, Variant: woxcomponent.ButtonText, OnTap: trailingLink.OnTap, Theme: theme,
			}),
		}}
	}
	return woxwidget.Container{Width: width, Height: 57, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		label,
		input,
	}}}
}

// cloudFormCheckbox renders the legal-consent field as a real checkbox instead of an On/Off value row.
func cloudFormCheckbox(field CloudFormFieldProps, width float32, theme woxcomponent.ControlTheme) woxwidget.Widget {
	var mark woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if field.Checked {
		mark = woxwidget.Align{Width: 16, Height: 16, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "✓", Style: woxui.TextStyle{Size: theme.Scaled(12), Weight: woxui.FontWeightSemibold}, Color: theme.Text,
		}}
	}
	outline := theme.TextSecondary
	if field.Focused {
		outline = theme.Text
	}
	checkbox := woxwidget.Container{Width: 18, Height: 18, Radius: 3, BorderColor: outline, BorderWidth: 1, Padding: woxwidget.UniformInsets(1), Child: mark}
	return woxwidget.Gesture{ID: field.ID, OnTap: field.OnTap, Child: woxwidget.Container{Width: width, Height: 24, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		checkbox,
		woxwidget.Expanded{Child: woxwidget.Align{Height: 24, Vertical: 0.5, Child: woxwidget.Text{Value: field.Label, Style: woxui.TextStyle{Size: theme.Scaled(12)}, Color: theme.Text}}},
	}}}}
}

func cloudFormHasCheckbox(fields []CloudFormFieldProps) bool {
	for _, field := range fields {
		if field.Kind == "checkbox" {
			return true
		}
	}
	return false
}

func cloudFormButtonWidth(label string, hasLeadingIcon bool) float32 {
	base := float32(len([]rune(label)))*8 + 40
	if hasLeadingIcon {
		base += 24
	}
	return max(float32(66), base)
}
