package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type cloudPlanTooltipState struct {
	anchor woxui.Rect
}

// buildCloudSettingsPage maps cloud state into the portable cloud settings view.
func (a *App) buildCloudSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	theme := snapshot.palette.componentTheme()
	message := snapshot.cloud.Error
	messageColor := theme.ErrorText
	return launcherview.CloudSettingsPage(launcherview.CloudSettingsPageProps{
		Width:        width,
		Height:       height,
		Title:        a.translate("i18n:ui_cloud_sync"),
		Description:  a.translate("i18n:ui_cloud_sync_description"),
		Intro:        a.cloudIntroViewProps(snapshot),
		Account:      a.cloudAccountViewProps(snapshot, contentWidth, imageScale),
		Sync:         a.cloudSyncViewProps(snapshot, contentWidth),
		Devices:      a.cloudDevicesViewProps(snapshot, contentWidth, imageScale),
		Plugins:      a.cloudPluginExclusionsViewProps(snapshot, imageScale),
		ConfigNotes:  a.cloudConfigNotesViewProps(snapshot, imageScale),
		Message:      message,
		MessageColor: messageColor,
		ActionMenu:   a.cloudActionMenuViewProps(snapshot),
		Theme:        theme,
		OnCloseMenu:  a.closeCloudActionMenu,
	})
}

