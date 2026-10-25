package launcher

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	woxcomponent "wox/ui/launcher/component"

	"wox/cloudsync"
	"wox/common/icons"
	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type cloudPlanTooltipState struct {
	anchor woxui.Rect
}

// buildCloudSettingsPage maps cloud state into the portable cloud settings view.
func (c *cloudSettingsController) buildCloudSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	contentWidth := launcherview.SettingsPageContentWidth(width)
	theme := snapshot.palette
	message := snapshot.cloud.Error
	messageColor := theme.Error
	account := c.cloudAccountViewProps(snapshot, contentWidth, imageScale)
	sync := c.cloudSyncViewProps(snapshot, imageScale)
	devices := c.cloudDevicesViewProps(snapshot, imageScale)
	// Share the widest translated action, including busy labels, across the three sections.
	actionWidth := theme.Scaled(100)
	if window := c.settings.settingsNativeWindow(); window != nil {
		for _, label := range []string{account.SupportLabel, sync.ButtonLabel, devices.RefreshLabel} {
			if metrics, err := window.MeasureText(label, woxui.TextStyle{Size: theme.Scaled(woxcomponent.CompactButtonFontSize)}); err == nil {
				actionWidth = max(actionWidth, metrics.Size.Width+48)
			}
		}
	}
	account.ActionWidth, sync.ActionWidth, devices.ActionWidth = actionWidth, actionWidth, actionWidth
	return launcherview.CloudSettingsPage(launcherview.CloudSettingsPageProps{
		Width:        width,
		Height:       height,
		Title:        c.language.translate("i18n:ui_cloud_sync"),
		Description:  c.language.translate("i18n:ui_cloud_sync_description"),
		Intro:        c.cloudIntroViewProps(snapshot, imageScale),
		Account:      account,
		Sync:         sync,
		Devices:      devices,
		Plugins:      c.cloudPluginExclusionsViewProps(snapshot, imageScale),
		ConfigNotes:  c.cloudConfigNotesViewProps(snapshot, imageScale),
		Message:      message,
		MessageColor: messageColor,
		ActionMenu:   c.cloudActionMenuViewProps(snapshot),
		Theme:        theme,
		OnCloseMenu:  c.closeCloudActionMenu,
	})
}

// cloudIntroViewProps prepares the signed-out Flutter-equivalent product and plan summary.
func (c *cloudSettingsController) cloudIntroViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudIntroProps {
	iconTint := snapshot.palette.Text
	freePrice := cloudBillingPriceText(snapshot.cloud.BillingPlan.Free.Price)
	if freePrice == "" {
		freePrice = "$0/month"
	}
	proPrice := cloudBillingPriceWithTrial(c.language, snapshot.cloud.BillingPlan.Pro.Price, snapshot.cloud.BillingPlan.Pro.Trial, snapshot.cloud.BillingLoaded)
	return launcherview.CloudIntroProps{
		SectionLabel: c.language.translate("i18n:ui_cloud_sync_intro_title"),
		Headline:     c.language.translate("i18n:ui_cloud_sync_intro_headline"),
		Description:  c.language.translate("i18n:ui_cloud_sync_intro_description"),
		HeroIcon:     c.imageForTint(settingNavIconSource("data.cloudsync"), &iconTint, physicalImageSize(28, imageScale)),
		HeroFallback: "☁",
		FreeLabel:    c.language.translate("i18n:ui_cloud_sync_plan_free"),
		ProLabel:     c.language.translate("i18n:ui_cloud_sync_plan_pro"),
		Features: []launcherview.CloudIntroFeatureProps{
			{Title: c.language.translate("i18n:ui_cloud_sync_intro_settings_title"), Description: c.language.translate("i18n:ui_cloud_sync_intro_settings_description"), Icon: c.imageForTint(settingNavIconSource("general"), &iconTint, physicalImageSize(17, imageScale)), FallbackIcon: "⚙"},
			{Title: c.language.translate("i18n:ui_cloud_sync_intro_plugins_title"), Description: c.language.translate("i18n:ui_cloud_sync_intro_plugins_description"), Icon: c.imageForTint(settingNavIconSource("plugins"), &iconTint, physicalImageSize(17, imageScale)), FallbackIcon: "◇"},
			{Title: c.language.translate("i18n:ui_cloud_sync_intro_keys_title"), Description: c.language.translate("i18n:ui_cloud_sync_intro_keys_description"), Icon: c.imageForTint(settingControlIconSource("key"), &iconTint, physicalImageSize(17, imageScale)), FallbackIcon: "⌁"},
		},
		PlanRows: []launcherview.CloudPlanRowProps{
			{Label: c.language.translate("i18n:ui_cloud_sync_plan_row_price"), FreeValue: freePrice, ProValue: proPrice},
			{Label: c.language.translate("i18n:ui_cloud_sync_plan_row_devices"), FreeValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_two_devices"), ProValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_unlimited_devices")},
			{Label: c.language.translate("i18n:ui_cloud_sync_plan_row_sync_mode"), FreeValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_manual_sync"), ProValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_auto_sync")},
			{Label: c.language.translate("i18n:ui_cloud_sync_plan_row_frequency"), FreeValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_strict_sync_limit"), ProValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_relaxed_sync_limit")},
			{Label: c.language.translate("i18n:ui_cloud_sync_plan_row_scope"), FreeValue: c.language.translate("i18n:ui_cloud_sync_plan_scope_free"), ProValue: c.language.translate("i18n:ui_cloud_sync_plan_feature_everything_free")},
		},
	}
}

