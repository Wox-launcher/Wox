package raycast

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode"

	"wox/common"
)

const raycastClipboardExtensionID = "e:r:clipboard-history"

// clipboardSettingsMeta is the Raycast clipboard preferences Wox can store.
// Universal clipboard and the Pro history upgrade have no Wox setting.
type clipboardSettingsMeta struct {
	HistoryDuration        string `json:"historyDuration"`
	TextRecognitionEnabled *bool  `json:"textRecognitionEnabled"`
}

// mapClipboardSettings maps clipboard preferences onto the Wox clipboard plugin.
// Keep history for becomes the text retention day count. Image retention stays unchanged.
func mapClipboardSettings(payload Payload, current woxSnapshot) []mappedSetting {
	meta, ok := clipboardSettingsFrom(payload)
	if !ok {
		return nil
	}
	var rows []mappedSetting
	if days, ok := raycastHistoryDays(meta.HistoryDuration); ok && days != current.textHistoryDays() {
		rows = append(rows, mappedSetting{
			id: "setting:clipboard_text_days", titleKey: "clipboard_text_days",
			key: "text_history_days", value: strconv.Itoa(days), detail: strconv.Itoa(days), detailKey: "clipboard_days",
			pluginID: common.ClipboardPluginID, selectable: true,
		})
	}
	if meta.TextRecognitionEnabled != nil {
		known := current.ImageTextRecognitionKnown
		value := current.ImageTextRecognition
		if !known {
			known = true
			value = true
		}
		row := boolMappedSetting("setting:clipboard_text_recognition", "clipboard_text_recognition", "image_text_recognition_enabled", *meta.TextRecognitionEnabled, known, value)
		row.pluginID = common.ClipboardPluginID
		rows = append(rows, row)
	}
	return rows
}

func clipboardSettingsFrom(payload Payload) (clipboardSettingsMeta, bool) {
	for _, extension := range payload.Settings.InternalExtensions {
		if extension.ID != raycastClipboardExtensionID || len(extension.Meta) == 0 {
			continue
		}
		var meta clipboardSettingsMeta
		if json.Unmarshal(extension.Meta, &meta) != nil {
			return clipboardSettingsMeta{}, false
		}
		return meta, true
	}
	return clipboardSettingsMeta{}, false
}

func (s woxSnapshot) textHistoryDays() int {
	if s.TextDays <= 0 {
		return 90
	}
	return s.TextDays
}

// raycastHistoryDays converts a Raycast ISO-8601 duration into the day count Wox stores.
// A month is 30 days and a year is 365 days.
func raycastHistoryDays(raw string) (int, bool) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if len(raw) < 3 || raw[0] != 'P' || strings.Contains(raw, "T") {
		return 0, false
	}
	total := 0
	number := 0
	sawDigit := false
	for _, char := range raw[1:] {
		if unicode.IsDigit(char) {
			number = number*10 + int(char-'0')
			sawDigit = true
			continue
		}
		if !sawDigit {
			return 0, false
		}
		switch char {
		case 'Y':
			total += number * 365
		case 'M':
			total += number * 30
		case 'W':
			total += number * 7
		case 'D':
			total += number
		default:
			return 0, false
		}
		number = 0
		sawDigit = false
	}
	if sawDigit || total <= 0 {
		return 0, false
	}
	return total, true
}
