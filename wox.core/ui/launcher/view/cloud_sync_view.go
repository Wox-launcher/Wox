package view

import (
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
	Theme        woxcomponent.Theme
	OnCloseMenu  func()
}

// CloudIntroProps contains the signed-out product summary and plan comparison.
type CloudIntroProps struct {
	SectionLabel     string
	Headline         string
	Description      string
	HeroIcon         *woxui.Image
	HeroFallback     string
	Features         []CloudIntroFeatureProps
	FreeLabel        string
	ProLabel         string
	RecommendedLabel string
	PlanRows         []CloudPlanRowProps
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
	SectionLabel           string
	LoggedIn               bool
	LoginLabel             string
	RegisterLabel          string
	EmailLabel             string
	Email                  string
	EmailTextWidth         float32
	PlanLabel              string
	PlanTips               string
	PlanStatus             string
	PlanStatusTextWidth    float32
	BillingLabel           string
	BillingTips            string
	SupportLabel           string
	ActionsEnabled         bool
	OnLogin                func()
	OnRegister             func()
	OnOpenAccountMenu      func()
	OnOpenSubscriptionMenu func()
	OnSupport              func()
}

// CloudSyncProps contains sync status presentation and its primary action.
type CloudSyncProps struct {
	SectionLabel  string
	StatusLabel   string
	Label         string
	Detail        string
	Color         woxui.Color
	ButtonLabel   string
	ButtonEnabled bool
	OnSync        func()
}

// CloudDevicesProps contains device rows and refresh state.
type CloudDevicesProps struct {
	SectionLabel   string
	RefreshLabel   string
	RefreshEnabled bool
	EmptyLabel     string
	Items          []CloudDeviceProps
	OnRefresh      func()
}

// CloudDeviceProps contains one device row and optional revoke action.
type CloudDeviceProps struct {
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
	SectionLabel string
	Tips         string
	EmptyLabel   string
	Items        []CloudPluginExclusionProps
}

// CloudPluginExclusionProps contains one plugin exclusion toggle row.
type CloudPluginExclusionProps struct {
	ID            string
	Name          string
	PluginID      string
	ButtonLabel   string
	ButtonEnabled bool
	Excluded      bool
	OnToggle      func()
}

// CloudConfigNotesProps contains translated configuration caveats.
type CloudConfigNotesProps struct {
	SectionLabel string
	Items        []string
}

// CloudActionMenuProps contains a positioned account or subscription action menu.
type CloudActionMenuProps struct {
	Top   float32
	Items []CloudActionMenuItemProps
}

// CloudActionMenuItemProps contains one cloud menu action.
type CloudActionMenuItemProps struct {
	ID    string
	Label string
	OnTap func()
}

