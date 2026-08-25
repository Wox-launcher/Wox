package launcher

import (
	"context"
	"sort"
	"strings"
	"time"

	"wox/account"
	"wox/cloudsync"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
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
	PluginDialog  *cloudPluginExclusionDialogSnapshot
	Plugins       []pluginSettingsPlugin
}

// cloudPluginExclusionDialogState keeps selection state separate from the persisted settings value.
type cloudPluginExclusionDialogState struct {
	selected     string
	choiceOpen   bool
	choiceAnchor woxui.Rect
}

// cloudPluginExclusionDialogSnapshot is the immutable copy rendered by the settings overlay.
type cloudPluginExclusionDialogSnapshot struct {
	Selected     string
	ChoiceOpen   bool
	ChoiceAnchor woxui.Rect
}

// cloudSettingsController owns the Cloud tab state (account, sync, billing plan,
// devices, plugins, loading flags, busy/error, form, action menu, plugin dialog). All 13 fields
// that used to live on App are held here; App methods became thin wrappers that call
// the controller's getters/setters while coordinating cross-domain state on the UI thread.
type cloudSettingsController struct {
	deps CommonDeps

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
	pluginDialog  *cloudPluginExclusionDialogState
	plugins       []pluginSettingsPlugin
}

type cloudReloadServices interface {
	contract.CloudSettingsServices
	contract.PluginCatalogSettingsServices
}

func newCloudSettingsController(deps CommonDeps) *cloudSettingsController {
	return &cloudSettingsController{deps: deps}
}

func (c *cloudSettingsController) Account() cloudAccountStatus {
	return c.account
}

func (c *cloudSettingsController) SetAccount(account cloudAccountStatus) {
	c.deps.OnUI("set cloud account", func() {
		c.account = account
	})
}

func (c *cloudSettingsController) Sync() cloudSyncStatus {
	return c.sync
}

func (c *cloudSettingsController) SetSync(sync cloudSyncStatus) {
	c.deps.OnUI("set cloud sync status", func() {
		c.sync = sync
	})
}

// SetSyncProgress updates only the Progress field on the sync status. Used by the
// contract adapter to apply transient sync progress pushed by core.
func (c *cloudSettingsController) SetSyncProgress(progress *cloudSyncProgress) {
	c.deps.OnUI("set cloud sync progress", func() {
		c.sync.Progress = progress
	})
}

func (c *cloudSettingsController) BillingPlan() cloudBillingPlan {
	return c.billingPlan
}

func (c *cloudSettingsController) SetBillingPlan(plan cloudBillingPlan) {
	c.deps.OnUI("set cloud billing plan", func() {
		c.billingPlan = plan
	})
}

func (c *cloudSettingsController) BillingLoaded() bool {
	return c.billingLoaded
}

func (c *cloudSettingsController) SetBillingLoaded(loaded bool) {
	c.deps.OnUI("set cloud billing loaded", func() {
		c.billingLoaded = loaded
	})
}

func (c *cloudSettingsController) Devices() cloudDeviceList {
	return cloneCloudDeviceList(c.devices)
}

func (c *cloudSettingsController) SetDevices(devices cloudDeviceList) {
	c.deps.OnUI("set cloud devices", func() {
		c.devices = devices
	})
}

func (c *cloudSettingsController) Loading() bool {
	return c.loading
}

func (c *cloudSettingsController) SetLoading(loading bool) {
	c.deps.OnUI("set cloud loading", func() {
		c.loading = loading
	})
}

func (c *cloudSettingsController) Loaded() bool {
	return c.loaded
}

func (c *cloudSettingsController) SetLoaded(loaded bool) {
	c.deps.OnUI("set cloud loaded", func() {
		c.loaded = loaded
	})
}

func (c *cloudSettingsController) Busy() string {
	return c.busy
}

func (c *cloudSettingsController) SetBusy(busy string) {
	c.deps.OnUI("set cloud busy", func() {
		c.busy = busy
	})
}

func (c *cloudSettingsController) Error() string {
	return c.errMsg
}

func (c *cloudSettingsController) SetError(msg string) {
	c.deps.OnUI("set cloud error", func() {
		c.errMsg = msg
	})
}

// Revision returns the current reload revision. Used by tests to verify the
// revision guard discards stale responses.
func (c *cloudSettingsController) Revision() uint64 {
	return c.revision
}

// Form returns the live cloud form pointer for UI-thread mutation.
func (c *cloudSettingsController) Form() *cloudFormState {
	return c.form
}