// cloudBillingPriceWithTrial appends the localized free-trial hint to the Pro price.
func cloudBillingPriceWithTrial(language *languageCatalog, price cloudBillingPlanPrice, trial *cloudBillingPlanTrial, loaded bool) string {
	proPrice := cloudBillingPriceText(price)
	if proPrice == "" {
		if loaded {
			return language.translate("i18n:ui_cloud_sync_plan_price_unavailable")
		}
		return language.translate("i18n:ui_cloud_sync_plan_price_loading")
	}
	trialText := cloudBillingTrialText(language, trial)
	if trialText == "" {
		return proPrice
	}
	return strings.ReplaceAll(strings.ReplaceAll(language.translate("i18n:ui_cloud_sync_plan_price_with_trial"), "{price}", proPrice), "{trial}", trialText)
}

func cloudBillingTrialText(language *languageCatalog, trial *cloudBillingPlanTrial) string {
	if trial == nil {
		return ""
	}
	if strings.EqualFold(trial.Interval, "month") && trial.IntervalCount == 1 {
		return language.translate("i18n:ui_cloud_sync_plan_trial_one_month")
	}
	if strings.EqualFold(trial.Interval, "month") && trial.IntervalCount > 1 {
		return strings.ReplaceAll(language.translate("i18n:ui_cloud_sync_plan_trial_months"), "{count}", fmt.Sprint(trial.IntervalCount))
	}
	if trial.Days > 0 {
		return strings.ReplaceAll(language.translate("i18n:ui_cloud_sync_plan_trial_days"), "{days}", fmt.Sprint(trial.Days))
	}
	return strings.TrimSpace(trial.Formatted)
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
func (c *cloudSettingsController) cloudAccountViewProps(snapshot settingsSnapshot, contentWidth, imageScale float32) launcherview.CloudAccountProps {
	status := c.language.translate("i18n:ui_cloud_sync_plan_free_status")
	if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
		status = c.language.translate("i18n:ui_cloud_sync_plan_pro_status")
	}
	if snapshot.cloud.Account.SessionExpired {
		status = c.language.translate("i18n:ui_cloud_sync_account_session_expired")
	}
	labelWidth := cloudSettingsLabelWidth(contentWidth, 220)
	return launcherview.CloudAccountProps{
		SectionLabel:           c.language.translate("i18n:ui_cloud_sync_account"),
		LoggedIn:               snapshot.cloud.Account.LoggedIn,
		LabelWidth:             labelWidth,
		LoginLabel:             c.language.translate("i18n:ui_cloud_sync_account_login"),
		RegisterLabel:          c.language.translate("i18n:ui_cloud_sync_account_register"),
		EmailLabel:             c.language.translate("i18n:ui_cloud_sync_account_email"),
		Email:                  snapshot.cloud.Account.Email,
		PlanLabel:              c.language.translate("i18n:ui_cloud_sync_plan_status"),
		PlanTips:               c.language.translate("i18n:ui_cloud_sync_plan_status_tips"),
		PlanStatus:             status,
		BillingLabel:           c.language.translate("i18n:ui_cloud_sync_billing_help"),
		BillingTips:            c.language.translate("i18n:ui_cloud_sync_billing_help_tips"),
		SupportLabel:           c.language.translate("i18n:ui_cloud_sync_contact_support"),
		InfoIcon:               c.imageForTint(settingNavIconSource("about"), &snapshot.palette.TextSecondary, physicalImageSize(14, imageScale)),
		SupportIcon:            c.imageForTint(settingControlIconSource("email"), &snapshot.palette.Text, physicalImageSize(16, imageScale)),
		ActionsEnabled:         snapshot.cloud.Busy == "",
		OnLogin:                func() { c.openCloudAccountForm("login") },
		OnRegister:             func() { c.openCloudAccountForm("register") },
		OnOpenAccountMenu:      func() { c.toggleCloudActionMenu("account") },
		OnOpenSubscriptionMenu: func() { c.toggleCloudActionMenu("subscription") },
		OnPlanTooltip:          c.setCloudPlanTooltip,
		OnSupport:              c.openCloudSupportEmail,
	}
}

