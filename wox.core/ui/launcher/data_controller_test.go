package launcher

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/ui/contract"
)

// dataFakeService can return or block individual typed data operations.
type dataFakeService struct {
	mu sync.Mutex

	location string
	backups  []backupInfo

	locationErr error
	backupsErr  error

	// Per-path optional error, overrides the typed errors above.
	pathErrors map[string]error

	// Optional blocking handshake keyed by the former route name for compact tests.
	entered chan<- struct{}
	release <-chan struct{}
	pathSel string
}

func (f *dataFakeService) operation(path string) error {
	if f.pathSel != "" && path == f.pathSel && f.entered != nil {
		close(f.entered)
		<-f.release
	}

	f.mu.Lock()
	if f.pathErrors != nil {
		if err, ok := f.pathErrors[path]; ok && err != nil {
			f.mu.Unlock()
			return err
		}
	}
	f.mu.Unlock()
	return nil
}

func (f *dataFakeService) DataLocation(_ context.Context, _ string) (string, error) {
	if err := f.operation("/setting/userdata/location"); err != nil {
		return "", err
	}
	return f.location, f.locationErr
}

func (f *dataFakeService) DataBackups(_ context.Context, _ string) ([]contract.DataBackup, error) {
	if err := f.operation("/backup/all"); err != nil {
		return nil, err
	}
	if f.backupsErr != nil {
		return nil, f.backupsErr
	}
	backups := make([]contract.DataBackup, len(f.backups))
	for index, backup := range f.backups {
		backups[index] = contract.DataBackup{ID: backup.ID, Name: backup.Name, Timestamp: backup.Timestamp, Type: backup.Type, Path: backup.Path}
	}
	return backups, nil
}

func (f *dataFakeService) CreateDataBackup(_ context.Context, _ string) error {
	return f.operation("/backup/now")
}

func (f *dataFakeService) RestoreDataBackup(_ context.Context, _ string, _ string) error {
	return f.operation("/backup/restore")
}

func (f *dataFakeService) ChangeDataLocation(_ context.Context, _ string, _ string) error {
	return f.operation("/setting/userdata/location/update")
}

func (f *dataFakeService) ClearLogs(_ context.Context, _ string) error {
	return f.operation("/log/clear")
}

func (f *dataFakeService) OpenPath(_ context.Context, _ string, _ string) error {
	return f.operation("/open")
}

func (f *dataFakeService) BackupFolder(_ context.Context, _ string) (string, error) {
	if err := f.operation("/backup/folder"); err != nil {
		return "", err
	}
	return "/backup", nil
}

func (f *dataFakeService) OpenLog(_ context.Context, _ string) error {
	return f.operation("/log/open")
}

func newDataController(deps CommonDeps) *dataSettingsController {
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func() error { return nil },
		func() (string, error) { return "", nil },
	)
	return c
}

func dataSnapshotOnUI(ui *testUIRunner, c *dataSettingsController) dataSettingsSnapshot {
	var snapshot dataSettingsSnapshot
	ui.Do(func() {
		snapshot = c.Snapshot()
	})
	return snapshot
}

func TestDataControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newDataController(deps)
	service := &dataFakeService{
		location: "/data/dir",
		backups: []backupInfo{
			{ID: "b1", Timestamp: 100, Type: "manual"},
			{ID: "b2", Timestamp: 200, Type: "auto"},
		},
	}
	c.Reload(context.Background(), service, "session")
	snap := c.Snapshot()

	if snap.Location != "/data/dir" {
		t.Fatalf("Location = %q, want /data/dir", snap.Location)
	}
	if len(snap.Backups) != 2 {
		t.Fatalf("Backups len = %d, want 2", len(snap.Backups))
	}
	// Backups should be sorted by timestamp descending.
	if snap.Backups[0].ID != "b2" || snap.Backups[1].ID != "b1" {
		t.Fatalf("Backups not sorted descending: %+v", snap.Backups)
	}
	if !snap.Loaded {
		t.Fatalf("Loaded should be true after successful reload")
	}
	if snap.Error != "" {
		t.Fatalf("Error should be empty, got %q", snap.Error)
	}
	if snap.Loading {
		t.Fatalf("Loading should be false after reload completes")
	}
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestDataControllerReloadAggregatesErrors(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newDataController(deps)
	service := &dataFakeService{
		locationErr: errors.New("location down"),
		backupsErr:  errors.New("backups down"),
	}
	c.Reload(context.Background(), service, "session")
	snap := c.Snapshot()

	if snap.Loaded {
		t.Fatalf("Loaded should be false when errors occurred")
	}
	if !strings.Contains(snap.Error, "load data location") {
		t.Fatalf("Error should mention location: %q", snap.Error)
	}
	if !strings.Contains(snap.Error, "load backups") {
		t.Fatalf("Error should mention backups: %q", snap.Error)
	}
	if !strings.Contains(snap.Error, "location down") || !strings.Contains(snap.Error, "backups down") {
		t.Fatalf("Error should include both underlying messages: %q", snap.Error)
	}
}

