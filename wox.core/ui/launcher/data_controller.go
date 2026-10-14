package launcher

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
	"wox/util"
)

// dataSettingsSnapshot is the immutable Data tab state consumed by the view layer.
type dataSettingsSnapshot struct {
	Backups          []backupInfo
	Location         string
	Loading          bool
	Loaded           bool
	Busy             string
	BackupRunning    bool
	DeletingBackupID string
	Error            string
	ErrorSection     string
	PendingLocation  string
	ClearLogsArmed   bool
}

// dataSettingsController owns the Data tab state (backups, location, restore, logs).
// Cross-domain needs (reloading all settings after a restore and picking a native
// directory) are injected via BindCrossDomain so the controller never depends on App.
type dataSettingsController struct {
	deps CommonDeps

	backups          []backupInfo
	location         string
	loading          bool
	loaded           bool
	busy             string
	backupRunning    bool
	deletingBackupID string
	unwatchBackup    func()
	errMsg           string
	errSection       string
	pendingLocation  string
	clearLogsArmed   bool

	// Cross-domain callbacks wired by App after construction.
	reloadSettings func() error
	pickDirectory  func() (string, error)
}

// Error sections match the data settings view. An empty section is page-level.
const (
	dataErrorSectionStorage = "storage"
	dataErrorSectionBackup  = "backup"
	dataErrorSectionLogs    = "logs"
)

func newDataSettingsController(deps CommonDeps) *dataSettingsController {
	return &dataSettingsController{deps: deps}
}

// setError records a failure beside the section the user just used.
func (c *dataSettingsController) setError(message, section string) {
	c.errMsg = message
	c.errSection = section
}

func (c *dataSettingsController) clearError() {
	c.errMsg = ""
	c.errSection = ""
}

// BindCrossDomain wires App-owned helpers used by data operations. Called by newApp
// after both the controller and App are constructed.
func (c *dataSettingsController) BindCrossDomain(reloadSettings func() error, pickDirectory func() (string, error)) {
	c.reloadSettings = reloadSettings
	c.pickDirectory = pickDirectory
}

