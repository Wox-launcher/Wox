package launcher

import (
	"fmt"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildCloudSettingsPage maps cloud state into the portable cloud settings view.
func (a *App) buildCloudSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	theme := snapshot.palette.componentTheme()
	message := snapshot.cloudError
	messageColor := theme.ErrorText
	if message == "" {
		message = snapshot.note
		messageColor = snapshot.palette.resultSubtitle
	}
	return launcherview.CloudSettingsPage(launcherview.CloudSettingsPageProps{
		Width:         width,
		Height:        height,
		Title:         a.translate("i18n:ui_cloud_sync"),
		Description:   a.translate("i18n:ui_cloud_sync_description"),
		Intro:         a.cloudIntroViewProps(snapshot),
		Account:       a.cloudAccountViewProps(snapshot, contentWidth),
		Sync:          a.cloudSyncViewProps(snapshot),
		Devices:       a.cloudDevicesViewProps(snapshot),
		Plugins:       a.cloudPluginExclusionsViewProps(snapshot),
		ConfigNotes:   a.cloudConfigNotesViewProps(),
		Message:       message,
		MessageColor:  messageColor,
		Scroll:        snapshot.cloudPageScroll,
		ActionMenu:    a.cloudActionMenuViewProps(snapshot),
		Theme:         theme,
		OnScroll:      a.scrollCloudPage,
		OnSetGeometry: a.setCloudPageGeometry,
		OnCloseMenu:   a.closeCloudActionMenu,
	})
}

// cloudIntroViewProps prepares the signed-out Flutter-equivalent product and plan summary.
func (a *App) cloudIntroViewProps(snapshot settingsSnapshot) launcherview.CloudIntroProps {
	iconTint := snapshot.palette.resultTitle
	freePrice := cloudBillingPriceText(snapshot.cloudBillingPlan.Free.Price)
	if freePrice == "" {
		freePrice = "$0/month"
	}
	proPrice := cloudBillingPriceText(snapshot.cloudBillingPlan.Pro.Price)
	if proPrice == "" {
		if snapshot.cloudBillingLoaded {
			proPrice = a.translate("i18n:ui_cloud_sync_plan_price_unavailable")
		} else {
			proPrice = a.translate("i18n:ui_cloud_sync_plan_price_loading")
		}
	}
	return launcherview.CloudIntroProps{
		SectionLabel:     a.translate("i18n:ui_cloud_sync_intro_title"),
		Headline:         a.translate("i18n:ui_cloud_sync_intro_headline"),
		Description:      a.translate("i18n:ui_cloud_sync_intro_description"),
		HeroIcon:         a.imageForTint(settingNavIconSource("data.cloudsync"), &iconTint, 28),
		HeroFallback:     "☁",
		FreeLabel:        a.translate("i18n:ui_cloud_sync_plan_free"),
		ProLabel:         a.translate("i18n:ui_cloud_sync_plan_pro"),
		RecommendedLabel: a.translate("i18n:ui_cloud_sync_plan_recommended"),
		Features: []launcherview.CloudIntroFeatureProps{
			{Title: a.translate("i18n:ui_cloud_sync_intro_settings_title"), Description: a.translate("i18n:ui_cloud_sync_intro_settings_description"), Icon: a.imageForTint(settingNavIconSource("general"), &iconTint, 18), FallbackIcon: "⚙"},
			{Title: a.translate("i18n:ui_cloud_sync_intro_plugins_title"), Description: a.translate("i18n:ui_cloud_sync_intro_plugins_description"), Icon: a.imageForTint(settingNavIconSource("plugins"), &iconTint, 18), FallbackIcon: "◇"},
			{Title: a.translate("i18n:ui_cloud_sync_intro_keys_title"), Description: a.translate("i18n:ui_cloud_sync_intro_keys_description"), Icon: a.imageForTint(settingControlIconSource("key"), &iconTint, 18), FallbackIcon: "⌁"},
		},
		PlanRows: []launcherview.CloudPlanRowProps{
			{Label: a.translate("i18n:ui_cloud_sync_plan_row_price"), FreeValue: freePrice, ProValue: proPrice},
			{Label: a.translate("i18n:ui_cloud_sync_plan_row_devices"), FreeValue: a.translate("i18n:ui_cloud_sync_plan_feature_two_devices"), ProValue: a.translate("i18n:ui_cloud_sync_plan_feature_unlimited_devices")},
			{Label: a.translate("i18n:ui_cloud_sync_plan_row_sync_mode"), FreeValue: a.translate("i18n:ui_cloud_sync_plan_feature_manual_sync"), ProValue: a.translate("i18n:ui_cloud_sync_plan_feature_auto_sync")},
			{Label: a.translate("i18n:ui_cloud_sync_plan_row_frequency"), FreeValue: a.translate("i18n:ui_cloud_sync_plan_feature_strict_sync_limit"), ProValue: a.translate("i18n:ui_cloud_sync_plan_feature_relaxed_sync_limit")},
			{Label: a.translate("i18n:ui_cloud_sync_plan_row_scope"), FreeValue: a.translate("i18n:ui_cloud_sync_plan_scope_free"), ProValue: a.translate("i18n:ui_cloud_sync_plan_feature_everything_free")},
		},
	}
}

