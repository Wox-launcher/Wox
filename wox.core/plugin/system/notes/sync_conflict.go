package notes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"wox/cloudsync"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/setting"
	"wox/util"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func init() {
	cloudsync.RegisterPluginConflictHandler(common.NotesPluginID, preserveNoteConflict)
}

func preserveNoteConflict(ctx context.Context, conflict cloudsync.Conflict) (cloudsync.PluginConflictResult, error) {
	if !strings.HasPrefix(conflict.Key, noteSettingPrefix) || database.GetDB() == nil {
		return cloudsync.PluginConflictResult{}, nil
	}
	store := setting.NewPluginSettingStore(database.GetDB(), common.NotesPluginID)
	key, raw, err := SyncConflictCopy(ctx, store, conflict)
	if err != nil {
		return cloudsync.PluginConflictResult{}, err
	}
	if key == "" {
		return cloudsync.PluginConflictResult{}, nil
	}
	return cloudsync.PluginConflictResult{
		ArchiveKind:  cloudsync.ConflictArchiveNote,
		SettingKey:   key,
		SettingValue: raw,
	}, nil
}

// SyncConflictCopy preserves rich content and attachment references using a retry-stable note ID.
func SyncConflictCopy(ctx context.Context, store *setting.PluginSettingStore, conflict cloudsync.Conflict) (string, string, error) {
	noteID, ok := strings.CutPrefix(conflict.Key, noteSettingPrefix)
	if !ok || noteID == "" {
		return "", "", nil
	}
	var draft, remote common.NoteRecord
	if json.Unmarshal([]byte(conflict.LocalValue), &draft) != nil || draft.ID != noteID {
		return "", "", nil
	}
	if json.Unmarshal([]byte(conflict.RemoteValue), &remote) == nil && reflect.DeepEqual(draft.Document, remote.Document) {
		return "", "", nil
	}
	if DocumentIsEmpty(draft.Document) {
		return "", "", nil
	}
	id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(conflict.ID())).String()
	now := util.GetSystemTimestamp()
	label := i18n.GetI18nManager().TranslateWox(ctx, "plugin_notes_sync_conflict_suffix")
	suffix := " (" + fmt.Sprintf(label, util.GetCurrentPlatform(), time.UnixMilli(conflict.LocalTimestamp).Format("2006-01-02 15:04")) + ")"
	copy := common.NoteRecord{
		SchemaVersion: noteSchemaVersion, ID: id, Revision: id,
		Document: AppendTitleSuffix(draft.Document, suffix), CreatedAt: now, UpdatedAt: now,
	}
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", "", err
	}
	key := noteSettingPrefix + id
	created := false
	err = store.UpdateWithSync(func(tx *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		created = false
		var existing common.NoteRecord
		if err := tx.Get(key, &existing); !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		changes := []setting.PluginSettingChange{{Key: key, Value: string(raw), Syncable: true}}
		var placement common.NotePlacement
		if err := tx.Get(notePlacementPrefix+noteID, &placement); err == nil {
			placement.NoteID = id
			changes = append(changes, setting.PluginSettingChange{Key: notePlacementPrefix + id, Value: placement, Syncable: true})
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		created = true
		return changes, nil
	})
	if err != nil || !created {
		return key, "", err
	}
	return key, string(raw), nil
}
