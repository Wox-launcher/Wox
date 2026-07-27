package launcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// cloudSettingsSnapshot is the immutable Cloud tab state consumed by the view layer.
// cloudRevision is intentionally excluded: it is runtime state for the reload
// revision guard and is never read by the view. cloudLoaded is also excluded:
// only the load decision reads it (via Loaded()), never the view.
type cloudSettingsSnapshot struct {
	Account       cloudAccountStatus
	Sync          cloudSyncStatus
	BillingPlan   cloudBillingPlan
	BillingLoaded bool
	Devices       cloudDeviceList
	Loading       bool
	Busy          string
	Error         string
	Form          *cloudFormSnapshot
	ActionMenu    string
	Plugins       []pluginSettingsPlugin
}

// cloudSettingsController owns the Cloud tab state (account, sync, billing plan,
// devices, plugins, loading flags, busy/error, form, action menu). All 13 fields
// that used to live on App are held here; App methods became thin wrappers that call
// the controller's getters/setters while still coordinating cross-domain state
// (form mutation, shared setting note, native window URLs) under a.mu before delegating.
//
// The controller's mu only guards pointer swaps and scalar stores. Form mutation by
// cross-domain code happens under a.mu — same convention as pluginSettings.Form()
// and aiSettings.Form(). The Form() getter returns the live *cloudFormState pointer
// for that purpose.
type cloudSettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

	account       cloudAccountStatus
	sync          cloudSyncStatus
	billingPlan   cloudBillingPlan
	billingLoaded bool
	devices       cloudDeviceList
	loading       bool
	loaded        bool
	busy          string
	errMsg        string
	revision      uint64
	form          *cloudFormState
	actionMenu    string
	plugins       []pluginSettingsPlugin
}

func newCloudSettingsController(deps CommonDeps) *cloudSettingsController {
	return &cloudSettingsController{deps: deps}
}

func (c *cloudSettingsController) Account() cloudAccountStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.account
}

func (c *cloudSettingsController) SetAccount(account cloudAccountStatus) {
	c.mu.Lock()
	c.account = account
	c.mu.Unlock()
}

func (c *cloudSettingsController) Sync() cloudSyncStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sync
}

func (c *cloudSettingsController) SetSync(sync cloudSyncStatus) {
	c.mu.Lock()
	c.sync = sync
	c.mu.Unlock()
}

// SetSyncProgress updates only the Progress field on the sync status. Used by the
// contract adapter to apply transient sync progress pushed by core.
func (c *cloudSettingsController) SetSyncProgress(progress *cloudSyncProgress) {
	c.mu.Lock()
	c.sync.Progress = progress
	c.mu.Unlock()
}

func (c *cloudSettingsController) BillingPlan() cloudBillingPlan {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.billingPlan
}

func (c *cloudSettingsController) SetBillingPlan(plan cloudBillingPlan) {
	c.mu.Lock()
	c.billingPlan = plan
	c.mu.Unlock()
}

func (c *cloudSettingsController) BillingLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.billingLoaded
}

func (c *cloudSettingsController) SetBillingLoaded(loaded bool) {
	c.mu.Lock()
	c.billingLoaded = loaded
	c.mu.Unlock()
}

func (c *cloudSettingsController) Devices() cloudDeviceList {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneCloudDeviceList(c.devices)
}

func (c *cloudSettingsController) SetDevices(devices cloudDeviceList) {
	c.mu.Lock()
	c.devices = devices
	c.mu.Unlock()
}

func (c *cloudSettingsController) Loading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loading
}

func (c *cloudSettingsController) SetLoading(loading bool) {
	c.mu.Lock()
	c.loading = loading
	c.mu.Unlock()
}

func (c *cloudSettingsController) Loaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loaded
}

func (c *cloudSettingsController) SetLoaded(loaded bool) {
	c.mu.Lock()
	c.loaded = loaded
	c.mu.Unlock()
}

func (c *cloudSettingsController) Busy() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.busy
}

func (c *cloudSettingsController) SetBusy(busy string) {
	c.mu.Lock()
	c.busy = busy
	c.mu.Unlock()
}

func (c *cloudSettingsController) Error() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.errMsg
}

func (c *cloudSettingsController) SetError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
}