// setCloudPlanTooltip keeps the rich comparison inside the settings window instead of falling back to the native plain-text tooltip service.
func (c *cloudSettingsController) setCloudPlanTooltip(inside bool, anchor woxui.Rect) {
	if !inside {
		c.settings.cloudPlanTooltipRevision.Add(1)
		if c.settings.cloudPlanTooltip == nil {
			return
		}
		c.settings.cloudPlanTooltip = nil
		c.settings.invalidateSettingsWindow()
		return
	}
	if c.settings.cloudPlanTooltip != nil && c.settings.cloudPlanTooltip.anchor == anchor {
		return
	}
	c.ensureCloudBillingPlan()
	revision := c.settings.cloudPlanTooltipRevision.Add(1)
	util.Go(c.lifecycleCtx, "show cloud plan tooltip after dwell", func() {
		if !c.waitHoverTooltipDelay(&c.settings.cloudPlanTooltipRevision, revision) {
			return
		}
		apply := func() {
			if revision != c.settings.cloudPlanTooltipRevision.Load() {
				return
			}
			if c.settings.cloudPlanTooltip != nil && c.settings.cloudPlanTooltip.anchor == anchor {
				return
			}
			c.settings.cloudPlanTooltip = &cloudPlanTooltipState{anchor: anchor}
			c.settings.invalidateSettingsWindow()
		}
		if err := c.runOnUI("show cloud plan tooltip", apply); err != nil {
			util.GetLogger().Warn(c.lifecycleCtx, err.Error())
		}
	})
}

// cloudSettingsLabelWidth keeps login-state fields on Flutter's wide-form grid while preserving room for their controls in narrow settings windows.
func cloudSettingsLabelWidth(contentWidth, reservedWidth float32) float32 {
	return min(float32(520), max(float32(220), contentWidth-reservedWidth))
}

// cloudSyncViewProps prepares status text and the sync or join action.
func (c *cloudSettingsController) cloudSyncViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudSyncProps {
	label, detail, color, statusIcon := c.cloudSyncPresentation(snapshot)
	ready := cloudSyncReady(snapshot)
	buttonLabel := c.language.translate("i18n:ui_cloud_sync_sync_now")
	buttonAction := func() {
		if !ready || !snapshot.cloud.Account.SyncEnabled || !snapshot.cloud.Sync.Enabled {
			c.beginCloudBootstrap()
			return
		}
		c.runCloudAction("sync", func(ctx context.Context) error { return c.services.SyncCloud(ctx, c.sessionID) })
	}
	if cloudCurrentDeviceRevoked(snapshot.cloud.Devices) {
		buttonLabel = c.language.translate("i18n:ui_cloud_sync_join")
		buttonAction = func() {
			c.runCloudAction("join", func(ctx context.Context) error {
				return c.services.JoinCloudDevice(ctx, c.sessionID)
			})
		}
	}
	detailColor := snapshot.palette.TextSecondary
	if statusIcon == icons.ControlError {
		detailColor = color
	}
	return launcherview.CloudSyncProps{
		SectionLabel:  c.language.translate("i18n:ui_cloud_sync_sync"),
		StatusIcon:    c.imageForTint(fromCoreImage(icons.Get(statusIcon)), &color, physicalImageSize(24, imageScale)),
		ButtonIcon:    c.imageForTint(fromCoreImage(icons.Get(icons.ControlRefresh)), &snapshot.palette.Text, physicalImageSize(16, imageScale)),
		Label:         label,
		Detail:        detail,
		Color:         detailColor,
		ButtonLabel:   cloudBusyLabel(snapshot, "sync", buttonLabel),
		ButtonEnabled: snapshot.cloud.Busy == "" && !snapshot.cloud.Loading && !snapshot.cloud.Account.SessionExpired && snapshot.cloud.Account.SyncEligible,
		OnSync:        buttonAction,
	}
}

