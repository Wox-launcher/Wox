package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// CloudSettingsPageProps contains cloud settings data and controller callbacks.
type CloudSettingsPageProps struct {
	Width         float32
	Height        float32
	Title         string
	Description   string
	Account       CloudAccountProps
	Sync          CloudSyncProps
	Devices       CloudDevicesProps
	Plugins       CloudPluginExclusionsProps
	ConfigNotes   CloudConfigNotesProps
	Message       string
	MessageColor  woxui.Color
	Scroll        float32
	ActionMenu    *CloudActionMenuProps
	Theme         woxcomponent.Theme
	OnScroll      func(float32)
	OnSetGeometry func(float32, float32)
	OnCloseMenu   func()
}

// CloudAccountProps contains account presentation and actions.
type CloudAccountProps struct {
	SectionLabel           string
	LoggedIn               bool
	IntroDescription       string
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
	SectionLabel  string
	Tips          string
	EmptyLabel    string
	Items         []CloudPluginExclusionProps
	Scroll        float32
	OnScroll      func(float32)
	OnSetViewport func(float32)
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

	appendChild(woxwidget.Container{Width: contentWidth, Height: 94, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
		woxwidget.Text{Value: props.Description, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
	}}}, 94)
	appendChild(woxcomponent.WoxSectionHeader(woxcomponent.SectionHeaderProps{Label: props.Account.SectionLabel, Width: contentWidth, Theme: props.Theme}), 43)
	accountHeight := float32(142)
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
		Gap: 4, Scroll: props.Scroll, OnScroll: props.OnScroll, OnSetGeometry: props.OnSetGeometry,
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

// cloudAccountCard switches between account entry points and signed-in details.
func cloudAccountCard(props CloudAccountProps, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	if !props.LoggedIn {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Top: 12, Right: 2, Bottom: 12}, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Text{Value: props.SectionLabel, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: theme.ResultTitle},
				woxwidget.TextBlock{Value: props.IntroDescription, Width: width - 36, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: theme.ResultSubtitle},
				woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
					woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-login", Label: props.LoginLabel, Width: 96, Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnLogin, Theme: theme}),
					woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-register", Label: props.RegisterLabel, Width: 124, Disabled: !props.ActionsEnabled, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnRegister, Theme: theme}),
				}},
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
	if props.OnSetViewport != nil {
		props.OnSetViewport(bodyHeight)
	}
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
		body = woxwidget.Gesture{ID: "cloud-plugin-scroll", OnScroll: func(delta woxui.Point) {
			if props.OnScroll != nil {
				props.OnScroll(-delta.Y)
			}
		}, Child: woxwidget.ScrollView{
			Width: width - 28, Height: bodyHeight, ContentHeight: max(bodyHeight, float32(len(rows))*46), Offset: props.Scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
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
	Feedback      string
	FeedbackColor woxui.Color
	SubmitLabel   string
	Saving        bool
	Theme         woxcomponent.Theme
	OnCancel      func()
	OnSubmit      func()
}

// CloudFormFieldProps contains one credential field's render state and controller callback.
type CloudFormFieldProps struct {
	ID        string
	Kind      string
	Label     string
	Value     string
	State     woxui.TextEditingState
	Focused   bool
	Protected bool
	Window    *woxui.Window
	OnCaret   func(int)
	OnTap     func()
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
	innerWidth := props.PanelWidth - 36
	rows := make([]woxwidget.Widget, 0, len(props.Fields))
	for _, field := range props.Fields {
		if field.Kind == "checkbox" {
			rows = append(rows, FormValueField(FormValueFieldProps{
				ID: field.ID, Label: field.Label, Value: field.Value, Width: innerWidth, Height: 56,
				Focused: field.Focused, Theme: props.Theme, OnTap: field.OnTap,
			}))
			continue
		}
		state := field.State
		if field.Protected {
			state.Text = strings.Repeat("•", len([]rune(state.Text)))
			state.Composition = strings.Repeat("•", len([]rune(state.Composition)))
		}
		rows = append(rows, FormTextField(FormTextFieldProps{
			ID: field.ID, Label: field.Label, Width: innerWidth, Height: 56, State: state, Focused: field.Focused,
			Protected: field.Protected, MaxLines: 1, Window: field.Window, Theme: props.Theme, OnCaret: field.OnCaret,
		}))
	}

	linkHeight := float32(0)
	var links woxwidget.Widget = woxwidget.Painter{Width: innerWidth}
	if len(props.Links) > 0 {
		linkHeight = 38
		linkChildren := make([]woxwidget.Widget, 0, len(props.Links)+1)
		if props.LinkPrefix != "" {
			linkChildren = append(linkChildren, woxwidget.Text{Value: props.LinkPrefix, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ActionHeader})
		}
		for _, link := range props.Links {
			linkChildren = append(linkChildren, woxcomponent.WoxButton(woxcomponent.ButtonProps{
				ID: link.ID, Label: link.Label, Width: link.Width, Disabled: props.Saving, Variant: woxcomponent.ButtonSecondary, OnTap: link.OnTap, Theme: props.Theme,
			}))
		}
		links = woxwidget.Container{Width: innerWidth, Height: linkHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: linkChildren}}
	}

	panelHeight := 36 + 44 + float32(len(rows))*56 + linkHeight + 42 + 48
	panelHeight = min(panelHeight, props.Height-36)
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{
		ID: "cloud-form-dialog", Label: props.Title, Width: props.PanelWidth, Height: panelHeight,
		OverlayWidth: props.Width, OverlayHeight: props.Height, BackdropID: "cloud-form-backdrop", BackdropColor: woxui.Color{R: 0, G: 0, B: 0, A: 112},
		Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 18, Bottom: 14}, Theme: props.Theme,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText},
			woxwidget.TextBlock{Value: props.Description, Width: innerWidth, Height: 44, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.Theme.ActionHeader},
			woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
			links,
			woxwidget.TextBlock{Value: props.Feedback, Width: innerWidth, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: props.FeedbackColor},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Painter{Width: max(float32(0), innerWidth-96-112-8), Height: 38},
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-form-cancel", Label: "Cancel", Width: 96, Disabled: props.Saving, Variant: woxcomponent.ButtonSecondary, OnTap: props.OnCancel, Theme: props.Theme}),
				woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "cloud-form-submit", Label: props.SubmitLabel, Width: 112, Disabled: props.Saving, Variant: woxcomponent.ButtonPrimary, OnTap: props.OnSubmit, Theme: props.Theme}),
			}},
		}},
	})
}
