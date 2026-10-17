package raycast

import (
	"strconv"
	"strings"
	"time"

	"wox/setting"
)

// woxSnapshot is the Wox state used to mark rows that already exist or conflict.
type woxSnapshot struct {
	MainHotkey       string
	QueryHotkeys     []queryRow
	QueryAliases     []queryRow
	AutostartKnown   bool
	Autostart        bool
	ShowTrayKnown    bool
	ShowTray         bool
	WebSearches      []webSearchRow
	Favorites        []favoriteRow
	OccupiedKeywords map[string]struct{}
	Notes            []noteRow
	ClipboardTexts   map[string]clipboardHit
	ClipboardImages  map[string]clipboardHit
	ResultAliases    []setting.ResultBinding
	ClipboardTrigger string
	EmojiTrigger     string
	TextDays         int
	ImageDays        int
	HistoryCount     int
	MaxHistory       int
	Now              time.Time
}

type queryRow struct {
	Hotkey string
	Alias  string
	Query  string
}

type webSearchRow struct {
	Keyword string
	URL     string
	Browser string
	Title   string
}

type favoriteRow struct {
	Name string
	Path string
}

type noteRow struct {
	Title       string
	Body        string
	Fingerprint string
	SourceID    string
	Deleted     bool
}

type clipboardHit struct {
	Favorite bool
}

func (s woxSnapshot) now() time.Time {
	if s.Now.IsZero() {
		return time.Now()
	}
	return s.Now
}

func (s woxSnapshot) clipboardTrigger() string {
	if strings.TrimSpace(s.ClipboardTrigger) == "" {
		return "cb"
	}
	return s.ClipboardTrigger
}

func (s woxSnapshot) emojiTrigger() string {
	if strings.TrimSpace(s.EmojiTrigger) == "" {
		return "emoji"
	}
	return s.EmojiTrigger
}

func buildMapped(payload Payload, current woxSnapshot) mappedExport {
	mapped := mappedExport{}
	mapped.hotkeys = mapHotkeyRows(payload, current)
	mapped.queries = mapQueryRows(payload, current)
	mapped.general = generalSettings(payload.Settings.General, current)
	mapped.quicklinks = mapQuicklinkRows(payload, current)
	mapped.snippets = mapSnippetRows(payload, current)
	mapped.clipboard, mapped.summary = mapClipboardRows(payload, current)
	return mapped
}

func mapHotkeyRows(payload Payload, current woxSnapshot) []mappedHotkey {
	var rows []mappedHotkey
	if raw, ok := payload.Settings.General["globalHotkey"]; ok {
		if row, ok := oneHotkey("setting:main_hotkey", "main_hotkey", "", raw, current); ok {
			row.kind = "main"
			rows = append(rows, row)
		}
	}
	for index, command := range payload.Settings.Commands {
		raw := commandHotkey(command)
		if len(raw) == 0 {
			continue
		}
		id := "setting:query_hotkey:" + strconv.Itoa(index)
		query, detailKey, detail, supported := commandQuery(command, current)
		value, keyDetail, rawText, ok := mapHotkey(raw)
		row := mappedHotkey{id: id, title: commandTitle(command), kind: "query", query: query, hotkey: value}
		switch {
		case !ok:
			row.detailKey = keyDetail
			row.detail = rawText
			row.status = statusUnsupported
		case !supported:
			row.detailKey = detailKey
			row.detail = detail
			row.status = statusUnsupported
		default:
			row.detail = hotkeyLabel(value) + "  " + query
			row.selectable = true
			if match, exists := hotkeyMatch(current.QueryHotkeys, value); exists {
				row.selectable = false
				if strings.TrimSpace(match.Query) == strings.TrimSpace(query) {
					row.status = statusExists
				} else {
					row.status = statusConflict
					row.detailKey = "raycast_conflict"
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func oneHotkey(id, titleKey, query string, raw []byte, current woxSnapshot) (mappedHotkey, bool) {
	value, detailKey, rawText, ok := mapHotkey(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return mappedHotkey{}, false
	}
	row := mappedHotkey{id: id, titleKey: titleKey, hotkey: value, query: query, detail: hotkeyLabel(value)}
	if !ok {
		row.detailKey = detailKey
		row.detail = rawText
		row.status = statusUnsupported
		return row, true
	}
	// The next onboarding step shows this chord and reports when another program holds it.
	row.selectable = true
	return row, true
}

func commandQuery(command Command, current woxSnapshot) (query, detailKey, detail string, supported bool) {
	switch command.ExtensionID {
	case "e:r:clipboard-history":
		return current.clipboardTrigger(), "", "", true
	case "e:r:emoji-picker":
		return current.emojiTrigger(), "", "", true
	case "e:r:applications":
		path := appPathFromCommand(command.ID)
		name, ok := resolveApp(path)
		if !ok {
			label := name
			if label == "" {
				label = path
			}
			return "", "raycast_app_missing", label, false
		}
		return appQuery(name), "", "", true
	default:
		return "", "raycast_unsupported_command", "", false
	}
}

func hotkeyMatch(rows []queryRow, value string) (queryRow, bool) {
	for _, row := range rows {
		if sameHotkey(row.Hotkey, value) {
			return row, true
		}
	}
	return queryRow{}, false
}

func mapQueryRows(payload Payload, current woxSnapshot) []mappedQuery {
	var rows []mappedQuery
	for index, command := range payload.Settings.Commands {
		alias := firstAliasWord(command.Alias)
		if alias == "" {
			continue
		}
		id := "setting:query_alias:" + strconv.Itoa(index)
		row := mappedQuery{id: id, title: commandTitle(command), alias: alias}
		if command.ExtensionID != "e:r:applications" {
			row.status = statusUnsupported
			row.detailKey = "raycast_unsupported_command"
			rows = append(rows, row)
			continue
		}
		path := appPathFromCommand(command.ID)
		name, ok := resolveApp(path)
		if !ok {
			label := path
			if name != "" {
				label = name
			}
			row.status = statusUnsupported
			row.detailKey = "raycast_app_missing"
			row.detail = label
			rows = append(rows, row)
			continue
		}
		row.query = appQuery(name)
		row.detail = alias + "  " + row.query
		if !aliasAllowed(alias, row.query, current.ResultAliases) {
			row.status = statusUnsupported
			row.detailKey = "raycast_alias_rejected"
			rows = append(rows, row)
			continue
		}
		row.selectable = true
		if match, exists := aliasMatch(current.QueryAliases, alias); exists {
			row.selectable = false
			if strings.TrimSpace(match.Query) == row.query {
				row.status = statusExists
			} else {
				row.status = statusConflict
				row.detailKey = "raycast_conflict"
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func aliasMatch(rows []queryRow, alias string) (queryRow, bool) {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Alias), strings.TrimSpace(alias)) {
			return row, true
		}
	}
	return queryRow{}, false
}
