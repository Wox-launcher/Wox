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
	contentWidth := max(float32(0), width-72)
	children := []woxwidget.Widget{
		woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), contentWidth-126), Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Cloud Sync", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: "Sync settings and plugin configuration with end-to-end encryption", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
			}}},
			a.buildFormTableButton("cloud-refresh", cloudRefreshLabel(snapshot), 118, !snapshot.cloudLoading && snapshot.cloudBusy == "", false, a.reloadCloudSync, snapshot.palette),
		}}},
	}
	accountHeight := float32(112)
	if !snapshot.cloudAccount.LoggedIn {
		accountHeight = 150
	}
	children = append(children, a.buildCloudAccountCard(snapshot, contentWidth, accountHeight))
	if snapshot.cloudAccount.LoggedIn {
		children = append(children, a.buildCloudSyncCard(snapshot, contentWidth))
		deviceHeight := float32(72 + len(snapshot.cloudDevices.Devices)*62)
		if len(snapshot.cloudDevices.Devices) == 0 {
			deviceHeight = 128
		}
		children = append(children, a.buildCloudDeviceCard(snapshot, contentWidth, deviceHeight))
		children = append(children, a.buildCloudPluginExclusionsCard(snapshot, contentWidth, 282))
		children = append(children, a.buildCloudConfigNotesCard(snapshot, contentWidth, 156))
	}
	message := snapshot.cloudError
	messageColor := woxui.Color{R: 232, G: 95, B: 95, A: 255}
	if message == "" {
		message = snapshot.note
		if message == "" {
			message = "Passwords and encryption keys stay in Wox core; the Go UI only owns transient editor state."
		}
		messageColor = snapshot.palette.resultSubtitle
	}
	children = append(children, woxwidget.Container{Width: contentWidth, Height: 34, Padding: woxwidget.Insets{Top: 9}, Child: woxwidget.TextBlock{
		Value: message, Width: contentWidth, Height: 22, MaxLines: 1, Style: woxui.TextStyle{Size: 10}, Color: messageColor,
	}})

	contentHeight := float32(62+accountHeight+34) + float32(len(children)-1)*12
	if snapshot.cloudAccount.LoggedIn {
		contentHeight += 136
		if len(snapshot.cloudDevices.Devices) == 0 {
			contentHeight += 128
		} else {
			contentHeight += float32(72 + len(snapshot.cloudDevices.Devices)*62)
		}
		contentHeight += 282 + 156
	}
	viewportHeight := max(float32(1), height-52)
	a.setCloudPageGeometry(viewportHeight, contentHeight)
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22}, Child: woxwidget.Gesture{
		ID: "cloud-page-scroll", OnScroll: func(delta woxui.Point) { a.scrollCloudPage(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: contentWidth, Height: viewportHeight, ContentHeight: max(viewportHeight, contentHeight), Offset: snapshot.cloudPageScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
		},
	}}
}

func cloudRefreshLabel(snapshot settingsSnapshot) string {
	if snapshot.cloudLoading {
		return "Refreshing…"
	}
	return "Refresh status"
}

func (a *App) buildCloudAccountCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	if !snapshot.cloudAccount.LoggedIn {
		return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 18, Bottom: 14}, Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Wox account", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.TextBlock{Value: "Sign in to keep Wox configuration available across devices. Synced values are encrypted before upload.", Width: width - 36, Height: 42, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: snapshot.palette.resultSubtitle},
				woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
					a.buildFormTableButton("cloud-login", "Log in", 96, snapshot.cloudBusy == "", true, func() { a.openCloudAccountForm("login") }, snapshot.palette),
					a.buildFormTableButton("cloud-register", "Create account", 124, snapshot.cloudBusy == "", false, func() { a.openCloudAccountForm("register") }, snapshot.palette),
				}},
			},
		}}
	}
	plan := strings.ToUpper(snapshot.cloudAccount.Plan)
	if plan == "" {
		plan = "FREE"
	}
	status := fmt.Sprintf("%s plan · %d devices", plan, snapshot.cloudAccount.DeviceCount)
	if snapshot.cloudAccount.SessionExpired {
		status = "Session expired · log in again"
	}
	labelWidth := max(float32(220), width-402)
	billingLabel := "Upgrade"
	if strings.EqualFold(snapshot.cloudAccount.Plan, "pro") {
		billingLabel = "Manage plan"
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 14, Bottom: 12}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 76, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: snapshot.cloudAccount.Email, Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: status, Style: woxui.TextStyle{Size: 12}, Color: snapshot.palette.resultSubtitle},
				woxwidget.Text{Value: "Account and subscription", Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Container{Width: max(float32(0), width-labelWidth-42), Height: 48, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				a.buildFormTableButton("cloud-billing", billingLabel, 112, snapshot.cloudBusy == "" && !snapshot.cloudAccount.SessionExpired, false, a.openCloudBilling, snapshot.palette),
				a.buildFormTableButton("cloud-password", "Password", 96, snapshot.cloudBusy == "" && !snapshot.cloudAccount.SessionExpired, false, func() { a.openCloudAccountForm("change-password") }, snapshot.palette),
				a.buildFormTableButton("cloud-logout", cloudBusyLabel(snapshot, "logout", "Log out"), 96, snapshot.cloudBusy == "", false, func() { a.runCloudAction("logout", "/account/logout", map[string]any{}) }, snapshot.palette),
			}}},
		},
	}}
}