// CloudSettingsPage builds the complete scrollable cloud settings route.
func CloudSettingsPage(props CloudSettingsPageProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	children := make([]woxwidget.Widget, 0, 12)
	contentHeight := float32(0)
	appendChild := func(widget woxwidget.Widget, height float32) {
		if len(children) > 0 {
			contentHeight += 4
		}
		children = append(children, widget)
		contentHeight += height
	}

	appendChild(woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{
		Title: props.Title, Description: props.Description, Width: contentWidth, Theme: props.Theme,
	}), woxcomponent.PageHeaderHeight)
	if !props.Account.LoggedIn {
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Intro.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
		intro, introHeight := cloudIntro(props.Intro, contentWidth, props.Theme)
		appendChild(intro, introHeight)
	}
	appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Account.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
	accountHeight := float32(62)
	if props.Account.LoggedIn {
		accountHeight = 190
	}
	appendChild(cloudAccountCard(props.Account, contentWidth, accountHeight, props.Theme), accountHeight)

	if props.Account.LoggedIn {
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Sync.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
		appendChild(cloudSyncCard(props.Sync, contentWidth, props.Theme), 118)
		refresh := woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: "cloud-refresh", Label: props.Devices.RefreshLabel, Width: 104, Disabled: !props.Devices.RefreshEnabled,
			Variant: woxcomponent.ButtonSecondary, OnTap: props.Devices.OnRefresh, Theme: props.Theme,
		})
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{
			Label: props.Devices.SectionLabel, Width: contentWidth, Action: refresh, ActionWidth: 104, Theme: props.Theme,
		}), 43)
		deviceHeight := float32(len(props.Devices.Items)) * 62
		if len(props.Devices.Items) == 0 {
			deviceHeight = 72
		}
		appendChild(cloudDeviceCard(props.Devices, contentWidth, deviceHeight, props.Theme), deviceHeight)
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Plugins.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
		appendChild(cloudPluginExclusionsCard(props.Plugins, contentWidth, 282, props.Theme), 282)
		appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.ConfigNotes.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
		appendChild(cloudConfigNotesCard(props.ConfigNotes, contentWidth, 136, props.Theme), 136)
	}
	if props.Message != "" {
		appendChild(woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.TextBlock{
			Value: props.Message, Width: contentWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: props.MessageColor,
		}}, 34)
	}

	page := SettingsPage(SettingsPageProps{
		ID: "cloud-page-scroll", Width: props.Width, Height: props.Height, Children: children, ContentHeight: contentHeight,
		Gap: 4,
	})
	if props.ActionMenu == nil {
		return page
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: page},
		{Child: woxwidget.Gesture{ID: "cloud-action-menu-shade", OnTap: props.OnCloseMenu, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}},
		{Left: max(float32(20), props.Width-236), Top: props.ActionMenu.Top, Child: cloudActionMenu(*props.ActionMenu, props.Theme)},
	}}
}

// cloudIntro mirrors Flutter's signed-out hero, capability cards, and responsive plan table.
func cloudIntro(props CloudIntroProps, width float32, theme woxcomponent.Theme) (woxwidget.Widget, float32) {
	stacked := width < 760
	compactPlan := width < 620
	heroHeight := float32(56)
	featuresHeight := float32(76)
	if stacked {
		heroHeight = 128
		featuresHeight = float32(len(props.Features))*76 + float32(max(0, len(props.Features)-1))*10
	}
	planHeight := float32(40 + len(props.PlanRows)*40)
	if compactPlan {
		planHeight = float32(50 + len(props.PlanRows)*70)
	}
	contentHeight := float32(24) + heroHeight + 18 + featuresHeight + 20 + planHeight + 24

	hero := cloudIntroHero(props, width, heroHeight, stacked, theme)
	features := cloudIntroFeatures(props.Features, width, featuresHeight, stacked, theme)
	plans := cloudPlanComparison(props, width, planHeight, compactPlan, theme)
	return woxwidget.Container{Width: width, Height: contentHeight, Padding: woxwidget.Insets{Top: 24, Bottom: 24}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 0, Children: []woxwidget.Widget{
			hero,
			woxwidget.Painter{Width: width, Height: 18},
			features,
			woxwidget.Painter{Width: width, Height: 20},
			plans,
		},
	}}, contentHeight
}

// cloudIntroHero keeps the cloud mark beside the copy when space permits and stacks it on narrow pages.
func cloudIntroHero(props CloudIntroProps, width, height float32, stacked bool, theme woxcomponent.Theme) woxwidget.Widget {
	copyHeight := float32(56)
	copyWidth := max(float32(0), width-72)
	if stacked {
		copyHeight = 58
		copyWidth = width
	}
	copy := woxwidget.Container{Width: copyWidth, Height: copyHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Headline, Style: woxui.TextStyle{Size: 20, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
		woxwidget.TextBlock{Value: props.Description, Width: copyWidth, Height: 30, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: theme.ResultSubtitle},
	}}}
	icon := cloudIntroIcon(props.HeroIcon, props.HeroFallback, 56, 28, theme)
	if stacked {
		return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 14, Children: []woxwidget.Widget{icon, copy}}}
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{icon, copy}}}
}

