package launcher

import (
	"runtime"
	"strings"
)

// formatHotkeyLabels applies Flutter's platform labels while keeping each physical key separate.
func formatHotkeyLabels(hotkey string) []string {
	hotkey = strings.TrimSpace(hotkey)
	hotkey = strings.TrimPrefix(hotkey, "hold:")
	parts := strings.Split(strings.TrimSpace(hotkey), "+")
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		switch strings.ToLower(part) {
		case "cmd", "command", "meta":
			if runtime.GOOS == "darwin" {
				part = "Cmd"
			} else if runtime.GOOS == "windows" {
				part = "Win"
			} else {
				part = "Super"
			}
		case "ctrl", "control":
			part = "Ctrl"
		case "alt", "option":
			if runtime.GOOS == "darwin" {
				part = "Option"
			} else {
				part = "Alt"
			}
		case "shift":
			part = "Shift"
		case "capslock", "caps_lock", "caps lock":
			part = "CapsLock"
		case "left_ctrl":
			part = "Left Ctrl"
		case "right_ctrl":
			part = "Right Ctrl"
		case "left_shift":
			part = "Left Shift"
		case "right_shift":
			part = "Right Shift"
		case "left_alt":
			if runtime.GOOS == "darwin" {
				part = "Left Option"
			} else {
				part = "Left Alt"
			}
		case "right_alt":
			if runtime.GOOS == "darwin" {
				part = "Right Option"
			} else {
				part = "Right Alt"
			}
		case "left_cmd", "left_win":
			if runtime.GOOS == "darwin" {
				part = "Left Cmd"
			} else if runtime.GOOS == "windows" {
				part = "Left Win"
			} else {
				part = "Left Super"
			}
		case "right_cmd", "right_win":
			if runtime.GOOS == "darwin" {
				part = "Right Cmd"
			} else if runtime.GOOS == "windows" {
				part = "Right Win"
			} else {
				part = "Right Super"
			}
		case "enter", "return":
			part = "⏎"
		case "space":
			part = "Space"
		case "escape", "esc":
			part = "Esc"
		case "backspace":
			part = "Backspace"
		case "delete", "del":
			part = "Delete"
		case "tab":
			part = "Tab"
		case "backquote", "tilde":
			part = "~"
		case "arrowup", "up":
			part = "↑"
		case "arrowdown", "down":
			part = "↓"
		case "arrowleft", "left":
			part = "←"
		case "arrowright", "right":
			part = "→"
		case "pageup":
			part = "PageUp"
		case "pagedown":
			part = "PageDown"
		case "home":
			part = "Home"
		case "end":
			part = "End"
		case "insert":
			part = "Insert"
		case "numlock":
			part = "NumLock"
		case "scrolllock":
			part = "ScrollLock"
		case "pause":
			part = "Pause"
		case "printscreen":
			part = "PrintScreen"
		default:
			if len([]rune(part)) == 1 {
				part = strings.ToUpper(part)
			}
		}
		if part != "" {
			labels = append(labels, part)
		}
	}
	return labels
}
