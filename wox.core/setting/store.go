package setting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"wox/database"
	"wox/util"

	"gorm.io/gorm"
)

// WoxSettingStore defines the unified interface for reading and writing settings
type WoxSettingStore interface {
	Get(key string, target interface{}) error
	Set(key string, value interface{}) error
	LogOplog(key string, value interface{}) error
}

// PluginSettingStore defines the interface for plugin settings
type PluginSettingStore interface {
	GetPluginSetting(pluginId, key string, target interface{}) error
	SetPluginSetting(pluginId, key string, value interface{}) error
	GetAllPluginSettings(pluginId string) (map[string]string, error)
	SetAllPluginSettings(pluginId string, settings map[string]string) error
}

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{
		db: db,
	}
}

func (s *Store) Get(key string, target interface{}) error {
	var setting database.WoxSetting
	if err := s.db.Where("key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return the default value (target should already contain it)
			return nil
		}
		logger.Error(context.Background(), fmt.Sprintf("Failed to read setting %s: %v", key, err))
		return err
	}

	return s.deserializeValue(setting.Value, target)
}

func (s *Store) Set(key string, value interface{}) error {
	strValue, err := s.serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value: %w", err)
	}

	// Use GORM's Save for upsert behavior
	return s.db.Save(&database.WoxSetting{Key: key, Value: strValue}).Error
}

func (s *Store) LogOplog(key string, value interface{}) error {
	strValue, err := s.serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize value for oplog: %w", err)
	}

	oplog := database.Oplog{
		EntityType: "setting",
		EntityID:   key,
		Operation:  "update",
		Value:      strValue,
		Timestamp:  util.GetSystemTimestamp(),
	}

	return s.db.Create(&oplog).Error
}

func (s *Store) GetPluginSetting(pluginId, key string, target interface{}) error {
	var setting database.PluginSetting
	if err := s.db.Where("plugin_id = ? AND key = ?", pluginId, key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return the default value (target should already contain it)
			return nil
		}
		logger.Error(context.Background(), fmt.Sprintf("Failed to read plugin setting %s.%s: %v", pluginId, key, err))
		return err
	}

	return s.deserializeValue(setting.Value, target)
}

func (s *Store) SetPluginSetting(pluginId, key string, value interface{}) error {
	strValue, err := s.serializeValue(value)
	if err != nil {
		return fmt.Errorf("failed to serialize plugin setting value: %w", err)
	}

	// Use GORM's Save for upsert behavior
	return s.db.Save(&database.PluginSetting{PluginID: pluginId, Key: key, Value: strValue}).Error
}

func (s *Store) GetAllPluginSettings(pluginId string) (map[string]string, error) {
	var settings []database.PluginSetting
	if err := s.db.Where("plugin_id = ?", pluginId).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to read plugin settings for %s: %w", pluginId, err)
	}

	result := make(map[string]string)
	for _, setting := range settings {
		result[setting.Key] = setting.Value
	}
	return result, nil
}

func (s *Store) SetAllPluginSettings(pluginId string, settings map[string]string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Clear existing settings for this plugin
		if err := tx.Where("plugin_id = ?", pluginId).Delete(&database.PluginSetting{}).Error; err != nil {
			return fmt.Errorf("failed to clear existing plugin settings: %w", err)
		}

		// Insert new settings
		for key, value := range settings {
			if err := tx.Create(&database.PluginSetting{
				PluginID: pluginId,
				Key:      key,
				Value:    value,
			}).Error; err != nil {
				return fmt.Errorf("failed to save plugin setting %s.%s: %w", pluginId, key, err)
			}
		}

		return nil
	})
}

func (s *Store) serializeValue(value interface{}) (string, error) {
	if value == nil {
		return "", nil
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

func (s *Store) deserializeValue(strValue string, target interface{}) error {
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
