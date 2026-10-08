package setting

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"wox/cloudsync"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// SettingStore defines the abstract interface for reading and writing settings
// This is the base interface that both WoxSettingStore and PluginSettingStore adapters implement
type SettingStore interface {
	Get(key string, target interface{}) error
	Set(key string, value interface{}) error
	Delete(key string) error
}

// SyncableStore defines the interface for setting stores that support syncable operations
// Any setting store implementing this interface will invoke SetWithSync/DeleteWithSync methods (instead of Set/Delete)
// when setting/deleting values
type SyncableStore interface {
	SetWithSync(key string, value interface{}, syncable bool) error
	DeleteWithSync(key string, syncable bool) error
}

type WoxSettingStore struct {
	db *gorm.DB
}

func NewWoxSettingStore(db *gorm.DB) *WoxSettingStore {
	return &WoxSettingStore{
		db: db,
	}
}

func (s *WoxSettingStore) Get(key string, target interface{}) error {
	var setting database.WoxSetting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return err
	}

	return deserializeValue(setting.Value, target)
}

func (s *WoxSettingStore) Set(key string, value interface{}) error {
	strValue, err := SerializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	return s.db.Save(&database.WoxSetting{Key: key, Value: strValue}).Error
}

func (s *WoxSettingStore) Delete(key string) error {
	return s.db.Delete(&database.WoxSetting{Key: key}).Error
}

func (s *WoxSettingStore) SetWithSync(key string, value interface{}, syncable bool) error {
	// The setting row and its oplog commit together. SettingValue.Set publishes
	// memory only after this returns nil, so a committed row with a failed oplog
	// stayed invisible to in-process readers such as Query History.
	return cloudsync.WithLocalSyncMutation(func() error {
		return retrySQLiteWrite(func() error {
			return s.db.Transaction(func(tx *gorm.DB) error {
				txStore := &WoxSettingStore{db: tx}
				if err := txStore.Set(key, value); err != nil {
					return err
				}
				if !syncable {
					return nil
				}
				return txStore.logOplog(key, value, cloudsync.OpUpsert)
			})
		})
	})
}

func (s *WoxSettingStore) DeleteWithSync(key string, syncable bool) error {
	return cloudsync.WithLocalSyncMutation(func() error {
		return retrySQLiteWrite(func() error {
			return s.db.Transaction(func(tx *gorm.DB) error {
				txStore := &WoxSettingStore{db: tx}
				result := txStore.db.Delete(&database.WoxSetting{Key: key})
				if result.Error != nil {
					return result.Error
				}
				if !syncable || result.RowsAffected == 0 {
					return nil
				}
				return txStore.logOplog(key, nil, cloudsync.OpDelete)
			})
		})
	})
}

func (s *WoxSettingStore) logOplog(key string, value interface{}, op string) error {
	strValue, err := SerializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value for oplog: %w", err)
	}

	oplog := database.Oplog{
		EntityType: cloudsync.EntityWoxSetting,
		EntityID:   key,
		Operation:  op,
		Key:        key,
		Value:      strValue,
	}

	return writeCloudSyncOplog(s.db, oplog)
}

// PluginSettingStore defines the interface for plugin settings
type PluginSettingStore struct {
	db       *gorm.DB
	pluginId string
}

func NewPluginSettingStore(db *gorm.DB, pluginId string) *PluginSettingStore {
	return &PluginSettingStore{
		db:       db,
		pluginId: pluginId,
	}
}

func (s *PluginSettingStore) Get(key string, target interface{}) error {
	var setting database.PluginSetting
	if err := s.db.Where("plugin_id = ? AND key = ?", s.pluginId, key).First(&setting).Error; err != nil {
		return err
	}

	return deserializeValue(setting.Value, target)
}

// ListByPrefix returns raw plugin setting values whose keys share prefix.
func (s *PluginSettingStore) ListByPrefix(prefix string) (map[string]string, error) {
	var settings []database.PluginSetting
	query := s.db.Where("plugin_id = ?", s.pluginId)
	if prefix != "" {
		escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)
		query = query.Where(`key LIKE ? ESCAPE '\'`, escaped+"%")
	}
	if err := query.Find(&settings).Error; err != nil {
		return nil, err
	}

	values := make(map[string]string, len(settings))
	for _, item := range settings {
		values[item.Key] = item.Value
	}
	return values, nil
}

