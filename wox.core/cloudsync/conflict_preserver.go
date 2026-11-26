package cloudsync

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/util"
)

const conflictDirectory = "sync_conflicts"

// ConflictArchiveKind classifies how a preserved sync conflict is stored on disk.
type ConflictArchiveKind string

const (
	ConflictArchiveGeneric ConflictArchiveKind = "conflict"
	ConflictArchiveNote    ConflictArchiveKind = "note"
)

// PluginConflictResult describes any local preservation side effects produced by a plugin handler.
type PluginConflictResult struct {
	ArchiveKind  ConflictArchiveKind
	SettingKey   string
	SettingValue string
}

// PluginConflictHandler preserves a plugin-owned record before a remote sync write overwrites it.
type PluginConflictHandler func(ctx context.Context, conflict Conflict) (PluginConflictResult, error)

var pluginConflictHandlers = util.NewHashMap[string, PluginConflictHandler]()

// RegisterPluginConflictHandler registers or clears a plugin-specific conflict preservation handler.
func RegisterPluginConflictHandler(pluginID string, handler PluginConflictHandler) {
	if pluginID == "" {
		return
	}
	if handler == nil {
		pluginConflictHandlers.Delete(pluginID)
		return
	}
	pluginConflictHandlers.Store(pluginID, handler)
}

// LookupPluginConflictHandler returns the conflict preservation handler registered for pluginID.
func LookupPluginConflictHandler(pluginID string) (PluginConflictHandler, bool) {
	return pluginConflictHandlers.Load(pluginID)
}

// Conflict carries the losing local write and winning remote state for one sync key.
type Conflict struct {
	EntityType      string
	PluginID        string
	Key             string
	LocalTimestamp  int64
	RemoteTimestamp int64
	LocalOplogID    uint
	LocalValue      string
	RemoteValue     string
}

// ConflictPreserver protects local drafts before the remote write is applied.
// Its optional callback runs after the sync mutation lock has been released.
type ConflictPreserver interface {
	PreserveConflict(ctx context.Context, conflict Conflict) (notify func(), err error)
}

func (c Conflict) ID() string {
	raw, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

// ArchiveConflict saves both versions in a private retry-safe file.
func ArchiveConflict(root string, conflict Conflict, note bool) error {
	dir := filepath.Join(root, conflictDirectory)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	prefix := "conflict-"
	if note {
		prefix = "note-"
	}
	target := filepath.Join(dir, prefix+conflict.ID()+".json")
	if info, err := os.Lstat(target); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("conflict archive destination is not a regular file")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.CreateTemp(dir, ".conflict-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	err = json.NewEncoder(file).Encode(conflict)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), target)
}

// ConflictCounts counts completed archives without opening any user content.
func ConflictCounts(root string) (total, notes int) {
	entries, _ := os.ReadDir(filepath.Join(root, conflictDirectory))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "note-") {
			notes++
			total++
		} else if strings.HasPrefix(entry.Name(), "conflict-") {
			total++
		}
	}
	return total, notes
}

// findConflict considers the newest pending user write retired by the winning upsert.
func findConflict(record CloudSyncRecord, raw string, pending []preApplyOplog) (Conflict, bool) {
	if record.Op != OpUpsert {
		return Conflict{}, false
	}
	var latest preApplyOplog
	for _, row := range pending {
		if row.Synced || row.ID == 0 {
			continue
		}
		if latest.ID == 0 || row.Timestamp > latest.Timestamp || row.Timestamp == latest.Timestamp && row.ID > latest.ID {
			latest = row
		}
	}
	if latest.Operation != OpUpsert || latest.Value == raw {
		return Conflict{}, false
	}
	return Conflict{
		EntityType: record.EntityType, PluginID: record.PluginID, Key: record.Key,
		LocalTimestamp: latest.Timestamp, RemoteTimestamp: record.ClientTs, LocalOplogID: latest.ID,
		LocalValue: latest.Value, RemoteValue: raw,
	}, true
}