// cloudBillingPriceText preserves server formatting and reconstructs a readable fallback when needed.
func cloudBillingPriceText(price cloudBillingPlanPrice) string {
	if strings.TrimSpace(price.Formatted) != "" {
		return price.Formatted
	}
	if price.UnitAmount == nil || strings.TrimSpace(price.Currency) == "" {
		return ""
	}
	amount := fmt.Sprintf("%.2f", float64(*price.UnitAmount)/100)
	if *price.UnitAmount%100 == 0 {
		amount = fmt.Sprintf("%d", *price.UnitAmount/100)
	}
	interval := ""
	if strings.TrimSpace(price.Interval) != "" {
		interval = "/" + price.Interval
	}
	return strings.ToUpper(price.Currency) + " " + amount + interval
}

// cloudAccountViewProps prepares translated account state and controller actions.
func (a *App) cloudAccountViewProps(snapshot settingsSnapshot, contentWidth float32) launcherview.CloudAccountProps {
	status := a.translate("i18n:ui_cloud_sync_plan_free_status")
	if strings.EqualFold(snapshot.cloudAccount.Plan, "pro") {
		status = a.translate("i18n:ui_cloud_sync_plan_pro_status")
	}
	if snapshot.cloudAccount.SessionExpired {
		status = a.translate("i18n:ui_cloud_sync_account_session_expired")
	}
	labelWidth := max(float32(220), contentWidth-390)
	valueWidth := max(float32(220), contentWidth-labelWidth)
	return launcherview.CloudAccountProps{
		SectionLabel:           a.translate("i18n:ui_cloud_sync_account"),
		LoggedIn:               snapshot.cloudAccount.LoggedIn,
		LoginLabel:             a.translate("i18n:ui_cloud_sync_account_login"),
		RegisterLabel:          a.translate("i18n:ui_cloud_sync_account_register"),
		EmailLabel:             a.translate("i18n:ui_cloud_sync_account_email"),
		Email:                  snapshot.cloudAccount.Email,
		EmailTextWidth:         a.measureCloudValueText(snapshot.cloudAccount.Email, valueWidth),
		PlanLabel:              a.translate("i18n:ui_cloud_sync_plan_status"),
		PlanTips:               a.translate("i18n:ui_cloud_sync_plan_status_tips"),
		PlanStatus:             status,
		PlanStatusTextWidth:    a.measureCloudValueText(status, valueWidth),
		BillingLabel:           a.translate("i18n:ui_cloud_sync_billing_help"),
		BillingTips:            a.translate("i18n:ui_cloud_sync_billing_help_tips"),
		SupportLabel:           a.translate("i18n:ui_cloud_sync_contact_support"),
		ActionsEnabled:         snapshot.cloudBusy == "",
		OnLogin:                func() { a.openCloudAccountForm("login") },
		OnRegister:             func() { a.openCloudAccountForm("register") },
		OnOpenAccountMenu:      func() { a.toggleCloudActionMenu("account") },
		OnOpenSubscriptionMenu: func() { a.toggleCloudActionMenu("subscription") },
		OnSupport:              a.openCloudSupportEmail,
	}
}

