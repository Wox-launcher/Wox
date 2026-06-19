package setting

import (
	"fmt"
	"strings"
	"sync"
	"wox/util"
)

// ValidatorFunc is a function type for validating setting values
type ValidatorFunc[T any] func(T) bool

// SettingValue is a generic type that represents a single, observable setting.
// It handles lazy loading and persisting of its value.
type SettingValue[T any] struct {
	key   string
	value T

	defaultValue T
	settingStore SettingStore
	validator    ValidatorFunc[T]
	syncable     bool
	isLoaded     bool
	mu           sync.RWMutex
}

// local setting value. Don't set this value directly, use get,set instead
// setting value that is only stored locally and not synced
type LocalSettingValue[T any] struct {
	*SettingValue[T]
}

// wox setting value. Don't set this value directly, use get,set instead
type WoxSettingValue[T any] struct {
	*SettingValue[T]
}

// platform specific setting value. Don't set this value directly, use get,set instead
// The physical storage key of a platform setting is automatically suffixed with @windows, @darwin, or @linux based on the current platform.
type PlatformValue[T any] struct {
	*WoxSettingValue[T]
}

type PluginSettingValue[T any] struct {
	*SettingValue[T]
	pluginId string
}

func NewWoxSettingValue[T any](store *WoxSettingStore, key string, defaultValue T) *WoxSettingValue[T] {
	return &WoxSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
			syncable:     true,
		},
	}
}

// NewLocalWoxSettingValue creates a Wox setting that is persisted only on the
// current device and is excluded from cloud sync replication.
func NewLocalWoxSettingValue[T any](store *WoxSettingStore, key string, defaultValue T) *WoxSettingValue[T] {
	return &WoxSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
			syncable:     false,
		},
	}
}

func NewWoxSettingValueWithValidator[T any](store *WoxSettingStore, key string, defaultValue T, validator ValidatorFunc[T]) *WoxSettingValue[T] {
	return &WoxSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
			validator:    validator,
			syncable:     true,
		},
	}
}

func NewPlatformValue[T any](store *WoxSettingStore, key string, winValue T, macValue T, linuxValue T) *PlatformValue[T] {
	currentDefaultValue := linuxValue
	if util.IsWindows() {
		currentDefaultValue = winValue
	} else if util.IsMacOS() {
		currentDefaultValue = macValue
	}

	// PlatformValue binds its physical storage key once at construction, so the
	// inherited Get/Set methods operate on MainHotkey@darwin-style keys directly.
	return &PlatformValue[T]{
		WoxSettingValue: NewWoxSettingValue(store, PlatformSettingKey(key, util.GetCurrentPlatform()), currentDefaultValue),
	}
}

// PlatformSettingKey builds the same physical key shape used by plugin platform settings.
func PlatformSettingKey(baseKey string, platform string) string {
	return fmt.Sprintf("%s@%s", baseKey, strings.ToLower(strings.TrimSpace(platform)))
}

// SplitPlatformSettingKey parses keys like MainHotkey@darwin.
func SplitPlatformSettingKey(key string) (string, string, bool) {
	index := strings.LastIndex(key, "@")
	if index <= 0 || index == len(key)-1 {
		return "", "", false
	}

	baseKey := key[:index]
	platform := strings.ToLower(strings.TrimSpace(key[index+1:]))
	if !util.IsSupportedPlatform(platform) {
		return "", "", false
	}
	return baseKey, platform, true
}

func NewPluginSettingValue[T any](store *PluginSettingStore, key string, defaultValue T) *PluginSettingValue[T] {
	return &PluginSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
			syncable:     true,
		},
		pluginId: store.pluginId,
	}
}

// Get returns the value of the setting, loading it from the store if necessary.
func (v *SettingValue[T]) Get() T {
	v.mu.RLock()
	if v.isLoaded {
		defer v.mu.RUnlock()
		return v.value
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()
	// Double-check in case another goroutine loaded it while we were waiting for the lock.
	if v.isLoaded {
		return v.value
	}

	// Load from unified store
	v.value = v.defaultValue // Start with default value
	if v.settingStore != nil {
		if err := v.settingStore.Get(v.key, &v.value); err != nil {
			// Log error and keep default value
			v.value = v.defaultValue
		}
	}

	// Apply validation if provided
	if v.validator != nil && !v.validator(v.value) {
		v.value = v.defaultValue
	}

	v.isLoaded = true
	return v.value
}

// Set updates the value of the setting and persists it to the store.
func (v *SettingValue[T]) Set(newValue T) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var err error
	if v.settingStore != nil {
		if syncStore, ok := v.settingStore.(SyncableStore); ok {
			err = syncStore.SetWithSync(v.key, newValue, v.syncable)
		} else {
			err = v.settingStore.Set(v.key, newValue)
		}
	} else {
		return fmt.Errorf("no store available")
	}

	if err != nil {
		return err
	}

	v.value = newValue
	v.isLoaded = true
	return nil
}

func (v *SettingValue[T]) Key() string {
	return v.key
}

func (v *SettingValue[T]) IsSyncable() bool {
	return v.syncable
}

func (v *SettingValue[T]) SetLocal(newValue T) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.settingStore == nil {
		return fmt.Errorf("no store available")
	}

	if err := v.settingStore.Set(v.key, newValue); err != nil {
		return err
	}

	v.value = newValue
	v.isLoaded = true
	return nil
}

func (v *SettingValue[T]) SetFromString(strValue string) error {
	var decoded T
	if err := deserializeValue(strValue, &decoded); err != nil {
		return err
	}
	return v.SetLocal(decoded)
}

func (v *SettingValue[T]) DeleteLocal() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.settingStore == nil {
		return fmt.Errorf("no store available")
	}

	if err := v.settingStore.Delete(v.key); err != nil {
		return err
	}

	v.value = v.defaultValue
	v.isLoaded = true
	return nil
}