func (c *cloudSettingsController) cloudSyncPresentation(snapshot settingsSnapshot) (string, string, woxui.Color, string) {
	muted := snapshot.palette.TextSecondary
	errorColor := snapshot.palette.Error
	if snapshot.cloud.Loading {
		return c.language.translate("i18n:ui_cloud_sync_loading"), "", muted, icons.ControlHourglassEmpty
	}
	if snapshot.cloud.Account.SessionExpired {
		return c.language.translate("i18n:ui_cloud_sync_sync_error"), c.language.translate("i18n:ui_cloud_sync_account_session_expired"), errorColor, icons.ControlError
	}
	if snapshot.cloud.Error != "" {
		return c.language.translate("i18n:ui_cloud_sync_sync_error"), snapshot.cloud.Error, errorColor, icons.ControlError
	}
	if progress := snapshot.cloud.Sync.Progress; progress != nil && progress.Active {
		return c.language.translate("i18n:ui_cloud_sync_syncing"), c.formatCloudSyncProgress(progress, snapshot.cloud.Busy == "sync"), muted, icons.ControlHourglassEmpty
	}
	if snapshot.cloud.Busy == "sync" {
		return c.language.translate("i18n:ui_cloud_sync_syncing"), c.language.translate("i18n:ui_cloud_sync_progress_starting"), muted, icons.ControlHourglassEmpty
	}
	if state := snapshot.cloud.Sync.State; state != nil && state.LastError != "" {
		return c.language.translate("i18n:ui_cloud_sync_sync_error"), state.LastError, errorColor, icons.ControlError
	}
	if !snapshot.cloud.Account.SyncEligible {
		return c.language.translate("i18n:ui_cloud_sync_unsynced"), c.language.translate("i18n:ui_cloud_sync_subscription_required"), muted, icons.SettingsDataCloudsync
	}
	if !cloudSyncReady(snapshot) {
		return c.language.translate("i18n:ui_cloud_sync_unsynced"), "", muted, icons.SettingsDataCloudsync
	}
	if !snapshot.cloud.Account.SyncEnabled || !snapshot.cloud.Sync.Enabled {
		return c.language.translate("i18n:ui_cloud_sync_disabled"), "", muted, icons.SettingsDataCloudsync
	}
	lastSync := max(cloudStateTimestamp(snapshot.cloud.Sync.State, true), cloudStateTimestamp(snapshot.cloud.Sync.State, false))
	return c.language.translate("i18n:ui_cloud_sync_synced"), c.language.translate("i18n:ui_cloud_sync_last_sync_time") + " · " + c.formatCloudActivityTime(lastSync, util.GetSystemTime()), snapshot.palette.Success, icons.ControlCheckCircle
}

func cloudSyncReady(snapshot settingsSnapshot) bool {
	return snapshot.cloud.Sync.KeyStatus.Available && snapshot.cloud.Sync.State != nil && snapshot.cloud.Sync.State.Bootstrapped
}

