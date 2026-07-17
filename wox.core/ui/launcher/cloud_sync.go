package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
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
	Price cloudBillingPlanPrice `json:"price"`
}

type cloudBillingPlanPrice struct {
	Currency   string `json:"currency"`
	UnitAmount *int   `json:"unit_amount"`
	Interval   string `json:"interval"`
	Formatted  string `json:"formatted"`
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

type cloudAccountResult struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"expires_at"`
}

type cloudBootstrapStatus struct {
	HasRemoteData bool `json:"has_remote_data"`
	HasRemoteKey  bool `json:"has_remote_key"`
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
func (a *App) reloadCloudSync() {
	a.mu.Lock()
	a.cloudRevision++
	revision := a.cloudRevision
	loadBillingPlan := !a.cloudBillingLoaded
	a.cloudLoading = true
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	if loadBillingPlan {
		go a.reloadCloudBillingPlan()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var account cloudAccountStatus
	var status cloudSyncStatus
	var devices cloudDeviceList
	var plugins []pluginSettingsPlugin
	accountErr := a.client.Post(ctx, "/account/status", map[string]any{}, &account)
	var statusErr error
	var devicesErr error
	var pluginsErr error
	if accountErr == nil {
		statusErr = a.client.Post(ctx, "/sync/status", map[string]any{}, &status)
		if account.LoggedIn {
			devicesErr = a.client.Post(ctx, "/sync/devices/list", map[string]any{}, &devices)
			pluginsErr = a.client.Post(ctx, "/plugin/installed", map[string]any{}, &plugins)
		}
	}
	sort.SliceStable(plugins, func(i, j int) bool {
		left := strings.ToLower(plugins[i].Name)
		right := strings.ToLower(plugins[j].Name)
		if left == right {
			return plugins[i].ID < plugins[j].ID
		}
		return left < right
	})

	errors := make([]string, 0, 3)
	if accountErr != nil {
		errors = append(errors, "account: "+accountErr.Error())
	}
	if statusErr != nil {
		errors = append(errors, "sync: "+statusErr.Error())
	}
	if devicesErr != nil {
		errors = append(errors, "devices: "+devicesErr.Error())
	}
	if pluginsErr != nil {
		errors = append(errors, "plugins: "+pluginsErr.Error())
	}
	a.mu.Lock()
	if revision != a.cloudRevision {
		a.mu.Unlock()
		return
	}
	a.cloudLoading = false
	if accountErr == nil {
		a.cloudAccount = account
	}
	if statusErr == nil {
		a.cloudSync = status
	}
	if !account.LoggedIn {
		a.cloudDevices = cloudDeviceList{}
		a.cloudPlugins = nil
	} else if devicesErr == nil {
		a.cloudDevices = devices
	}
	if account.LoggedIn && pluginsErr == nil {
		a.cloudPlugins = plugins
	}
	a.cloudLoaded = accountErr == nil && statusErr == nil
	a.cloudError = strings.Join(errors, " · ")
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// reloadCloudBillingPlan fetches display pricing independently so it cannot delay local sync status.
func (a *App) reloadCloudBillingPlan() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var plan cloudBillingPlan
	err := a.client.Post(ctx, "/account/billing/plan", map[string]any{}, &plan)
	a.mu.Lock()
	a.cloudBillingLoaded = true
	if err == nil {
		a.cloudBillingPlan = plan
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// runCloudAction serializes account and sync mutations before reloading their shared status.
func (a *App) runCloudAction(name, route string, payload any) {
	a.mu.Lock()
	if a.cloudBusy != "" {
		a.mu.Unlock()
		return
	}
	a.cloudBusy = name
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.client.Post(ctx, route, payload, nil)
		if err == nil && name == "sync" {
			err = a.client.Post(ctx, "/sync/pull", map[string]any{}, nil)
		}
		cancel()
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = fmt.Sprintf("%s failed: %v", cloudActionLabel(name), err)
		}
		a.mu.Unlock()
		a.reloadCloudSync()
		if err != nil {
			a.mu.Lock()
			a.cloudError = fmt.Sprintf("%s failed: %v", cloudActionLabel(name), err)
			a.mu.Unlock()
			a.invalidateSettingsWindow()
		}
	}()
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
func (a *App) openCloudBilling() {
	a.mu.RLock()
	isPro := strings.EqualFold(a.cloudAccount.Plan, "pro")
	a.mu.RUnlock()
	route := "/account/billing/checkout"
	if isPro {
		route = "/account/billing/portal"
	}
	a.mu.Lock()
	if a.cloudBusy != "" {
		a.mu.Unlock()
		return
	}
	a.cloudBusy = "billing"
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var session struct {
			URL string `json:"url"`
		}
		err := a.client.Post(ctx, route, map[string]any{}, &session)
		cancel()
		if err == nil && session.URL != "" {
			err = a.settingsNativeWindow().OpenExternalURL(session.URL)
		}
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = "Could not open billing: " + err.Error()
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}()
}

// openCloudSupportEmail opens the localized billing-support draft in the default mail application.
func (a *App) openCloudSupportEmail() {
	subject := url.QueryEscape(a.translate("i18n:ui_cloud_sync_billing_help_email_subject"))
	if err := a.settingsNativeWindow().OpenExternalURL("mailto:billing@woxlauncher.com?subject=" + subject); err != nil {
		a.mu.Lock()
		a.cloudError = err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}

// toggleCloudActionMenu opens the compact account or subscription menu used by the flat settings rows.
func (a *App) toggleCloudActionMenu(menu string) {
	a.mu.Lock()
	if a.cloudActionMenu == menu {
		a.cloudActionMenu = ""
	} else {
		a.cloudActionMenu = menu
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) closeCloudActionMenu() {
	a.mu.Lock()
	a.cloudActionMenu = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// runCloudMenuAction preserves Flutter's account and subscription actions behind compact row menus.
func (a *App) runCloudMenuAction(action string) {
	a.mu.Lock()
	a.cloudActionMenu = ""
	a.mu.Unlock()
	switch action {
	case "change-password":
		a.openCloudAccountForm("change-password")
	case "logout":
		a.runCloudAction("logout", "/account/logout", map[string]any{})
	case "refresh":
		go a.reloadCloudSync()
	case "billing":
		a.openCloudBilling()
	}
}

// beginCloudBootstrap reuses a local key when possible or opens the recovery-password form required by core.
func (a *App) beginCloudBootstrap() {
	a.mu.RLock()
	keyAvailable := a.cloudSync.KeyStatus.Available
	busy := a.cloudBusy != ""
	a.mu.RUnlock()
	if busy {
		return
	}
	if keyAvailable {
		a.runCloudBootstrap("")
		return
	}
	a.mu.Lock()
	a.cloudBusy = "bootstrap-status"
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		var status cloudBootstrapStatus
		err := a.client.Post(ctx, "/sync/bootstrap/status", map[string]any{}, &status)
		cancel()
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = "Could not prepare cloud sync: " + err.Error()
			a.mu.Unlock()
			a.invalidateSettingsWindow()
			return
		}
		a.cloudForm = newCloudBootstrapForm(status)
		a.mu.Unlock()
		a.updateSettingsTextInput(true)
		a.invalidateSettingsWindow()
	}()
}

func newCloudBootstrapForm(status cloudBootstrapStatus) *cloudFormState {
	definitions := []formDefinition{{Type: "password", Value: formDefinitionValue{Key: "RecoveryCode", Label: "Encryption password", MaxLines: 1}}}
	if !status.HasRemoteData {
		definitions = append(definitions, formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmRecoveryCode", Label: "Confirm password", MaxLines: 1}})
	}
	title := "Enable Cloud Sync"
	if status.HasRemoteData {
		title = "Restore Cloud Sync"
	}
	fields := newFormFieldsState(definitions, nil, true)
	state := newCloudFormState(fields, "bootstrap", title)
	state.hasRemoteData = status.HasRemoteData
	return state
}

func (a *App) runCloudBootstrap(recoveryCode string) {
	a.mu.Lock()
	if a.cloudBusy != "" {
		a.mu.Unlock()
		return
	}
	a.cloudBusy = "bootstrap"
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := a.client.Post(ctx, "/sync/bootstrap/start", map[string]string{"recovery_code": recoveryCode}, nil)
		cancel()
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = "Could not start cloud sync: " + err.Error()
		}
		a.mu.Unlock()
		a.reloadCloudSync()
		if err == nil {
			time.AfterFunc(2*time.Second, a.reloadCloudSync)
		}
	}()
}

// openCloudAccountForm creates account lifecycle forms from the shared text-editing engine.
func (a *App) openCloudAccountForm(kind string) {
	definitions := []formDefinition{}
	title := a.translate("i18n:ui_cloud_sync_account_login")
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
		title = a.translate("i18n:ui_cloud_sync_account_register")
		definitions = append(definitions,
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmPassword", Label: "i18n:ui_cloud_sync_account_confirm_password", MaxLines: 1}},
			formDefinition{Type: "checkbox", Value: formDefinitionValue{Key: "AcceptedLegal", Label: "i18n:ui_cloud_sync_account_accept_prefix"}},
		)
	}
	a.mu.Lock()
	values := map[string]string{"Email": a.cloudAccount.Email}
	fields := newFormFieldsState(definitions, values, true)
	a.cloudForm = newCloudFormState(fields, kind, title)
	a.mu.Unlock()
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

func (a *App) openCloudVerificationForm(email string) {
	definitions := []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "Code", Label: "Verification code", MaxLines: 1}}}
	fields := newFormFieldsState(definitions, nil, true)
	a.mu.Lock()
	a.cloudForm = newCloudFormState(fields, "verify", "Verify email")
	a.cloudForm.email = email
	a.mu.Unlock()
	a.updateSettingsTextInput(true)
	a.invalidateSettingsWindow()
}

// resendCloudVerification keeps the current verification editor open while requesting a fresh code.
func (a *App) resendCloudVerification() {
	a.mu.Lock()
	if a.cloudForm == nil || a.cloudForm.kind != "verify" || a.cloudForm.saving {
		a.mu.Unlock()
		return
	}
	email := a.cloudForm.email
	lang := a.settings.LangCode
	a.cloudForm.saving = true
	a.cloudForm.error = ""
	a.cloudForm.notice = ""
	a.cloudForm.notice = ""
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err := a.client.Post(ctx, "/account/resend_verification", map[string]string{"email": email, "lang": lang}, nil)
		cancel()
		a.mu.Lock()
		if a.cloudForm != nil && a.cloudForm.kind == "verify" {
			a.cloudForm.saving = false
			if err != nil {
				a.cloudForm.error = err.Error()
			} else {
				a.cloudForm.notice = "A new verification code was sent."
			}
		}
		a.mu.Unlock()
		a.updateSettingsTextInput(true)
		a.invalidateSettingsWindow()
	}()
}

func (a *App) closeCloudForm() {
	a.mu.Lock()
	a.cloudForm = nil
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) focusCloudFormField(index int) {
	var focusNode *woxwidget.FocusNode
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving {
		setCloudFormFocusLocked(a.cloudForm, index)
		if index >= 0 && index < len(a.cloudForm.focusNodes) {
			focusNode = a.cloudForm.focusNodes[index]
		}
	}
	a.mu.Unlock()
	if focusNode != nil {
		focusNode.RequestFocus()
	}
	a.invalidateSettingsWindow()
}

// setCloudFormText keeps business form values synchronized with retained field controllers.
func (a *App) setCloudFormText(index int, value string) {
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving && index >= 0 && index < len(a.cloudForm.definitions) {
		definition := a.cloudForm.definitions[index]
		if formDefinitionTextEditable(definition) {
			a.cloudForm.values[definition.Value.Key] = value
		}
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

// setCloudFormFieldFocused mirrors Host focus into form navigation and scroll state.
func (a *App) setCloudFormFieldFocused(index int, focused bool) {
	if !focused {
		return
	}
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving && index >= 0 && index < len(a.cloudForm.definitions) {
		setCloudFormFocusLocked(a.cloudForm, index)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) changeCloudFormField(index, delta int) {
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving {
		changeFormFieldsChoiceLocked(&a.cloudForm.formFieldsState, index, delta)
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func (a *App) moveCloudFormFocus(delta int) {
	var focusNode *woxwidget.FocusNode
	a.mu.Lock()
	if a.cloudForm == nil || len(a.cloudForm.definitions) == 0 || a.cloudForm.saving {
		a.mu.Unlock()
		return
	}
	index := a.cloudForm.focused
	for step := 0; step < len(a.cloudForm.definitions); step++ {
		index = (index + delta + len(a.cloudForm.definitions)) % len(a.cloudForm.definitions)
		if formDefinitionFocusable(a.cloudForm.definitions[index]) {
			setCloudFormFocusLocked(a.cloudForm, index)
			if index < len(a.cloudForm.focusNodes) {
				focusNode = a.cloudForm.focusNodes[index]
			}
			break
		}
	}
	a.mu.Unlock()
	if focusNode != nil {
		focusNode.RequestFocus()
	}
	a.invalidateSettingsWindow()
}

// onCloudSettingsKey gives the active account/bootstrap modal exclusive keyboard ownership.
func (a *App) onCloudSettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	menuActive := a.settingsOpen && a.cloudActionMenu != ""
	active := a.settingsOpen && a.cloudForm != nil
	saving := active && a.cloudForm.saving
	a.mu.RUnlock()
	if menuActive && !active {
		if event.Key == woxui.KeyEscape {
			a.closeCloudActionMenu()
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
		a.closeCloudForm()
	case woxui.KeyTab, woxui.KeyArrowDown, woxui.KeyArrowUp:
		delta := 1
		if event.Key == woxui.KeyArrowUp || event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		a.moveCloudFormFocus(delta)
	case woxui.KeyEnter:
		a.submitCloudForm()
	case woxui.KeySpace:
		a.mu.RLock()
		focused := -1
		fieldType := ""
		if a.cloudForm != nil {
			focused = a.cloudForm.focused
			if focused >= 0 && focused < len(a.cloudForm.definitions) {
				fieldType = a.cloudForm.definitions[focused].Type
			}
		}
		a.mu.RUnlock()
		if fieldType == "checkbox" {
			a.changeCloudFormField(focused, 1)
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
func (a *App) onCloudFormTextInput(_ woxui.TextInputEvent) bool {
	a.mu.RLock()
	active := a.settingsOpen && a.cloudForm != nil
	a.mu.RUnlock()
	return active
}

// submitCloudForm validates local invariants before sending credentials or recovery data to core.
func (a *App) submitCloudForm() {
	a.mu.Lock()
	if a.cloudForm == nil || a.cloudForm.saving {
		a.mu.Unlock()
		return
	}
	syncCloudFormControllersLocked(a.cloudForm)
	kind := a.cloudForm.kind
	values := make(map[string]string, len(a.cloudForm.values))
	for key, value := range a.cloudForm.values {
		values[key] = value
	}
	values["Email"] = strings.TrimSpace(values["Email"])
	values["Code"] = strings.TrimSpace(values["Code"])
	values["RecoveryCode"] = strings.TrimSpace(values["RecoveryCode"])
	values["ConfirmRecoveryCode"] = strings.TrimSpace(values["ConfirmRecoveryCode"])
	values["Token"] = strings.TrimSpace(values["Token"])
	email := a.cloudForm.email
	hasRemoteData := a.cloudForm.hasRemoteData
	validationError := validateCloudForm(kind, values, hasRemoteData)
	if validationError != "" {
		a.cloudForm.error = validationError
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	a.cloudForm.saving = true
	a.cloudForm.error = ""
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	go a.submitCloudFormRequest(kind, values, email)
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
			return "Encryption password is required."
		}
		if !hasRemoteData && values["RecoveryCode"] != values["ConfirmRecoveryCode"] {
			return "Encryption passwords do not match."
		}
	}
	return ""
}

func (a *App) submitCloudFormRequest(kind string, values map[string]string, email string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var result cloudAccountResult
	var err error
	switch kind {
	case "login", "register":
		route := "/account/login"
		if kind == "register" {
			route = "/account/register"
		}
		a.mu.RLock()
		lang := a.settings.LangCode
		a.mu.RUnlock()
		err = a.client.Post(ctx, route, map[string]string{"email": values["Email"], "password": values["Password"], "lang": lang}, &result)
	case "verify":
		a.mu.RLock()
		lang := a.settings.LangCode
		a.mu.RUnlock()
		err = a.client.Post(ctx, "/account/verify_email", map[string]string{"email": email, "code": values["Code"], "lang": lang}, &result)
	case "reset-request":
		a.mu.RLock()
		lang := a.settings.LangCode
		a.mu.RUnlock()
		err = a.client.Post(ctx, "/account/password_reset/request", map[string]string{"email": values["Email"], "lang": lang}, nil)
	case "reset-confirm":
		a.mu.RLock()
		lang := a.settings.LangCode
		a.mu.RUnlock()
		err = a.client.Post(ctx, "/account/password_reset/confirm", map[string]string{"token": values["Token"], "password": values["Password"], "lang": lang}, nil)
	case "change-password":
		a.mu.RLock()
		lang := a.settings.LangCode
		a.mu.RUnlock()
		err = a.client.Post(ctx, "/account/change_password", map[string]string{"current_password": values["CurrentPassword"], "new_password": values["Password"], "lang": lang}, nil)
	case "bootstrap":
		err = a.client.Post(ctx, "/sync/bootstrap/start", map[string]string{"recovery_code": values["RecoveryCode"]}, nil)
	}
	if err == nil && (kind == "login" || kind == "register") && result.Code == "need_verify_email" {
		verificationEmail := result.Email
		if verificationEmail == "" {
			verificationEmail = values["Email"]
		}
		a.openCloudVerificationForm(verificationEmail)
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
		a.mu.Lock()
		if a.cloudForm != nil && a.cloudForm.kind == kind {
			a.cloudForm.saving = false
			a.cloudForm.error = err.Error()
		}
		textInputActive := a.cloudForm != nil && a.cloudForm.editor != nil
		a.mu.Unlock()
		a.updateSettingsTextInput(textInputActive)
		a.invalidateSettingsWindow()
		return
	}
	if kind == "reset-request" {
		a.openCloudAccountForm("reset-confirm")
		return
	}
	if kind == "reset-confirm" {
		a.openCloudAccountForm("login")
		return
	}
	if kind == "change-password" {
		a.mu.Lock()
		a.settingNote = "Account password changed."
		a.mu.Unlock()
	}
	a.closeCloudForm()
	a.reloadCloudSync()
	if kind == "bootstrap" {
		time.AfterFunc(2*time.Second, a.reloadCloudSync)
	}
}

// toggleCloudPluginExclusion persists the exact plugin ID list expected by Wox core.
func (a *App) toggleCloudPluginExclusion(pluginID string) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return
	}
	a.mu.Lock()
	if a.cloudBusy != "" {
		a.mu.Unlock()
		return
	}
	excluded := append([]string(nil), a.settings.CloudSyncDisabledPlugins...)
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
		a.cloudError = "Could not encode plugin exclusions: " + err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
		return
	}
	a.cloudBusy = "exclusion-" + pluginID
	a.cloudError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.client.Post(ctx, "/setting/wox/update", map[string]string{"Key": "CloudSyncDisabledPlugins", "Value": string(encoded)}, nil)
		cancel()
		if err == nil {
			err = a.reloadSettings()
		}
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = "Could not save plugin exclusions: " + err.Error()
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}()
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
func (a *App) openCloudLegalPage(path string) {
	a.mu.RLock()
	lang := strings.ToLower(a.settings.LangCode)
	a.mu.RUnlock()
	prefix := ""
	if strings.HasPrefix(lang, "zh") {
		prefix = "/zh"
	}
	if err := a.settingsNativeWindow().OpenExternalURL("https://sync.woxlauncher.com" + prefix + path); err != nil {
		a.mu.Lock()
		if a.cloudForm != nil {
			a.cloudForm.error = "Could not open legal page: " + err.Error()
		}
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}
