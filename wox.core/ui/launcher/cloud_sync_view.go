package launcher

import (
	"fmt"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildCloudSettingsPage renders account, encrypted sync, and device state through core-owned routes.
func (a *App) buildCloudSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-82)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 94, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync"), Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
			woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_description"), Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
		}}},
	}
	accountHeight := float32(190)
	if !snapshot.cloudAccount.LoggedIn {
		accountHeight = 142
	}
	children = append(children,
		a.buildCloudSectionHeader(snapshot, a.translate("i18n:ui_cloud_sync_account"), contentWidth, nil, 0),
		a.buildCloudAccountCard(snapshot, contentWidth, accountHeight),
	)
	if snapshot.cloudAccount.LoggedIn {
		children = append(children,
			a.buildCloudSectionHeader(snapshot, a.translate("i18n:ui_cloud_sync_sync_status"), contentWidth, nil, 0),
			a.buildCloudSyncCard(snapshot, contentWidth),
		)
		deviceHeight := float32(len(snapshot.cloudDevices.Devices) * 62)
		if len(snapshot.cloudDevices.Devices) == 0 {
			deviceHeight = 72
		}
		refresh := a.buildFormTableButton("cloud-refresh", cloudRefreshLabel(a, snapshot), 104, !snapshot.cloudLoading && snapshot.cloudBusy == "", false, func() { go a.reloadCloudSync() }, snapshot.palette)
		children = append(children,
			a.buildCloudSectionHeader(snapshot, a.translate("i18n:ui_cloud_sync_devices"), contentWidth, refresh, 104),
			a.buildCloudDeviceCard(snapshot, contentWidth, deviceHeight),
			a.buildCloudSectionHeader(snapshot, a.translate("i18n:ui_cloud_sync_plugin_exclusions"), contentWidth, nil, 0),
		)
		children = append(children, a.buildCloudPluginExclusionsCard(snapshot, contentWidth, 282))
		children = append(children,
			a.buildCloudSectionHeader(snapshot, a.translate("i18n:ui_cloud_sync_config_notes"), contentWidth, nil, 0),
			a.buildCloudConfigNotesCard(snapshot, contentWidth, 136),
		)
	}
	message := snapshot.cloudError
	messageColor := woxui.Color{R: 232, G: 95, B: 95, A: 255}
	if message == "" {
		message = snapshot.note
		messageColor = snapshot.palette.resultSubtitle
	}
	footerHeight := float32(0)
	if message != "" {
		footerHeight = 34
		children = append(children, woxwidget.Container{Width: contentWidth, Height: footerHeight, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.TextBlock{
			Value: message, Width: contentWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: messageColor,
		}})
	}

	contentHeight := 94 + 42 + accountHeight + footerHeight + float32(len(children)-1)*4
	if snapshot.cloudAccount.LoggedIn {
		contentHeight += 42 + 118
		if len(snapshot.cloudDevices.Devices) == 0 {
			contentHeight += 42 + 72
		} else {
			contentHeight += 42 + float32(len(snapshot.cloudDevices.Devices)*62)
		}
		contentHeight += 42 + 282 + 42 + 136
	}
	viewportHeight := max(float32(1), height-58)
	a.setCloudPageGeometry(viewportHeight, contentHeight)
	page := woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 38, Top: 34, Right: 44, Bottom: 24}, Child: woxwidget.Gesture{
		ID: "cloud-page-scroll", OnScroll: func(delta woxui.Point) { a.scrollCloudPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.cloudPageScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: children},
		},
	}}
	if snapshot.cloudActionMenu == "" {
		return page
	}
	menuTop := float32(145)
	if snapshot.cloudActionMenu == "subscription" {
		menuTop = 205
	}
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: page},
		{Child: woxwidget.Gesture{ID: "cloud-action-menu-shade", OnTap: a.closeCloudActionMenu, Child: woxwidget.Container{Width: width, Height: height}}},
		{Left: max(float32(20), width-236), Top: menuTop, Child: a.buildCloudActionMenu(snapshot, 196)},
	}}
}