func (c *cloudSettingsController) SetForm(form *cloudFormState) {
	c.deps.OnUI("set cloud form", func() {
		c.form = form
	})
}

func (c *cloudSettingsController) ActionMenu() string {
	return c.actionMenu
}

func (c *cloudSettingsController) SetActionMenu(menu string) {
	c.deps.OnUI("set cloud action menu", func() {
		c.actionMenu = menu
	})
}

// PluginDialog returns the transient add-exclusion dialog state.
func (c *cloudSettingsController) PluginDialog() *cloudPluginExclusionDialogState {
	if c.pluginDialog == nil {
		return nil
	}
	copy := *c.pluginDialog
	return &copy
}

// SetPluginDialog updates the transient add-exclusion dialog state on the UI thread.
func (c *cloudSettingsController) SetPluginDialog(state *cloudPluginExclusionDialogState) {
	c.deps.OnUI("set cloud plugin exclusion dialog", func() {
		if state == nil {
			c.pluginDialog = nil
			return
		}
		copy := *state
		c.pluginDialog = &copy
	})
}

func (c *cloudSettingsController) Plugins() []pluginSettingsPlugin {
	return append([]pluginSettingsPlugin(nil), c.plugins...)
}

func (c *cloudSettingsController) SetPlugins(plugins []pluginSettingsPlugin) {
	c.deps.OnUI("set cloud plugins", func() {
		c.plugins = append([]pluginSettingsPlugin(nil), plugins...)
	})
}