// formatCloudSyncProgress mirrors the Flutter cloud-sync progress copy so users see the active stage and item count.
func (c *cloudSettingsController) formatCloudSyncProgress(progress *cloudSyncProgress, isBusy bool) string {
	if progress == nil || !progress.Active || strings.TrimSpace(progress.Operation) == "" {
		if isBusy {
			return c.language.translate("i18n:ui_cloud_sync_progress_starting")
		}
		return ""
	}

	countSuffix := ""
	if progress.Current > 0 || progress.Total > 0 {
		countText := fmt.Sprintf("%d", progress.Current)
		if progress.Total > 0 {
			countText = fmt.Sprintf("%d/%d", progress.Current, progress.Total)
		}
		countSuffix = strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_progress_count"), "{count}", countText)
	}

	switch progress.Operation {
	case cloudsync.CloudSyncProgressOperationSnapshot:
		return c.language.translate("i18n:ui_cloud_sync_progress_snapshot")
	case cloudsync.CloudSyncProgressOperationPush:
		return strings.ReplaceAll(strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_progress_uploading"), "{target}", c.cloudSyncProgressTarget(progress)), "{count}", countSuffix)
	case cloudsync.CloudSyncProgressOperationPull:
		return strings.ReplaceAll(strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_progress_downloading"), "{target}", c.cloudSyncProgressTarget(progress)), "{count}", countSuffix)
	case cloudsync.CloudSyncProgressOperationRestore:
		return strings.ReplaceAll(strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_progress_restoring"), "{target}", c.cloudSyncProgressTarget(progress)), "{count}", countSuffix)
	default:
		return c.language.translate("i18n:ui_cloud_sync_progress_starting")
	}
}

// cloudSyncProgressTarget resolves the current sync item into a short localized label.
func (c *cloudSettingsController) cloudSyncProgressTarget(progress *cloudSyncProgress) string {
	if progress == nil {
		return c.language.translate("i18n:ui_cloud_sync_progress_data")
	}
	switch progress.EntityType {
	case cloudsync.EntityWoxSetting:
		return c.language.translate("i18n:ui_cloud_sync_progress_wox_setting")
	case cloudsync.EntityPluginSetting, cloudsync.EntityInstalledPlugin:
		pluginName := c.cloudSyncProgressPluginName(progress.PluginID)
		if pluginName == "" {
			pluginName = c.cloudSyncProgressPluginName(progress.Key)
		}
		if pluginName == "" {
			pluginName = c.language.translate("i18n:ui_cloud_sync_progress_data")
		}
		return strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_progress_plugin"), "{plugin}", pluginName)
	default:
		return c.language.translate("i18n:ui_cloud_sync_progress_data")
	}
}

// cloudSyncProgressPluginName prefers installed/store display names over raw plugin IDs.
func (c *cloudSettingsController) cloudSyncProgressPluginName(pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return ""
	}
	if c.pluginSettings() != nil {
		for _, catalog := range []bool{false, true} {
			plugins, _ := c.pluginSettings().CachedPlugins(catalog)
			for _, item := range plugins {
				if item.ID != pluginID {
					continue
				}
				if name := strings.TrimSpace(item.Name); name != "" {
					return name
				}
			}
		}
	}
	return plugin.ResolvePluginDisplayName(context.Background(), pluginID)
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

const cloudDeviceInactiveAfter = 14 * 24 * time.Hour

// cloudDeviceInactive reports whether a device should be omitted from the settings
// list because it has not been seen for more than two weeks. The current device
// stays visible so this machine cannot disappear from the list.
func cloudDeviceInactive(device cloudDevice, now time.Time) bool {
	if device.Current {
		return false
	}
	if device.LastSeenAt <= 0 {
		return false
	}
	return now.Sub(time.UnixMilli(device.LastSeenAt)) > cloudDeviceInactiveAfter
}

// cloudDevicesViewProps prepares device labels and revoke callbacks.
func (c *cloudSettingsController) cloudDevicesViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudDevicesProps {
	items := make([]launcherview.CloudDeviceProps, 0, len(snapshot.cloud.Devices.Devices))
	now := util.GetSystemTime()
	for index, device := range snapshot.cloud.Devices.Devices {
		if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") && device.RevokedAt > 0 {
			continue
		}
		if cloudDeviceInactive(device, now) {
			continue
		}
		name := device.DeviceName
		if strings.TrimSpace(name) == "" {
			name = device.DeviceID
		}
		currentLabel := ""
		if device.Current {
			currentLabel = c.language.translate("i18n:ui_cloud_sync_devices_current")
		}
		items = append(items, launcherview.CloudDeviceProps{
			ID:            fmt.Sprintf("cloud-revoke-%d", index),
			Name:          name,
			Detail:        cloudDevicePlatform(c.language, device.Platform),
			LastSeen:      c.formatCloudActivityTime(device.LastSeenAt, now),
			CurrentLabel:  currentLabel,
			Icon:          c.imageForTint(fromCoreImage(icons.Get(cloudDevicePlatformIcon(device.Platform))), &snapshot.palette.TextSecondary, physicalImageSize(24, imageScale)),
			RevokeLabel:   c.language.translate("i18n:ui_cloud_sync_devices_revoke"),
			ShowRevoke:    !strings.EqualFold(snapshot.cloud.Account.Plan, "pro") && !device.Current && device.RevokedAt == 0,
			RevokeEnabled: snapshot.cloud.Busy == "",
			OnRevoke: func() {
				c.runCloudAction("revoke", func(ctx context.Context) error {
					_, err := c.services.RevokeCloudDevice(ctx, c.sessionID, device.DeviceID)
					return err
				})
			},
		})
	}
	tips := c.language.translate("i18n:ui_cloud_sync_devices_pro_tips")
	if !strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
		limit := 2
		if snapshot.cloud.Devices.DeviceLimit != nil && *snapshot.cloud.Devices.DeviceLimit > 0 {
			limit = *snapshot.cloud.Devices.DeviceLimit
		}
		tips = strings.ReplaceAll(c.language.translate("i18n:ui_cloud_sync_devices_free_tips"), "{count}", fmt.Sprint(snapshot.cloud.Devices.DeviceCount))
		tips = strings.ReplaceAll(tips, "{limit}", fmt.Sprint(limit))
	}
	return launcherview.CloudDevicesProps{
		SectionLabel:    c.language.translate("i18n:ui_cloud_sync_devices"),
		Tips:            tips,
		LastActiveLabel: c.language.translate("i18n:ui_cloud_sync_devices_last_active"),
		RefreshLabel:    cloudRefreshLabel(c.language, snapshot),
		RefreshIcon:     c.imageForTint(settingControlIconSource("refresh"), &snapshot.palette.Text, physicalImageSize(16, imageScale)),
		RefreshEnabled:  !snapshot.cloud.Loading && snapshot.cloud.Busy == "",
		EmptyLabel:      c.language.translate("i18n:ui_cloud_sync_devices_empty"),
		Items:           items,
		OnRefresh:       func() { util.Go(c.lifecycleCtx, "reload cloud sync devices", c.reloadCloudSync) },
	}
}

// cloudDevicePlatformIcon keeps platform aliases consistent with device labels.
func cloudDevicePlatformIcon(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "darwin", "macos", "mac":
		return icons.SystemMacOS
	case "windows", "win32", "win":
		return icons.SystemWindows
	case "linux":
		return icons.SystemLinux
	default:
		return icons.SystemDevice
	}
}

