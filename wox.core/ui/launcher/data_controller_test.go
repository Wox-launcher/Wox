package launcher

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// dataFakeBackend is a path-aware fake that can serve multiple routes from
// pre-populated values, return per-path errors, or block on a path for
// handshake-driven tests. It is separate from the simple fakeBackendClient so
// existing tests keep their terse construction.
type dataFakeBackend struct {
	mu sync.Mutex

	location string
	backups  []backupInfo

	locationErr error
	backupsErr  error

	// Per-path optional error, overrides the typed errors above.
	pathErrors map[string]error

	// Optional blocking handshake: when pathSel is set, Post closes entered and
	// waits on release before returning. Used to assert mid-flight state.
	entered chan<- struct{}
	release <-chan struct{}
	pathSel string
}

func (f *dataFakeBackend) Post(_ context.Context, path string, _ any, out any) error {
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

	switch path {
	case "/setting/userdata/location":
		if f.locationErr != nil {
			return f.locationErr
		}
		if ptr, ok := out.(*string); ok {
			*ptr = f.location
		}
	case "/backup/all":
		if f.backupsErr != nil {
			return f.backupsErr
		}
		if ptr, ok := out.(*[]backupInfo); ok {
			*ptr = append([]backupInfo(nil), f.backups...)
		}
	default:
		// Other paths (e.g. /backup/now, /log/clear) succeed by default.
	}
	return nil
}

func newDataController(deps CommonDeps) *dataSettingsController {
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func(string) {},
		func() error { return nil },
		func() (string, error) { return "", nil },
	)
	return c
}

func TestDataControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newDataController(deps)
	client := &dataFakeBackend{
		location: "/data/dir",
		backups: []backupInfo{
			{ID: "b1", Timestamp: 100, Type: "manual"},
			{ID: "b2", Timestamp: 200, Type: "auto"},
		},
	}
	c.Reload(context.Background(), client)
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
	client := &dataFakeBackend{
		locationErr: errors.New("location down"),
		backupsErr:  errors.New("backups down"),
	}
	c.Reload(context.Background(), client)
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
	var noteMu sync.Mutex
	var lastNote string
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func(note string) { noteMu.Lock(); lastNote = note; noteMu.Unlock() },
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingClient := &dataFakeBackend{
		pathSel: "/backup/now",
		entered: entered,
		release: release,
	}

	c.CreateBackup(context.Background(), blockingClient)

	// Busy must be set immediately after CreateBackup returns.
	if got := c.Snapshot().Busy; got != "backup" {
		t.Fatalf("Busy = %q during backup, want \"backup\"", got)
	}

	// Wait for the goroutine to enter Post; then release it.
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for backup Post to be entered")
	}
	close(release)

	// Wait for Busy to clear and the note to be set.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c.Snapshot().Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := c.Snapshot().Busy; got != "" {
		t.Fatalf("Busy = %q after goroutine completed, want empty", got)
	}

	noteMu.Lock()
	got := lastNote
	noteMu.Unlock()
	if got != "Manual backup created" {
		t.Fatalf("setNote = %q, want \"Manual backup created\"", got)
	}
}

func TestDataControllerRestoreBackupTwoStepArming(t *testing.T) {
	var noteMu sync.Mutex
	var notes []string
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func(note string) { noteMu.Lock(); notes = append(notes, note); noteMu.Unlock() },
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingClient := &dataFakeBackend{
		pathSel: "/backup/restore",
		entered: entered,
		release: release,
	}

	// First activation: arms confirmation, no Post.
	c.RestoreBackup(context.Background(), blockingClient, "backup-1")
	snap := c.Snapshot()
	if snap.RestoreArmed != "backup-1" {
		t.Fatalf("RestoreArmed = %q after first activation, want \"backup-1\"", snap.RestoreArmed)
	}
	if snap.Busy != "" {
		t.Fatalf("Busy = %q after first activation, want empty (no Post yet)", snap.Busy)
	}

	// Second activation: clears RestoreArmed, sets Busy, fires Post.
	c.RestoreBackup(context.Background(), blockingClient, "backup-1")
	snap = c.Snapshot()
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
		if c.Snapshot().Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := c.Snapshot().Busy; got != "" {
		t.Fatalf("Busy = %q after restore completed, want empty", got)
	}

	noteMu.Lock()
	defer noteMu.Unlock()
	// Expect at least the arming note and the success note.
	wantArming := false
	wantSuccess := false
	for _, n := range notes {
		if strings.Contains(n, "Press Confirm restore") {
			wantArming = true
		}
		if n == "Backup restored" {
			wantSuccess = true
		}
	}
	if !wantArming {
		t.Fatalf("arming note not set, notes = %+v", notes)
	}
	if !wantSuccess {
		t.Fatalf("success note not set, notes = %+v", notes)
	}
}

func TestDataControllerClearLogsTwoStepArming(t *testing.T) {
	var noteMu sync.Mutex
	var notes []string
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newDataSettingsController(deps)
	c.BindCrossDomain(
		func(note string) { noteMu.Lock(); notes = append(notes, note); noteMu.Unlock() },
		func() error { return nil },
		func() (string, error) { return "", nil },
	)

	entered := make(chan struct{})
	release := make(chan struct{})
	blockingClient := &dataFakeBackend{
		pathSel: "/log/clear",
		entered: entered,
		release: release,
	}

	// First activation: arms confirmation, no Post.
	c.ClearLogs(context.Background(), blockingClient)
	snap := c.Snapshot()
	if !snap.ClearLogsArmed {
		t.Fatalf("ClearLogsArmed should be true after first activation")
	}
	if snap.Busy != "" {
		t.Fatalf("Busy = %q after first activation, want empty (no Post yet)", snap.Busy)
	}

	// Second activation: clears armed flag, sets Busy, fires Post.
	c.ClearLogs(context.Background(), blockingClient)
	snap = c.Snapshot()
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
		if c.Snapshot().Busy == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := c.Snapshot().Busy; got != "" {
		t.Fatalf("Busy = %q after clear completed, want empty", got)
	}

	noteMu.Lock()
	defer noteMu.Unlock()
	wantArming := false
	wantSuccess := false
	for _, n := range notes {
		if strings.Contains(n, "Press Confirm clear") {
			wantArming = true
		}
		if n == "Logs cleared" {
			wantSuccess = true
		}
	}
	if !wantArming {
		t.Fatalf("arming note not set, notes = %+v", notes)
	}
	if !wantSuccess {
		t.Fatalf("success note not set, notes = %+v", notes)
	}
}