func (a *App) buildCloudSectionHeader(snapshot settingsSnapshot, label string, width float32, action woxwidget.Widget, actionWidth float32) woxwidget.Widget {
	if action == nil {
		action = woxwidget.Painter{Width: 0, Height: 0}
	}
	return woxwidget.Container{Width: width, Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 1, Color: snapshot.palette.previewSplit},
		woxwidget.Container{Width: width, Height: 41, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), width-actionWidth), Height: 32, Padding: woxwidget.Insets{Top: 6}, Child: woxwidget.Text{Value: strings.ToUpper(label), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle}},
			action,
		}},
		}}}}
}

func cloudRefreshLabel(a *App, snapshot settingsSnapshot) string {
	if snapshot.cloudLoading {
		return a.translate("i18n:ui_cloud_sync_loading")
	}
	return a.translate("i18n:ui_cloud_sync_refresh_status")
}

func (a *App) buildCloudAccountCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if !snapshot.cloudAccount.LoggedIn {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Top: 12, Right: 2, Bottom: 12}, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_account"), Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.TextBlock{Value: a.translate("i18n:ui_cloud_sync_intro_description"), Width: width - 36, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: snapshot.palette.resultSubtitle},
				woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
					a.buildFormTableButton("cloud-login", a.translate("i18n:ui_cloud_sync_account_login"), 96, snapshot.cloudBusy == "", true, func() { a.openCloudAccountForm("login") }, snapshot.palette),
					a.buildFormTableButton("cloud-register", a.translate("i18n:ui_cloud_sync_account_register"), 124, snapshot.cloudBusy == "", false, func() { a.openCloudAccountForm("register") }, snapshot.palette),
				}},
			},
		}}
	}
	status := a.translate("i18n:ui_cloud_sync_plan_free_status")
	if strings.EqualFold(snapshot.cloudAccount.Plan, "pro") {
		status = a.translate("i18n:ui_cloud_sync_plan_pro_status")
	}
	if snapshot.cloudAccount.SessionExpired {
		status = a.translate("i18n:ui_cloud_sync_account_session_expired")
	}
	labelWidth := max(float32(220), width-390)
	valueWidth := max(float32(220), width-labelWidth)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Right: 2}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width - 4, Height: 50, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 50, Padding: woxwidget.Insets{Top: 15}, Child: woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_account_email"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
			a.buildCloudValueAction(snapshot, "cloud-account-action", snapshot.cloudAccount.Email, valueWidth, func() { a.toggleCloudActionMenu("account") }),
		}}},
		woxwidget.Container{Width: width - 4, Height: 66, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 66, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_plan_status"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_plan_status_tips"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
			}}},
			a.buildCloudValueAction(snapshot, "cloud-plan-action", status, valueWidth, func() { a.toggleCloudActionMenu("subscription") }),
		}}},
		woxwidget.Container{Width: width - 4, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_billing_help"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_billing_help_tips"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Container{Width: valueWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				woxwidget.Painter{Width: max(float32(0), valueWidth-132), Height: 38},
				a.buildFormTableButton("cloud-support", a.translate("i18n:ui_cloud_sync_contact_support"), 132, snapshot.cloudBusy == "", false, a.openCloudSupportEmail, snapshot.palette),
			}}},
		}}},
	}}}
}

func (a *App) buildCloudValueAction(snapshot settingsSnapshot, id, value string, width float32, onTap func()) woxwidget.Widget {
	style := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	textWidth := min(max(float32(0), width-24), float32(len([]rune(value)))*8+8)
	if window := a.settingsNativeWindow(); window != nil {
		if metrics, err := window.MeasureText(value, style); err == nil {
			textWidth = min(max(float32(0), width-24), metrics.Size.Width+6)
		}
	}
	return woxwidget.Container{Width: width, Height: 50, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), width-textWidth-24), Height: 28},
		woxwidget.Container{Width: textWidth, Height: 28, Child: woxwidget.Text{Value: value, Style: style, Color: snapshot.palette.resultTitle}},
		woxwidget.Gesture{ID: id, OnTap: onTap, Child: woxwidget.Container{Width: 24, Height: 28, Padding: woxwidget.Insets{Left: 8, Top: 2}, Child: woxwidget.Text{Value: "⌄", Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle}}},
	}}}
}