// cloudIntroFeatures lays out the three capability cards responsively.
func cloudIntroFeatures(features []CloudIntroFeatureProps, width, height float32, stacked bool, theme woxcomponent.Theme) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(features))
	cardWidth := width
	if !stacked && len(features) > 0 {
		cardWidth = max(float32(0), (width-float32(len(features)-1)*10)/float32(len(features)))
	}
	for _, feature := range features {
		children = append(children, cloudIntroFeature(feature, cardWidth, theme))
	}
	axis := woxwidget.Horizontal
	if stacked {
		axis = woxwidget.Vertical
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: axis, Gap: 10, Children: children}}
}

// cloudIntroFeature builds one bordered capability summary.
func cloudIntroFeature(feature CloudIntroFeatureProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	contentWidth := max(float32(0), width-68)
	return woxwidget.Container{Width: width, Height: 76, Radius: 8, BorderColor: cloudAlpha(theme.PreviewSplit, 180), BorderWidth: 1, Padding: woxwidget.Insets{Left: 12, Top: 12, Right: 12, Bottom: 12}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			cloudIntroIcon(feature.Icon, feature.FallbackIcon, 34, 17, theme),
			woxwidget.Container{Width: contentWidth, Height: 52, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: feature.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.TextBlock{Value: feature.Description, Width: contentWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 16, Color: theme.ResultSubtitle},
			}}},
		},
	}}
}

// cloudIntroIcon applies the shared outlined icon treatment from the Flutter page.
func cloudIntroIcon(icon *woxui.Image, fallback string, size, iconSize float32, theme woxcomponent.Theme) woxwidget.Widget {
	var mark woxwidget.Widget = woxwidget.Text{Value: fallback, Style: woxui.TextStyle{Size: iconSize, Weight: woxui.FontWeightSemibold}, Color: cloudAlpha(theme.ResultTitle, 194)}
	if icon != nil {
		mark = woxwidget.Image{Source: icon, Width: iconSize, Height: iconSize}
	}
	return woxwidget.Container{Width: size, Height: size, Radius: 8, BorderColor: cloudAlpha(theme.PreviewSplit, 210), BorderWidth: 1, Child: woxwidget.Align{
		Width: size, Height: size, Horizontal: 0.5, Vertical: 0.5, Child: mark,
	}}
}

// cloudPlanComparison builds the responsive Free and Pro table used by Flutter.
func cloudPlanComparison(props CloudIntroProps, width, height float32, compact bool, theme woxcomponent.Theme) woxwidget.Widget {
	children := []woxwidget.Widget{cloudPlanHeader(props, width, compact, theme)}
	for _, row := range props.PlanRows {
		children = append(children, cloudPlanRow(row, width, compact, theme))
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 8, BorderColor: cloudAlpha(theme.PreviewSplit, 220), BorderWidth: 1, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Children: children,
	}}
}

// cloudPlanHeader builds the plan names and Pro recommendation badge.
func cloudPlanHeader(props CloudIntroProps, width float32, compact bool, theme woxcomponent.Theme) woxwidget.Widget {
	labelWidth := float32(132)
	horizontalPadding := float32(14)
	headerHeight := float32(40)
	topPadding := float32(10)
	if compact {
		labelWidth = 0
		horizontalPadding = 12
		headerHeight = 50
		topPadding = 15
	}
	valueWidth := max(float32(0), (width-horizontalPadding*2-labelWidth-10)/2)
	badgeWidth := max(float32(54), float32(len([]rune(props.RecommendedLabel)))*7+16)
	pro := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.ProLabel, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
		woxwidget.Container{Width: badgeWidth, Height: 18, Radius: 9, Color: woxui.Color{R: 11, G: 107, B: 211, A: 255}, Padding: woxwidget.Insets{Left: 8, Top: 3, Right: 8}, Child: woxwidget.Text{
			Value: props.RecommendedLabel, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		}},
	}}
	return woxwidget.Container{Width: width, Height: headerHeight, Padding: woxwidget.Insets{Left: horizontalPadding, Top: topPadding, Right: horizontalPadding}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Painter{Width: labelWidth, Height: 20},
			woxwidget.Container{Width: valueWidth, Height: 20, Child: woxwidget.Text{Value: props.FreeLabel, Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
			woxwidget.Container{Width: valueWidth, Height: 20, Child: pro},
		},
	}}
}

