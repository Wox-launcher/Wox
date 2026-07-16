package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
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
	}
}

// reloadCloudSync refreshes account, sync, and device state as one revisioned settings snapshot.
func (a *App) reloadCloudSync() {
	a.mu.Lock()
	a.cloudRevision++
	revision := a.cloudRevision
	a.cloudLoading = true
	a.cloudError = ""
	a.mu.Unlock()
	_ = a.window.Invalidate()

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
		a.clampCloudPluginScrollLocked()
	}
	a.cloudLoaded = accountErr == nil && statusErr == nil
	a.cloudError = strings.Join(errors, " · ")
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

// applyCloudSyncProgress updates transient progress immediately and reloads authoritative state when work completes.
func (a *App) applyCloudSyncProgress(data json.RawMessage) error {
	var progress cloudSyncProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return err
	}
	a.mu.Lock()
	if progress.Active {
		copy := progress
		a.cloudSync.Progress = &copy
	} else {
		a.cloudSync.Progress = nil
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
	if !progress.Active {
		go a.reloadCloudSync()
	}
	return nil
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
	_ = a.window.Invalidate()
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
			_ = a.window.Invalidate()
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
	_ = a.window.Invalidate()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		var session struct {
			URL string `json:"url"`
		}
		err := a.client.Post(ctx, route, map[string]any{}, &session)
		cancel()
		if err == nil && session.URL != "" {
			err = a.window.OpenExternalURL(session.URL)
		}
		a.mu.Lock()
		a.cloudBusy = ""
		if err != nil {
			a.cloudError = "Could not open billing: " + err.Error()
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
	}()
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
	_ = a.window.Invalidate()
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
			_ = a.window.Invalidate()
			return
		}
		a.cloudForm = newCloudBootstrapForm(status)
		a.mu.Unlock()
		a.updateFormTextInput(true)
		_ = a.window.Invalidate()
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
	return &cloudFormState{formFieldsState: fields, kind: "bootstrap", title: title, hasRemoteData: status.HasRemoteData}
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
	_ = a.window.Invalidate()
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
	title := "Log in"
	switch kind {
	case "login", "register":
		definitions = append(definitions,
			formDefinition{Type: "textbox", Value: formDefinitionValue{Key: "Email", Label: "Email", MaxLines: 1}},
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "Password", Label: "Password", MaxLines: 1}},
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
		title = "Create account"
		definitions = append(definitions,
			formDefinition{Type: "password", Value: formDefinitionValue{Key: "ConfirmPassword", Label: "Confirm password", MaxLines: 1}},
			formDefinition{Type: "checkbox", Value: formDefinitionValue{Key: "AcceptedLegal", Label: "Accept Terms & Privacy"}},
		)
	}
	a.mu.Lock()
	values := map[string]string{"Email": a.cloudAccount.Email}
	fields := newFormFieldsState(definitions, values, true)
	a.cloudForm = &cloudFormState{formFieldsState: fields, kind: kind, title: title}
	a.mu.Unlock()
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) openCloudVerificationForm(email string) {
	definitions := []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "Code", Label: "Verification code", MaxLines: 1}}}
	fields := newFormFieldsState(definitions, nil, true)
	a.mu.Lock()
	a.cloudForm = &cloudFormState{formFieldsState: fields, kind: "verify", title: "Verify email", email: email}
	a.mu.Unlock()
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
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
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
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
		a.updateFormTextInput(true)
		_ = a.window.Invalidate()
	}()
}

func (a *App) closeCloudForm() {
	a.mu.Lock()
	a.cloudForm = nil
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
}

func (a *App) focusCloudFormField(index int) {
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving {
		syncFormFieldsEditorLocked(&a.cloudForm.formFieldsState)
		setFormFieldsFocusLocked(&a.cloudForm.formFieldsState, index)
	}
	active := a.cloudForm != nil && a.cloudForm.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(active)
	_ = a.window.Invalidate()
}

func (a *App) setCloudFormCaret(index, offset int) {
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving {
		if a.cloudForm.focused != index {
			syncFormFieldsEditorLocked(&a.cloudForm.formFieldsState)
			setFormFieldsFocusLocked(&a.cloudForm.formFieldsState, index)
		}
		if a.cloudForm.editor != nil {
			a.cloudForm.editor.SetCaret(offset)
		}
	}
	a.mu.Unlock()
	a.updateFormTextInput(true)
	_ = a.window.Invalidate()
}

func (a *App) changeCloudFormField(index, delta int) {
	a.mu.Lock()
	if a.cloudForm != nil && !a.cloudForm.saving {
		changeFormFieldsChoiceLocked(&a.cloudForm.formFieldsState, index, delta)
	}
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) moveCloudFormFocus(delta int) {
	a.mu.Lock()
	if a.cloudForm == nil || len(a.cloudForm.definitions) == 0 || a.cloudForm.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&a.cloudForm.formFieldsState)
	index := a.cloudForm.focused
	for step := 0; step < len(a.cloudForm.definitions); step++ {
		index = (index + delta + len(a.cloudForm.definitions)) % len(a.cloudForm.definitions)
		if formDefinitionFocusable(a.cloudForm.definitions[index]) {
			setFormFieldsFocusLocked(&a.cloudForm.formFieldsState, index)
			break
		}
	}
	active := a.cloudForm.editor != nil
	a.mu.Unlock()
	a.updateFormTextInput(active)
	_ = a.window.Invalidate()
}