func (a *App) buildCloudActionMenu(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	type menuAction struct {
		id     string
		label  string
		action string
	}
	actions := []menuAction{
		{id: "change-password", label: a.translate("i18n:ui_cloud_sync_account_change_password"), action: "change-password"},
		{id: "logout", label: a.translate("i18n:ui_cloud_sync_account_logout"), action: "logout"},
	}
	if snapshot.cloudActionMenu == "subscription" {
		billingLabel := a.translate("i18n:ui_cloud_sync_subscribe")
		if strings.EqualFold(snapshot.cloudAccount.Plan, "pro") {
			billingLabel = a.translate("i18n:ui_cloud_sync_manage_subscription")
		}
		actions = []menuAction{
			{id: "refresh", label: a.translate("i18n:ui_cloud_sync_refresh_status"), action: "refresh"},
			{id: "billing", label: billingLabel, action: "billing"},
		}
	}
	rows := make([]woxwidget.Widget, 0, len(actions))
	for _, entry := range actions {
		entry := entry
		rows = append(rows, woxwidget.Gesture{ID: "cloud-menu-" + entry.id, OnTap: func() { a.runCloudMenuAction(entry.action) }, Child: woxwidget.Container{
			Width: width - 12, Height: 40, Radius: 5, Color: snapshot.palette.actionBackground, Padding: woxwidget.Insets{Left: 12, Top: 11, Right: 12},
			Child: woxwidget.Text{Value: entry.label, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.actionText},
		}})
	}
	return woxwidget.Container{Width: width, Height: float32(len(rows))*40 + 12, Radius: 8, Color: snapshot.palette.actionBackground, Padding: woxwidget.UniformInsets(6), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

func (a *App) buildCloudSyncCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	label, detail, color := a.cloudSyncPresentation(snapshot)
	currentRevoked := cloudCurrentDeviceRevoked(snapshot.cloudDevices)
	ready := cloudSyncReady(snapshot)
	enabled := snapshot.cloudBusy == "" && !snapshot.cloudLoading && !snapshot.cloudAccount.SessionExpired && snapshot.cloudAccount.SyncEligible
	buttonLabel := a.translate("i18n:ui_cloud_sync_sync")
	buttonAction := func() {
		if !ready || !snapshot.cloudAccount.SyncEnabled || !snapshot.cloudSync.Enabled {
			a.beginCloudBootstrap()
			return
		}
		a.runCloudAction("sync", "/sync/push", map[string]any{})
	}
	if currentRevoked {
		buttonLabel = a.translate("i18n:ui_cloud_sync_join")
		buttonAction = func() { a.runCloudAction("join", "/sync/devices/join", map[string]any{}) }
	}
	button := a.buildFormTableButton("cloud-sync", cloudBusyLabel(snapshot, "sync", buttonLabel), 102, enabled, true, buttonAction, snapshot.palette)
	labelWidth := max(float32(220), width-260)
	return woxwidget.Container{Width: width, Height: 118, Padding: woxwidget.Insets{Left: 2, Top: 10, Right: 2, Bottom: 8}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 98, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_sync_status"), Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: color},
				woxwidget.TextBlock{Value: detail, Width: labelWidth, Height: 48, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Container{Width: max(float32(0), width-labelWidth-42), Height: 52, Padding: woxwidget.Insets{Top: 14}, Child: button},
		},
	}}
}

func (a *App) cloudSyncPresentation(snapshot settingsSnapshot) (string, string, woxui.Color) {
	muted := snapshot.palette.resultSubtitle
	errorColor := woxui.Color{R: 232, G: 95, B: 95, A: 255}
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

func (a *App) buildCloudDeviceCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(snapshot.cloudDevices.Devices))
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
		detail := strings.Title(strings.ToLower(device.Platform))
		labelWidth := max(float32(160), width-276)
		timeWidth := float32(150)
		var action woxwidget.Widget = woxwidget.Painter{Width: 104, Height: 38}
		if !device.Current && device.RevokedAt == 0 {
			action = a.buildFormTableButton(fmt.Sprintf("cloud-revoke-%d", index), a.translate("i18n:ui_cloud_sync_devices_revoke"), 96, snapshot.cloudBusy == "", false, func() {
				a.runCloudAction("revoke", "/sync/devices/revoke", map[string]string{"target_device_id": device.DeviceID})
			}, snapshot.palette)
		}
		rows = append(rows, woxwidget.Container{Width: width, Height: 62, Padding: woxwidget.Insets{Left: 2, Top: 9, Right: 2}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: labelWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
					woxwidget.Text{Value: name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
					woxwidget.Text{Value: detail, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
				}}},
				woxwidget.Container{Width: timeWidth, Height: 44, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{Value: a.formatCloudTime(device.LastSeenAt), Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle}},
				action,
			},
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: width, Height: 56, Padding: woxwidget.Insets{Left: 2, Top: 18}, Child: woxwidget.Text{
			Value: a.translate("i18n:ui_cloud_sync_devices_empty"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}})
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}