// cloudIntroViewProps prepares the signed-out Flutter-equivalent product and plan summary.
func (a *App) cloudIntroViewProps(snapshot settingsSnapshot) launcherview.CloudIntroProps {
	iconTint := snapshot.palette.resultTitle
	freePrice := cloudBillingPriceText(snapshot.cloud.BillingPlan.Free.Price)
	if freePrice == "" {
		freePrice = "$0/month"
	}
	proPrice := cloudBillingPriceText(snapshot.cloud.BillingPlan.Pro.Price)
	if proPrice == "" {
		if snapshot.cloud.BillingLoaded {
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
func (a *App) cloudAccountViewProps(snapshot settingsSnapshot, contentWidth, imageScale float32) launcherview.CloudAccountProps {
	status := a.translate("i18n:ui_cloud_sync_plan_free_status")
	if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
		status = a.translate("i18n:ui_cloud_sync_plan_pro_status")
	}
	if snapshot.cloud.Account.SessionExpired {
		status = a.translate("i18n:ui_cloud_sync_account_session_expired")
	}
	labelWidth := cloudSettingsLabelWidth(contentWidth, 220)
	return launcherview.CloudAccountProps{
		SectionLabel:           a.translate("i18n:ui_cloud_sync_account"),
		LoggedIn:               snapshot.cloud.Account.LoggedIn,
		LabelWidth:             labelWidth,
		LoginLabel:             a.translate("i18n:ui_cloud_sync_account_login"),
		RegisterLabel:          a.translate("i18n:ui_cloud_sync_account_register"),
		EmailLabel:             a.translate("i18n:ui_cloud_sync_account_email"),
		Email:                  snapshot.cloud.Account.Email,
		PlanLabel:              a.translate("i18n:ui_cloud_sync_plan_status"),
		PlanTips:               a.translate("i18n:ui_cloud_sync_plan_status_tips"),
		PlanStatus:             status,
		BillingLabel:           a.translate("i18n:ui_cloud_sync_billing_help"),
		BillingTips:            a.translate("i18n:ui_cloud_sync_billing_help_tips"),
		SupportLabel:           a.translate("i18n:ui_cloud_sync_contact_support"),
		InfoIcon:               a.imageForTint(settingNavIconSource("about"), &snapshot.palette.resultSubtitle, physicalImageSize(14, imageScale)),
		SupportIcon:            a.imageForTint(settingControlIconSource("email"), &snapshot.palette.resultTitle, physicalImageSize(16, imageScale)),
		ActionsEnabled:         snapshot.cloud.Busy == "",
		OnLogin:                func() { a.openCloudAccountForm("login") },
		OnRegister:             func() { a.openCloudAccountForm("register") },
		OnOpenAccountMenu:      func() { a.toggleCloudActionMenu("account") },
		OnOpenSubscriptionMenu: func() { a.toggleCloudActionMenu("subscription") },
		OnPlanTooltip:          a.setCloudPlanTooltip,
		OnSupport:              a.openCloudSupportEmail,
	}
}

// setCloudPlanTooltip keeps the rich comparison inside the settings window instead of falling back to the native plain-text tooltip service.
func (a *App) setCloudPlanTooltip(inside bool, anchor woxui.Rect) {
	if !inside {
		if a.cloudPlanTooltip == nil {
			return
		}
		a.cloudPlanTooltip = nil
		a.invalidateSettingsWindow()
		return
	}
	if a.cloudPlanTooltip != nil && a.cloudPlanTooltip.anchor == anchor {
		return
	}
	a.cloudPlanTooltip = &cloudPlanTooltipState{anchor: anchor}
	a.invalidateSettingsWindow()
}

// cloudSettingsLabelWidth keeps login-state fields on Flutter's wide-form grid while preserving room for their controls in narrow settings windows.
func cloudSettingsLabelWidth(contentWidth, reservedWidth float32) float32 {
	return min(float32(520), max(float32(220), contentWidth-reservedWidth))
}

// cloudSyncViewProps prepares status text and the sync or join action.
func (a *App) cloudSyncViewProps(snapshot settingsSnapshot, contentWidth float32) launcherview.CloudSyncProps {
	label, detail, color := a.cloudSyncPresentation(snapshot)
	ready := cloudSyncReady(snapshot)
	buttonLabel := a.translate("i18n:ui_cloud_sync_sync")
	buttonAction := func() {
		if !ready || !snapshot.cloud.Account.SyncEnabled || !snapshot.cloud.Sync.Enabled {
			a.beginCloudBootstrap()
			return
		}
		a.runCloudAction("sync", func(ctx context.Context) error { return a.services.SyncCloud(ctx, a.sessionID) })
	}
	if cloudCurrentDeviceRevoked(snapshot.cloud.Devices) {
		buttonLabel = a.translate("i18n:ui_cloud_sync_join")
		buttonAction = func() {
			a.runCloudAction("join", func(ctx context.Context) error {
				return a.services.JoinCloudDevice(ctx, a.sessionID)
			})
		}
	}
	return launcherview.CloudSyncProps{
		SectionLabel:  a.translate("i18n:ui_cloud_sync_sync"),
		StatusLabel:   a.translate("i18n:ui_cloud_sync_sync_status"),
		LabelWidth:    cloudSettingsLabelWidth(contentWidth, 154),
		Label:         label,
		Detail:        detail,
		Color:         color,
		ButtonLabel:   cloudBusyLabel(snapshot, "sync", buttonLabel),
		ButtonEnabled: snapshot.cloud.Busy == "" && !snapshot.cloud.Loading && !snapshot.cloud.Account.SessionExpired && snapshot.cloud.Account.SyncEligible,
		OnSync:        buttonAction,
	}
}

func (a *App) cloudSyncPresentation(snapshot settingsSnapshot) (string, string, woxui.Color) {
	muted := snapshot.palette.resultSubtitle
	errorColor := snapshot.palette.componentTheme().ErrorText
	if snapshot.cloud.Loading {
		return a.translate("i18n:ui_cloud_sync_loading"), "", muted
	}
	if snapshot.cloud.Account.SessionExpired {
		return a.translate("i18n:ui_cloud_sync_sync_error"), a.translate("i18n:ui_cloud_sync_account_session_expired"), errorColor
	}
	if snapshot.cloud.Error != "" {
		return a.translate("i18n:ui_cloud_sync_sync_error"), snapshot.cloud.Error, errorColor
	}
	if progress := snapshot.cloud.Sync.Progress; progress != nil && progress.Active {
		detail := strings.Title(progress.Operation)
		if progress.Total > 0 {
			detail = fmt.Sprintf("%s · %d / %d", detail, progress.Current, progress.Total)
		} else if progress.Current > 0 {
			detail = fmt.Sprintf("%s · %d", detail, progress.Current)
		}
		return a.translate("i18n:ui_cloud_sync_syncing"), detail, muted
	}
	if snapshot.cloud.Busy == "sync" {
		return a.translate("i18n:ui_cloud_sync_syncing"), a.translate("i18n:ui_cloud_sync_progress_starting"), muted
	}
	if state := snapshot.cloud.Sync.State; state != nil && state.LastError != "" {
		return a.translate("i18n:ui_cloud_sync_sync_error"), state.LastError, errorColor
	}
	if !snapshot.cloud.Account.SyncEligible {
		return a.translate("i18n:ui_cloud_sync_unsynced"), a.translate("i18n:ui_cloud_sync_subscription_required"), muted
	}
	if !cloudSyncReady(snapshot) {
		return a.translate("i18n:ui_cloud_sync_unsynced"), "", muted
	}
	if !snapshot.cloud.Account.SyncEnabled || !snapshot.cloud.Sync.Enabled {
		return a.translate("i18n:ui_cloud_sync_disabled"), "", muted
	}
	lastSync := max(cloudStateTimestamp(snapshot.cloud.Sync.State, true), cloudStateTimestamp(snapshot.cloud.Sync.State, false))
	return a.translate("i18n:ui_cloud_sync_synced"), a.translate("i18n:ui_cloud_sync_last_sync_time") + ": " + a.formatCloudTime(lastSync), muted
}

func cloudSyncReady(snapshot settingsSnapshot) bool {
	return snapshot.cloud.Sync.KeyStatus.Available && snapshot.cloud.Sync.State != nil && snapshot.cloud.Sync.State.Bootstrapped
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
	if snapshot.cloud.Busy == operation || (operation == "bootstrap" && snapshot.cloud.Busy == "bootstrap-status") {
		return label + "…"
	}
	return label
}

// cloudDevicesViewProps prepares device labels and revoke callbacks.
func (a *App) cloudDevicesViewProps(snapshot settingsSnapshot, contentWidth, imageScale float32) launcherview.CloudDevicesProps {
	items := make([]launcherview.CloudDeviceProps, 0, len(snapshot.cloud.Devices.Devices))
	for index, device := range snapshot.cloud.Devices.Devices {
		if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") && device.RevokedAt > 0 {
			continue
		}
		name := device.DeviceName
		if strings.TrimSpace(name) == "" {
			name = device.DeviceID
		}
		if device.Current {
			name += " " + a.translate("i18n:ui_cloud_sync_devices_current")
		}
		items = append(items, launcherview.CloudDeviceProps{
			ID:            fmt.Sprintf("cloud-revoke-%d", index),
			Name:          name,
			Detail:        cloudDevicePlatform(a, device.Platform),
			LastSeen:      a.formatCloudTime(device.LastSeenAt),
			RevokeLabel:   a.translate("i18n:ui_cloud_sync_devices_revoke"),
			ShowRevoke:    !strings.EqualFold(snapshot.cloud.Account.Plan, "pro") && !device.Current && device.RevokedAt == 0,
			RevokeEnabled: snapshot.cloud.Busy == "",
			OnRevoke: func() {
				a.runCloudAction("revoke", func(ctx context.Context) error {
					_, err := a.services.RevokeCloudDevice(ctx, a.sessionID, device.DeviceID)
					return err
				})
			},
		})
	}
	tips := a.translate("i18n:ui_cloud_sync_devices_pro_tips")
	if !strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
		limit := 2
		if snapshot.cloud.Devices.DeviceLimit != nil && *snapshot.cloud.Devices.DeviceLimit > 0 {
			limit = *snapshot.cloud.Devices.DeviceLimit
		}
		tips = strings.ReplaceAll(a.translate("i18n:ui_cloud_sync_devices_free_tips"), "{count}", fmt.Sprint(snapshot.cloud.Devices.DeviceCount))
		tips = strings.ReplaceAll(tips, "{limit}", fmt.Sprint(limit))
	}
	return launcherview.CloudDevicesProps{
		SectionLabel:   a.translate("i18n:ui_cloud_sync_devices"),
		Tips:           tips,
		LabelWidth:     cloudSettingsLabelWidth(contentWidth, 154),
		RefreshLabel:   cloudRefreshLabel(a, snapshot),
		RefreshIcon:    a.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.resultTitle, physicalImageSize(16, imageScale)),
		RefreshEnabled: !snapshot.cloud.Loading && snapshot.cloud.Busy == "",
		EmptyLabel:     a.translate("i18n:ui_cloud_sync_devices_empty"),
		Items:          items,
		OnRefresh:      func() { util.Go(a.lifecycleCtx, "reload cloud sync devices", a.reloadCloudSync) },
	}
}

