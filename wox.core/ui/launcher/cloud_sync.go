package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"wox/account"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type cloudAccountStatus struct {
	LoggedIn       bool            `json:"logged_in"`
	Email          string          `json:"email"`
	SyncEligible   bool            `json:"sync_eligible"`
	Plan           string          `json:"plan"`
	SyncLimits     cloudSyncLimits `json:"sync_limits"`
	DeviceCount    int             `json:"device_count"`
	SyncEnabled    bool            `json:"sync_enabled"`
	SessionExpired bool            `json:"session_expired"`
}

type cloudSyncLimits struct {
	DeviceLimit *int `json:"device_limit"`
}

type cloudBillingPlan struct {
	Free cloudBillingPlanTier `json:"free"`
	Pro  cloudBillingPlanTier `json:"pro"`
}

type cloudBillingPlanTier struct {
	Price cloudBillingPlanPrice  `json:"price"`
	Trial *cloudBillingPlanTrial `json:"trial,omitempty"`
}

type cloudBillingPlanPrice struct {
	Currency   string `json:"currency"`
	UnitAmount *int   `json:"unit_amount"`
	Interval   string `json:"interval"`
	Formatted  string `json:"formatted"`
}

type cloudBillingPlanTrial struct {
	Days          int    `json:"days"`
	Interval      string `json:"interval"`
	IntervalCount int    `json:"interval_count"`
	Formatted     string `json:"formatted"`
}

type cloudSyncStatus struct {
	Enabled   bool               `json:"enabled"`
	DeviceID  string             `json:"device_id"`
	KeyStatus cloudSyncKeyStatus `json:"key_status"`
	State     *cloudSyncState    `json:"state"`
	Progress  *cloudSyncProgress `json:"progress"`
}

type cloudSyncKeyStatus struct {
	Available bool `json:"available"`
	Version   int  `json:"version"`
}

type cloudSyncState struct {
	Cursor       string `json:"cursor"`
	LastPullTS   int64  `json:"last_pull_ts"`
	LastPushTS   int64  `json:"last_push_ts"`
	BackoffUntil int64  `json:"backoff_until"`
	RetryCount   int    `json:"retry_count"`
	LastError    string `json:"last_error"`
	Bootstrapped bool   `json:"bootstrapped"`
}

type cloudSyncProgress struct {
	Active     bool   `json:"active"`
	Operation  string `json:"operation"`
	EntityType string `json:"entity_type"`
	PluginID   string `json:"plugin_id"`
	Key        string `json:"key"`
	Current    int    `json:"current"`
	Total      int    `json:"total"`
}