// Revision returns the current reload revision. Used by tests to verify the
// revision guard discards stale responses.
func (c *cloudSettingsController) Revision() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.revision
}

// Form returns the live cloud form pointer. Cross-domain callers mutate the form
// in place under a.mu. The controller's mu only guards the pointer swap, not the
// form's fields — same convention as pluginSettings.Form() and aiSettings.Form().
func (c *cloudSettingsController) Form() *cloudFormState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.form
}

func (c *cloudSettingsController) SetForm(form *cloudFormState) {
	c.mu.Lock()
	c.form = form
	c.mu.Unlock()
}

func (c *cloudSettingsController) ActionMenu() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.actionMenu
}

func (c *cloudSettingsController) SetActionMenu(menu string) {
	c.mu.Lock()
	c.actionMenu = menu
	c.mu.Unlock()
}

func (c *cloudSettingsController) Plugins() []pluginSettingsPlugin {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

func (c *cloudSettingsController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.mu.Lock()
	c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
	c.mu.Unlock()
}

// ReloadCloudSync refreshes account, sync, devices, and plugins as one revisioned
// settings snapshot. onNeedBilling is called (outside the controller lock) when the
// billing plan has not been loaded yet, so the App can kick off a billing reload
// without coupling the controller to the billing reload path. Responses from
// superseded refreshes are discarded via the revision guard.
func (c *cloudSettingsController) ReloadCloudSync(ctx context.Context, client backendClient, onNeedBilling func()) {
	c.mu.Lock()
	c.revision++
	revision := c.revision
	needBilling := !c.billingLoaded
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()
	if needBilling && onNeedBilling != nil {
		onNeedBilling()
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var account cloudAccountStatus
	var status cloudSyncStatus
	var devices cloudDeviceList
	var plugins []pluginSettingsPlugin
	accountErr := client.Post(timeoutCtx, "/account/status", map[string]any{}, &account)
	var statusErr error
	var devicesErr error
	var pluginsErr error
	if accountErr == nil {
		statusErr = client.Post(timeoutCtx, "/sync/status", map[string]any{}, &status)
		if account.LoggedIn {
			devicesErr = client.Post(timeoutCtx, "/sync/devices/list", map[string]any{}, &devices)
			pluginsErr = client.Post(timeoutCtx, "/plugin/installed", map[string]any{}, &plugins)
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
	c.mu.Lock()
	if revision != c.revision {
		c.mu.Unlock()
		return
	}
	c.loading = false
	if accountErr == nil {
		c.account = account
	}
	if statusErr == nil {
		c.sync = status
	}
	if !account.LoggedIn {
		c.devices = cloudDeviceList{}
		c.plugins = nil
	} else if devicesErr == nil {
		c.devices = devices
	}
	if account.LoggedIn && pluginsErr == nil {
		c.plugins = plugins
	}
	c.loaded = accountErr == nil && statusErr == nil
	c.errMsg = strings.Join(errors, " · ")
	c.mu.Unlock()
	c.deps.Invalidate()
}

// ReloadBillingPlan fetches display pricing independently so it cannot delay local
// sync status. On success BillingLoaded becomes true and BillingPlan is stored.
func (c *cloudSettingsController) ReloadBillingPlan(ctx context.Context, client backendClient) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var plan cloudBillingPlan
	err := client.Post(timeoutCtx, "/account/billing/plan", map[string]any{}, &plan)
	c.mu.Lock()
	c.billingLoaded = true
	if err == nil {
		c.billingPlan = plan
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Cloud state for the view layer. Form is deep-copied
// via snapshotCloudFormLocked so the snapshot is safe to read outside the lock;
// all other fields are value types or copied slices.
func (c *cloudSettingsController) Snapshot() cloudSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloudSettingsSnapshot{
		Account:       c.account,
		Sync:          c.sync,
		BillingPlan:   c.billingPlan,
		BillingLoaded: c.billingLoaded,
		Devices:       cloneCloudDeviceList(c.devices),
		Loading:       c.loading,
		Busy:          c.busy,
		Error:         c.errMsg,
		Form:          snapshotCloudFormLocked(c.form),
		ActionMenu:    c.actionMenu,
		Plugins:       append([]pluginSettingsPlugin(nil), c.plugins...),
	}
}