// buildCloudPluginExclusionsCard keeps the synced plugin boundary visible without opening another table modal.
func (a *App) buildCloudPluginExclusionsCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	rowsData := cloudPluginExclusionRows(snapshot.cloudPlugins, snapshot.data.CloudSyncDisabledPlugins)
	excluded := make(map[string]bool, len(snapshot.data.CloudSyncDisabledPlugins))
	for _, pluginID := range snapshot.data.CloudSyncDisabledPlugins {
		excluded[strings.TrimSpace(pluginID)] = true
	}
	bodyHeight := float32(206)
	a.setCloudPluginViewport(bodyHeight)
	rows := make([]woxwidget.Widget, 0, len(rowsData))
	for index, plugin := range rowsData {
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
		busy := snapshot.cloudBusy == "exclusion-"+plugin.ID
		if busy {
			buttonLabel += "…"
		}
		labelWidth := max(float32(120), width-148)
		rows = append(rows, woxwidget.Container{Width: width - 28, Height: 46, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 12, Top: 5, Right: 8}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: labelWidth, Height: 36, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
					woxwidget.Text{Value: name, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
					woxwidget.Text{Value: plugin.ID, Style: woxui.TextStyle{Size: 8}, Color: snapshot.palette.resultSubtitle},
				}}},
				a.buildFormTableButton(fmt.Sprintf("cloud-plugin-%d", index), buttonLabel, 96, snapshot.cloudBusy == "", isExcluded, func() { a.toggleCloudPluginExclusion(plugin.ID) }, snapshot.palette),
			},
		}})
	}
	var body woxwidget.Widget
	if len(rows) == 0 {
		body = woxwidget.Container{Width: width - 28, Height: bodyHeight, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 18}, Child: woxwidget.Text{
			Value: a.translate("i18n:ui_cloud_sync_plugin_exclusions_empty"), Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}}
	} else {
		body = woxwidget.Gesture{ID: "cloud-plugin-scroll", OnScroll: func(delta woxui.Point) { a.scrollCloudPlugins(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: width - 28, Height: bodyHeight, ContentHeight: max(bodyHeight, float32(len(rows))*46), Offset: snapshot.cloudPluginScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Container{Width: width, Height: 26, Child: woxwidget.Text{Value: a.translate("i18n:ui_cloud_sync_plugin_exclusions_tips"), Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle}},
		body,
	}}}
}

// buildCloudConfigNotesCard documents the platform-aware portions of the otherwise shared sync model.
func (a *App) buildCloudConfigNotesCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	notes := [][2]string{
		{"ui_cloud_sync_config_note_clipboard", "ui_cloud_sync_config_note_clipboard_tips"},
		{"ui_cloud_sync_config_note_query_hotkeys", "ui_cloud_sync_config_note_query_hotkeys_tips"},
		{"ui_cloud_sync_config_note_autostart", "ui_cloud_sync_config_note_autostart_tips"},
		{"ui_cloud_sync_config_note_runtime_paths", "ui_cloud_sync_config_note_runtime_paths_tips"},
	}
	rows := make([]woxwidget.Widget, 0, len(notes))
	for _, note := range notes {
		rows = append(rows, woxwidget.Text{Value: "• " + a.translate("i18n:"+note[0]) + " · " + a.translate("i18n:"+note[1]), Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 2, Top: 10, Right: 2, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows}}
}

func (a *App) formatCloudTime(timestamp int64) string {
	if timestamp <= 0 {
		return a.translate("i18n:ui_cloud_sync_never")
	}
	return time.UnixMilli(timestamp).Local().Format("2006-01-02 15:04")
}