// onCloudSettingsKey gives the active account/bootstrap modal exclusive keyboard ownership.
func (a *App) onCloudSettingsKey(event woxui.KeyEvent) bool {
	a.mu.RLock()
	active := a.mode == viewSettings && a.cloudForm != nil
	saving := active && a.cloudForm.saving
	a.mu.RUnlock()
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
	case woxui.KeySpace, woxui.KeyArrowLeft, woxui.KeyArrowRight:
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
			a.mu.Lock()
			if a.cloudForm != nil && a.cloudForm.editor != nil && focused >= 0 && focused < len(a.cloudForm.definitions) {
				handleFormEditorKey(a.cloudForm.editor, a.cloudForm.definitions[focused], event)
			}
			a.mu.Unlock()
			_ = a.window.Invalidate()
		}
	default:
		a.mu.Lock()
		if a.cloudForm != nil && a.cloudForm.editor != nil && a.cloudForm.focused >= 0 && a.cloudForm.focused < len(a.cloudForm.definitions) {
			handleFormEditorKey(a.cloudForm.editor, a.cloudForm.definitions[a.cloudForm.focused], event)
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
	}
	return true
}

// onCloudFormTextInput commits native IME input only while a cloud modal owns focus.
func (a *App) onCloudFormTextInput(event woxui.TextInputEvent) bool {
	a.mu.Lock()
	if a.mode != viewSettings || a.cloudForm == nil || a.cloudForm.saving || a.cloudForm.editor == nil {
		a.mu.Unlock()
		return false
	}
	a.cloudForm.editor.HandleTextInput(event)
	a.mu.Unlock()
	_ = a.window.Invalidate()
	return true
}

// submitCloudForm validates local invariants before sending credentials or recovery data to core.
func (a *App) submitCloudForm() {
	a.mu.Lock()
	if a.cloudForm == nil || a.cloudForm.saving {
		a.mu.Unlock()
		return
	}
	syncFormFieldsEditorLocked(&a.cloudForm.formFieldsState)
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
		_ = a.window.Invalidate()
		return
	}
	a.cloudForm.saving = true
	a.cloudForm.error = ""
	a.mu.Unlock()
	_ = a.window.SetTextInputState(woxui.TextInputState{})
	_ = a.window.Invalidate()
	go a.submitCloudFormRequest(kind, values, email)
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
		a.updateFormTextInput(textInputActive)
		_ = a.window.Invalidate()
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

func (a *App) setCloudPageGeometry(viewport, content float32) {
	a.mu.Lock()
	a.cloudPageViewport = max(float32(1), viewport)
	a.cloudPageContent = max(content, viewport)
	a.clampCloudPageScrollLocked()
	a.mu.Unlock()
}

func (a *App) scrollCloudPage(delta float32) {
	a.mu.Lock()
	a.cloudPageScroll += delta
	a.clampCloudPageScrollLocked()
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) clampCloudPageScrollLocked() {
	maximum := max(float32(0), a.cloudPageContent-a.cloudPageViewport)
	a.cloudPageScroll = min(max(float32(0), a.cloudPageScroll), maximum)
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
		_ = a.window.Invalidate()
		return
	}
	a.cloudBusy = "exclusion-" + pluginID
	a.cloudError = ""
	a.mu.Unlock()
	_ = a.window.Invalidate()
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
		_ = a.window.Invalidate()
	}()
}

func (a *App) setCloudPluginViewport(height float32) {
	a.mu.Lock()
	a.cloudPluginViewport = max(float32(1), height)
	a.clampCloudPluginScrollLocked()
	a.mu.Unlock()
}

func (a *App) scrollCloudPlugins(delta float32) {
	a.mu.Lock()
	a.cloudPluginScroll += delta
	a.clampCloudPluginScrollLocked()
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) clampCloudPluginScrollLocked() {
	rowCount := len(cloudPluginExclusionRows(a.cloudPlugins, a.settings.CloudSyncDisabledPlugins))
	maximum := max(float32(0), float32(rowCount)*46-a.cloudPluginViewport)
	a.cloudPluginScroll = min(max(float32(0), a.cloudPluginScroll), maximum)
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
	if err := a.window.OpenExternalURL("https://sync.woxlauncher.com" + prefix + path); err != nil {
		a.mu.Lock()
		if a.cloudForm != nil {
			a.cloudForm.error = "Could not open legal page: " + err.Error()
		}
		a.mu.Unlock()
		_ = a.window.Invalidate()
	}
}