// measureCloudValueText preserves native text sizing while the view owns placement.
func (a *App) measureCloudValueText(value string, width float32) float32 {
	style := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	textWidth := min(max(float32(0), width-24), float32(len([]rune(value)))*8+8)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(value, style); err == nil {
			textWidth = min(max(float32(0), width-24), metrics.Size.Width+6)
		}
	}
	return textWidth
}

// cloudSyncViewProps prepares status text and the sync or join action.
func (a *App) cloudSyncViewProps(snapshot settingsSnapshot) launcherview.CloudSyncProps {
	label, detail, color := a.cloudSyncPresentation(snapshot)
	ready := cloudSyncReady(snapshot)
	buttonLabel := a.translate("i18n:ui_cloud_sync_sync")
	buttonAction := func() {
		if !ready || !snapshot.cloudAccount.SyncEnabled || !snapshot.cloudSync.Enabled {
			a.beginCloudBootstrap()
			return
		}
		a.runCloudAction("sync", "/sync/push", map[string]any{})
	}
	if cloudCurrentDeviceRevoked(snapshot.cloudDevices) {
		buttonLabel = a.translate("i18n:ui_cloud_sync_join")
		buttonAction = func() { a.runCloudAction("join", "/sync/devices/join", map[string]any{}) }
	}
	return launcherview.CloudSyncProps{
		SectionLabel:  a.translate("i18n:ui_cloud_sync_sync_status"),
		StatusLabel:   a.translate("i18n:ui_cloud_sync_sync_status"),
		Label:         label,
		Detail:        detail,
		Color:         color,
		ButtonLabel:   cloudBusyLabel(snapshot, "sync", buttonLabel),
		ButtonEnabled: snapshot.cloudBusy == "" && !snapshot.cloudLoading && !snapshot.cloudAccount.SessionExpired && snapshot.cloudAccount.SyncEligible,
		OnSync:        buttonAction,
	}
}

func (a *App) cloudSyncPresentation(snapshot settingsSnapshot) (string, string, woxui.Color) {
	muted := snapshot.palette.resultSubtitle
	errorColor := snapshot.palette.componentTheme().ErrorText
	if snapshot.cloudLoading {
		return a.translate("i18n:ui_cloud_sync_loading"), "", muted
	}
	if snapshot.cloudAccount.SessionExpired {
		return a.translate("i18n:ui_cloud_sync_sync_error"), a.translate("i18n:ui_cloud_sync_account_session_expired"), errorColor
	}
	if snapshot.cloudError != "" {
		return a.translate("i18n:ui_cloud_sync_sync_error"), snapshot.cloudError, errorColor
	}
	if progress := snapshot.cloudSync.Progress; progress != nil && progress.Active {
		detail := strings.Title(progress.Operation)
		if progress.Total > 0 {
			detail = fmt.Sprintf("%s · %d / %d", detail, progress.Current, progress.Total)
		}
		return a.translate("i18n:ui_cloud_sync_syncing"), detail, muted
	}
	if state := snapshot.cloudSync.State; state != nil && state.LastError != "" {
		return a.translate("i18n:ui_cloud_sync_sync_error"), state.LastError, errorColor
	}
	if !snapshot.cloudAccount.SyncEligible {
		return a.translate("i18n:ui_cloud_sync_unsynced"), a.translate("i18n:ui_cloud_sync_subscription_required"), muted
	}
	if !cloudSyncReady(snapshot) {
		return a.translate("i18n:ui_cloud_sync_unsynced"), "", muted
	}
	if !snapshot.cloudAccount.SyncEnabled || !snapshot.cloudSync.Enabled {
		return a.translate("i18n:ui_cloud_sync_disabled"), "", muted
	}
	lastSync := max(cloudStateTimestamp(snapshot.cloudSync.State, true), cloudStateTimestamp(snapshot.cloudSync.State, false))
	return a.translate("i18n:ui_cloud_sync_synced"), a.translate("i18n:ui_cloud_sync_last_sync_time") + ": " + a.formatCloudTime(lastSync), woxui.Color{R: 72, G: 190, B: 112, A: 255}
}

func cloudSyncReady(snapshot settingsSnapshot) bool {
	return snapshot.cloudSync.KeyStatus.Available && snapshot.cloudSync.State != nil && snapshot.cloudSync.State.Bootstrapped
}