func TestDataControllerCreateBackupSetsBusyThenClears(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingService := &dataFakeService{
		pathSel: "/backup/now",
		entered: entered,
		release: release,
	}

	c.CreateBackup(context.Background(), blockingService, "session")

	// Busy must be set immediately after CreateBackup returns.
	if got := dataSnapshotOnUI(ui, c).Busy; got != "backup" {
		t.Fatalf("Busy = %q during backup, want \"backup\"", got)
	}

	// Wait for the goroutine to enter Post; then release it.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for backup Post to be entered")
	}
	close(release)

	// Wait for Busy to clear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if dataSnapshotOnUI(ui, c).Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := dataSnapshotOnUI(ui, c).Busy; got != "" {
		t.Fatalf("Busy = %q after goroutine completed, want empty", got)
	}

}

func TestDataControllerRestoreBackupTwoStepArming(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingService := &dataFakeService{
		pathSel: "/backup/restore",
		entered: entered,
		release: release,
	}

	// First activation: arms confirmation, no Post.
	c.RestoreBackup(context.Background(), blockingService, "session", "backup-1")
	snap := dataSnapshotOnUI(ui, c)
	if snap.RestoreArmed != "backup-1" {
		t.Fatalf("RestoreArmed = %q after first activation, want \"backup-1\"", snap.RestoreArmed)
	}
	if snap.Busy != "" {
		t.Fatalf("Busy = %q after first activation, want empty (no Post yet)", snap.Busy)
	}

	// Second activation: clears RestoreArmed, sets Busy, fires Post.
	c.RestoreBackup(context.Background(), blockingService, "session", "backup-1")
	snap = dataSnapshotOnUI(ui, c)
	if snap.RestoreArmed != "" {
		t.Fatalf("RestoreArmed = %q after second activation, want empty", snap.RestoreArmed)
	}
	if snap.Busy != "restore" {
		t.Fatalf("Busy = %q after second activation, want \"restore\"", snap.Busy)
	}

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for restore Post to be entered")
	}
	close(release)

	// Wait for Busy to clear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if dataSnapshotOnUI(ui, c).Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := dataSnapshotOnUI(ui, c).Busy; got != "" {
		t.Fatalf("Busy = %q after restore completed, want empty", got)
	}

}

func TestDataControllerClearLogsTwoStepArming(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingService := &dataFakeService{
		pathSel: "/log/clear",
		entered: entered,
		release: release,
	}

	// First activation: arms confirmation, no Post.
	c.ClearLogs(context.Background(), blockingService, "session")
	snap := dataSnapshotOnUI(ui, c)
	if !snap.ClearLogsArmed {
		t.Fatalf("ClearLogsArmed should be true after first activation")
	}
	if snap.Busy != "" {
		t.Fatalf("Busy = %q after first activation, want empty (no Post yet)", snap.Busy)
	}

	// Second activation: clears armed flag, sets Busy, fires Post.
	c.ClearLogs(context.Background(), blockingService, "session")
	snap = dataSnapshotOnUI(ui, c)
	if snap.ClearLogsArmed {
		t.Fatalf("ClearLogsArmed should be false after second activation")
	}
	if snap.Busy != "logs" {
		t.Fatalf("Busy = %q after second activation, want \"logs\"", snap.Busy)
	}

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for clear-logs Post to be entered")
	}
	close(release)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if dataSnapshotOnUI(ui, c).Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := dataSnapshotOnUI(ui, c).Busy; got != "" {
		t.Fatalf("Busy = %q after clear completed, want empty", got)
	}

}