// cloudDevicePlatform matches the user-facing platform names used by Flutter.
func cloudDevicePlatform(language *languageCatalog, platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "darwin", "macos", "mac":
		return "macOS"
	case "windows", "win32", "win":
		return "Windows"
	case "linux":
		return "Linux"
	default:
		return language.translate("i18n:ui_cloud_sync_devices_unknown_platform")
	}
}

func cloudRefreshLabel(language *languageCatalog, snapshot settingsSnapshot) string {
	if snapshot.cloud.Loading {
		return language.translate("i18n:ui_cloud_sync_loading")
	}
	return language.translate("i18n:ui_cloud_sync_refresh")
}

// cloudPluginExclusionsViewProps prepares the visible plugin boundary and toggle actions.
func (c *cloudSettingsController) cloudPluginExclusionsViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudPluginExclusionsProps {
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
			name = pluginID + " (" + c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_uninstalled") + ")"
		}
		var icon *woxui.Image
		if plugin.Icon.ImageType != "" {
			icon = c.imageForSurface(plugin.Icon, physicalImageSize(18, imageScale), settingsPalette().Background)
		}
		items = append(items, launcherview.CloudPluginExclusionProps{
			ID: fmt.Sprintf("cloud-plugin-%d", index), Name: name, PluginID: pluginID, Icon: icon,
			OnDelete: func() { c.toggleCloudPluginExclusion(pluginID) },
		})
	}
	foreground := snapshot.palette.TextSecondary
	return launcherview.CloudPluginExclusionsProps{
		SectionLabel:   c.language.translate("i18n:ui_cloud_sync_plugin_exclusions"),
		Tips:           c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_tips"),
		ColumnLabel:    c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_plugin"),
		Items:          items,
		AddLabel:       c.language.translate("i18n:ui_add"),
		OperationLabel: c.language.translate("i18n:ui_operation"),
		AddIcon:        c.imageForTint(settingControlIconSource("add"), &foreground, physicalImageSize(15, imageScale)),
		DeleteIcon:     c.imageForTint(settingControlIconSource("delete"), &foreground, physicalImageSize(16, imageScale)),
		OnAdd:          c.openCloudPluginExclusionDialog,
	}
}

// buildCloudPluginExclusionOverlay maps the transient row editor onto the Flutter-style dialog shell.
func (c *cloudSettingsController) buildCloudPluginExclusionOverlay(dialog *cloudPluginExclusionDialogSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	plugins := c.cloudPluginExclusionPlugins()
	choices := make([]launcherview.SettingsChoice, 0, len(plugins))
	selectedName := dialog.Selected
	var selectedIcon *woxui.Image
	for _, plugin := range plugins {
		label := plugin.Name
		if strings.TrimSpace(label) == "" {
			label = plugin.ID
		}
		leading := (*woxui.Image)(nil)
		if plugin.Icon.ImageType != "" {
			leading = c.imageForSurface(plugin.Icon, physicalImageSize(18, imageScale), settingsPalette().Background)
		}
		choices = append(choices, launcherview.SettingsChoice{Value: plugin.ID, Label: label, Leading: leading})
		if plugin.ID == dialog.Selected {
			selectedName = label
			selectedIcon = leading
		}
	}
	return launcherview.CloudPluginExclusionDialog(launcherview.CloudPluginExclusionDialogProps{
		Width: width, Height: height, PanelWidth: 648, PanelHeight: launcherview.CloudPluginExclusionDialogHeight,
		FieldLabel: c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_plugin"), Description: c.language.translate("i18n:ui_cloud_sync_plugin_exclusions_plugin_tips"),
		Selected: dialog.Selected, SelectedName: selectedName, SelectedIcon: selectedIcon, Choices: choices, ChoiceAnchor: dialog.ChoiceAnchor, ChoiceOpen: dialog.ChoiceOpen,
		CancelLabel: c.language.translate("i18n:ui_cancel"), SaveLabel: c.language.translate("i18n:ui_save"), Window: c.settings.settingsNativeWindow(), Theme: palette,
		OnChoiceTap: c.openCloudPluginExclusionChoice, OnChoose: c.chooseCloudPluginExclusion, OnCancel: c.closeCloudPluginExclusionDialog, OnSave: c.saveCloudPluginExclusion,
	})
}