func (s *PluginSettingStore) Set(key string, value interface{}) error {
	return s.set(key, value, false)
}

func (s *PluginSettingStore) set(key string, value interface{}, isLocal bool) error {
	strValue, err := SerializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize plugin setting value: %w", err)
	}

	return s.db.Save(&database.PluginSetting{PluginID: s.pluginId, Key: key, Value: strValue, IsLocal: isLocal}).Error
}

func (s *PluginSettingStore) Delete(key string) error {
	return s.db.Delete(&database.PluginSetting{PluginID: s.pluginId, Key: key}).Error
}

func (s *PluginSettingStore) DeleteAll() error {
	return cloudsync.WithLocalSyncMutation(func() error {
		var settings []database.PluginSetting
		if err := s.db.Where("plugin_id = ?", s.pluginId).Find(&settings).Error; err != nil {
			return err
		}

		if err := s.db.Where("plugin_id = ?", s.pluginId).Delete(&database.PluginSetting{}).Error; err != nil {
			return err
		}

		for _, setting := range settings {
			if setting.IsLocal {
				continue
			}
			if err := s.logOplog(setting.Key, nil, cloudsync.OpDelete); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *PluginSettingStore) SetWithSync(key string, value interface{}, syncable bool) error {
	return cloudsync.WithLocalSyncMutation(func() error {
		return retrySQLiteWrite(func() error {
			return s.db.Transaction(func(tx *gorm.DB) error {
				txStore := &PluginSettingStore{db: tx, pluginId: s.pluginId}
				if err := txStore.set(key, value, !syncable); err != nil {
					return err
				}
				if !syncable {
					return txStore.discardPendingOplogs(key)
				}
				return txStore.logOplog(key, value, cloudsync.OpUpsert)
			})
		})
	})
}

func (s *PluginSettingStore) DeleteWithSync(key string, syncable bool) error {
	return cloudsync.WithLocalSyncMutation(func() error {
		return retrySQLiteWrite(func() error {
			return s.db.Transaction(func(tx *gorm.DB) error {
				txStore := &PluginSettingStore{db: tx, pluginId: s.pluginId}
				wasLocal := false
				if syncable {
					var existing database.PluginSetting
					findErr := txStore.db.Select("is_local").Where("plugin_id = ? AND key = ?", s.pluginId, key).First(&existing).Error
					if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
						return findErr
					}
					wasLocal = findErr == nil && existing.IsLocal
				}

				result := txStore.db.Delete(&database.PluginSetting{PluginID: s.pluginId, Key: key})
				if result.Error != nil {
					return result.Error
				}
				if !syncable || wasLocal || result.RowsAffected == 0 {
					return nil
				}
				return txStore.logOplog(key, nil, cloudsync.OpDelete)
			})
		})
	})
}

// discardPendingOplogs prevents a value switched to local-only from being uploaded by an older queued write.
func (s *PluginSettingStore) discardPendingOplogs(key string) error {
	return s.db.Model(&database.Oplog{}).
		Where("entity_type = ? AND entity_id = ? AND key = ? AND synced_to_cloud = ? AND cloud_sync_discarded = ?",
			cloudsync.EntityPluginSetting, s.pluginId, key, false, false).
		Update("cloud_sync_discarded", true).Error
}

func (s *PluginSettingStore) logOplog(key string, value interface{}, op string) error {
	strValue, err := SerializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize plugin setting value for oplog: %w", err)
	}

	oplog := database.Oplog{
		EntityType: cloudsync.EntityPluginSetting,
		EntityID:   s.pluginId,
		Operation:  op,
		Key:        key,
		Value:      strValue,
	}

	return writeCloudSyncOplog(s.db, oplog)
}

const sqliteWriteAttempts = 3

// retrySQLiteWrite repeats a short transaction after SQLite reports that another
// connection holds the write lock. Busy waits already happen inside the driver.
func retrySQLiteWrite(operation func() error) error {
	var err error
	for attempt := 0; attempt < sqliteWriteAttempts; attempt++ {
		err = operation()
		if err == nil || !isSQLiteLockError(err) || attempt == sqliteWriteAttempts-1 {
			return err
		}
		time.Sleep(time.Duration(20*(attempt+1)) * time.Millisecond)
	}
	return err
}

func isSQLiteLockError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "database is locked") ||
		strings.Contains(text, "database table is locked") ||
		strings.Contains(text, "sqlite_busy") ||
		strings.Contains(text, "sqlite_locked")
}