// cloudPlanRow builds one wide or compact comparison row.
func cloudPlanRow(row CloudPlanRowProps, width float32, compact bool, theme woxcomponent.Theme) woxwidget.Widget {
	rowHeight := float32(40)
	if compact {
		rowHeight = 70
	}
	content := cloudPlanWideRow(row, width, theme)
	if compact {
		content = cloudPlanCompactRow(row, width, theme)
	}
	return woxwidget.Stack{Width: width, Height: rowHeight, Children: []woxwidget.StackChild{
		{Child: woxwidget.Container{Width: width, Height: 1, Color: cloudAlpha(theme.PreviewSplit, 180)}},
		{Top: 1, Child: content},
	}}
}

// cloudPlanWideRow keeps labels and both plan values in three aligned columns.
func cloudPlanWideRow(row CloudPlanRowProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	const labelWidth = float32(132)
	const horizontalPadding = float32(14)
	valueWidth := max(float32(0), (width-horizontalPadding*2-labelWidth-10)/2)
	return woxwidget.Container{Width: width, Height: 39, Padding: woxwidget.Insets{Left: horizontalPadding, Top: 10, Right: horizontalPadding}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 22, Child: woxwidget.Text{Value: row.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle}},
			woxwidget.Container{Width: valueWidth, Height: 22, Child: woxwidget.TextBlock{Value: row.FreeValue, Width: valueWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultTitle}},
			woxwidget.Container{Width: valueWidth, Height: 22, Child: woxwidget.TextBlock{Value: row.ProValue, Width: valueWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
		},
	}}
}

// cloudPlanCompactRow moves the row label above the two plan values on narrow pages.
func cloudPlanCompactRow(row CloudPlanRowProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	const horizontalPadding = float32(12)
	valueWidth := max(float32(0), (width-horizontalPadding*2-10)/2)
	return woxwidget.Container{Width: width, Height: 69, Padding: woxwidget.Insets{Left: horizontalPadding, Top: 9, Right: horizontalPadding}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
			woxwidget.Text{Value: row.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ResultSubtitle},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.TextBlock{Value: row.FreeValue, Width: valueWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 13}, LineHeight: 16, Color: theme.ResultTitle},
				woxwidget.TextBlock{Value: row.ProValue, Width: valueWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, LineHeight: 16, Color: theme.ResultTitle},
			}},
		},
	}}
}

func cloudAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// cloudAccountCard switches between account entry points and signed-in details.
func cloudAccountCard(props CloudAccountProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	if !props.LoggedIn {
		const actionsWidth = float32(184)
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Top: 10, Right: 2, Bottom: 10}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Container{Width: max(float32(0), width-actionsWidth-4), Height: 42, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{
					Value: props.SectionLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle,
				}},
				woxwidget.Container{Width: actionsWidth, Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
					woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-login", Label: props.LoginLabel, Width: 88, Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnLogin, Theme: theme}),
					woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-register", Label: props.RegisterLabel, Width: 88, Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonOutline, OnTap: props.OnRegister, Theme: theme}),
				}}},
			},
		}}
	}
	labelWidth := max(float32(220), width-390)
	valueWidth := max(float32(220), width-labelWidth)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Right: 2}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width - 4, Height: 50, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 50, Padding: woxwidget.Insets{Top: 15}, Child: woxwidget.Text{Value: props.EmailLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
			cloudValueAction("cloud-account-action", props.Email, valueWidth, props.EmailTextWidth, props.OnOpenAccountMenu, theme),
		}}},
		woxwidget.Container{Width: width - 4, Height: 66, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 66, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.PlanLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.Text{Value: props.PlanTips, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
			}}},
			cloudValueAction("cloud-plan-action", props.PlanStatus, valueWidth, props.PlanStatusTextWidth, props.OnOpenSubscriptionMenu, theme),
		}}},
		woxwidget.Container{Width: width - 4, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.BillingLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.Text{Value: props.BillingTips, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle},
			}}},
			woxwidget.Container{Width: valueWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Painter{Width: max(float32(0), valueWidth-132), Height: 38},
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-support", Label: props.SupportLabel, Width: 132, Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnSupport, Theme: theme}),
			}}},
		}}},
	}}}
}