// Reload fetches the storage location and backup catalog. It is a no-op if a reload
// is already in flight and aggregates location/backups errors into a single message.
func (c *dataSettingsController) Reload(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	// Subscribe before the loading callback. The first watch event is synchronous, and
	// the test UI runner deadlocks if that callback re-enters OnUI.
	c.ensureBackupWatch(ctx, service, sessionID)
	shouldLoad := false
	if !c.deps.OnUI("start loading data settings", func() {
		if c.loading {
			return
		}
		c.loading = true
		c.clearError()
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var location string
	var backups []backupInfo
	location, locationErr := service.DataLocation(timeoutCtx, sessionID)
	loadedBackups, backupsErr := service.DataBackups(timeoutCtx, sessionID)
	backups = make([]backupInfo, len(loadedBackups))
	for index, backup := range loadedBackups {
		backups[index] = backupInfo{ID: backup.ID, Name: backup.Name, Timestamp: backup.Timestamp, Type: backup.Type, Path: backup.Path}
	}
	sort.SliceStable(backups, func(i, j int) bool { return backups[i].Timestamp > backups[j].Timestamp })

	errorText := ""
	errorSection := ""
	if locationErr != nil {
		errorText = "load data location: " + locationErr.Error()
		errorSection = dataErrorSectionStorage
	}
	if backupsErr != nil {
		if errorText != "" {
			errorText += " · "
			errorSection = ""
		} else {
			errorSection = dataErrorSectionBackup
		}
		errorText += "load backups: " + backupsErr.Error()
	}

	c.deps.OnUI("apply data settings", func() {
		c.loading = false
		c.loaded = errorText == ""
		if locationErr == nil {
			c.location = location
		}
		if backupsErr == nil {
			c.backups = backups
		}
		c.setError(errorText, errorSection)
		c.deps.Invalidate()
	})
}

// CreateBackup starts a manual backup. The button stays on "backup" until the shared
// run finishes, including when an automatic backup is the run that is already copying.
func (c *dataSettingsController) CreateBackup(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	c.ensureBackupWatch(ctx, service, sessionID)
	if c.backupRunning || c.busy != "" {
		return
	}
	c.busy = "backup"
	c.clearError()
	c.deps.Invalidate()

	util.Go(ctx, "create data backup", func() {
		// User data can be large. The copy does not observe this context, so this wait
		// covers the copy and the list refresh happens after Backup returns.
		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		err := service.CreateDataBackup(timeoutCtx, sessionID)
		cancel()

		c.deps.OnUI("apply data backup creation", func() {
			// Another run owns the button. This request did not start a copy.
			if errors.Is(err, contract.ErrBackupInProgress) {
				c.deps.Invalidate()
				return
			}
			// The watcher may already have cleared the optimistic busy when the run ended.
			if c.busy == "backup" && !c.backupRunning {
				c.busy = ""
			}
			if err != nil {
				c.setError("Could not create backup: "+err.Error(), dataErrorSectionBackup)
			}
			c.deps.Invalidate()
		})
		if err == nil {
			c.Reload(ctx, service, sessionID)
		}
	})
}

// ensureBackupWatch subscribes once to the shared backup run.
// Call it outside OnUI: the first callback is synchronous and applies state through OnUI.
func (c *dataSettingsController) ensureBackupWatch(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	if c.unwatchBackup != nil || service == nil {
		return
	}
	cancel := service.WatchDataBackup(ctx, sessionID, func(running bool, failure error) {
		c.onBackupChanged(ctx, service, sessionID, running, failure)
	})
	if cancel == nil {
		cancel = func() {}
	}
	c.unwatchBackup = cancel
}

// onBackupChanged mirrors the singleton onto the Backup Now button.
// The initial snapshot is not a finished run, so an optimistic click stays busy until that run ends.
// A successful finish reloads the list after OnUI returns.
func (c *dataSettingsController) onBackupChanged(ctx context.Context, service contract.DataSettingsServices, sessionID string, running bool, failure error) {
	refresh := false
	c.deps.OnUI("apply backup run", func() {
		ended := c.backupRunning && !running
		c.backupRunning = running
		if running {
			if c.busy == "" {
				c.busy = "backup"
			}
		} else if ended && c.busy == "backup" {
			c.busy = ""
		}
		if !running && failure != nil && !errors.Is(failure, contract.ErrBackupInProgress) {
			c.setError("Could not create backup: "+failure.Error(), dataErrorSectionBackup)
		}
		refresh = ended && failure == nil
		c.deps.Invalidate()
	})
	if refresh {
		c.Reload(ctx, service, sessionID)
	}
}

// RestoreBackup replaces current settings with one backup. The button owns the
// second-click confirmation, so this starts the restore immediately.
func (c *dataSettingsController) RestoreBackup(ctx context.Context, service contract.DataSettingsServices, sessionID string, id string) {
	if c.busy != "" || strings.TrimSpace(id) == "" {
		return
	}
	c.busy = "restore"
	c.clearError()
	c.deps.Invalidate()

	util.Go(ctx, "restore data backup", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		err := service.RestoreDataBackup(timeoutCtx, sessionID, id)
		cancel()
		if err == nil && c.reloadSettings != nil {
			err = c.reloadSettings()
		}

		c.deps.OnUI("apply data backup restore", func() {
			c.busy = ""
			if err != nil {
				c.setError("Could not restore backup: "+err.Error(), dataErrorSectionBackup)
			}
			c.deps.Invalidate()
		})
	})
}

// DeleteBackup removes one backup after the row button confirms.
// The catalog refreshes once that directory is gone.
func (c *dataSettingsController) DeleteBackup(ctx context.Context, service contract.DataSettingsServices, sessionID string, id string) {
	if c.busy != "" || c.backupRunning || strings.TrimSpace(id) == "" {
		return
	}
	c.busy = "delete"
	c.deletingBackupID = id
	c.clearError()
	c.deps.Invalidate()

	util.Go(ctx, "delete data backup", func() {
		// A backup directory is a full copy of user data, so removal can take a while.
		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err := service.DeleteDataBackup(timeoutCtx, sessionID, id)
		cancel()
		c.deps.OnUI("apply data backup deletion", func() {
			if c.deletingBackupID == id {
				c.deletingBackupID = ""
			}
			if c.busy == "delete" {
				c.busy = ""
				if c.backupRunning {
					c.busy = "backup"
				}
			}
			if err != nil {
				c.setError("Could not delete backup: "+err.Error(), dataErrorSectionBackup)
			} else {
				kept := c.backups[:0]
				for _, backup := range c.backups {
					if backup.ID != id {
						kept = append(kept, backup)
					}
				}
				c.backups = kept
			}
			c.deps.Invalidate()
		})
		if err == nil {
			c.Reload(ctx, service, sessionID)
		}
	})
}

// ChooseLocation opens the native directory picker and stages the selected path
// for explicit confirmation. The actual move happens in ConfirmLocationChange.
func (c *dataSettingsController) ChooseLocation() {
	if c.pickDirectory == nil {
		return
	}
	path, err := c.pickDirectory()
	if err != nil {
		c.setError("Could not select data directory: "+err.Error(), dataErrorSectionStorage)
	} else if strings.TrimSpace(path) != "" && path != c.location {
		c.pendingLocation = path
	}
	c.deps.Invalidate()
}

// CancelLocationChange clears any staged directory.
func (c *dataSettingsController) CancelLocationChange() {
	c.pendingLocation = ""
	c.deps.Invalidate()
}

// ConfirmLocationChange delegates the actual data migration to core after the
// visible confirmation step. On failure the staged path is restored so the user
// can retry without re-picking the directory.
func (c *dataSettingsController) ConfirmLocationChange(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	location := c.pendingLocation
	if c.busy != "" || strings.TrimSpace(location) == "" {
		return
	}
	c.pendingLocation = ""
	c.busy = "location"
	c.clearError()
	c.deps.Invalidate()

	util.Go(ctx, "change data location", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err := service.ChangeDataLocation(timeoutCtx, sessionID, location)
		cancel()

		c.deps.OnUI("apply data location change", func() {
			c.busy = ""
			if err != nil {
				c.pendingLocation = location
				c.setError("Could not move data directory: "+err.Error(), dataErrorSectionStorage)
			} else {
				c.location = location
			}
			c.deps.Invalidate()
		})
	})
}

