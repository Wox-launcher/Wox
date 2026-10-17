package raycast

import (
	"encoding/json"
	"strconv"
	"strings"

	"wox/util/hotkey"
)

func boolSetting(general map[string]json.RawMessage, key string) (bool, bool) {
	raw, ok := general[key]
	if !ok || len(raw) == 0 {
		return false, false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

func generalSettings(general map[string]json.RawMessage, current woxSnapshot) []mappedSetting {
	var items []mappedSetting
	if value, ok := boolSetting(general, "openAtLogin"); ok {
		items = append(items, boolMappedSetting("setting:autostart", "autostart", "EnableAutostart", value, current.AutostartKnown, current.Autostart))
	}
	if value, ok := boolSetting(general, "showInMenuBar"); ok {
		items = append(items, boolMappedSetting("setting:show_tray", "show_tray", "ShowTray", value, current.ShowTrayKnown, current.ShowTray))
	}
	return items
}

func boolMappedSetting(id, titleKey, key string, value, known, current bool) mappedSetting {
	item := mappedSetting{
		id: id, titleKey: titleKey, key: key, value: strconv.FormatBool(value),
		detailKey: boolDetail(value), selectable: true,
	}
	if known && value == current {
		item.selectable = false
		item.status = statusExists
	}
	return item
}

func boolDetail(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func sameHotkey(left, right string) bool {
	leftKey, leftErr := hotkey.BindingKey(left)
	rightKey, rightErr := hotkey.BindingKey(right)
	if leftErr == nil && rightErr == nil {
		return leftKey == rightKey
	}
	return strings.EqualFold(strings.ReplaceAll(left, " ", ""), strings.ReplaceAll(right, " ", ""))
}