// cloudValueAction right-aligns an account value beside its menu affordance.
func cloudValueAction(id, value string, width, textWidth float32, onTap func(), theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: 50, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), width-textWidth-24), Height: 28},
		woxwidget.Container{Width: textWidth, Height: 28, Child: woxwidget.Text{Value: value, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle}},
		woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{Width: 24, Height: 28, Padding: woxwidget.Insets{Left: 8, Top: 2}, Child: woxwidget.Text{Value: "⌄", Style: woxui.TextStyle{Size: 12}, Color: theme.ResultSubtitle}}},
	}}}
}

// cloudSyncCard renders current sync state and its primary action.
func cloudSyncCard(props CloudSyncProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	labelWidth := max(float32(220), width-260)
	button := woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-sync", Label: props.ButtonLabel, Width: 102, Disabled: !props.ButtonEnabled, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSync, Theme: theme})
	return woxwidget.Container{Width: width, Height: 118, Padding: woxwidget.Insets{Left: 2, Top: 10, Right: 2, Bottom: 8}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 98, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.StatusLabel, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Color},
				woxwidget.TextBlock{Value: props.Detail, Width: labelWidth, Height: 48, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: theme.ResultSubtitle},
			}}},
			woxwidget.Container{Width: max(float32(0), width-labelWidth-42), Height: 52, Padding: woxwidget.Insets{Top: 14}, Child: button},
		},
	}}
}

// cloudDeviceCard renders device activity and optional revoke actions.
func cloudDeviceCard(props CloudDevicesProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, max(1, len(props.Items)))
	for _, item := range props.Items {
		action := woxwidget.Widget(woxwidget.Painter{Width: 104, Height: 38})
		if item.ShowRevoke {
			action = woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: item.ID, Label: item.RevokeLabel, Width: 96, Disabled: !item.RevokeEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: item.OnRevoke, Theme: theme})
		}
		labelWidth := max(float32(160), width-276)
		rows = append(rows, woxwidget.Container{Width: width, Height: 62, Padding: woxwidget.Insets{Left: 2, Top: 9, Right: 2}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: labelWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
					woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
					woxwidget.Text{Value: item.Detail, Style: woxui.TextStyle{Size: 10}, Color: theme.ResultSubtitle},
				}}},
				woxwidget.Container{Width: 150, Height: 44, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{Value: item.LastSeen, Style: woxui.TextStyle{Size: 10}, Color: theme.ResultSubtitle}},
				action,
			},
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: 56, Padding: woxwidget.Insets{Left: 2, Top: 18}, Child: woxwidget.Text{
			Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle,
		}})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