type cloudDevice struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	Platform   string `json:"platform"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	RevokedAt  int64  `json:"revoked_at"`
	Current    bool   `json:"current"`
}

type cloudDeviceList struct {
	Devices         []cloudDevice `json:"devices"`
	CurrentDeviceID string        `json:"current_device_id"`
	DeviceLimit     *int          `json:"device_limit"`
	DeviceCount     int           `json:"device_count"`
}

type cloudFormState struct {
	formFieldsState
	kind          string
	title         string
	error         string
	notice        string
	saving        bool
	email         string
	hasRemoteData bool
	controllers   []*woxwidget.TextEditingController
	focusNodes    []*woxwidget.FocusNode
}

type cloudFormSnapshot struct {
	formFieldsSnapshot
	kind          string
	title         string
	error         string
	notice        string
	saving        bool
	email         string
	hasRemoteData bool
	controllers   []*woxwidget.TextEditingController
	focusNodes    []*woxwidget.FocusNode
}

func cloneCloudDeviceList(source cloudDeviceList) cloudDeviceList {
	cloned := source
	cloned.Devices = append([]cloudDevice(nil), source.Devices...)
	return cloned
}

func snapshotCloudFormLocked(state *cloudFormState) *cloudFormSnapshot {
	if state == nil {
		return nil
	}
	fields := snapshotFormFieldsLocked(&state.formFieldsState)
	fields.active = !state.saving
	return &cloudFormSnapshot{
		formFieldsSnapshot: fields,
		kind:               state.kind, title: state.title, error: state.error, notice: state.notice, saving: state.saving, email: state.email, hasRemoteData: state.hasRemoteData,
		controllers: append([]*woxwidget.TextEditingController(nil), state.controllers...),
		focusNodes:  append([]*woxwidget.FocusNode(nil), state.focusNodes...),
	}
}

// newCloudFormState gives every credential field stable editor and focus identities across settings rebuilds.
func newCloudFormState(fields formFieldsState, kind, title string) *cloudFormState {
	fields.editor = nil
	controllers := make([]*woxwidget.TextEditingController, len(fields.definitions))
	focusNodes := make([]*woxwidget.FocusNode, len(fields.definitions))
	for index, definition := range fields.definitions {
		focusNodes[index] = woxwidget.NewFocusNode()
		if formDefinitionTextEditable(definition) {
			controllers[index] = woxwidget.NewTextEditingController(fields.values[definition.Value.Key])
		}
	}
	return &cloudFormState{formFieldsState: fields, kind: kind, title: title, controllers: controllers, focusNodes: focusNodes}
}

// reloadCloudSync refreshes account, sync, and device state as one revisioned settings snapshot.
// Billing prices are fetched independently so a local sync-status refresh cannot leave the
// plan comparison stuck on the loading placeholder.
func (c *cloudSettingsController) reloadCloudSync() {
	c.ReloadCloudSync(context.Background(), c.services, c.sessionID, c.ensureCloudBillingPlan, true)
}

// reloadCloudSyncSilently refreshes final sync state without replacing operation progress with a loading placeholder.
func (c *cloudSettingsController) reloadCloudSyncSilently() {
	c.ReloadCloudSync(context.Background(), c.services, c.sessionID, c.ensureCloudBillingPlan, false)
}

// ensureCloudBillingPlan starts the independent price fetch when display pricing is still missing.
func (c *cloudSettingsController) ensureCloudBillingPlan() {
	if c == nil || c.services == nil || c.BillingLoaded() {
		return
	}
	util.Go(c.lifecycleCtx, "reload cloud billing plan", c.reloadCloudBillingPlan)
}

// reloadCloudBillingPlan fetches display pricing independently so it cannot delay local sync status.
func (c *cloudSettingsController) reloadCloudBillingPlan() {
	if err := c.ReloadBillingPlan(context.Background(), c.services, c.sessionID); err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("failed to load cloud billing plan: %v", err))
	}
}

// runCloudAction serializes account and sync mutations before reloading their shared status.
func (c *cloudSettingsController) runCloudAction(name string, action func(context.Context) error) {
	if c.Busy() != "" {
		return
	}
	c.SetBusy(name)
	c.SetError("")
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "run cloud action", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := action(ctx)
		cancel()
		if err != nil {
			c.SetError(fmt.Sprintf("%s failed: %v", cloudActionLabel(name), err))
		}
		c.SetBusy("")
		if name == "sync" {
			c.reloadCloudSyncSilently()
		} else {
			c.reloadCloudSync()
		}
		if err != nil {
			c.SetError(fmt.Sprintf("%s failed: %v", cloudActionLabel(name), err))
			c.settings.invalidateSettingsWindow()
		}
	})
}

func cloudActionLabel(name string) string {
	switch name {
	case "sync":
		return "Sync"
	case "logout":
		return "Logout"
	case "revoke":
		return "Revoke device"
	case "join":
		return "Join device"
	default:
		return strings.Title(name)
	}
}

// openCloudBilling starts checkout or subscription management in the platform browser.
func (c *cloudSettingsController) openCloudBilling() {
	isPro := strings.EqualFold(c.Account().Plan, "pro")
	kind := contract.BillingSessionCheckout
	if isPro {
		kind = contract.BillingSessionPortal
	}
	if c.Busy() != "" {
		return
	}
	c.SetBusy("billing")
	c.SetError("")
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "open cloud billing", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		session, err := c.services.BillingSession(ctx, c.sessionID, kind)
		cancel()
		if err == nil && session.URL != "" {
			err = c.settings.settingsNativeWindow().OpenExternalURL(session.URL)
		}
		if err != nil {
			c.SetError("Could not open billing: " + err.Error())
		}
		c.SetBusy("")
		c.settings.invalidateSettingsWindow()
	})
}

// openCloudSupportEmail opens the localized billing-support draft in the default mail application.
func (c *cloudSettingsController) openCloudSupportEmail() {
	subject := url.QueryEscape(c.language.translate("i18n:ui_cloud_sync_billing_help_email_subject"))
	if err := c.settings.settingsNativeWindow().OpenExternalURL("mailto:billing@woxlauncher.com?subject=" + subject); err != nil {
		util.GetLogger().Error(context.Background(), "failed to open cloud support email: "+err.Error())
	}
}

// toggleCloudActionMenu opens the compact account or subscription menu used by the flat settings rows.
func (c *cloudSettingsController) toggleCloudActionMenu(menu string) {
	current := c.ActionMenu()
	if current == menu {
		c.SetActionMenu("")
	} else {
		c.SetActionMenu(menu)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *cloudSettingsController) closeCloudActionMenu() {
	c.SetActionMenu("")
	c.settings.invalidateSettingsWindow()
}

// openCloudPluginExclusionDialog opens Flutter's single-row editor with the first available plugin selected.
func (c *cloudSettingsController) openCloudPluginExclusionDialog() {
	excluded := make(map[string]bool, len(c.generalSettings().Data().CloudSyncDisabledPlugins))
	for _, pluginID := range c.generalSettings().Data().CloudSyncDisabledPlugins {
		excluded[strings.TrimSpace(pluginID)] = true
	}
	selected := ""
	for _, plugin := range c.Plugins() {
		pluginID := strings.TrimSpace(plugin.ID)
		if pluginID != "" && !excluded[pluginID] {
			selected = pluginID
			break
		}
	}
	if selected == "" {
		return
	}
	c.SetActionMenu("")
	c.SetPluginDialog(&cloudPluginExclusionDialogState{selected: selected})
	c.settings.invalidateSettingsWindow()
}

// closeCloudPluginExclusionDialog dismisses the add-exclusion row editor and its choice menu.
func (c *cloudSettingsController) closeCloudPluginExclusionDialog() {
	c.SetPluginDialog(nil)
	c.settings.invalidateSettingsWindow()
}

// openCloudPluginExclusionChoice opens the field-anchored plugin selector inside the row editor.
func (c *cloudSettingsController) openCloudPluginExclusionChoice(anchor woxui.Rect) {
	state := c.PluginDialog()
	if state == nil {
		return
	}
	state.choiceOpen = true
	state.choiceAnchor = anchor
	c.SetPluginDialog(state)
	c.settings.invalidateSettingsWindow()
}

// chooseCloudPluginExclusion updates the uncommitted plugin selection in the row editor.
func (c *cloudSettingsController) chooseCloudPluginExclusion(index int) {
	state := c.PluginDialog()
	if state == nil {
		return
	}
	choices := c.cloudPluginExclusionPlugins()
	if index < 0 || index >= len(choices) {
		return
	}
	state.selected = choices[index].ID
	state.choiceOpen = false
	c.SetPluginDialog(state)
	c.settings.invalidateSettingsWindow()
}

// saveCloudPluginExclusion commits the selected plugin using the existing cloud setting update path.
func (c *cloudSettingsController) saveCloudPluginExclusion() {
	state := c.PluginDialog()
	if state == nil || strings.TrimSpace(state.selected) == "" {
		return
	}
	pluginID := state.selected
	c.closeCloudPluginExclusionDialog()
	c.toggleCloudPluginExclusion(pluginID)
}

// cloudPluginExclusionPlugins returns installed plugins that are not already excluded.
func (c *cloudSettingsController) cloudPluginExclusionPlugins() []pluginSettingsPlugin {
	excluded := make(map[string]bool, len(c.generalSettings().Data().CloudSyncDisabledPlugins))
	for _, pluginID := range c.generalSettings().Data().CloudSyncDisabledPlugins {
		excluded[strings.TrimSpace(pluginID)] = true
	}
	plugins := make([]pluginSettingsPlugin, 0, len(c.Plugins()))
	for _, plugin := range c.Plugins() {
		if pluginID := strings.TrimSpace(plugin.ID); pluginID != "" && !excluded[pluginID] {
			plugins = append(plugins, plugin)
		}
	}
	return plugins
}

// runCloudMenuAction preserves Flutter's account and subscription actions behind compact row menus.
func (c *cloudSettingsController) runCloudMenuAction(action string) {
	c.SetActionMenu("")
	switch action {
	case "change-password":
		c.openCloudAccountForm("change-password")
	case "logout":
		c.runCloudAction("logout", func(ctx context.Context) error {
			return c.services.LogoutAccount(ctx, c.sessionID)
		})
	case "refresh":
		util.Go(c.lifecycleCtx, "reload cloud sync", c.reloadCloudSync)
	case "billing":
		c.openCloudBilling()
	}
}

// beginCloudBootstrap reuses a local key when possible or opens the recovery-password form required by core.
func (c *cloudSettingsController) beginCloudBootstrap() {
	keyAvailable := c.Sync().KeyStatus.Available
	if c.Busy() != "" {
		return
	}
	if keyAvailable {
		c.runCloudBootstrap("")
		return
	}
	c.SetBusy("bootstrap-status")
	c.SetError("")
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "prepare cloud bootstrap", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		status, err := c.services.CloudBootstrapStatus(ctx, c.sessionID)
		cancel()
		if err != nil {
			c.SetError("Could not prepare cloud sync: " + err.Error())
			c.SetBusy("")
			c.settings.invalidateSettingsWindow()
			return
		}
		c.SetBusy("")
		c.SetForm(newCloudBootstrapForm(status))
		c.updateSettingsText(true)
		c.settings.invalidateSettingsWindow()
	})
}

// cloudSyncNeedsBootstrap mirrors the post-authentication setup gate used by the Flutter settings UI.
func cloudSyncNeedsBootstrap(snapshot cloudSettingsSnapshot) bool {
	account := snapshot.Account
	if !account.LoggedIn || account.SessionExpired || !account.SyncEligible {
		return false
	}
	state := snapshot.Sync.State
	return !account.SyncEnabled || !snapshot.Sync.KeyStatus.Available || state == nil || !state.Bootstrapped
}

func newCloudBootstrapForm(status contract.CloudBootstrapStatus) *cloudFormState {
	definitions := []formDefinition{{Type: "password", Value: formDefinitionValue{Key: "RecoveryCode", Label: "i18n:ui_cloud_sync_recovery_code", MaxLines: 1}}}
	if !status.HasRemoteData {
		definitions = append(definitions, formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmRecoveryCode", Label: "i18n:ui_cloud_sync_recovery_code_confirm", MaxLines: 1}})
	}
	title := "i18n:ui_cloud_sync_bootstrap_start_title"
	if status.HasRemoteData {
		title = "i18n:ui_cloud_sync_bootstrap_restore_title"
	}
	fields := newFormFieldsState(definitions, nil, true)
	state := newCloudFormState(fields, "bootstrap", title)
	state.hasRemoteData = status.HasRemoteData
	return state
}

func (c *cloudSettingsController) runCloudBootstrap(recoveryCode string) {
	if c.Busy() != "" {
		return
	}
	c.SetBusy("bootstrap")
	c.SetError("")
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "start cloud bootstrap", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := c.services.StartCloudBootstrap(ctx, c.sessionID, recoveryCode)
		cancel()
		if err != nil {
			c.SetError("Could not start cloud sync: " + err.Error())
		}
		c.SetBusy("")
		c.reloadCloudSync()
		if err == nil {
			time.AfterFunc(2*time.Second, func() {
				util.Go(c.lifecycleCtx, "reload cloud sync after bootstrap", c.reloadCloudSync)
			})
		}
	})
}

// openCloudAccountForm creates account lifecycle forms from the shared text-editing engine.
func (c *cloudSettingsController) openCloudAccountForm(kind string) {
	definitions := []formDefinition{}
	title := c.language.translate("i18n:ui_cloud_sync_account_login")
	switch kind {
	case "login", "register":
		definitions = append(definitions,
			formDefinition{Type: "textbox", Value: formDefinitionValue{Key: "Email", Label: "i18n:ui_cloud_sync_account_email", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "Password", Label: "i18n:ui_cloud_sync_account_password", MaxLines: 1}},
		)
	case "reset-request":
		title = "Reset password"
		definitions = append(definitions, formDefinition{Type: "textbox", Value: formDefinitionValue{Key: "Email", Label: "Email", MaxLines: 1}})
	case "reset-confirm":
		title = "Set new password"
		definitions = append(definitions,
			formDefinition{Type: "textbox", Value: formDefinitionValue{Key: "Token", Label: "Reset code", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "Password", Label: "New password", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmPassword", Label: "Confirm password", MaxLines: 1}},
		)
	case "change-password":
		title = "Change password"
		definitions = append(definitions,
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "CurrentPassword", Label: "Current password", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "Password", Label: "New password", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmPassword", Label: "Confirm password", MaxLines: 1}},
		)
	}
	if kind == "register" {
		title = c.language.translate("i18n:ui_cloud_sync_account_register")
		definitions = append(definitions,
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmPassword", Label: "i18n:ui_cloud_sync_account_confirm_password", MaxLines: 1}},
			formDefinition{Type: "checkbox", Value: formDefinitionValue{Key: "AcceptedLegal", Label: "i18n:ui_cloud_sync_account_accept_prefix"}},
		)
	}
	values := map[string]string{"Email": c.Account().Email}
	fields := newFormFieldsState(definitions, values, true)
	c.SetForm(newCloudFormState(fields, kind, title))
	c.updateSettingsText(true)
	c.settings.invalidateSettingsWindow()
}

func (c *cloudSettingsController) openCloudVerificationForm(email string) {
	definitions := []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "Code", Label: "Verification code", MaxLines: 1}}}
	fields := newFormFieldsState(definitions, nil, true)
	form := newCloudFormState(fields, "verify", "Verify email")
	form.email = email
	c.SetForm(form)
	c.updateSettingsText(true)
	c.settings.invalidateSettingsWindow()
}

// resendCloudVerification keeps the current verification editor open while requesting a fresh code.
func (c *cloudSettingsController) resendCloudVerification() {
	form := c.Form()
	if form == nil || form.kind != "verify" || form.saving {
		return
	}
	email := form.email
	lang := c.generalSettings().Data().LangCode
	form.saving = true
	form.error = ""
	form.notice = ""
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "resend cloud verification", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := c.services.ResendAccountVerification(ctx, c.sessionID, email, lang)
		cancel()
		_ = c.runOnUI("apply resent cloud verification", func() {
			form := c.Form()
			if form != nil && form.kind == "verify" {
				form.saving = false
				if err != nil {
					form.error = err.Error()
				} else {
					form.notice = "A new verification code was sent."
				}
			}
			c.updateSettingsText(true)
			c.settings.invalidateSettingsWindow()
		})
	})
}

func (c *cloudSettingsController) closeCloudForm() {
	c.SetForm(nil)
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *cloudSettingsController) focusCloudFormField(index int) {
	var focusNode *woxwidget.FocusNode
	form := c.Form()
	if form != nil && !form.saving {
		setCloudFormFocusLocked(form, index)
		if index >= 0 && index < len(form.focusNodes) {
			focusNode = form.focusNodes[index]
		}
	}
	if focusNode != nil {
		focusNode.RequestFocus()
	}
	c.settings.invalidateSettingsWindow()
}

// setCloudFormText keeps business form values synchronized with retained field controllers.
func (c *cloudSettingsController) setCloudFormText(index int, value string) {
	form := c.Form()
	if form != nil && !form.saving && index >= 0 && index < len(form.definitions) {
		definition := form.definitions[index]
		if formDefinitionTextEditable(definition) {
			form.values[definition.Value.Key] = value
		}
	}
	c.settings.invalidateSettingsWindow()
}

// setCloudFormFieldFocused mirrors Host focus into form navigation and scroll state.
func (c *cloudSettingsController) setCloudFormFieldFocused(index int, focused bool) {
	if !focused {
		return
	}
	form := c.Form()
	if form != nil && !form.saving && index >= 0 && index < len(form.definitions) {
		setCloudFormFocusLocked(form, index)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *cloudSettingsController) changeCloudFormField(index, delta int) {
	form := c.Form()
	if form != nil && !form.saving {
		changeFormFieldsChoiceLocked(&form.formFieldsState, index, delta)
	}
	c.settings.invalidateSettingsWindow()
}

func (c *cloudSettingsController) moveCloudFormFocus(delta int) {
	var focusNode *woxwidget.FocusNode
	form := c.Form()
	if form == nil || len(form.definitions) == 0 || form.saving {
		return
	}
	index := form.focused
	for step := 0; step < len(form.definitions); step++ {
		index = (index + delta + len(form.definitions)) % len(form.definitions)
		if formDefinitionFocusable(form.definitions[index]) {
			setCloudFormFocusLocked(form, index)
			if index < len(form.focusNodes) {
				focusNode = form.focusNodes[index]
			}
			break
		}
	}
	if focusNode != nil {
		focusNode.RequestFocus()
	}
	c.settings.invalidateSettingsWindow()
}

// onCloudSettingsKey gives the active account/bootstrap modal exclusive keyboard ownership.
func (c *cloudSettingsController) onCloudSettingsKey(event woxui.KeyEvent) bool {
	pluginDialog := c.settings.open && c.PluginDialog() != nil
	if pluginDialog {
		if event.Key == woxui.KeyEscape {
			state := c.PluginDialog()
			if state != nil && state.choiceOpen {
				state.choiceOpen = false
				c.SetPluginDialog(state)
				c.settings.invalidateSettingsWindow()
			} else {
				c.closeCloudPluginExclusionDialog()
			}
		}
		return true
	}
	menuActive := c.settings.open && c.ActionMenu() != ""
	form := c.Form()
	active := c.settings.open && form != nil
	saving := active && form.saving
	if menuActive && !active {
		if event.Key == woxui.KeyEscape {
			c.closeCloudActionMenu()
		}
		return true
	}
	if !active {
		return false
	}
	if saving {
		return true
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.closeCloudForm()
	case woxui.KeyTab, woxui.KeyArrowDown, woxui.KeyArrowUp:
		delta := 1
		if event.Key == woxui.KeyArrowUp || event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		c.moveCloudFormFocus(delta)
	case woxui.KeyEnter:
		c.submitCloudForm()
	case woxui.KeySpace:
		focused := -1
		fieldType := ""
		form := c.Form()
		if form != nil {
			focused = form.focused
			if focused >= 0 && focused < len(form.definitions) {
				fieldType = form.definitions[focused].Type
			}
		}
		if fieldType == "checkbox" {
			c.changeCloudFormField(focused, 1)
		} else {
			return false
		}
	case woxui.KeyArrowLeft, woxui.KeyArrowRight:
		return false
	default:
		return false
	}
	return true
}

// onCloudFormTextInput blocks fallback routing while retained text fields own cloud modal input.
func (c *cloudSettingsController) onCloudFormTextInput(_ woxui.TextInputEvent) bool {
	return c.settings.open && c.Form() != nil
}

// submitCloudForm validates local invariants before sending credentials or recovery data to core.
func (c *cloudSettingsController) submitCloudForm() {
	form := c.Form()
	if form == nil || form.saving {
		return
	}
	syncCloudFormControllersLocked(form)
	kind := form.kind
	values := make(map[string]string, len(form.values))
	for key, value := range form.values {
		values[key] = value
	}
	values["Email"] = strings.TrimSpace(values["Email"])
	values["Code"] = strings.TrimSpace(values["Code"])
	values["RecoveryCode"] = strings.TrimSpace(values["RecoveryCode"])
	values["ConfirmRecoveryCode"] = strings.TrimSpace(values["ConfirmRecoveryCode"])
	values["Token"] = strings.TrimSpace(values["Token"])
	email := form.email
	hasRemoteData := form.hasRemoteData
	validationError := validateCloudForm(kind, values, hasRemoteData)
	if validationError != "" {
		form.error = validationError
		c.settings.invalidateSettingsWindow()
		return
	}
	form.saving = true
	form.error = ""
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "submit cloud form", func() {
		c.submitCloudFormRequest(kind, values, email)
	})
}

// syncCloudFormControllersLocked captures authoritative retained text before validation and submission.
func syncCloudFormControllersLocked(state *cloudFormState) {
	if state == nil {
		return
	}
	for index, controller := range state.controllers {
		if controller == nil || index >= len(state.definitions) {
			continue
		}
		definition := state.definitions[index]
		if formDefinitionTextEditable(definition) {
			state.values[definition.Value.Key] = controller.Text()
		}
	}
}

// setCloudFormFocusLocked updates navigation metadata without recreating a second text editor.
func setCloudFormFocusLocked(state *cloudFormState, index int) {
	if state == nil || index < 0 || index >= len(state.definitions) {
		return
	}
	state.focused = index
	state.active = true
	state.editor = nil
}

func validateCloudForm(kind string, values map[string]string, hasRemoteData bool) string {
	switch kind {
	case "login", "register":
		if !strings.Contains(values["Email"], "@") {
			return "Enter a valid email address."
		}
		if values["Password"] == "" {
			return "Password is required."
		}
		if kind == "register" && len([]rune(values["Password"])) < 12 {
			return "Password must contain at least 12 characters."
		}
		if kind == "register" && values["Password"] != values["ConfirmPassword"] {
			return "Passwords do not match."
		}
		if kind == "register" && values["AcceptedLegal"] != "true" {
			return "Accept the Terms of Service and Privacy Policy to continue."
		}
	case "verify":
		if values["Code"] == "" {
			return "Verification code is required."
		}
	case "reset-request":
		if !strings.Contains(values["Email"], "@") {
			return "Enter a valid email address."
		}
	case "reset-confirm":
		if values["Token"] == "" {
			return "Reset code is required."
		}
		if len([]rune(values["Password"])) < 12 {
			return "Password must contain at least 12 characters."
		}
		if values["Password"] != values["ConfirmPassword"] {
			return "Passwords do not match."
		}
	case "change-password":
		if values["CurrentPassword"] == "" {
			return "Current password is required."
		}
		if len([]rune(values["Password"])) < 12 {
			return "New password must contain at least 12 characters."
		}
		if values["Password"] != values["ConfirmPassword"] {
			return "Passwords do not match."
		}
	case "bootstrap":
		if values["RecoveryCode"] == "" {
			return "i18n:ui_cloud_sync_recovery_code_required"
		}
		if !hasRemoteData {
			if values["ConfirmRecoveryCode"] == "" {
				return "i18n:ui_cloud_sync_recovery_code_confirm_required"
			}
			if values["RecoveryCode"] != values["ConfirmRecoveryCode"] {
				return "i18n:ui_cloud_sync_recovery_code_mismatch"
			}
		}
	}
	return ""
}

func (c *cloudSettingsController) submitCloudFormRequest(kind string, values map[string]string, email string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result account.ActionResult
	var err error
	lang := c.generalSettings().Data().LangCode
	switch kind {
	case "login", "register":
		if kind == "register" {
			result, err = c.services.RegisterAccount(ctx, c.sessionID, values["Email"], values["Password"], lang)
		} else {
			result, err = c.services.LoginAccount(ctx, c.sessionID, values["Email"], values["Password"], lang)
		}
	case "verify":
		result, err = c.services.VerifyAccountEmail(ctx, c.sessionID, email, values["Code"], lang)
	case "reset-request":
		err = c.services.RequestAccountPasswordReset(ctx, c.sessionID, values["Email"], lang)
	case "reset-confirm":
		err = c.services.ConfirmAccountPasswordReset(ctx, c.sessionID, values["Token"], values["Password"], lang)
	case "change-password":
		err = c.services.ChangeAccountPassword(ctx, c.sessionID, values["CurrentPassword"], values["Password"], lang)
	case "bootstrap":
		err = c.services.StartCloudBootstrap(ctx, c.sessionID, values["RecoveryCode"])
	}
	if err == nil && (kind == "login" || kind == "register") && result.Code == "need_verify_email" {
		verificationEmail := result.Email
		if verificationEmail == "" {
			verificationEmail = values["Email"]
		}
		_ = c.runOnUI("open cloud verification form", func() {
			c.openCloudVerificationForm(verificationEmail)
		})
		return
	}
	if err == nil && kind != "bootstrap" && result.Code != "" && result.Code != "ok" {
		message := result.Message
		if strings.TrimSpace(message) == "" {
			message = result.Code
		}
		err = fmt.Errorf("%s", message)
	}
	if err != nil {
		_ = c.runOnUI("apply cloud form error", func() {
			form := c.Form()
			if form != nil && form.kind == kind {
				form.saving = false
				form.error = err.Error()
			}
			c.updateSettingsText(form != nil && form.editor != nil)
			c.settings.invalidateSettingsWindow()
		})
		return
	}
	reload := false
	_ = c.runOnUI("apply cloud form success", func() {
		switch kind {
		case "reset-request":
			c.openCloudAccountForm("reset-confirm")
			return
		case "reset-confirm":
			c.openCloudAccountForm("login")
			return
		}
		c.closeCloudForm()
		reload = true
	})
	if !reload {
		return
	}
	c.reloadCloudSync()
	if (kind == "login" || kind == "register" || kind == "verify") && cloudSyncNeedsBootstrap(c.Snapshot()) {
		c.beginCloudBootstrap()
	}
	if kind == "bootstrap" {
		util.Go(c.lifecycleCtx, "reload cloud sync after form bootstrap", func() {
			timer := time.NewTimer(2 * time.Second)
			defer timer.Stop()
			select {
			case <-c.lifecycleCtx.Done():
			case <-timer.C:
				c.reloadCloudSync()
			}
		})
	}
}

// toggleCloudPluginExclusion persists the exact plugin ID list expected by Wox core.
func (c *cloudSettingsController) toggleCloudPluginExclusion(pluginID string) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return
	}
	if c.Busy() != "" {
		return
	}
	excluded := append([]string(nil), c.generalSettings().Data().CloudSyncDisabledPlugins...)
	found := false
	next := make([]string, 0, len(excluded)+1)
	for _, candidate := range excluded {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == pluginID {
			found = true
			continue
		}
		next = append(next, candidate)
	}
	if !found {
		next = append(next, pluginID)
	}
	sort.Strings(next)
	encoded, err := json.Marshal(next)
	if err != nil {
		c.SetError("Could not encode plugin exclusions: " + err.Error())
		c.settings.invalidateSettingsWindow()
		return
	}
	c.SetBusy("exclusion-" + pluginID)
	c.SetError("")
	c.settings.invalidateSettingsWindow()
	util.Go(c.lifecycleCtx, "save cloud plugin exclusions", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, "CloudSyncDisabledPlugins", string(encoded))
		cancel()
		if err == nil {
			err = c.reloadSettings()
		}
		if err != nil {
			c.SetError("Could not save plugin exclusions: " + err.Error())
		}
		c.SetBusy("")
		c.settings.invalidateSettingsWindow()
	})
}

func cloudPluginExclusionRows(plugins []pluginSettingsPlugin, excluded []string) []pluginSettingsPlugin {
	rows := append([]pluginSettingsPlugin(nil), plugins...)
	seen := make(map[string]bool, len(rows))
	for _, plugin := range rows {
		seen[plugin.ID] = true
	}
	for _, pluginID := range excluded {
		pluginID = strings.TrimSpace(pluginID)
		if pluginID != "" && !seen[pluginID] {
			seen[pluginID] = true
			rows = append(rows, pluginSettingsPlugin{ID: pluginID, Name: pluginID + " · Uninstalled"})
		}
	}
	return rows
}

// openCloudLegalPage uses the current Wox language while leaving browser integration in the window abstraction.
func (c *cloudSettingsController) openCloudLegalPage(path string) {
	lang := strings.ToLower(c.generalSettings().Data().LangCode)
	prefix := ""
	if strings.HasPrefix(lang, "zh") {
		prefix = "/zh"
	}
	if err := c.settings.settingsNativeWindow().OpenExternalURL("https://sync.woxlauncher.com" + prefix + path); err != nil {
		form := c.Form()
		if form != nil {
			form.error = "Could not open legal page: " + err.Error()
		}
		c.settings.invalidateSettingsWindow()
	}
}
