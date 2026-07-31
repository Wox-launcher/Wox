package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"wox/account"
	"wox/cloudsync"
	"wox/ui/contract"
)

// cloudFakeService serves typed cloud operations with optional blocking.
type cloudFakeService struct {
	mu sync.Mutex

	account account.Status
	sync    cloudsync.ServiceStatus
	devices cloudsync.CloudSyncDeviceListResponse
	plugins []contract.PluginCatalogItem
	plan    account.BillingPlan

	accountErr error
	syncErr    error
	devicesErr error
	pluginsErr error
	planErr    error

	// Blocking: when pathSel matches, entered is closed and the call blocks on release.
	entered chan<- struct{}
	release <-chan struct{}
	pathSel string
}

func (f *cloudFakeService) operation(path string) {
	if path == f.pathSel && f.entered != nil && f.release != nil {
		close(f.entered)
		<-f.release
	}
}

func (f *cloudFakeService) AccountStatus(_ context.Context, _ string) (account.Status, error) {
	f.operation("/account/status")
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.account, f.accountErr
}

func (f *cloudFakeService) CloudSyncStatus(_ context.Context, _ string) (cloudsync.ServiceStatus, error) {
	f.operation("/sync/status")
	return f.sync, f.syncErr
}

func (f *cloudFakeService) CloudDevices(_ context.Context, _ string) (cloudsync.CloudSyncDeviceListResponse, error) {
	f.operation("/sync/devices/list")
	return f.devices, f.devicesErr
}

func (f *cloudFakeService) BillingPlan(_ context.Context, _ string) (account.BillingPlan, error) {
	f.operation("/account/billing/plan")
	return f.plan, f.planErr
}

func (f *cloudFakeService) Plugins(_ context.Context, _ string, _ contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	f.operation("/plugin/installed")
	return append([]contract.PluginCatalogItem(nil), f.plugins...), f.pluginsErr
}

func newCloudControllerDeps() (CommonDeps, *int) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	return deps, &invalidateCalled
}

func TestCloudControllerAccount(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	c.SetAccount(cloudAccountStatus{LoggedIn: true, Email: "a@b.com", Plan: "pro"})
	got := c.Account()
	if !got.LoggedIn || got.Email != "a@b.com" || got.Plan != "pro" {
		t.Fatalf("Account = %+v, want LoggedIn/Email/Plan", got)
	}
}

func TestCloudControllerDevices(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	c.SetDevices(cloudDeviceList{Devices: []cloudDevice{{DeviceID: "d1", DeviceName: "Mac"}}, CurrentDeviceID: "d1"})
	got := c.Devices()
	if len(got.Devices) != 1 || got.Devices[0].DeviceID != "d1" || got.CurrentDeviceID != "d1" {
		t.Fatalf("Devices = %+v, want [d1] with CurrentDeviceID=d1", got)
	}
	// Verify Devices() returns a copy so the controller state is not aliased.
	got.Devices[0].DeviceID = "mutated"
	if c.Devices().Devices[0].DeviceID != "d1" {
		t.Fatalf("Devices() should return a copy, but mutation leaked into controller state")
	}
}

func TestCloudControllerForm(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	form := &cloudFormState{kind: "login", title: "Login"}
	c.SetForm(form)
	if got := c.Form(); got != form {
		t.Fatalf("Form mismatch: got %v, want %v", got, form)
	}
}

