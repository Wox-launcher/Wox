package setting

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
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
	strValue, err := serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	return s.db.Save(&database.WoxSetting{Key: key, Value: strValue}).Error
}

func (s *WoxSettingStore) Delete(key string) error {
	return s.db.Delete(&database.WoxSetting{Key: key}).Error
}

func (s *WoxSettingStore) SetWithSync(key string, value interface{}, syncable bool) error {
	if err := s.Set(key, value); err != nil {
		return err
	}
	if !syncable {
		return nil
	}
	return s.logOplog(key, value, cloudsync.OpUpsert)
}

func (s *WoxSettingStore) DeleteWithSync(key string, syncable bool) error {
	if err := s.Delete(key); err != nil {
		return err
	}
	if !syncable {
		return nil
	}
	return s.logOplog(key, nil, cloudsync.OpDelete)
}

func (s *WoxSettingStore) logOplog(key string, value interface{}, op string) error {
	strValue, err := serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value for oplog: %w", err)
	}

	oplog := database.Oplog{
		EntityType: cloudsync.EntityWoxSetting,
		EntityID:   key,
		Operation:  op,
		Key:        key,
		Value:      strValue,
		Timestamp:  util.GetSystemTimestamp(),
	}

	if err := s.db.Create(&oplog).Error; err != nil {
		return err
	}
	cloudsync.NotifyOplogChanged()
	return nil
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

func (s *PluginSettingStore) Set(key string, value interface{}) error {
	strValue, err := serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize plugin setting value: %w", err)
	}

	return s.db.Save(&database.PluginSetting{PluginID: s.pluginId, Key: key, Value: strValue}).Error
}

func (s *PluginSettingStore) Delete(key string) error {
	return s.db.Delete(&database.PluginSetting{PluginID: s.pluginId, Key: key}).Error
}

func (s *PluginSettingStore) DeleteAll() error {
	var settings []database.PluginSetting
	if err := s.db.Where("plugin_id = ?", s.pluginId).Find(&settings).Error; err != nil {
		return err
	}

	if err := s.db.Where("plugin_id = ?", s.pluginId).Delete(&database.PluginSetting{}).Error; err != nil {
		return err
	}

	for _, setting := range settings {
		if err := s.logOplog(setting.Key, nil, cloudsync.OpDelete); err != nil {
			return err
		}
	}

	return nil
}

func (s *PluginSettingStore) SetWithSync(key string, value interface{}, syncable bool) error {
	if err := s.Set(key, value); err != nil {
		return err
	}
	if !syncable {
		return nil
	}
	return s.logOplog(key, value, cloudsync.OpUpsert)
}

func (s *PluginSettingStore) DeleteWithSync(key string, syncable bool) error {
	if err := s.Delete(key); err != nil {
		return err
	}
	if !syncable {
		return nil
	}
	return s.logOplog(key, nil, cloudsync.OpDelete)
}

func (s *PluginSettingStore) logOplog(key string, value interface{}, op string) error {
	strValue, err := serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize plugin setting value for oplog: %w", err)
	}

	oplog := database.Oplog{
		EntityType: cloudsync.EntityPluginSetting,
		EntityID:   s.pluginId,
		Operation:  op,
		Key:        key,
		Value:      strValue,
		Timestamp:  util.GetSystemTimestamp(),
	}

	if err := s.db.Create(&oplog).Error; err != nil {
		return err
	}
	cloudsync.NotifyOplogChanged()
	return nil
}

func serializeValue(value interface{}) (string, error) {
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