// cloudDevicePlatform matches the user-facing platform names used by Flutter.
func cloudDevicePlatform(a *App, platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "darwin", "macos", "mac":
		return "macOS"
	case "windows", "win32", "win":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return a.translate("i18n:ui_cloud_sync_devices_unknown_platform")
	}
}

func cloudRefreshLabel(a *App, snapshot settingsSnapshot) string {
	if snapshot.cloud.Loading {
		return a.translate("i18n:ui_cloud_sync_loading")
	}
	return a.translate("i18n:ui_cloud_sync_refresh")
}

// cloudPluginExclusionsViewProps prepares the visible plugin boundary and toggle actions.
func (a *App) cloudPluginExclusionsViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudPluginExclusionsProps {
	rows := cloudPluginExclusionRows(snapshot.cloud.Plugins, snapshot.general.Data.CloudSyncDisabledPlugins)
	plugins := make(map[string]pluginSettingsPlugin, len(rows))
	for _, plugin := range rows {
		plugins[plugin.ID] = plugin
	}
	items := make([]launcherview.CloudPluginExclusionProps, 0, len(snapshot.general.Data.CloudSyncDisabledPlugins))
	for index, pluginID := range snapshot.general.Data.CloudSyncDisabledPlugins {
		pluginID := strings.TrimSpace(pluginID)
		if pluginID == "" {
			continue
		}
		plugin := plugins[pluginID]
		name := plugin.Name
		if strings.TrimSpace(name) == "" {
			name = pluginID + " (" + a.translate("i18n:ui_cloud_sync_plugin_exclusions_uninstalled") + ")"
		}
		items = append(items, launcherview.CloudPluginExclusionProps{
			ID:       fmt.Sprintf("cloud-plugin-%d", index),
			Name:     name,
			PluginID: pluginID,
			OnDelete: func() { a.toggleCloudPluginExclusion(pluginID) },
		})
	}
	foreground := snapshot.palette.resultSubtitle
	return launcherview.CloudPluginExclusionsProps{
		SectionLabel:   a.translate("i18n:ui_cloud_sync_plugin_exclusions"),
		Tips:           a.translate("i18n:ui_cloud_sync_plugin_exclusions_tips"),
		ColumnLabel:    a.translate("i18n:ui_cloud_sync_plugin_exclusions_plugin"),
		EmptyLabel:     a.translate("i18n:ui_no_data"),
		Items:          items,
		AddLabel:       a.translate("i18n:ui_add"),
		OperationLabel: a.translate("i18n:ui_operation"),
		AddIcon:        a.imageForTint(settingControlIconSource("add"), &foreground, physicalImageSize(15, imageScale)),
		DeleteIcon:     a.imageForTint(settingControlIconSource("delete"), &foreground, physicalImageSize(16, imageScale)),
		EmptyIcon:      a.imageForTint(settingControlIconSource("inbox"), &foreground, physicalImageSize(24, imageScale)),
		OnAdd:          func() { a.toggleCloudActionMenu("plugins") },
	}
}

