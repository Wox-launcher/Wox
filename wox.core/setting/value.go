package setting

import (
	"fmt"
	"sync"
	"wox/util"
)

// ValidatorFunc is a function type for validating setting values
type ValidatorFunc[T any] func(T) bool

// Value is a generic type that represents a single, observable setting.
// It handles lazy loading and persisting of its value.
type Value[T any] struct {
	key          string
	defaultValue T
	value        T
	isLoaded     bool
	store        WoxSettingStore
	validator    ValidatorFunc[T]
	syncable     bool
	mu           sync.RWMutex
}

// platform specific setting value. Don't set this value directly, use get,set instead
type PlatformValue[T any] struct {
	*Value[struct {
		MacValue   T
		WinValue   T
		LinuxValue T
	}]
}

func (p *PlatformValue[T]) Get() T {
	if util.IsWindows() {
		return p.Value.Get().WinValue
	} else if util.IsMacOS() {
		return p.Value.Get().MacValue
	} else if util.IsLinux() {
		return p.Value.Get().LinuxValue
	}

	panic("unknown platform")
}

func (p *PlatformValue[T]) Set(t T) {
	if util.IsWindows() {
		p.value.WinValue = t
		p.Value.Set(p.value)
		return
	} else if util.IsMacOS() {
		p.value.MacValue = t
		p.Value.Set(p.value)
		return
	} else if util.IsLinux() {
		p.value.LinuxValue = t
		p.Value.Set(p.value)
		return
	}

	panic("unknown platform")
}

// NewValue creates a new setting value using the unified store interface.
func NewValue[T any](store WoxSettingStore, key string, defaultValue T) *Value[T] {
	return &Value[T]{
		store:        store,
		key:          key,
		defaultValue: defaultValue,
	}
}

// NewValueWithValidator creates a new setting value with a validator function using the unified store interface.
func NewValueWithValidator[T any](store WoxSettingStore, key string, defaultValue T, validator ValidatorFunc[T]) *Value[T] {
	return &Value[T]{
		store:        store,
		key:          key,
		defaultValue: defaultValue,
		validator:    validator,
	}
}

func NewPlatformValue[T any](store WoxSettingStore, key string, winValue T, macValue T, linuxValue T) *PlatformValue[T] {
	return &PlatformValue[T]{
		Value: NewValue(store, key, struct {
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

// Get returns the value of the setting, loading it from the store if necessary.
func (v *Value[T]) Get() T {
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
	if v.store != nil {
		if err := v.store.Get(v.key, &v.value); err != nil {
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
func (v *Value[T]) Set(newValue T) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	var err error
	if v.store != nil {
		err = v.store.Set(v.key, newValue)
	} else {
		return fmt.Errorf("no store available")
	}

	if err == nil {
		v.value = newValue
		v.isLoaded = true

		if v.syncable {
			return v.store.LogOplog(v.key, newValue)
		}
	}

	return err
}