// writeCloudSyncOplog persists a local sync row according to the built-in CloudSync timing policy.
func writeCloudSyncOplog(db *gorm.DB, oplog database.Oplog) error {
	now := util.GetSystemTimestamp()
	oplog.Timestamp = now

	if oplog.Operation == cloudsync.OpDelete {
		if err := writeImmediateDeleteCloudSyncOplog(db, oplog); err != nil {
			return err
		}
		return nil
	}

	policy := cloudsync.ResolveOplogSyncPolicy(oplog.EntityType, oplog.EntityID, oplog.Key, oplog.Operation)
	if policy.Delay <= 0 {
		if err := db.Create(&oplog).Error; err != nil {
			return err
		}
		return nil
	}

	return upsertDeferredCloudSyncOplog(db, oplog, now+policy.Delay.Milliseconds(), now)
}

// upsertDeferredCloudSyncOplog coalesces high-churn settings into one pending latest-wins row.
func upsertDeferredCloudSyncOplog(db *gorm.DB, oplog database.Oplog, syncAfter int64, now int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var existing []database.Oplog
		if err := tx.Where(
			"synced_to_cloud = ? AND cloud_sync_discarded = ? AND entity_type = ? AND entity_id = ? AND operation = ? AND key = ?",
			false,
			false,
			oplog.EntityType,
			oplog.EntityID,
			oplog.Operation,
			oplog.Key,
		).Order("id asc").Find(&existing).Error; err != nil {
			return err
		}
		if len(existing) == 0 {
			oplog.SyncAfter = syncAfter
			return tx.Create(&oplog).Error
		}

		keepID := existing[0].ID
		if len(existing) > 1 {
			supersededIDs := make([]uint, 0, len(existing)-1)
			for _, row := range existing[1:] {
				supersededIDs = append(supersededIDs, row.ID)
			}
			if err := tx.Model(&database.Oplog{}).Where("id IN ?", supersededIDs).Update("synced_to_cloud", true).Error; err != nil {
				return err
			}
		}

		// Keep one pending row per delayed setting identity so rate limits or offline time do not replay stale intermediate values.
		return tx.Model(&database.Oplog{}).Where("id = ?", keepID).Updates(map[string]interface{}{
			"value":                        oplog.Value,
			"timestamp":                    now,
			"sync_after":                   syncAfter,
			"cloud_sync_push_failed_count": 0,
			"cloud_sync_last_push_error":   "",
		}).Error
	})
}

// writeImmediateDeleteCloudSyncOplog prevents an older delayed upsert from resurrecting a deleted setting remotely.
func writeImmediateDeleteCloudSyncOplog(db *gorm.DB, oplog database.Oplog) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Oplog{}).Where(
			"synced_to_cloud = ? AND cloud_sync_discarded = ? AND entity_type = ? AND entity_id = ? AND operation = ? AND key = ?",
			false,
			false,
			oplog.EntityType,
			oplog.EntityID,
			cloudsync.OpUpsert,
			oplog.Key,
		).Update("synced_to_cloud", true).Error; err != nil {
			return err
		}
		return tx.Create(&oplog).Error
	})
}

func SerializeValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}

	// Use reflection to check if it's a string-based type
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.String {
		return rv.String(), nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	default:
		// For complex types, marshal to JSON
		bytes, err := json.Marshal(v)
		return string(bytes), err
	}
}

func deserializeValue(strValue string, target interface{}) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	elem := rv.Elem()
	switch elem.Kind() {
	case reflect.String:
		elem.SetString(strValue)
		return nil
	case reflect.Int:
		i, err := strconv.Atoi(strValue)
		if err != nil {
			return fmt.Errorf("failed to parse int: %w", err)
		}
		elem.SetInt(int64(i))
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(strValue)
		if err != nil {
			return fmt.Errorf("failed to parse bool: %w", err)
		}
		elem.SetBool(b)
		return nil
	default:
		// For complex types, unmarshal from JSON
		if elem.Type().Kind() == reflect.String {
			// Custom string-based types (like LangCode)
			elem.Set(reflect.ValueOf(strValue).Convert(elem.Type()))
			return nil
		}

		// Try JSON unmarshaling for complex types
		return json.Unmarshal([]byte(strValue), target)
	}
}