// cloudConfigNotesViewProps translates platform-aware sync caveats for the view.
func (a *App) cloudConfigNotesViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudConfigNotesProps {
	notes := [][3]string{
		{"clipboard", "partial", "clipboard"}, {"query_hotkeys", "platform", "query_hotkeys"}, {"launch_hotkeys", "platform", "launch_hotkeys"},
		{"ignored_hotkey_apps", "platform", "ignored_hotkey_apps"}, {"autostart", "platform", "autostart"}, {"http_proxy", "platform", "http_proxy"},
		{"runtime_paths", "platform", "runtime_paths"}, {"app_font", "platform", "app_font"}, {"app_indexing", "platform", "app_indexing"},
		{"file_search", "platform", "file_search"}, {"explorer_quick_jump", "platform", "explorer_quick_jump"}, {"local_plugin_directories", "platform", "local_plugin_directories"},
		{"folder_favorites", "platform", "folder_favorites"}, {"shell", "platform", "shell"}, {"browser_bookmarks", "platform", "browser_bookmarks"},
		{"space_quick_look", "platform", "space_quick_look"}, {"plugin_install_state", "reproducible", "plugin_install_state"}, {"custom_themes", "synced", "custom_themes"},
	}
	items := make([]launcherview.CloudConfigNoteProps, 0, len(notes))
	for _, note := range notes {
		items = append(items, launcherview.CloudConfigNoteProps{
			Item:    a.translate("i18n:ui_cloud_sync_config_note_" + note[0]),
			Mode:    a.translate("i18n:ui_cloud_sync_config_notes_mode_" + note[1]),
			Tooltip: a.translate("i18n:ui_cloud_sync_config_note_" + note[2] + "_tips"),
		})
	}
	return launcherview.CloudConfigNotesProps{
		SectionLabel: a.translate("i18n:ui_cloud_sync_config_notes"),
		Tips:         a.translate("i18n:ui_cloud_sync_config_notes_tips"),
		ItemLabel:    a.translate("i18n:ui_cloud_sync_config_notes_item"),
		ModeLabel:    a.translate("i18n:ui_cloud_sync_config_notes_mode"),
		InfoIcon:     a.imageForTint(settingNavIconSource("about"), &snapshot.palette.resultSubtitle, physicalImageSize(14, imageScale)),
		Items:        items,
		OnTooltip:    a.setSettingChoiceTooltip,
	}
}

