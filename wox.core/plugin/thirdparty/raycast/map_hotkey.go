package raycast

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"wox/util/hotkey"
)

// Apple virtual key codes that Wox can store. Source: HIToolbox Events.h.
var layoutKeyNames = map[int]string{
	0: "a", 1: "s", 2: "d", 3: "f", 4: "h", 5: "g", 6: "z", 7: "x", 8: "c", 9: "v",
	11: "b", 12: "q", 13: "w", 14: "e", 15: "r", 16: "y", 17: "t",
	18: "1", 19: "2", 20: "3", 21: "4", 22: "6", 23: "5",
	24: "=", 25: "9", 26: "7", 27: "-", 28: "8", 29: "0",
	30: "]", 31: "o", 32: "u", 33: "[", 34: "i", 35: "p",
	36: "enter", 37: "l", 38: "j", 39: "'", 40: "k", 41: ";", 42: "\\", 43: ",", 44: "/", 45: "n", 46: "m", 47: ".",
	48: "tab", 49: "space", 50: "`", 53: "escape",
	96: "f5", 97: "f6", 98: "f7", 99: "f3", 100: "f8", 101: "f9", 103: "f11", 109: "f10", 111: "f12",
	118: "f4", 120: "f2", 122: "f1",
	123: "left", 124: "right", 125: "down", 126: "up",
}

type raycastHotkey struct {
	Kind struct {
		Shortcut struct {
			Key struct {
				Type string `json:"type"`
				Code *int   `json:"code"`
			} `json:"key"`
			Modifiers []struct {
				Modifier string `json:"modifier"`
			} `json:"modifiers"`
		} `json:"shortcut"`
	} `json:"kind"`
}

// mapHotkey converts one Raycast shortcut into a Wox chord. An unknown modifier drops the whole shortcut.
func mapHotkey(raw json.RawMessage) (value, detailKey, rawText string, ok bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", "", "", false
	}
	var hotkeyValue raycastHotkey
	if err := json.Unmarshal(raw, &hotkeyValue); err != nil {
		return "", "raycast_unknown_key", string(raw), false
	}
	shortcut := hotkeyValue.Kind.Shortcut
	rawText = describeHotkey(shortcut.Key.Type, shortcut.Key.Code, shortcut.Modifiers)
	if shortcut.Key.Type != "LayoutIndependent" || shortcut.Key.Code == nil {
		return "", "raycast_unknown_key", rawText, false
	}
	name, known := layoutIndependentKey(*shortcut.Key.Code)
	if !known {
		return "", "raycast_unknown_key", rawText, false
	}
	parts := make([]string, 0, len(shortcut.Modifiers)+1)
	seen := map[string]struct{}{}
	for _, entry := range shortcut.Modifiers {
		var token string
		switch entry.Modifier {
		case "Meta":
			token = "win"
		case "Ctrl":
			token = "ctrl"
		case "Alt":
			token = "alt"
		case "Shift":
			token = "shift"
		default:
			return "", "raycast_unknown_modifier", rawText, false
		}
		if _, exists := seen[token]; exists {
			continue
		}
		seen[token] = struct{}{}
		parts = append(parts, token)
	}
	parts = append(parts, name)
	value = strings.Join(parts, "+")
	if _, err := hotkey.BindingKey(value); err != nil {
		return "", "raycast_unknown_key", rawText, false
	}
	return value, "", rawText, true
}

// layoutIndependentKey names one physical key.
// Windows Raycast stores these as keyboard scan codes, so 57 is Space.
// macOS exports use Carbon virtual key codes, where 49 is Space.
func layoutIndependentKey(code int) (string, bool) {
	if runtime.GOOS == "windows" {
		name, ok := windowsScanKeyNames[code]
		return name, ok
	}
	name, ok := layoutKeyNames[code]
	return name, ok
}

// windowsScanKeyNames maps set-1 scan codes to Wox key names.
var windowsScanKeyNames = map[int]string{
	1: "escape",
	2: "1", 3: "2", 4: "3", 5: "4", 6: "5", 7: "6", 8: "7", 9: "8", 10: "9", 11: "0",
	12: "-", 13: "=", 15: "tab",
	16: "q", 17: "w", 18: "e", 19: "r", 20: "t", 21: "y", 22: "u", 23: "i", 24: "o", 25: "p",
	26: "[", 27: "]", 28: "enter",
	30: "a", 31: "s", 32: "d", 33: "f", 34: "g", 35: "h", 36: "j", 37: "k", 38: "l",
	39: ";", 40: "'", 41: "`", 43: "\\",
	44: "z", 45: "x", 46: "c", 47: "v", 48: "b", 49: "n", 50: "m", 51: ",", 52: ".", 53: "/",
	57: "space",
	59: "f1", 60: "f2", 61: "f3", 62: "f4", 63: "f5", 64: "f6", 65: "f7", 66: "f8", 67: "f9", 68: "f10",
	71: "home", 72: "up", 73: "pageup", 75: "left", 77: "right", 79: "end", 80: "down", 81: "pagedown",
	82: "insert", 83: "delete",
	87: "f11", 88: "f12",
}

func hotkeyLabel(value string) string {
	parts := strings.Split(value, "+")
	for index, part := range parts {
		switch part {
		case "ctrl":
			parts[index] = "Ctrl"
		case "alt":
			parts[index] = "Alt"
		case "shift":
			parts[index] = "Shift"
		case "win":
			parts[index] = "Win"
		case "space":
			parts[index] = "Space"
		case "enter", "return":
			parts[index] = "Enter"
		case "escape":
			parts[index] = "Esc"
		case "tab":
			parts[index] = "Tab"
		default:
			if len(part) == 1 {
				parts[index] = strings.ToUpper(part)
			}
		}
	}
	return strings.Join(parts, " + ")
}

func describeHotkey(keyType string, code *int, modifiers []struct {
	Modifier string `json:"modifier"`
}) string {
	codeText := "?"
	if code != nil {
		codeText = strconv.Itoa(*code)
	}
	names := make([]string, 0, len(modifiers))
	for _, entry := range modifiers {
		names = append(names, entry.Modifier)
	}
	return fmt.Sprintf("%s %s %s", keyType, codeText, strings.Join(names, "+"))
}

func commandHotkey(command Command) json.RawMessage {
	for _, raw := range []json.RawMessage{command.MacOSHotkey, command.Hotkey, command.WindowsHotkey} {
		if len(raw) > 0 && string(raw) != "null" {
			return raw
		}
	}
	return nil
}

func commandTitle(command Command) string {
	if name := strings.TrimSpace(command.Name); name != "" {
		return name
	}
	if title := strings.TrimSpace(command.Title); title != "" {
		return title
	}
	if id := strings.TrimSpace(command.ExtensionID); id != "" {
		return id
	}
	return strings.TrimSpace(command.ID)
}