// cloudConfigNotesViewProps translates platform-aware sync caveats for the view.
func (c *cloudSettingsController) cloudConfigNotesViewProps(snapshot settingsSnapshot, imageScale float32) launcherview.CloudConfigNotesProps {
	notes := [][3]string{
		{"clipboard", "partial", "clipboard"}, {"notes", "partial", "notes"}, {"query_hotkeys", "platform", "query_hotkeys"}, {"launch_hotkeys", "platform", "launch_hotkeys"},
		{"ignored_hotkey_apps", "platform", "ignored_hotkey_apps"}, {"autostart", "platform", "autostart"}, {"http_proxy", "platform", "http_proxy"},
		{"runtime_paths", "platform", "runtime_paths"}, {"app_font", "platform", "app_font"}, {"app_indexing", "platform", "app_indexing"},
		{"file_search", "platform", "file_search"}, {"quickjump", "platform", "quickjump"}, {"local_plugin_directories", "platform", "local_plugin_directories"},
		{"folder_favorites", "platform", "folder_favorites"}, {"shell", "platform", "shell"}, {"browser_bookmarks", "platform", "browser_bookmarks"},
		{"space_quick_look", "platform", "space_quick_look"}, {"plugin_install_state", "reproducible", "plugin_install_state"}, {"custom_themes", "synced", "custom_themes"},
	}
	items := make([]launcherview.CloudConfigNoteProps, 0, len(notes))
	for _, note := range notes {
		items = append(items, launcherview.CloudConfigNoteProps{
			Item:    c.language.translate("i18n:ui_cloud_sync_config_note_" + note[0]),
			Mode:    c.language.translate("i18n:ui_cloud_sync_config_notes_mode_" + note[1]),
			Tooltip: c.language.translate("i18n:ui_cloud_sync_config_note_" + note[2] + "_tips"),
		})
	}
	return launcherview.CloudConfigNotesProps{
		SectionLabel: c.language.translate("i18n:ui_cloud_sync_config_notes"),
		Tips:         c.language.translate("i18n:ui_cloud_sync_config_notes_tips"),
		ItemLabel:    c.language.translate("i18n:ui_cloud_sync_config_notes_item"),
		ModeLabel:    c.language.translate("i18n:ui_cloud_sync_config_notes_mode"),
		InfoIcon:     c.imageForTint(settingNavIconSource("about"), &snapshot.palette.TextSecondary, physicalImageSize(14, imageScale)),
		Items:        items,
		OnTooltip:    c.setSettingChoiceTooltip,
	}
}