// cloudActionMenuViewProps prepares the active account or subscription menu.
func (a *App) cloudActionMenuViewProps(snapshot settingsSnapshot) *launcherview.CloudActionMenuProps {
	if snapshot.cloud.ActionMenu == "" {
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
	if snapshot.cloud.ActionMenu == "subscription" {
		billingLabel := a.translate("i18n:ui_cloud_sync_subscribe")
		if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
			billingLabel = a.translate("i18n:ui_cloud_sync_manage_subscription")
		}
		actions = []menuAction{
			{id: "refresh", label: a.translate("i18n:ui_cloud_sync_refresh_status"), action: "refresh"},
			{id: "billing", label: billingLabel, action: "billing"},
		}
		top = 205
	}
	if snapshot.cloud.ActionMenu == "plugins" {
		excluded := make(map[string]bool, len(snapshot.general.Data.CloudSyncDisabledPlugins))
		for _, pluginID := range snapshot.general.Data.CloudSyncDisabledPlugins {
			excluded[strings.TrimSpace(pluginID)] = true
		}
		actions = actions[:0]
		for _, plugin := range snapshot.cloud.Plugins {
			if strings.TrimSpace(plugin.ID) == "" || excluded[plugin.ID] {
				continue
			}
			label := plugin.Name
			if strings.TrimSpace(label) == "" {
				label = plugin.ID
			}
			actions = append(actions, menuAction{id: plugin.ID, label: label, action: "plugin:" + plugin.ID})
		}
		sort.Slice(actions, func(i, j int) bool { return strings.ToLower(actions[i].label) < strings.ToLower(actions[j].label) })
		top = 320
	}
	items := make([]launcherview.CloudActionMenuItemProps, 0, len(actions))
	for _, entry := range actions {
		onTap := func() { a.runCloudMenuAction(entry.action) }
		if strings.HasPrefix(entry.action, "plugin:") {
			pluginID := strings.TrimPrefix(entry.action, "plugin:")
			onTap = func() {
				a.closeCloudActionMenu()
				a.toggleCloudPluginExclusion(pluginID)
			}
		}
		items = append(items, launcherview.CloudActionMenuItemProps{ID: "cloud-menu-" + entry.id, Label: entry.label, OnTap: onTap})
	}
	return &launcherview.CloudActionMenuProps{Top: top, Modal: snapshot.cloud.ActionMenu == "plugins", Items: items}
}

func (a *App) formatCloudTime(timestamp int64) string {
	if timestamp <= 0 {
		return a.translate("i18n:ui_cloud_sync_never")
	}
	return time.UnixMilli(timestamp).Local().Format("2006-01-02 15:04:05")
}

// buildCloudFormOverlay maps account form state into typed view props.
func (a *App) buildCloudFormOverlay(snapshot *cloudFormSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(408), max(float32(320), width-64))
	fields := make([]launcherview.CloudFormFieldProps, 0, len(snapshot.definitions))
	window := a.formFieldNativeWindow("cloud-form")
	for index, definition := range snapshot.definitions {
		focused := snapshot.active && snapshot.focused == index
		state := snapshot.editing
		if !focused {
			state = woxui.TextEditingState{Text: snapshot.values[definition.Value.Key]}
		}
		var controller *woxwidget.TextEditingController
		if index < len(snapshot.controllers) {
			controller = snapshot.controllers[index]
		}
		var focusNode *woxwidget.FocusNode
		if index < len(snapshot.focusNodes) {
			focusNode = snapshot.focusNodes[index]
		}
		field := launcherview.CloudFormFieldProps{
			ID:            fmt.Sprintf("cloud-form-field-%d", index),
			Kind:          definition.Type,
			Label:         a.translate(definition.Value.Label),
			State:         state,
			Focused:       focused,
			Autofocus:     focused,
			Protected:     definition.Type == "password",
			Window:        window,
			Controller:    controller,
			FocusNode:     focusNode,
			OnChanged:     func(value string) { a.setCloudFormText(index, value) },
			OnFocusChange: func(focused bool) { a.setCloudFormFieldFocused(index, focused) },
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
