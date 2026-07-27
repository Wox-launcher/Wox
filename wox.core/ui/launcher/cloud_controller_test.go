package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// cloudFakeBackend serves the cloud routes from pre-populated payloads, with
// optional per-route errors. pathSel/release/entered enable blocking behavior
// for the revision-guard test.
type cloudFakeBackend struct {
	mu sync.Mutex

	account cloudAccountStatus
	sync    cloudSyncStatus
	devices cloudDeviceList
	plugins []pluginSettingsPlugin
	plan    cloudBillingPlan

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

func (f *cloudFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
	if path == f.pathSel && f.entered != nil && f.release != nil {
		close(f.entered)
		<-f.release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch path {
	case "/account/status":
		if f.accountErr != nil {
			return f.accountErr
		}
		if ptr, ok := out.(*cloudAccountStatus); ok {
			*ptr = f.account
		}
	case "/sync/status":
		if f.syncErr != nil {
			return f.syncErr
		}
		if ptr, ok := out.(*cloudSyncStatus); ok {
			*ptr = f.sync
		}
	case "/sync/devices/list":
		if f.devicesErr != nil {
			return f.devicesErr
		}
		if ptr, ok := out.(*cloudDeviceList); ok {
			*ptr = f.devices
		}
	case "/plugin/installed":
		if f.pluginsErr != nil {
			return f.pluginsErr
		}
		if ptr, ok := out.(*[]pluginSettingsPlugin); ok {
			*ptr = append([]pluginSettingsPlugin(nil), f.plugins...)
		}
	case "/account/billing/plan":
		if f.planErr != nil {
			return f.planErr
		}
		if ptr, ok := out.(*cloudBillingPlan); ok {
			*ptr = f.plan
		}
	}
	return nil
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
	client := &cloudFakeBackend{
		account: cloudAccountStatus{LoggedIn: true, Email: "u@x.com", SyncEnabled: true},
		sync:    cloudSyncStatus{Enabled: true},
		devices: cloudDeviceList{Devices: []cloudDevice{{DeviceID: "d1"}}},
		plugins: []pluginSettingsPlugin{{ID: "p1", Name: "Plugin One"}},
	}
	c.ReloadCloudSync(context.Background(), client, nil)
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
	client := &cloudFakeBackend{accountErr: errors.New("network down")}
	c.ReloadCloudSync(context.Background(), client, nil)
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

// TestCloudControllerRevisionGuard verifies that a response from a superseded
// reload is discarded. The first reload blocks on /account/status; while it is
// blocked, a second reload increments the revision. When the first reload is
// released, its response must be ignored because revision no longer matches.
func TestCloudControllerRevisionGuard(t *testing.T) {
	deps, _ := newCloudControllerDeps()
	c := newCloudSettingsController(deps)

	enteredAccount := make(chan struct{})
	releaseAccount := make(chan struct{})
	blockingClient := &cloudFakeBackend{
		account: cloudAccountStatus{LoggedIn: true, Email: "stale@x.com"},
		sync:    cloudSyncStatus{Enabled: true},
		entered: enteredAccount,
		release: releaseAccount,
		pathSel: "/account/status",
	}

	// First reload blocks inside Post("/account/status").
	go c.ReloadCloudSync(context.Background(), blockingClient, nil)

	// Wait until the first reload has entered Post and is blocked.
	select {
	case <-enteredAccount:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first reload to enter Post")
	}

	// Second reload runs on a non-blocking client and completes immediately,
	// bumping the revision past the first reload's.
	secondClient := &cloudFakeBackend{
		account: cloudAccountStatus{LoggedIn: false, Email: "fresh@x.com"},
		sync:    cloudSyncStatus{Enabled: false},
	}
	c.ReloadCloudSync(context.Background(), secondClient, nil)

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
