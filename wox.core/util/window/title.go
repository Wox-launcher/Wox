package window

import (
	"strings"
	"unicode/utf8"
)

// ActionTitleMaxRunes is the display cap for window titles embedded in
// actions such as "Paste to %s".
const ActionTitleMaxRunes = 28

// AppName returns the trailing application name from a window title.
// Titles such as "file.go - project - Visual Studio Code" become
// "Visual Studio Code" whether or not the original string is long.
func AppName(title string) string {
	title = strings.TrimSpace(title)
	if name := trailingAppName(title); name != "" {
		return name
	}
	return title
}

// CompactTitle uses AppName, then rune-truncates leftover long names that
// have no application separator.
func CompactTitle(title string, maxRunes int) string {
	title = AppName(title)
	if title == "" || maxRunes <= 0 || utf8.RuneCountInString(title) <= maxRunes {
		return title
	}
	if maxRunes == 1 {
		return "…"
	}
	return string([]rune(title)[:maxRunes-1]) + "…"
}

func trailingAppName(title string) string {
	for _, sep := range []string{" — ", " – ", " - "} {
		if i := strings.LastIndex(title, sep); i >= 0 {
			name := strings.TrimSpace(title[i+len(sep):])
			if name != "" {
				return name
			}
		}
	}
	return ""
}
