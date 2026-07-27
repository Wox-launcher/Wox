package launcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// dataSettingsSnapshot is the immutable Data tab state consumed by the view layer.
type dataSettingsSnapshot struct {
	Backups         []backupInfo
	Location        string
	Loading         bool
	Loaded          bool
	Busy            string
	Error           string
	RestoreArmed    string
	PendingLocation string
	ClearLogsArmed  bool
}

// dataSettingsController owns the Data tab state (backups, location, restore, logs).
// Cross-domain needs (writing the shared settings note, reloading all settings after
// a restore, and picking a native directory) are injected via BindCrossDomain so the
// controller never depends on *App directly.
type dataSettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

	backups         []backupInfo
	location        string
	loading         bool
	loaded          bool
	busy            string
	errMsg          string
	restoreArmed    string
	pendingLocation string
	clearLogsArmed  bool

	// Cross-domain callbacks wired by App after construction. They touch App-owned
	// state (a.settingNote, a.settings, native windows), so the controller must NOT
	// hold c.mu while invoking them.
	setNote        func(string)
	reloadSettings func() error
	pickDirectory  func() (string, error)
}

func newDataSettingsController(deps CommonDeps) *dataSettingsController {
	return &dataSettingsController{deps: deps}
}

// BindCrossDomain wires App-owned helpers used by data operations. Called by newApp
// after both the controller and App are constructed.
func (c *dataSettingsController) BindCrossDomain(setNote func(string), reloadSettings func() error, pickDirectory func() (string, error)) {
	c.setNote = setNote
	c.reloadSettings = reloadSettings
	c.pickDirectory = pickDirectory
}

// Reload fetches the storage location and backup catalog. It is a no-op if a reload
// is already in flight and aggregates location/backups errors into a single message.
func (c *dataSettingsController) Reload(ctx context.Context, client backendClient) {
	c.mu.Lock()
	if c.loading {
		c.mu.Unlock()
		return
	}
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var location string
	var backups []backupInfo
	locationErr := client.Post(timeoutCtx, "/setting/userdata/location", map[string]any{}, &location)
	backupsErr := client.Post(timeoutCtx, "/backup/all", map[string]any{}, &backups)
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].Timestamp > backups[j].Timestamp })

	errorText := ""
	if locationErr != nil {
		errorText = "load data location: " + locationErr.Error()
	}
	if backupsErr != nil {
		if errorText != "" {
			errorText += " · "
		}
		errorText += "load backups: " + backupsErr.Error()
	}

	c.mu.Lock()
	c.loading = false
	c.loaded = errorText == ""
	if locationErr == nil {
		c.location = location
	}
	if backupsErr == nil {
		c.backups = backups
	}
	c.errMsg = errorText
	c.mu.Unlock()
	c.deps.Invalidate()
}

// CreateBackup starts a manual backup. While the async Post is in flight Busy is set
// to "backup"; on success the shared note is updated and the catalog is refreshed.
func (c *dataSettingsController) CreateBackup(ctx context.Context, client backendClient) {
	c.mu.Lock()
	if c.busy != "" {
		c.mu.Unlock()
		return
	}
	c.busy = "backup"
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		err := client.Post(timeoutCtx, "/backup/now", map[string]any{}, nil)
		cancel()

		c.mu.Lock()
		c.busy = ""
		if err != nil {
			c.errMsg = "Could not create backup: " + err.Error()
		}
		c.mu.Unlock()

		if err == nil {
			if c.setNote != nil {
				c.setNote("Manual backup created")
			}
			c.deps.Invalidate()
			c.Reload(ctx, client)
		} else {
			c.deps.Invalidate()
		}
	}()
}

// RestoreBackup requires two explicit activations before core replaces current
// settings. The first call arms confirmation for the given backup id; the second
// fires the async restore and reloads all settings on success.
func (c *dataSettingsController) RestoreBackup(ctx context.Context, client backendClient, id string) {
	c.mu.Lock()
	if c.busy != "" || strings.TrimSpace(id) == "" {
		c.mu.Unlock()
		return
	}
	if c.restoreArmed != id {
		c.restoreArmed = id
		c.mu.Unlock()
		if c.setNote != nil {
			c.setNote("Press Confirm restore to replace current settings with this backup.")
		}
		c.deps.Invalidate()
		return
	}
	c.restoreArmed = ""
	c.busy = "restore"
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		err := client.Post(timeoutCtx, "/backup/restore", map[string]string{"id": id}, nil)
		cancel()
		if err == nil && c.reloadSettings != nil {
			err = c.reloadSettings()
		}

		c.mu.Lock()
		c.busy = ""
		if err != nil {
			c.errMsg = "Could not restore backup: " + err.Error()
		}
		c.mu.Unlock()

		if err == nil {
			if c.setNote != nil {
				c.setNote("Backup restored")
			}
		}
		c.deps.Invalidate()
	}()
}