func cloudStateTimestamp(state *cloudSyncState, pull bool) int64 {
	if state == nil {
		return 0
	}
	if pull {
		return state.LastPullTS
	}
	return state.LastPushTS
}

func cloudCurrentDeviceRevoked(devices cloudDeviceList) bool {
	for _, device := range devices.Devices {
		if device.RevokedAt > 0 && (device.Current || (devices.CurrentDeviceID != "" && device.DeviceID == devices.CurrentDeviceID)) {
			return true
		}
	}
	return false
}

func cloudBusyLabel(snapshot settingsSnapshot, operation, label string) string {
	if snapshot.cloudBusy == operation || (operation == "bootstrap" && snapshot.cloudBusy == "bootstrap-status") {
		return label + "…"
	}
	return label
}

// cloudDevicesViewProps prepares device labels and revoke callbacks.
func (a *App) cloudDevicesViewProps(snapshot settingsSnapshot) launcherview.CloudDevicesProps {
	items := make([]launcherview.CloudDeviceProps, 0, len(snapshot.cloudDevices.Devices))
	for index, device := range snapshot.cloudDevices.Devices {
		index := index
		device := device
		name := device.DeviceName
		if strings.TrimSpace(name) == "" {
			name = device.DeviceID
		}
		if device.Current {
			name += " · " + a.translate("i18n:ui_cloud_sync_devices_current")
		}
		if device.RevokedAt > 0 {
			name += " · " + a.translate("i18n:ui_cloud_sync_devices_revoked")
		}
		items = append(items, launcherview.CloudDeviceProps{
			ID:            fmt.Sprintf("cloud-revoke-%d", index),
			Name:          name,
			Detail:        strings.Title(strings.ToLower(device.Platform)),
			LastSeen:      a.formatCloudTime(device.LastSeenAt),
			RevokeLabel:   a.translate("i18n:ui_cloud_sync_devices_revoke"),
			ShowRevoke:    !device.Current && device.RevokedAt == 0,
			RevokeEnabled: snapshot.cloudBusy == "",
			OnRevoke: func() {
				a.runCloudAction("revoke", "/sync/devices/revoke", map[string]string{"target_device_id": device.DeviceID})
			},
		})
	}
	return launcherview.CloudDevicesProps{
		SectionLabel:   a.translate("i18n:ui_cloud_sync_devices"),
		RefreshLabel:   cloudRefreshLabel(a, snapshot),
		RefreshEnabled: !snapshot.cloudLoading && snapshot.cloudBusy == "",
		EmptyLabel:     a.translate("i18n:ui_cloud_sync_devices_empty"),
		Items:          items,
		OnRefresh:      func() { go a.reloadCloudSync() },
	}
}

func cloudRefreshLabel(a *App, snapshot settingsSnapshot) string {
	if snapshot.cloudLoading {
		return a.translate("i18n:ui_cloud_sync_loading")
	}
	return a.translate("i18n:ui_cloud_sync_refresh_status")
}

// cloudPluginExclusionsViewProps prepares the visible plugin boundary and toggle actions.
func (a *App) cloudPluginExclusionsViewProps(snapshot settingsSnapshot) launcherview.CloudPluginExclusionsProps {
	rows := cloudPluginExclusionRows(snapshot.cloudPlugins, snapshot.data.CloudSyncDisabledPlugins)
	excluded := make(map[string]bool, len(snapshot.data.CloudSyncDisabledPlugins))
	for _, pluginID := range snapshot.data.CloudSyncDisabledPlugins {
		excluded[strings.TrimSpace(pluginID)] = true
	}
	items := make([]launcherview.CloudPluginExclusionProps, 0, len(rows))
	for index, plugin := range rows {
		index := index
		plugin := plugin
		name := plugin.Name
		if strings.TrimSpace(name) == "" {
			name = plugin.ID
		}
		isExcluded := excluded[plugin.ID]
		buttonLabel := a.translate("i18n:ui_cloud_sync_enabled")
		if isExcluded {
			buttonLabel = a.translate("i18n:ui_cloud_sync_disabled")
		}
		if snapshot.cloudBusy == "exclusion-"+plugin.ID {
			buttonLabel += "…"
		}
		items = append(items, launcherview.CloudPluginExclusionProps{
			ID:            fmt.Sprintf("cloud-plugin-%d", index),
			Name:          name,
			PluginID:      plugin.ID,
			ButtonLabel:   buttonLabel,
			ButtonEnabled: snapshot.cloudBusy == "",
			Excluded:      isExcluded,
			OnToggle:      func() { a.toggleCloudPluginExclusion(plugin.ID) },
		})
	}
	return launcherview.CloudPluginExclusionsProps{
		SectionLabel:  a.translate("i18n:ui_cloud_sync_plugin_exclusions"),
		Tips:          a.translate("i18n:ui_cloud_sync_plugin_exclusions_tips"),
		EmptyLabel:    a.translate("i18n:ui_cloud_sync_plugin_exclusions_empty"),
		Items:         items,
		Scroll:        snapshot.cloudPluginScroll,
		OnScroll:      a.scrollCloudPlugins,
		OnSetViewport: a.setCloudPluginViewport,
	}
}

