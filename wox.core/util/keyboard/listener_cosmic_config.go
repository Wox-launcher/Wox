package keyboard

import (
	"fmt"
	"strconv"
	"strings"
	"text/scanner"
)

const cosmicHotkeyDescription = "Wox global hotkey"

// cosmicShortcut retains the original RON so edits never reserialize user actions.
type cosmicShortcut struct {
	modifiers                 Modifier
	key, description, command string
	raw                       string
	keycode                   bool
}

type cosmicShortcutConfig struct {
	prefix, suffix string
	entries        []cosmicShortcut
}

type cosmicRONToken struct {
	text       string
	start, end int
}

// parseCosmicShortcuts accepts the shortcut schema emitted by COSMIC Settings.
// Unsupported RON syntax fails closed; it must never turn an unreadable config into an empty map.
func parseCosmicShortcuts(source string) (cosmicShortcutConfig, error) {
	var scan scanner.Scanner
	scan.Init(strings.NewReader(source))
	scan.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanStrings | scanner.ScanComments | scanner.SkipComments
	var scanErr error
	scan.Error = func(s *scanner.Scanner, message string) {
		scanErr = fmt.Errorf("COSMIC shortcuts at %s: %s", s.Position, message)
	}
	var tokens []cosmicRONToken
	for tok := scan.Scan(); tok != scanner.EOF; tok = scan.Scan() {
		tokens = append(tokens, cosmicRONToken{scan.TokenText(), scan.Position.Offset, scan.Pos().Offset})
	}
	if scanErr != nil {
		return cosmicShortcutConfig{}, scanErr
	}
	if len(tokens) < 2 || tokens[0].text != "{" || tokens[len(tokens)-1].text != "}" {
		return cosmicShortcutConfig{}, fmt.Errorf("COSMIC shortcuts must be a RON map")
	}
	config := cosmicShortcutConfig{prefix: source[:tokens[0].end]}
	previousEnd := tokens[0].end
	for i := 1; i < len(tokens)-1; {
		if tokens[i].text != "(" {
			return config, fmt.Errorf("invalid COSMIC shortcut binding")
		}
		end, err := cosmicRONEnd(tokens, i)
		if err != nil {
			return config, err
		}
		entry, err := parseCosmicBinding(tokens[i+1 : end-1])
		if err != nil {
			return config, err
		}
		if end >= len(tokens)-1 || tokens[end].text != ":" {
			return config, fmt.Errorf("missing COSMIC shortcut action")
		}
		i = end + 1
		actionStart := i
		if i >= len(tokens)-1 || !cosmicRONIdentifier(tokens[i].text) {
			return config, fmt.Errorf("invalid COSMIC shortcut action")
		}
		i++
		if i < len(tokens)-1 && tokens[i].text == "(" {
			i, err = cosmicRONEnd(tokens, i)
			if err != nil {
				return config, err
			}
		}
		if tokens[actionStart].text == "Spawn" && i-actionStart == 4 {
			entry.command, err = strconv.Unquote(tokens[actionStart+2].text)
			if err != nil {
				return config, fmt.Errorf("invalid COSMIC Spawn command: %w", err)
			}
		}
		entryEnd := tokens[i-1].end
		if i < len(tokens)-1 {
			if tokens[i].text != "," {
				return config, fmt.Errorf("missing comma after COSMIC shortcut")
			}
			entryEnd = tokens[i].end
			i++
		}
		entry.raw = source[previousEnd:entryEnd]
		if tokens[i-1].text != "," {
			entry.raw += ","
		}
		previousEnd = entryEnd
		config.entries = append(config.entries, entry)
	}
	config.suffix = source[previousEnd:]
	return config, nil
}

func cosmicRONIdentifier(value string) bool {
	return value != "" && (value[0] >= 'A' && value[0] <= 'Z' || value[0] >= 'a' && value[0] <= 'z' || value[0] == '_')
}

// cosmicRONEnd skips a balanced RON value while respecting tokenized strings and comments.
func cosmicRONEnd(tokens []cosmicRONToken, start int) (int, error) {
	var stack []string
	for i := start; i < len(tokens); i++ {
		switch tokens[i].text {
		case "(":
			stack = append(stack, ")")
		case "[":
			stack = append(stack, "]")
		case "{":
			stack = append(stack, "}")
		case ")", "]", "}":
			if len(stack) == 0 || stack[len(stack)-1] != tokens[i].text {
				return 0, fmt.Errorf("unbalanced COSMIC shortcut config")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unterminated COSMIC shortcut config")
}

// parseCosmicBinding reads only conflict/ownership fields; unknown fields stop mutation.
func parseCosmicBinding(tokens []cosmicRONToken) (cosmicShortcut, error) {
	var entry cosmicShortcut
	seen := map[string]bool{}
	for i := 0; i < len(tokens); {
		field := tokens[i].text
		if seen[field] || i+2 >= len(tokens) || tokens[i+1].text != ":" {
			return entry, fmt.Errorf("invalid COSMIC binding field %q", field)
		}
		seen[field] = true
		i += 2
		switch field {
		case "modifiers":
			if tokens[i].text != "[" {
				return entry, fmt.Errorf("invalid COSMIC modifiers")
			}
			i++
			for i < len(tokens) && tokens[i].text != "]" {
				switch tokens[i].text {
				case "Ctrl":
					entry.modifiers |= ModifierCtrl
				case "Alt":
					entry.modifiers |= ModifierAlt
				case "Shift":
					entry.modifiers |= ModifierShift
				case "Super":
					entry.modifiers |= ModifierSuper
				default:
					return entry, fmt.Errorf("unknown COSMIC modifier %q", tokens[i].text)
				}
				i++
				if i < len(tokens) && tokens[i].text == "," {
					i++
				} else if i < len(tokens) && tokens[i].text != "]" {
					return entry, fmt.Errorf("invalid COSMIC modifier separator")
				}
			}
			if i >= len(tokens) {
				return entry, fmt.Errorf("unterminated COSMIC modifiers")
			}
			i++
		case "key":
			value, err := strconv.Unquote(tokens[i].text)
			if err != nil {
				return entry, fmt.Errorf("invalid COSMIC key: %w", err)
			}
			entry.key = value
			i++
		case "description", "keycode":
			if tokens[i].text == "None" {
				i++
				break
			}
			if i+3 >= len(tokens) || tokens[i].text != "Some" || tokens[i+1].text != "(" || tokens[i+3].text != ")" {
				return entry, fmt.Errorf("invalid COSMIC %s", field)
			}
			if field == "description" {
				value, err := strconv.Unquote(tokens[i+2].text)
				if err != nil {
					return entry, err
				}
				entry.description = value
			} else {
				if _, err := strconv.ParseUint(tokens[i+2].text, 10, 32); err != nil {
					return entry, err
				}
				entry.keycode = true
			}
			i += 4
		default:
			return entry, fmt.Errorf("unsupported COSMIC binding field %q", field)
		}
		if i < len(tokens) {
			if tokens[i].text != "," {
				return entry, fmt.Errorf("invalid COSMIC binding separator")
			}
			i++
		}
	}
	if !seen["modifiers"] {
		return entry, fmt.Errorf("COSMIC binding has no modifiers field")
	}
	return entry, nil
}

func (config cosmicShortcutConfig) render() string {
	var out strings.Builder
	out.WriteString(config.prefix)
	for _, entry := range config.entries {
		out.WriteString(entry.raw)
	}
	out.WriteString(config.suffix)
	return out.String()
}