func TestCloudControllerReloadSuccess(t *testing.T) {
	deps, invalidateCalled := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	service := &cloudFakeService{
		account: account.Status{LoggedIn: true, Email: "u@x.com", SyncEnabled: true},
		sync:    cloudsync.ServiceStatus{Enabled: true},
		devices: cloudsync.CloudSyncDeviceListResponse{Devices: []cloudsync.CloudSyncDevice{{DeviceID: "d1"}}},
		plugins: []contract.PluginCatalogItem{{ID: "p1", Name: "Plugin One"}},
	}
	c.ReloadCloudSync(context.Background(), service, "session", nil, true)
	snap := c.Snapshot()
	if !c.Loaded() {
		t.Fatalf("Loaded should be true after successful reload")
	}
	if snap.Error != "" {
		t.Fatalf("Error should be empty, got %q", snap.Error)
	}
	if snap.Loading {
		t.Fatalf("Loading should be false after reload completes")
	}
	if !snap.Account.LoggedIn || snap.Account.Email != "u@x.com" {
		t.Fatalf("Account = %+v, want LoggedIn/Email", snap.Account)
	}
	if !snap.Sync.Enabled {
		t.Fatalf("Sync should be enabled")
	}
	if len(snap.Devices.Devices) != 1 {
		t.Fatalf("Devices len = %d, want 1", len(snap.Devices.Devices))
	}
	if len(snap.Plugins) != 1 || snap.Plugins[0].ID != "p1" {
		t.Fatalf("Plugins = %+v, want [p1]", snap.Plugins)
	}
	if *invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", *invalidateCalled)
	}
}

func TestCloudControllerReloadError(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	service := &cloudFakeService{accountErr: errors.New("network down")}
	c.ReloadCloudSync(context.Background(), service, "session", nil, true)
	snap := c.Snapshot()
	if c.Loaded() {
		t.Fatalf("Loaded should be false after error")
	}
	if snap.Error == "" {
		t.Fatalf("Error should be recorded, got empty")
	}
	if snap.Loading {
		t.Fatalf("Loading should be false after error")
	}
}

func TestCloudControllerSilentReloadDoesNotShowLoading(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	service := &cloudFakeService{entered: entered, release: release, pathSel: "/account/status"}

	go func() {
		c.ReloadCloudSync(context.Background(), service, "session", nil, false)
		close(done)
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for silent reload")
	}
	if c.Snapshot().Loading {
		t.Fatal("silent reload should keep loading hidden")
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for silent reload to finish")
	}
}

// TestCloudControllerRevisionGuard verifies that a response from a superseded
// reload is discarded. The first reload blocks on /account/status; while it is
// blocked, a second reload increments the revision. When the first reload is
// released, its response must be ignored because revision no longer matches.
func TestCloudControllerRevisionGuard(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)

	enteredAccount := make(chan struct{})
	releaseAccount := make(chan struct{})
	blockingService := &cloudFakeService{
		account: account.Status{LoggedIn: true, Email: "stale@x.com"},
		sync:    cloudsync.ServiceStatus{Enabled: true},
		entered: enteredAccount,
		release: releaseAccount,
		pathSel: "/account/status",
	}

	// First reload blocks inside AccountStatus.
	go c.ReloadCloudSync(context.Background(), blockingService, "session", nil, true)

	// Wait until the first reload has entered Post and is blocked.
	select {
	case <-enteredAccount:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first reload to enter Post")
	}

	// Second reload runs on a non-blocking client and completes immediately,
	// bumping the revision past the first reload's.
	secondService := &cloudFakeService{
		account: account.Status{LoggedIn: false, Email: "fresh@x.com"},
		sync:    cloudsync.ServiceStatus{Enabled: false},
	}
	c.ReloadCloudSync(context.Background(), secondService, "session", nil, true)

	// After the second reload, Loaded must be true (both account and sync succeeded),
	// and the account must be the second client's (fresh), not the first's (stale).
	if !c.Loaded() {
		t.Fatalf("Loaded should be true after second reload")
	}
	if got := c.Account().Email; got != "fresh@x.com" {
		t.Fatalf("Account.Email = %q after second reload, want fresh@x.com (stale response leaked)", got)
	}
	if got := c.Sync().Enabled; got {
		t.Fatalf("Sync.Enabled = true after second reload, want false (stale response leaked)")
	}

	// Release the first reload's blocked Post. Its response must be discarded.
	close(releaseAccount)

	// Give the first reload's goroutine a moment to run its post-Post commit path.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Account().Email != "fresh@x.com" || c.Sync().Enabled {
			t.Fatalf("stale response from first reload overwrote fresh state: Account=%q Sync=%v", c.Account().Email, c.Sync().Enabled)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