// cloudConfigNotesViewProps translates platform-aware sync caveats for the view.
func (a *App) cloudConfigNotesViewProps() launcherview.CloudConfigNotesProps {
	notes := [][2]string{
		{"ui_cloud_sync_config_note_clipboard", "ui_cloud_sync_config_note_clipboard_tips"},
		{"ui_cloud_sync_config_note_query_hotkeys", "ui_cloud_sync_config_note_query_hotkeys_tips"},
		{"ui_cloud_sync_config_note_autostart", "ui_cloud_sync_config_note_autostart_tips"},
		{"ui_cloud_sync_config_note_runtime_paths", "ui_cloud_sync_config_note_runtime_paths_tips"},
	}
	items := make([]string, 0, len(notes))
	for _, note := range notes {
		items = append(items, a.translate("i18n:"+note[0])+" · "+a.translate("i18n:"+note[1]))
	}
	return launcherview.CloudConfigNotesProps{SectionLabel: a.translate("i18n:ui_cloud_sync_config_notes"), Items: items}
}

// cloudActionMenuViewProps prepares the active account or subscription menu.
func (a *App) cloudActionMenuViewProps(snapshot settingsSnapshot) *launcherview.CloudActionMenuProps {
	if snapshot.cloudActionMenu == "" {
		return nil
	}
	type menuAction struct {
		id     string
		label  string
		action string
	}
	actions := []menuAction{
		{id: "change-password", label: a.translate("i18n:ui_cloud_sync_account_change_password"), action: "change-password"},
		{id: "logout", label: a.translate("i18n:ui_cloud_sync_account_logout"), action: "logout"},
	}
	top := float32(145)
	if snapshot.cloudActionMenu == "subscription" {
		billingLabel := a.translate("i18n:ui_cloud_sync_subscribe")
		if strings.EqualFold(snapshot.cloudAccount.Plan, "pro") {
			billingLabel = a.translate("i18n:ui_cloud_sync_manage_subscription")
		}
		actions = []menuAction{
			{id: "refresh", label: a.translate("i18n:ui_cloud_sync_refresh_status"), action: "refresh"},
			{id: "billing", label: billingLabel, action: "billing"},
		}
		top = 205
	}
	items := make([]launcherview.CloudActionMenuItemProps, 0, len(actions))
	for _, entry := range actions {
		entry := entry
		items = append(items, launcherview.CloudActionMenuItemProps{
			ID: "cloud-menu-" + entry.id, Label: entry.label, OnTap: func() { a.runCloudMenuAction(entry.action) },
		})
	}
	return &launcherview.CloudActionMenuProps{Top: top, Items: items}
}

func (a *App) formatCloudTime(timestamp int64) string {
	if timestamp <= 0 {
		return a.translate("i18n:ui_cloud_sync_never")
	}
	return time.UnixMilli(timestamp).Local().Format("2006-01-02 15:04")
}