func (a *App) buildCloudSyncCard(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	label, detail, color := cloudSyncPresentation(snapshot)
	currentRevoked := cloudCurrentDeviceRevoked(snapshot.cloudDevices)
	ready := cloudSyncReady(snapshot)
	buttons := []woxwidget.Widget{}
	enabled := snapshot.cloudBusy == "" && !snapshot.cloudLoading && !snapshot.cloudAccount.SessionExpired
	if currentRevoked {
		buttons = append(buttons, a.buildFormTableButton("cloud-join", cloudBusyLabel(snapshot, "join", "Join device"), 106, enabled, true, func() { a.runCloudAction("join", "/sync/devices/join", map[string]any{}) }, snapshot.palette))
	} else if !snapshot.cloudAccount.SyncEligible {
		buttons = append(buttons, a.buildFormTableButton("cloud-subscribe", "Upgrade plan", 112, enabled, true, a.openCloudBilling, snapshot.palette))
	} else if !ready {
		buttons = append(buttons, a.buildFormTableButton("cloud-bootstrap", cloudBusyLabel(snapshot, "bootstrap", "Set up sync"), 112, enabled, true, a.beginCloudBootstrap, snapshot.palette))
	} else if !snapshot.cloudAccount.SyncEnabled || !snapshot.cloudSync.Enabled {
		buttons = append(buttons, a.buildFormTableButton("cloud-enable", cloudBusyLabel(snapshot, "enable", "Enable sync"), 108, enabled, true, func() { a.runCloudAction("enable", "/sync/enable", map[string]any{}) }, snapshot.palette))
	} else {
		buttons = append(buttons,
			a.buildFormTableButton("cloud-sync", cloudBusyLabel(snapshot, "sync", "Sync now"), 102, enabled, true, func() { a.runCloudAction("sync", "/sync/push", map[string]any{}) }, snapshot.palette),
			a.buildFormTableButton("cloud-disable", cloudBusyLabel(snapshot, "disable", "Disable"), 92, enabled, false, func() { a.runCloudAction("disable", "/sync/disable", map[string]any{}) }, snapshot.palette),
		)
	}
	labelWidth := max(float32(220), width-260)
	return woxwidget.Container{Width: width, Height: 136, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 14, Bottom: 12}, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: labelWidth, Height: 106, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Encrypted sync", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: color},
				woxwidget.TextBlock{Value: detail, Width: labelWidth, Height: 48, MaxLines: 2, Style: woxui.TextStyle{Size: 11}, LineHeight: 17, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Container{Width: max(float32(0), width-labelWidth-42), Height: 52, Padding: woxwidget.Insets{Top: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: buttons}},
		},
	}}
}

