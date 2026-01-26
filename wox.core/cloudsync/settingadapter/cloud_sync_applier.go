package settingadapter

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"wox/cloudsync"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
)

type LocalSettingApplier struct{}

func NewLocalSettingApplier() *LocalSettingApplier {
	return &LocalSettingApplier{}
}

func (a *LocalSettingApplier) ApplyWoxSetting(ctx context.Context, key string, op string, rawValue string) error {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting == nil {
		return fmt.Errorf("wox setting not initialized")
	}

	store := setting.NewWoxSettingStore(database.GetDB())
	previousValue, hadPrevious := loadStoredString(store, key)

	if value, ok := findWoxSettingValueByKey(woxSetting, key); ok {
		switch op {
		case cloudsync.OpDelete:
			return value.DeleteLocal()
		case cloudsync.OpUpsert:
			if err := value.SetFromString(rawValue); err != nil {
				return err
			}
			if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
				ui.GetUIManager().PostSettingUpdate(ctx, key, rawValue)
			}
			return nil
		default:
			return fmt.Errorf("unknown oplog op: %s", op)
		}
	}

	switch op {
	case cloudsync.OpDelete:
		return store.Delete(key)
	case cloudsync.OpUpsert:
		if err := store.Set(key, rawValue); err != nil {
			return err
		}
		if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
			ui.GetUIManager().PostSettingUpdate(ctx, key, rawValue)
		}
		return nil
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

func (a *LocalSettingApplier) ApplyPluginSetting(ctx context.Context, pluginID string, key string, op string, rawValue string) error {
	store := setting.NewPluginSettingStore(database.GetDB(), pluginID)
	previousValue, hadPrevious := loadStoredStringPlugin(store, key)

	switch op {
	case cloudsync.OpDelete:
		return store.Delete(key)
	case cloudsync.OpUpsert:
		if err := store.Set(key, rawValue); err != nil {
			return err
		}
		if shouldNotifySettingChange(op, hadPrevious, previousValue, rawValue) {
			notifyPluginSettingChanged(ctx, pluginID, normalizePluginSettingKey(key), rawValue)
		}
		return nil
	default:
		return fmt.Errorf("unknown oplog op: %s", op)
	}
}

type syncValue interface {
	Key() string
	SetFromString(value string) error
	DeleteLocal() error
}

func findWoxSettingValueByKey(woxSetting *setting.WoxSetting, key string) (syncValue, bool) {
	if woxSetting == nil {
		return nil, false
	}

	v := reflect.ValueOf(woxSetting).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() != reflect.Pointer || field.IsNil() {
			continue
		}
		value, ok := field.Interface().(syncValue)
		if !ok {
			continue
		}
		if value.Key() == key {
			return value, true
		}
	}

	return nil, false
}

func loadStoredString(store *setting.WoxSettingStore, key string) (string, bool) {
	var value string
	if err := store.Get(key, &value); err != nil {
		return "", false
	}
	return value, true
}

func loadStoredStringPlugin(store *setting.PluginSettingStore, key string) (string, bool) {
	var value string
	if err := store.Get(key, &value); err != nil {
		return "", false
	}
	return value, true
}

func shouldNotifySettingChange(op string, hadPrevious bool, previousValue string, newValue string) bool {
	if op != cloudsync.OpUpsert {
		return false
	}
	if !hadPrevious {
		return true
	}
	return previousValue != newValue
}

func normalizePluginSettingKey(key string) string {
	suffix := "@" + util.GetCurrentPlatform()
	if strings.HasSuffix(key, suffix) {
		return strings.TrimSuffix(key, suffix)
	}
	return key
}

func notifyPluginSettingChanged(ctx context.Context, pluginID string, key string, value string) {
	instances := plugin.GetPluginManager().GetPluginInstances()
	for _, instance := range instances {
		if instance.Metadata.Id != pluginID {
			continue
		}
		for _, callback := range instance.SettingChangeCallbacks {
			callback(ctx, key, value)
		}
		return
	}
}