// ReloadCloudSync refreshes account, sync, devices, and plugins as one revisioned
// settings snapshot. onNeedBilling is called (outside the controller lock) when the
// billing plan has not been loaded yet, so the App can kick off a billing reload
// without coupling the controller to the billing reload path. Responses from
// superseded refreshes are discarded via the revision guard.
func (c *cloudSettingsController) ReloadCloudSync(ctx context.Context, service cloudReloadServices, sessionID string, onNeedBilling func(), showLoading bool) {
	var revision uint64
	if !c.deps.OnUI("start loading cloud sync", func() {
		c.revision++
		revision = c.revision
		needBilling := !c.billingLoaded
		c.loading = showLoading
		c.errMsg = ""
		c.deps.Invalidate()
		if needBilling && onNeedBilling != nil {
			onNeedBilling()
		}
	}) {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var accountStatus cloudAccountStatus
	var syncStatus cloudSyncStatus
	var deviceList cloudDeviceList
	var plugins []pluginSettingsPlugin
	loadedAccount, accountErr := service.AccountStatus(timeoutCtx, sessionID)
	if accountErr == nil {
		accountStatus = cloudAccountStatusFromContract(loadedAccount)
	}
	var statusErr error
	var devicesErr error
	var pluginsErr error
	if accountErr == nil {
		loadedStatus, err := service.CloudSyncStatus(timeoutCtx, sessionID)
		statusErr = err
		if err == nil {
			syncStatus = cloudSyncStatusFromContract(loadedStatus)
		}
		if accountStatus.LoggedIn {
			loadedDevices, err := service.CloudDevices(timeoutCtx, sessionID)
			devicesErr = err
			if err == nil {
				deviceList = cloudDeviceListFromContract(loadedDevices)
			}
			loadedPlugins, err := service.Plugins(timeoutCtx, sessionID, contract.PluginCatalogInstalled)
			pluginsErr = err
			if err == nil {
				plugins, pluginsErr = pluginSettingsPluginsFromContract(loadedPlugins)
			}
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
	c.deps.OnUI("apply cloud sync", func() {
		if revision != c.revision {
			return
		}
		c.loading = false
		if accountErr == nil {
			c.account = accountStatus
		}
		if statusErr == nil {
			c.sync = syncStatus
		}
		if !accountStatus.LoggedIn {
			c.devices = cloudDeviceList{}
			c.plugins = nil
		} else if devicesErr == nil {
			c.devices = deviceList
		}
		if accountStatus.LoggedIn && pluginsErr == nil {
			c.plugins = plugins
		}
		c.loaded = accountErr == nil && statusErr == nil
		c.errMsg = strings.Join(errors, " · ")
		c.deps.Invalidate()
	})
}

// ReloadBillingPlan fetches display pricing independently so it cannot delay local
// sync status. BillingLoaded becomes true after the attempt so the comparison
// table can leave the loading placeholder even when the server has no price.
func (c *cloudSettingsController) ReloadBillingPlan(ctx context.Context, service contract.CloudSettingsServices, sessionID string) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	loaded, err := service.BillingPlan(timeoutCtx, sessionID)
	plan := cloudBillingPlanFromContract(loaded)
	c.deps.OnUI("apply cloud billing plan", func() {
		c.billingLoaded = true
		if err == nil {
			c.billingPlan = plan
		}
		c.deps.Invalidate()
	})
	return err
}

// cloudAccountStatusFromContract adapts core account state to launcher-owned view state.
func cloudAccountStatusFromContract(status account.Status) cloudAccountStatus {
	return cloudAccountStatus{
		LoggedIn: status.LoggedIn, Email: status.Email, SyncEligible: status.SyncEligible, Plan: status.Plan,
		SyncLimits: cloudSyncLimits{DeviceLimit: status.SyncLimits.DeviceLimit}, DeviceCount: status.DeviceCount,
		SyncEnabled: status.SyncEnabled, SessionExpired: status.SessionExpired,
	}
}

// cloudSyncStatusFromContract deep-copies optional sync state and progress.
func cloudSyncStatusFromContract(status cloudsync.ServiceStatus) cloudSyncStatus {
	result := cloudSyncStatus{
		Enabled: status.Enabled, DeviceID: status.DeviceID,
		KeyStatus: cloudSyncKeyStatus{Available: status.KeyStatus.Available, Version: status.KeyStatus.Version},
	}
	if status.State != nil {
		result.State = &cloudSyncState{
			Cursor: status.State.Cursor, LastPullTS: status.State.LastPullTs, LastPushTS: status.State.LastPushTs,
			BackoffUntil: status.State.BackoffUntil, RetryCount: status.State.RetryCount, LastError: status.State.LastError, Bootstrapped: status.State.Bootstrapped,
		}
	}
	if status.Progress != nil {
		result.Progress = &cloudSyncProgress{
			Active: status.Progress.Active, Operation: status.Progress.Operation, EntityType: status.Progress.EntityType,
			PluginID: status.Progress.PluginID, Key: status.Progress.Key, Current: status.Progress.Current, Total: status.Progress.Total,
		}
	}
	return result
}

// cloudDeviceListFromContract isolates launcher device state from core response slices.
func cloudDeviceListFromContract(source cloudsync.CloudSyncDeviceListResponse) cloudDeviceList {
	result := cloudDeviceList{
		Devices: make([]cloudDevice, len(source.Devices)), CurrentDeviceID: source.CurrentDeviceID,
		DeviceLimit: source.DeviceLimit, DeviceCount: source.DeviceCount,
	}
	for index, device := range source.Devices {
		result.Devices[index] = cloudDevice{
			DeviceID: device.DeviceID, DeviceName: device.DeviceName, Platform: device.Platform,
			CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt, LastSeenAt: device.LastSeenAt,
			RevokedAt: device.RevokedAt, Current: device.Current,
		}
	}
	return result
}

// cloudBillingPlanFromContract adapts display pricing to launcher-owned state.
func cloudBillingPlanFromContract(plan account.BillingPlan) cloudBillingPlan {
	return cloudBillingPlan{
		Free: cloudBillingPlanTier{Price: cloudBillingPlanPrice{
			Currency: plan.Free.Price.Currency, UnitAmount: plan.Free.Price.UnitAmount, Interval: plan.Free.Price.Interval, Formatted: plan.Free.Price.Formatted,
		}},
		Pro: cloudBillingPlanTier{Price: cloudBillingPlanPrice{
			Currency: plan.Pro.Price.Currency, UnitAmount: plan.Pro.Price.UnitAmount, Interval: plan.Pro.Price.Interval, Formatted: plan.Pro.Price.Formatted,
		}},
	}
}

// Snapshot returns a copy of the Cloud state for the view layer. Form is deep-copied
// via snapshotCloudFormLocked so the snapshot is safe to read outside the lock;
// all other fields are value types or copied slices.
func (c *cloudSettingsController) Snapshot() cloudSettingsSnapshot {
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
		PluginDialog: func() *cloudPluginExclusionDialogSnapshot {
			if c.pluginDialog == nil {
				return nil
			}
			return &cloudPluginExclusionDialogSnapshot{Selected: c.pluginDialog.selected, ChoiceOpen: c.pluginDialog.choiceOpen, ChoiceAnchor: c.pluginDialog.choiceAnchor}
		}(),
		Plugins: append([]pluginSettingsPlugin(nil), c.plugins...),
	}
}