func cloudSyncPresentation(snapshot settingsSnapshot) (string, string, woxui.Color) {
	muted := snapshot.palette.resultSubtitle
	errorColor := woxui.Color{R: 232, G: 95, B: 95, A: 255}
	if snapshot.cloudLoading {
		return "Loading…", "Reading account and encrypted sync state from Wox core.", muted
	}
	if snapshot.cloudAccount.SessionExpired {
		return "Session expired", "Log out and sign in again before syncing.", errorColor
	}
	if snapshot.cloudError != "" {
		return "Sync error", snapshot.cloudError, errorColor
	}
	if progress := snapshot.cloudSync.Progress; progress != nil && progress.Active {
		detail := strings.Title(progress.Operation)
		if progress.Total > 0 {
			detail = fmt.Sprintf("%s · %d / %d", detail, progress.Current, progress.Total)
		}
		return "Syncing…", detail, muted
	}
	if state := snapshot.cloudSync.State; state != nil && state.LastError != "" {
		return "Sync error", state.LastError, errorColor
	}
	if !snapshot.cloudAccount.SyncEligible {
		return "Plan upgrade required", "Cloud sync is not available for the current account plan.", muted
	}
	if !cloudSyncReady(snapshot) {
		return "Not set up", "Create or restore the encryption key to start syncing this device.", muted
	}
	if !snapshot.cloudAccount.SyncEnabled || !snapshot.cloudSync.Enabled {
		return "Disabled", "The encryption key is available, but scheduled sync is disabled.", muted
	}
	lastSync := max(cloudStateTimestamp(snapshot.cloudSync.State, true), cloudStateTimestamp(snapshot.cloudSync.State, false))
	return "Synced", "Last sync: " + formatCloudTime(lastSync), woxui.Color{R: 72, G: 190, B: 112, A: 255}
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
			name += " · Current"
		}
		if device.RevokedAt > 0 {
			name += " · Revoked"
		}
		detail := strings.Title(strings.ToLower(device.Platform)) + " · Last seen " + formatCloudTime(device.LastSeenAt)
		labelWidth := max(float32(160), width-150)
		var action woxwidget.Widget = woxwidget.Painter{Width: 104, Height: 38}
		if !device.Current && device.RevokedAt == 0 {
			action = a.buildFormTableButton(fmt.Sprintf("cloud-revoke-%d", index), "Revoke", 96, snapshot.cloudBusy == "", false, func() {
				a.runCloudAction("revoke", "/sync/devices/revoke", map[string]string{"target_device_id": device.DeviceID})
			}, snapshot.palette)
		}
		rows = append(rows, woxwidget.Container{Width: width - 28, Height: 62, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 9, Right: 8}, Child: woxwidget.Flex{
			Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
				woxwidget.Container{Width: labelWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
					woxwidget.Text{Value: name, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
					woxwidget.Text{Value: detail, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle},
				}}},
				action,
			},
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, woxwidget.Container{Width: width - 28, Height: 56, Radius: 8, Color: snapshot.palette.toolbarBackground, Padding: woxwidget.Insets{Left: 14, Top: 18}, Child: woxwidget.Text{
			Value: "No registered devices reported.", Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}})
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 6, Children: append([]woxwidget.Widget{
			woxwidget.Container{Width: width - 28, Height: 38, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.Text{Value: "Devices", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle}},
		}, rows...),
	}}
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
		buttonLabel := "Sync"
		if isExcluded {
			buttonLabel = "Excluded"
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
			Value: "No installed plugins.", Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
		}}
	} else {
		body = woxwidget.Gesture{ID: "cloud-plugin-scroll", OnScroll: func(delta woxui.Point) { a.scrollCloudPlugins(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: width - 28, Height: bodyHeight, ContentHeight: max(bodyHeight, float32(len(rows))*46), Offset: snapshot.cloudPluginScroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
			woxwidget.Container{Width: width - 28, Height: 34, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 3, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "Plugin sync exclusions", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
				woxwidget.Text{Value: "Excluded plugins keep their data and settings on this device.", Style: woxui.TextStyle{Size: 9}, Color: snapshot.palette.resultSubtitle},
			}}},
			body,
		},
	}}
}

// buildCloudConfigNotesCard documents the platform-aware portions of the otherwise shared sync model.
func (a *App) buildCloudConfigNotesCard(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	notes := []string{
		"Clipboard · only favorite items are synced",
		"Hotkeys and ignored apps · synced per platform",
		"Autostart and proxy · synced per platform",
		"Local paths · synced only when another device can reproduce them",
	}
	rows := make([]woxwidget.Widget, 0, len(notes))
	for _, note := range notes {
		rows = append(rows, woxwidget.Text{Value: "• " + note, Style: woxui.TextStyle{Size: 10}, Color: snapshot.palette.resultSubtitle})
	}
	return woxwidget.Container{Width: width, Height: height, Radius: 10, Color: snapshot.palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 14, Right: 16, Bottom: 12}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 8, Children: append([]woxwidget.Widget{
			woxwidget.Text{Value: "Configuration sync notes", Style: woxui.TextStyle{Size: 14, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultTitle},
		}, rows...),
	}}
}

func formatCloudTime(timestamp int64) string {
	if timestamp <= 0 {
		return "Never"
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