// cloudPluginExclusionsCard owns the bounded exclusion list and its scroll surface.
func cloudPluginExclusionsCard(props CloudPluginExclusionsProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	const bodyHeight = float32(206)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		labelWidth := max(float32(120), width-148)
		variant := woxcomponent.ButtonSecondary
		if item.Excluded {
			variant = woxcomponent.ButtonPrimary
		}
		rows = append(rows, woxwidget.Container{Width: width - 28, Height: 46, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 12, Top: 5, Right: 8}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: labelWidth, Height: 36, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
					woxwidget.Text{Value: item.Name, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
					woxwidget.Text{Value: item.PluginID, Style: woxui.TextStyle{Size: 8}, Color: theme.ResultSubtitle},
				}}},
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: item.ID, Label: item.ButtonLabel, Width: 96, Disabled: !item.ButtonEnabled, Variant: variant, OnTap: item.OnToggle, Theme: theme}),
			},
		}})
	}
	var body woxwidget.Widget
	if len(rows) == 0 {
		body = woxwidget.Container{Width: width - 28, Height: bodyHeight, Radius: 8, Color: theme.ToolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 18}, Child: woxwidget.Text{
			Value: props.EmptyLabel, Style: woxui.TextStyle{Size: 11}, Color: theme.ResultSubtitle,
		}}
	} else {
		body = woxwidget.ScrollView{
			Key: "cloud-plugin-scroll", ID: "cloud-plugin-scroll",
			Width: width - 28, Height: bodyHeight, ContentHeight: max(bodyHeight, float32(len(rows))*46),
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 26, Child: woxwidget.Text{Value: props.Tips, Style: woxui.TextStyle{Size: 10}, Color: theme.ResultSubtitle}},
		body,
	}}}
}

// cloudConfigNotesCard renders translated platform sync caveats.
func cloudConfigNotesCard(props CloudConfigNotesProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, note := range props.Items {
		rows = append(rows, woxwidget.Text{Value: "• " + note, Style: woxui.TextStyle{Size: 10}, Color: theme.ResultSubtitle})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Top: 10, Right: 2, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows}}
}

// cloudActionMenu renders the active account or subscription menu.
func cloudActionMenu(props CloudActionMenuProps, theme woxcomponent.Theme) woxwidget.Widget {
	const width = float32(196)
	rows := make([]woxwidget.Widget, 0, len(props.Items))
	for _, item := range props.Items {
		rows = append(rows, woxwidget.Gesture{ID: item.ID, OnTap: item.OnTap, Child: woxwidget.Container{
			Width: width - 12, Height: 40, Radius: 5, Color: theme.ActionBackground, Padding: woxwidget.Insets{Left: 12, Top: 11, Right: 12},
			Child: woxwidget.Text{Value: item.Label, Style: woxui.TextStyle{Size: 12}, Color: theme.ActionText},
		}})
	}
	return woxwidget.Container{Width: width, Height: float32(len(rows))*40 + 12, Radius: 8, Color: theme.ActionBackground, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
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
	Theme         woxcomponent.Theme
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
		linkHeight = 30
		linkChildren := make([]woxwidget.Widget, 0, len(props.Links)+1)
		if props.LinkPrefix != "" && !cloudFormHasCheckbox(props.Fields) {
			linkChildren = append(linkChildren, woxwidget.Text{Value: props.LinkPrefix, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ActionHeader})
		}
		for _, link := range props.Links {
			linkChildren = append(linkChildren, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: link.ID, Label: link.Label, Width: link.Width, Height: 30, Radius: 4, Padding: woxwidget.Insets{Left: 4, Right: 4}, FontSize: 12,
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
	appendContent(woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText}, 20, 0)
	if props.Description != "" {
		appendContent(woxwidget.TextBlock{Value: props.Description, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ActionHeader}, 34, 12)
	}
	appendContent(woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: rows}, rowsHeight, 24)
	if linkHeight > 0 {
		appendContent(links, linkHeight, 10)
	}
	if props.Feedback != "" {
		appendContent(woxwidget.TextBlock{Value: props.Feedback, Width: innerWidth, Height: 34, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.FeedbackColor}, 34, 10)
	}

	cancelWidth := cloudFormButtonWidth(props.CancelLabel)
	submitWidth := cloudFormButtonWidth(props.SubmitLabel)
	actions := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-cancelWidth-submitWidth-8), Height: 36},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-form-cancel", Label: props.CancelLabel, Width: cancelWidth, Height: 36, Radius: 4, FontSize: 13, Disabled: props.Saving, Variant: woxcomponent.ButtonOutline, OnTap: props.OnCancel, Theme: props.Theme}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-form-submit", Label: props.SubmitLabel, Width: submitWidth, Height: 36, Radius: 4, FontSize: 13, Disabled: !props.SubmitEnabled, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSubmit, Theme: props.Theme}),
	}}
	appendContent(actions, 36, 12)

	panelHeight := min(contentHeight+48, props.Height-56)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "cloud-form-dialog", Label: props.Title, Width: props.PanelWidth, Height: panelHeight, InitialFocus: "cloud-form-field-0",
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "cloud-form-backdrop", BackdropColor: woxui.Color{R: 0, G: 0, B: 0, A: 112},
		Radius: 20, Padding: woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}, BorderColor: props.Theme.PreviewSplit, BorderWidth: 1, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: content},
	})
}

