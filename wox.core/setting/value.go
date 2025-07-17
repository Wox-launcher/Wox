package setting

import (
	"fmt"
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
	syncStore    SynableStore
	validator    ValidatorFunc[T]
	syncable     bool
	isLoaded     bool
	mu           sync.RWMutex
}

type WoxSettingValue[T any] struct {
	*SettingValue[T]
}

// platform specific setting value. Don't set this value directly, use get,set instead
type PlatformValue[T any] struct {
	*WoxSettingValue[struct {
		MacValue   T
		WinValue   T
		LinuxValue T
	}]
}

type PluginSettingValue[T any] struct {
	*SettingValue[T]
	pluginId string
}

func (p *PlatformValue[T]) Get() T {
	if util.IsWindows() {
		return p.SettingValue.Get().WinValue
	} else if util.IsMacOS() {
		return p.SettingValue.Get().MacValue
	} else if util.IsLinux() {
		return p.SettingValue.Get().LinuxValue
	}

	panic("unknown platform")
}

func (p *PlatformValue[T]) Set(t T) {
	if util.IsWindows() {
		p.value.WinValue = t
		p.SettingValue.Set(p.value)
		return
	} else if util.IsMacOS() {
		p.value.MacValue = t
		p.SettingValue.Set(p.value)
		return
	} else if util.IsLinux() {
		p.value.LinuxValue = t
		p.SettingValue.Set(p.value)
		return
	}

	panic("unknown platform")
}

func NewWoxSettingValue[T any](store *WoxSettingStore, key string, defaultValue T) *WoxSettingValue[T] {
	return &WoxSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
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
		},
	}
}

func NewPlatformValue[T any](store *WoxSettingStore, key string, winValue T, macValue T, linuxValue T) *PlatformValue[T] {
	return &PlatformValue[T]{
		WoxSettingValue: NewWoxSettingValue(store, key, struct {
			MacValue   T
			WinValue   T
			LinuxValue T
		}{
			MacValue:   macValue,
			WinValue:   winValue,
			LinuxValue: linuxValue,
		}),
	}
}

func NewPluginSettingValue[T any](store *PluginSettingStore, key string, defaultValue T) *PluginSettingValue[T] {
	return &PluginSettingValue[T]{
		SettingValue: &SettingValue[T]{
			settingStore: store,
			key:          key,
			defaultValue: defaultValue,
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
		err = v.settingStore.Set(v.key, newValue)
	} else {
		return fmt.Errorf("no store available")
	}

	if err != nil {
		return err
	}

	v.value = newValue
	v.isLoaded = true

	if v.syncable {
		return v.syncStore.LogOplog(v.key, newValue)
	}

	return nil
}