// buildCloudFormOverlay mounts credential and recovery forms over the portable settings surface.
func (a *App) buildCloudFormOverlay(snapshot *cloudFormSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	panelWidth := min(float32(560), max(float32(360), width-48))
	rows := make([]woxwidget.Widget, 0, len(snapshot.definitions))
	callbacks := formFieldCallbacks{idPrefix: "cloud-form", focus: a.focusCloudFormField, change: a.changeCloudFormField, setCaret: a.setCloudFormCaret}
	for index, definition := range snapshot.definitions {
		rows = append(rows, a.buildFormField(snapshot.formFieldsSnapshot, callbacks, palette, index, definition, panelWidth-36, formDefinitionHeight(definition)))
	}
	linkHeight := float32(0)
	var links woxwidget.Widget = woxwidget.Painter{Width: panelWidth - 36, Height: 0}
	if snapshot.kind == "register" {
		linkHeight = 38
		links = woxwidget.Container{Width: panelWidth - 36, Height: linkHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "Read:", Style: woxui.TextStyle{Size: 11}, Color: palette.actionHeader},
			a.buildFormTableButton("cloud-terms", "Terms ↗", 90, !snapshot.saving, false, func() { a.openCloudLegalPage("/terms") }, palette),
			a.buildFormTableButton("cloud-privacy", "Privacy ↗", 90, !snapshot.saving, false, func() { a.openCloudLegalPage("/privacy") }, palette),
		}}}
	} else if snapshot.kind == "login" {
		linkHeight = 38
		links = woxwidget.Container{Width: panelWidth - 36, Height: linkHeight, Child: a.buildFormTableButton("cloud-forgot-password", "Forgot password", 132, !snapshot.saving, false, func() {
			a.openCloudAccountForm("reset-request")
		}, palette)}
	} else if snapshot.kind == "verify" {
		linkHeight = 38
		links = woxwidget.Container{Width: panelWidth - 36, Height: linkHeight, Child: a.buildFormTableButton("cloud-resend-code", "Resend code", 112, !snapshot.saving, false, a.resendCloudVerification, palette)}
	}
	description := cloudFormDescription(snapshot)
	formHeight := formDefinitionsContentHeight(snapshot.definitions)
	panelHeight := 36 + 44 + formHeight + linkHeight + 42 + 48
	panelHeight = min(panelHeight, height-36)
	feedback := snapshot.notice
	errorColor := palette.actionHeader
	if snapshot.error != "" {
		feedback = snapshot.error
		errorColor = woxui.Color{R: 232, G: 95, B: 95, A: 255}
	}
	submitLabel := "Continue"
	if snapshot.saving {
		submitLabel = "Working…"
	}
	panel := woxwidget.Container{Width: panelWidth, Height: panelHeight, Radius: 12, Color: palette.actionBackground, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 18, Bottom: 14}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 8, Children: []woxwidget.Widget{
			woxwidget.Text{Value: snapshot.title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: palette.actionText},
			woxwidget.TextBlock{Value: description, Width: panelWidth - 36, Height: 44, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: palette.actionHeader},
			woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
			links,
			woxwidget.TextBlock{Value: feedback, Width: panelWidth - 36, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: errorColor},
			woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Painter{Width: max(float32(0), panelWidth-36-96-112-8), Height: 38},
				a.buildFormTableButton("cloud-form-cancel", "Cancel", 96, !snapshot.saving, false, a.closeCloudForm, palette),
				a.buildFormTableButton("cloud-form-submit", submitLabel, 112, !snapshot.saving, true, a.submitCloudForm, palette),
			}},
		},
	}}
	left := max(float32(0), (width-panelWidth)*0.5)
	top := max(float32(0), (height-panelHeight)*0.5)
	return woxwidget.Stack{Width: width, Height: height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "cloud-form-backdrop", OnTap: func() {}, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: width, Height: height, Color: woxui.Color{R: 0, G: 0, B: 0, A: 112}}}},
		{Left: left, Top: top, Child: panel},
	}}
}

func cloudFormDescription(snapshot *cloudFormSnapshot) string {
	switch snapshot.kind {
	case "register":
		return "Create an account with a 12-character password, then verify the code sent to your email."
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