// buildCloudFormOverlay maps account form state into typed view props.
func (a *App) buildCloudFormOverlay(snapshot *cloudFormSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(408), max(float32(320), width-64))
	fields := make([]launcherview.CloudFormFieldProps, 0, len(snapshot.definitions))
	window := a.formFieldNativeWindow("cloud-form")
	for index, definition := range snapshot.definitions {
		index := index
		definition := definition
		focused := snapshot.active && snapshot.focused == index
		state := snapshot.editing
		if !focused {
			state = woxui.TextEditingState{Text: snapshot.values[definition.Value.Key]}
		}
		field := launcherview.CloudFormFieldProps{
			ID:        fmt.Sprintf("cloud-form-field-%d", index),
			Kind:      definition.Type,
			Label:     a.translate(definition.Value.Label),
			State:     state,
			Focused:   focused,
			Protected: definition.Type == "password",
			Window:    window,
			OnCaret: func(offset int) {
				a.focusCloudFormField(index)
				a.setCloudFormCaret(index, offset)
			},
		}
		if definition.Type == "checkbox" {
			field.Checked = snapshot.values[definition.Value.Key] == "true"
			field.OnTap = func() {
				a.focusCloudFormField(index)
				a.changeCloudFormField(index, 1)
			}
		}
		fields = append(fields, field)
	}

	linkPrefix := ""
	links := []launcherview.CloudFormLinkProps{}
	var fieldLink *launcherview.CloudFormLinkProps
	switch snapshot.kind {
	case "register":
		linkPrefix = a.translate("i18n:ui_cloud_sync_account_accept_prefix")
		links = append(links,
			launcherview.CloudFormLinkProps{ID: "cloud-terms", Label: a.translate("i18n:ui_cloud_sync_account_terms"), Width: 112, OnTap: func() { a.openCloudLegalPage("/terms") }},
			launcherview.CloudFormLinkProps{ID: "cloud-privacy", Label: a.translate("i18n:ui_cloud_sync_account_privacy"), Width: 112, OnTap: func() { a.openCloudLegalPage("/privacy") }},
		)
	case "login":
		fieldLink = &launcherview.CloudFormLinkProps{ID: "cloud-forgot-password", Label: a.translate("i18n:ui_cloud_sync_account_reset_request"), Width: 96, OnTap: func() { a.openCloudAccountForm("reset-request") }}
	case "verify":
		links = append(links, launcherview.CloudFormLinkProps{ID: "cloud-resend-code", Label: "Resend code", Width: 112, OnTap: a.resendCloudVerification})
	}

	feedback := snapshot.notice
	feedbackColor := palette.actionHeader
	theme := palette.componentTheme()
	if snapshot.error != "" {
		feedback = snapshot.error
		feedbackColor = theme.ErrorText
	}
	submitLabel := a.translate("i18n:ui_cloud_sync_confirm")
	if snapshot.saving {
		submitLabel = a.translate("i18n:ui_cloud_sync_loading")
	}
	submitEnabled := !snapshot.saving
	if snapshot.kind == "register" && snapshot.values["AcceptedLegal"] != "true" {
		submitEnabled = false
	}
	return launcherview.CloudFormOverlay(launcherview.CloudFormOverlayProps{
		Width:         width,
		Height:        height,
		PanelWidth:    panelWidth,
		Title:         snapshot.title,
		Description:   cloudFormDescription(snapshot),
		Fields:        fields,
		LinkPrefix:    linkPrefix,
		Links:         links,
		FieldLink:     fieldLink,
		Feedback:      feedback,
		FeedbackColor: feedbackColor,
		CancelLabel:   a.translate("i18n:ui_cloud_sync_cancel"),
		SubmitLabel:   submitLabel,
		SubmitEnabled: submitEnabled,
		Saving:        snapshot.saving,
		Theme:         theme,
		OnCancel:      a.closeCloudForm,
		OnSubmit:      a.submitCloudForm,
	})
}

func cloudFormDescription(snapshot *cloudFormSnapshot) string {
	switch snapshot.kind {
	case "login", "register":
		return ""
	case "verify":
		return "Enter the verification code sent to " + snapshot.email + "."
	case "reset-request":
		return "Enter your account email and Wox will send a password reset code."
	case "reset-confirm":
		return "Enter the reset code from your email and choose a new 12-character password."
	case "change-password":
		return "Confirm the current password before setting a new 12-character password."
	case "bootstrap":
		if snapshot.hasRemoteData {
			return "A cloud backup exists. Enter its encryption password to restore this device."
		}
		return "Choose an encryption password. It cannot be recovered, so store it safely."
	default:
		return "Use your Wox account credentials to continue."
	}
}