// ClearLogs uses the same two-step confirmation as backup restore to avoid
// accidental data loss.
func (c *dataSettingsController) ClearLogs(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	if c.busy != "" {
		return
	}
	if !c.clearLogsArmed {
		c.clearLogsArmed = true
		c.deps.Invalidate()
		return
	}
	c.clearLogsArmed = false
	c.busy = "logs"
	c.clearError()
	c.deps.Invalidate()

	util.Go(ctx, "clear logs", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err := service.ClearLogs(timeoutCtx, sessionID)
		cancel()

		c.deps.OnUI("apply clear logs result", func() {
			c.busy = ""
			if err != nil {
				c.setError("Could not clear logs: "+err.Error(), dataErrorSectionLogs)
			}
			c.deps.Invalidate()
		})
	})
}

// OpenPath delegates platform shell behavior to the core data service.
func (c *dataSettingsController) OpenPath(ctx context.Context, service contract.DataSettingsServices, sessionID string, path, section string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	util.Go(ctx, "open data path", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := service.OpenPath(timeoutCtx, sessionID, path)
		cancel()
		if err != nil {
			c.deps.OnUI("apply open data path error", func() {
				c.setError("Could not open path: "+err.Error(), section)
				c.deps.Invalidate()
			})
		}
	})
}

// OpenBackupFolder resolves the configured folder in core before asking the desktop
// to open it.
func (c *dataSettingsController) OpenBackupFolder(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	util.Go(ctx, "open backup folder", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		path, err := service.BackupFolder(timeoutCtx, sessionID)
		cancel()
		if err != nil {
			c.deps.OnUI("apply open backup folder error", func() {
				c.setError("Could not open backup folder: "+err.Error(), dataErrorSectionBackup)
				c.deps.Invalidate()
			})
			return
		}
		c.OpenPath(ctx, service, sessionID, path, dataErrorSectionBackup)
	})
}

// OpenLog lets core create and reveal the current log file with its platform shell adapter.
func (c *dataSettingsController) OpenLog(ctx context.Context, service contract.DataSettingsServices, sessionID string) {
	util.Go(ctx, "open log file", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		err := service.OpenLog(timeoutCtx, sessionID)
		cancel()
		if err != nil {
			c.deps.OnUI("apply open log error", func() {
				c.setError("Could not open log: "+err.Error(), dataErrorSectionLogs)
				c.deps.Invalidate()
			})
		}
	})
}

// Snapshot returns a copy of the Data state for the view layer.
func (c *dataSettingsController) Snapshot() dataSettingsSnapshot {
	return dataSettingsSnapshot{
		Backups:          append([]backupInfo(nil), c.backups...),
		Location:         c.location,
		Loading:          c.loading,
		Loaded:           c.loaded,
		Busy:             c.busy,
		BackupRunning:    c.backupRunning,
		DeletingBackupID: c.deletingBackupID,
		Error:            c.errMsg,
		ErrorSection:     c.errSection,
		PendingLocation:  c.pendingLocation,
		ClearLogsArmed:   c.clearLogsArmed,
	}
}
