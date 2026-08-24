package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wox/common"
	"wox/setting"
	"wox/util"
)

const noteSettingPrefix = "note:"

// Repository persists Notes records independently so Cloud Sync can merge different notes.
type Repository struct {
	mu        sync.Mutex
	store     *setting.PluginSettingStore
	observers []func(string)
}

// NewRepository binds Notes storage to one plugin setting store.
func NewRepository(store *setting.PluginSettingStore) *Repository {
	return &Repository{store: store}
}

// Observe registers an in-process refresh listener for local and remote changes.
func (r *Repository) Observe(callback func(string)) {
	if callback == nil {
		return
	}
	r.mu.Lock()
	r.observers = append(r.observers, callback)
	r.mu.Unlock()
}

// notify snapshots observers so callbacks never run while the repository mutex is held.
func (r *Repository) notify(id string) {
	r.mu.Lock()
	callbacks := append([]func(string){}, r.observers...)
	r.mu.Unlock()
	for _, callback := range callbacks {
		callback(id)
	}
}

// ExternalChanged refreshes in-process consumers after Cloud Sync applies a setting.
func (r *Repository) ExternalChanged(id string) {
	r.notify(id)
}

// List returns all valid records ordered by pin state and recent activity.
func (r *Repository) List(includeDeleted bool) ([]common.NoteRecord, error) {
	values, err := r.store.ListByPrefix(noteSettingPrefix)
	if err != nil {
		return nil, err
	}
	records := make([]common.NoteRecord, 0, len(values))
	for key, raw := range values {
		var record common.NoteRecord
		if err := json.Unmarshal([]byte(raw), &record); err != nil || record.ID == "" {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("skip invalid Notes record %s", key))
			continue
		}
		record.Document = NormalizeDocument(record.Document)
		if includeDeleted || record.DeletedAt == 0 {
			records = append(records, record)
		}
	}
	sort.SliceStable(records, func(i, j int) bool {
		if (records[i].PinnedAt > 0) != (records[j].PinnedAt > 0) {
			return records[i].PinnedAt > 0
		}
		if records[i].PinnedAt != records[j].PinnedAt {
			return records[i].PinnedAt > records[j].PinnedAt
		}
		return records[i].UpdatedAt > records[j].UpdatedAt
	})
	return records, nil
}

// Get loads one note by stable id.
func (r *Repository) Get(id string) (common.NoteRecord, error) {
	var record common.NoteRecord
	if strings.TrimSpace(id) == "" {
		return record, errors.New("note id is empty")
	}
	if err := r.store.Get(noteSettingPrefix+id, &record); err != nil {
		return record, err
	}
	record.Document = NormalizeDocument(record.Document)
	return record, nil
}

// Create returns an in-memory empty draft. Empty notes are not persisted until the user types.
func (r *Repository) Create() (common.NoteRecord, error) {
	now := util.GetSystemTimestamp()
	return common.NoteRecord{
		SchemaVersion: noteSchemaVersion,
		ID:            uuid.NewString(),
		Document:      EmptyDocument(),
		CreatedAt:     now,
		UpdatedAt:     now,
		Revision:      uuid.NewString(),
	}, nil
}

// Discard permanently removes a note, including unsaved empty drafts that were never stored.
func (r *Repository) Discard(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("note id is empty")
	}
	if err := r.store.DeleteWithSync(noteSettingPrefix+id, true); err != nil {
		return err
	}
	go r.notify(id)
	return nil
}

// PurgeEmpty permanently removes notes that have no user-entered content.
func (r *Repository) PurgeEmpty() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.List(true)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, record := range records {
		if !DocumentIsEmpty(record.Document) {
			continue
		}
		if err := r.store.DeleteWithSync(noteSettingPrefix+record.ID, true); err != nil {
			return count, err
		}
		count++
		go r.notify(record.ID)
	}
	return count, nil
}