// cloudFormTextField renders Flutter's label-above-input account field.
func cloudFormTextField(field CloudFormFieldProps, trailingLink *CloudFormLinkProps, width float32, disabled bool, theme woxcomponent.Theme) woxwidget.Widget {
	focused := field.Focused
	if field.FocusNode != nil {
		focused = field.FocusNode.HasFocus()
	}
	border := theme.ResultSubtitle
	if focused {
		border = theme.ActionText
	}
	input := woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: field.ID, Label: field.Label, Width: width, Height: 34, Radius: 4, Padding: woxwidget.Insets{Left: 8, Top: 7, Right: 8, Bottom: 6}, Transparent: true,
		BorderColor: border, BorderWidth: 1, Style: woxui.TextStyle{Size: 13}, Value: field.State.Text, Focused: focused, Autofocus: field.Autofocus, Protected: field.Protected,
		MaxLines: 1, Window: field.Window, Theme: theme, Controller: field.Controller, FocusNode: field.FocusNode, OnChanged: field.OnChanged, OnFocusChange: field.OnFocusChange,
	})
	var label woxwidget.Widget = woxwidget.Text{Value: field.Label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ActionText}
	if trailingLink != nil {
		linkWidth := min(trailingLink.Width, max(float32(0), width-100))
		label = woxwidget.Container{Width: width, Height: 17, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), width-linkWidth), Height: 17, Child: label},
			woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: trailingLink.ID, Label: trailingLink.Label, Width: linkWidth, Height: 17, Radius: 4, Padding: woxwidget.Insets{Left: 1, Right: 1}, FontSize: 11, Disabled: disabled, Variant: woxcomponent.ButtonText, OnTap: trailingLink.OnTap, Theme: theme}),
		}}}
	}
	return woxwidget.Container{Width: width, Height: 57, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		label,
		input,
	}}}
}

// cloudFormCheckbox renders the legal-consent field as a real checkbox instead of an On/Off value row.
func cloudFormCheckbox(field CloudFormFieldProps, width float32, theme woxcomponent.Theme) woxwidget.Widget {
	var mark woxwidget.Widget = woxwidget.Container{Width: 16, Height: 16}
	if field.Checked {
		mark = woxwidget.Align{Width: 16, Height: 16, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "✓", Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: theme.ActionText,
		}}
	}
	outline := theme.ResultSubtitle
	if field.Focused {
		outline = theme.ActionText
	}
	checkbox := woxwidget.Container{Width: 18, Height: 18, Radius: 3, BorderColor: outline, BorderWidth: 1, Padding: woxwidget.UniformInsets(1), Child: mark}
	return woxwidget.Gesture{ID: field.ID, OnTap: field.OnTap, Child: woxwidget.Container{Width: width, Height: 24, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		checkbox,
		woxwidget.Container{Width: max(float32(0), width-26), Height: 20, Padding: woxwidget.Insets{Top: 2}, Child: woxwidget.Text{Value: field.Label, Style: woxui.TextStyle{Size: 12}, Color: theme.ActionText}},
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

func cloudFormButtonWidth(label string) float32 {
	return max(float32(66), float32(len([]rune(label)))*8+40)
}