// cloudActionMenuViewProps prepares the active account or subscription menu.
func (c *cloudSettingsController) cloudActionMenuViewProps(snapshot settingsSnapshot) *launcherview.CloudActionMenuProps {
	if snapshot.cloud.ActionMenu == "" {
		return nil
	}
	type menuAction struct {
		id     string
		label  string
		action string
	}
	actions := []menuAction{
		{id: "change-password", label: c.language.translate("i18n:ui_cloud_sync_account_change_password"), action: "change-password"},
		{id: "logout", label: c.language.translate("i18n:ui_cloud_sync_account_logout"), action: "logout"},
	}
	top := float32(145)
	if snapshot.cloud.ActionMenu == "subscription" {
		billingLabel := c.language.translate("i18n:ui_cloud_sync_subscribe")
		if strings.EqualFold(snapshot.cloud.Account.Plan, "pro") {
			billingLabel = c.language.translate("i18n:ui_cloud_sync_manage_subscription")
		}
		actions = []menuAction{
			{id: "refresh", label: c.language.translate("i18n:ui_cloud_sync_refresh_status"), action: "refresh"},
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
		onTap := func() { c.runCloudMenuAction(entry.action) }
		if strings.HasPrefix(entry.action, "plugin:") {
			pluginID := strings.TrimPrefix(entry.action, "plugin:")
			onTap = func() {
				c.closeCloudActionMenu()
				c.toggleCloudPluginExclusion(pluginID)
			}
		}
		items = append(items, launcherview.CloudActionMenuItemProps{ID: "cloud-menu-" + entry.id, Label: entry.label, OnTap: onTap})
	}
	return &launcherview.CloudActionMenuProps{Top: top, Modal: snapshot.cloud.ActionMenu == "plugins", Items: items}
}

func (c *cloudSettingsController) formatCloudTime(timestamp int64) string {
	if timestamp <= 0 {
		return c.language.translate("i18n:ui_cloud_sync_never")
	}
	return time.UnixMilli(timestamp).Local().Format("2006-01-02 15:04:05")
}

// formatCloudActivityTime keeps recent activity compact while retaining the year for older dates.
func (c *cloudSettingsController) formatCloudActivityTime(timestamp int64, now time.Time) string {
	if timestamp <= 0 {
		return c.language.translate("i18n:ui_cloud_sync_never")
	}
	date := time.UnixMilli(timestamp).In(now.Location())
	if date.Format("2006-01-02") == now.Format("2006-01-02") {
		return c.language.translate("i18n:ui_cloud_sync_today") + " " + date.Format("15:04")
	}
	if date.Year() == now.Year() {
		return date.Format("01-02 15:04")
	}
	return date.Format("2006-01-02 15:04")
}

// buildCloudFormOverlay maps account form state into typed view props.
func (c *cloudSettingsController) buildCloudFormOverlay(snapshot *cloudFormSnapshot, palette woxcomponent.ControlTheme, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(408), max(float32(320), width-64))
	fields := make([]launcherview.CloudFormFieldProps, 0, len(snapshot.definitions))
	window := c.settings.settingsNativeWindow()
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
			Label:         c.language.translate(definition.Value.Label),
			State:         state,
			Focused:       focused,
			Autofocus:     focused,
			Protected:     definition.Type == "password",
			Window:        window,
			Controller:    controller,
			FocusNode:     focusNode,
			OnChanged:     func(value string) { c.setCloudFormText(index, value) },
			OnFocusChange: func(focused bool) { c.setCloudFormFieldFocused(index, focused) },
		}
		if definition.Type == "checkbox" {
			field.Checked = snapshot.values[definition.Value.Key] == "true"
			field.OnTap = func() {
				c.focusCloudFormField(index)
				c.changeCloudFormField(index, 1)
			}
		}
		fields = append(fields, field)
	}

	linkPrefix := ""
	links := []launcherview.CloudFormLinkProps{}
	var fieldLink *launcherview.CloudFormLinkProps
	switch snapshot.kind {
	case "register":
		linkPrefix = c.language.translate("i18n:ui_cloud_sync_account_accept_prefix")
		links = append(links,
			launcherview.CloudFormLinkProps{ID: "cloud-terms", Label: c.language.translate("i18n:ui_cloud_sync_account_terms"), Width: 112, OnTap: func() { c.openCloudLegalPage("/terms") }},
			launcherview.CloudFormLinkProps{ID: "cloud-privacy", Label: c.language.translate("i18n:ui_cloud_sync_account_privacy"), Width: 112, OnTap: func() { c.openCloudLegalPage("/privacy") }},
		)
	case "login":
		fieldLink = &launcherview.CloudFormLinkProps{ID: "cloud-forgot-password", Label: c.language.translate("i18n:ui_cloud_sync_account_reset_request"), Width: 96, OnTap: func() { c.openCloudAccountForm("reset-request") }}
	case "verify":
		links = append(links, launcherview.CloudFormLinkProps{ID: "cloud-resend-code", Label: "Resend code", Width: 112, OnTap: c.resendCloudVerification})
	}

	feedback := c.language.translate(snapshot.notice)
	feedbackColor := palette.TextSecondary
	theme := palette
	if snapshot.error != "" {
		feedback = c.language.translate(snapshot.error)
		feedbackColor = theme.Error
	}
	submitLabel := c.language.translate("i18n:ui_cloud_sync_confirm")
	if snapshot.saving {
		submitLabel = c.language.translate("i18n:ui_cloud_sync_loading")
	}
	submitEnabled := !snapshot.saving
	if snapshot.kind == "register" && snapshot.values["AcceptedLegal"] != "true" {
		submitEnabled = false
	}
	return launcherview.CloudFormOverlay(launcherview.CloudFormOverlayProps{
		Width:         width,
		Height:        height,
		PanelWidth:    panelWidth,
		Title:         c.language.translate(snapshot.title),
		Description:   c.language.translate(cloudFormDescription(snapshot)),
		Fields:        fields,
		LinkPrefix:    linkPrefix,
		Links:         links,
		FieldLink:     fieldLink,
		Feedback:      feedback,
		FeedbackColor: feedbackColor,
		CancelLabel:   c.language.translate("i18n:ui_cloud_sync_cancel"),
		SubmitLabel:   submitLabel,
		SubmitEnabled: submitEnabled,
		Saving:        snapshot.saving,
		Theme:         theme,
		OnCancel:      c.closeCloudForm,
		OnSubmit:      c.submitCloudForm,
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
			return "i18n:ui_cloud_sync_bootstrap_restore_description"
		}
		return "i18n:ui_cloud_sync_bootstrap_start_description"
	default:
		return "Use your Wox account credentials to continue."
	}
}
