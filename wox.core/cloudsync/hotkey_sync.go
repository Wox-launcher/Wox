package cloudsync

import (
	"encoding/json"
	"errors"
	"strings"
)

const CrossPlatformHotkeysSetting = "CloudSyncCrossPlatformHotkeys"
const sharedHotkeyPrefix = "SharedHotkey."

var ErrSyncedHotkeyUnavailable = errors.New("shortcut synchronized but unavailable on this device")
var errSkippedHotkeyPolicy = errors.New("shortcut record does not match the current sync policy")

// SharedHotkey carries the source platform so the receiver can map modifier semantics.
type SharedHotkey struct {
	Platform string
	Value    json.RawMessage
}

// SharedHotkeyKey returns the common cloud identity for a supported shortcut setting.
func SharedHotkeyKey(base string) string {
	switch base {
	case "MainHotkey", "SelectionHotkey", "ActionPanelHotkey", "RefinementHotkey", "QueryHotkeys", "ResultBindings":
		return sharedHotkeyPrefix + base
	default:
		return ""
	}
}

func SharedHotkeyBase(key string) string {
	base, ok := strings.CutPrefix(key, sharedHotkeyPrefix)
	if !ok || SharedHotkeyKey(base) == "" {
		return ""
	}
	return base
}

// CrossPlatformHotkeyPolicy exposes the account opt-in to the sync engine.
type CrossPlatformHotkeyPolicy interface {
	CrossPlatformHotkeysEnabled() bool
}

func isHotkeySyncRecord(record CloudSyncRecord) bool {
	if record.EntityType != EntityWoxSetting {
		return false
	}
	base, _, ok := strings.Cut(record.Key, "@")
	return SharedHotkeyBase(record.Key) != "" || (ok && SharedHotkeyKey(base) != "")
}

func isCrossPlatformHotkeyPolicy(record CloudSyncRecord) bool {
	return record.EntityType == EntityWoxSetting && record.Key == CrossPlatformHotkeysSetting
}

// orderHotkeyPolicy applies the account switch before any shortcuts in the same response.
func orderHotkeyPolicy(records []CloudSyncRecord) []CloudSyncRecord {
	for i, record := range records {
		if i == 0 || !isCrossPlatformHotkeyPolicy(record) {
			continue
		}
		ordered := make([]CloudSyncRecord, 0, len(records))
		for _, record := range records {
			if isCrossPlatformHotkeyPolicy(record) {
				ordered = append(ordered, record)
			}
		}
		for _, record := range records {
			if !isCrossPlatformHotkeyPolicy(record) {
				ordered = append(ordered, record)
			}
		}
		return ordered
	}
	return records
}

func (m *CloudSyncManager) shouldApplyHotkeyRecord(record CloudSyncRecord) bool {
	if !isHotkeySyncRecord(record) {
		return true
	}
	policy, ok := m.applier.(CrossPlatformHotkeyPolicy)
	enabled := ok && policy.CrossPlatformHotkeysEnabled()
	if base, _, _ := strings.Cut(record.Key, "@"); base == "ResultBindings" {
		// This record still carries same-platform aliases while hotkeys use the shared record.
		return true
	}
	return enabled == (SharedHotkeyBase(record.Key) != "")
}