// ChooseLocation opens the native directory picker and stages the selected path
// for explicit confirmation. The actual move happens in ConfirmLocationChange.
func (c *dataSettingsController) ChooseLocation() {
	if c.pickDirectory == nil {
		return
	}
	path, err := c.pickDirectory()
	// Decide whether to set the confirmation note under the lock, but defer the
	// setNote call itself until after unlock so we never hold c.mu while touching
	// App-owned state (a.settingNote).
	var note string
	c.mu.Lock()
	if err != nil {
		c.errMsg = "Could not select data directory: " + err.Error()
	} else if strings.TrimSpace(path) != "" && path != c.location {
		c.pendingLocation = path
		note = "Confirm the new data directory before Wox moves its files."
	}
	c.mu.Unlock()
	if note != "" && c.setNote != nil {
		c.setNote(note)
	}
	c.deps.Invalidate()
}

// CancelLocationChange clears any staged directory and the shared note.
func (c *dataSettingsController) CancelLocationChange() {
	c.mu.Lock()
	c.pendingLocation = ""
	c.mu.Unlock()
	if c.setNote != nil {
		c.setNote("")
	}
	c.deps.Invalidate()
}

// ConfirmLocationChange delegates the actual data migration to core after the
// visible confirmation step. On failure the staged path is restored so the user
// can retry without re-picking the directory.
func (c *dataSettingsController) ConfirmLocationChange(ctx context.Context, client backendClient) {
	c.mu.Lock()
	location := c.pendingLocation
	if c.busy != "" || strings.TrimSpace(location) == "" {
		c.mu.Unlock()
		return
	}
	c.pendingLocation = ""
	c.busy = "location"
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err := client.Post(timeoutCtx, "/setting/userdata/location/update", map[string]string{"location": location}, nil)
		cancel()

		c.mu.Lock()
		c.busy = ""
		if err != nil {
			c.pendingLocation = location
			c.errMsg = "Could not move data directory: " + err.Error()
		} else {
			c.location = location
		}
		c.mu.Unlock()

		if err == nil {
			if c.setNote != nil {
				c.setNote("Data directory updated")
			}
		}
		c.deps.Invalidate()
	}()
}

// ClearLogs uses the same two-step confirmation as backup restore to avoid
// accidental data loss.
func (c *dataSettingsController) ClearLogs(ctx context.Context, client backendClient) {
	c.mu.Lock()
	if c.busy != "" {
		c.mu.Unlock()
		return
	}
	if !c.clearLogsArmed {
		c.clearLogsArmed = true
		c.mu.Unlock()
		if c.setNote != nil {
			c.setNote("Press Confirm clear to delete historical logs.")
		}
		c.deps.Invalidate()
		return
	}
	c.clearLogsArmed = false
	c.busy = "logs"
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := client.Post(timeoutCtx, "/log/clear", map[string]any{}, nil)
		cancel()

		c.mu.Lock()
		c.busy = ""
		if err != nil {
			c.errMsg = "Could not clear logs: " + err.Error()
		}
		c.mu.Unlock()

		if err == nil {
			if c.setNote != nil {
				c.setNote("Logs cleared")
			}
		}
		c.deps.Invalidate()
	}()
}

// OpenPath delegates platform shell behavior to core's existing cross-platform route.
func (c *dataSettingsController) OpenPath(ctx context.Context, client backendClient, path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := client.Post(timeoutCtx, "/open", map[string]string{"path": path}, nil)
		cancel()
		if err != nil {
			c.mu.Lock()
			c.errMsg = "Could not open path: " + err.Error()
			c.mu.Unlock()
			c.deps.Invalidate()
		}
	}()
}

// OpenBackupFolder resolves the configured folder in core before asking the desktop
// to open it.
func (c *dataSettingsController) OpenBackupFolder(ctx context.Context, client backendClient) {
	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		var path string
		err := client.Post(timeoutCtx, "/backup/folder", map[string]any{}, &path)
		cancel()
		if err != nil {
			c.mu.Lock()
			c.errMsg = "Could not open backup folder: " + err.Error()
			c.mu.Unlock()
			c.deps.Invalidate()
			return
		}
		c.OpenPath(ctx, client, path)
	}()
}

// OpenLog lets core create and reveal the current log file with its platform shell adapter.
func (c *dataSettingsController) OpenLog(ctx context.Context, client backendClient) {
	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := client.Post(timeoutCtx, "/log/open", map[string]any{}, nil)
		cancel()
		if err != nil {
			c.mu.Lock()
			c.errMsg = "Could not open log: " + err.Error()
			c.mu.Unlock()
			c.deps.Invalidate()
		}
	}()
}

// Snapshot returns a copy of the Data state for the view layer.
func (c *dataSettingsController) Snapshot() dataSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return dataSettingsSnapshot{
		Backups:         append([]backupInfo(nil), c.backups...),
		Location:        c.location,
		Loading:         c.loading,
		Loaded:          c.loaded,
		Busy:            c.busy,
		Error:           c.errMsg,
		RestoreArmed:    c.restoreArmed,
		PendingLocation: c.pendingLocation,
		ClearLogsArmed:  c.clearLogsArmed,
	}
}