// Save applies one optimistic rich-document update and preserves both versions on conflict.
func (r *Repository) Save(id, expectedRevision string, document common.NoteDocument) (common.NoteRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	document = NormalizeDocument(document)
	current, err := r.Get(id)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return common.NoteRecord{}, false, err
		}
		if DocumentIsEmpty(document) {
			return emptyDraftRecord(id, expectedRevision, document), false, nil
		}
		now := util.GetSystemTimestamp()
		record := common.NoteRecord{
			SchemaVersion: noteSchemaVersion,
			ID:            id,
			Document:      document,
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      uuid.NewString(),
		}
		if err := r.write(record); err != nil {
			return common.NoteRecord{}, false, err
		}
		go r.notify(record.ID)
		return record, false, nil
	}
	if DocumentIsEmpty(document) {
		current.Document = document
		return current, false, nil
	}
	now := util.GetSystemTimestamp()
	if current.Revision != expectedRevision {
		conflict := common.NoteRecord{
			SchemaVersion: noteSchemaVersion,
			ID:            uuid.NewString(),
			Document:      AppendTitleSuffix(document, conflictTitleSuffix),
			CreatedAt:     now,
			UpdatedAt:     now,
			Revision:      uuid.NewString(),
		}
		if err := r.write(conflict); err != nil {
			return common.NoteRecord{}, false, err
		}
		go r.notify(conflict.ID)
		return conflict, true, nil
	}
	current.Document = NormalizeDocument(document)
	current.UpdatedAt = now
	current.Revision = uuid.NewString()
	if err := r.write(current); err != nil {
		return common.NoteRecord{}, false, err
	}
	go r.notify(current.ID)
	return current, false, nil
}

// SetPinned updates pin order without rewriting note content.
func (r *Repository) SetPinned(id string, pinned bool) (common.NoteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, err := r.Get(id)
	if err != nil {
		return common.NoteRecord{}, err
	}
	if pinned {
		record.PinnedAt = util.GetSystemTimestamp()
	} else {
		record.PinnedAt = 0
	}
	record.UpdatedAt = util.GetSystemTimestamp()
	record.Revision = uuid.NewString()
	if err := r.write(record); err != nil {
		return common.NoteRecord{}, err
	}
	go r.notify(id)
	return record, nil
}

// Delete moves a note to the 60-day trash.
func (r *Repository) Delete(id string) (common.NoteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, err := r.Get(id)
	if err != nil {
		return common.NoteRecord{}, err
	}
	record.DeletedAt = util.GetSystemTimestamp()
	record.UpdatedAt = record.DeletedAt
	record.Revision = uuid.NewString()
	if err := r.write(record); err != nil {
		return common.NoteRecord{}, err
	}
	go r.notify(id)
	return record, nil
}

// Restore returns a trashed note to the active list.
func (r *Repository) Restore(id string) (common.NoteRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	record, err := r.Get(id)
	if err != nil {
		return common.NoteRecord{}, err
	}
	record.DeletedAt = 0
	record.UpdatedAt = util.GetSystemTimestamp()
	record.Revision = uuid.NewString()
	if err := r.write(record); err != nil {
		return common.NoteRecord{}, err
	}
	go r.notify(id)
	return record, nil
}

// PurgeDeletedBefore permanently removes expired trash and syncs tombstones.
func (r *Repository) PurgeDeletedBefore(cutoff time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.List(true)
	if err != nil {
		return 0, err
	}
	cutoffMs := cutoff.UnixMilli()
	count := 0
	for _, record := range records {
		if record.DeletedAt == 0 || record.DeletedAt > cutoffMs {
			continue
		}
		if err := r.store.DeleteWithSync(noteSettingPrefix+record.ID, true); err != nil {
			return count, err
		}
		count++
		go r.notify(record.ID)
	}
	return count, nil
}

// GetLocal loads one device-only Notes preference.
func (r *Repository) GetLocal(key string) string {
	var value string
	if err := r.store.Get(key, &value); err != nil {
		return ""
	}
	return value
}

// SetLocal persists one device-only Notes preference.
func (r *Repository) SetLocal(key, value string) error {
	return r.store.SetWithSync(key, value, false)
}

// write normalizes and syncs one independently keyed record.
func (r *Repository) write(record common.NoteRecord) error {
	record.SchemaVersion = noteSchemaVersion
	record.Document = NormalizeDocument(record.Document)
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return r.store.SetWithSync(noteSettingPrefix+record.ID, string(raw), true)
}

// emptyDraftRecord keeps an unsaved empty note addressable without writing it to the store.
func emptyDraftRecord(id, revision string, document common.NoteDocument) common.NoteRecord {
	now := util.GetSystemTimestamp()
	if revision == "" {
		revision = uuid.NewString()
	}
	return common.NoteRecord{
		SchemaVersion: noteSchemaVersion,
		ID:            id,
		Document:      document,
		CreatedAt:     now,
		UpdatedAt:     now,
		Revision:      revision,
	}
}
