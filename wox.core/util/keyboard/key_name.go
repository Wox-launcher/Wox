package keyboard

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// KeyName normalizes key names without requiring a global registration backend to support them.
// Local editors also use navigation keys that are not exposed by every native hotkey backend.
func KeyName(token string) (string, error) {
	name := normalizeKeyAlias(token)
	switch name {
	case "backspace", "home", "end", "pageup", "pagedown":
		return name, nil
	}
	key, err := ParseKey(name)
	if err != nil {
		// Local key events can carry printable Unicode even when global registration has no native key code.
		if utf8.RuneCountInString(name) == 1 {
			character, _ := utf8.DecodeRuneInString(name)
			if unicode.IsPrint(character) && !unicode.IsSpace(character) {
				return name, nil
			}
		}
		return "", err
	}
	switch key {
	case KeySpace:
		return "space", nil
	case KeyReturn:
		return "return", nil
	case KeyEscape:
		return "escape", nil
	case KeyTab:
		return "tab", nil
	case KeyDelete:
		return "delete", nil
	case KeyLeft:
		return "left", nil
	case KeyRight:
		return "right", nil
	case KeyUp:
		return "up", nil
	case KeyDown:
		return "down", nil
	case KeyCapsLock:
		return "capslock", nil
	case KeyBackquote:
		return "backquote", nil
	}
	return key.Character(), nil
}

// normalizeKeyAlias gives native registration and local shortcuts the same spelling rules.
func normalizeKeyAlias(token string) string {
	switch name := strings.ToLower(strings.TrimSpace(token)); name {
	case "enter":
		return "return"
	case "esc":
		return "escape"
	case "del":
		return "delete"
	case "arrow-left":
		return "left"
	case "arrow-right":
		return "right"
	case "arrow-up":
		return "up"
	case "arrow-down":
		return "down"
	case "page-up":
		return "pageup"
	case "page-down":
		return "pagedown"
	default:
		return name
	}
}
