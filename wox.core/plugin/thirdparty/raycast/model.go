package raycast

import (
	"time"

	"wox/common"
)

const (
	statusExists      = "imported"
	statusConflict    = "conflict"
	statusUnsupported = "unsupported"
)

// mappedExport is the session data kept after the raw JSON is released.
type mappedExport struct {
	hotkeys           []mappedHotkey
	shortcuts         []mappedShortcut
	queries           []mappedQuery
	general           []mappedSetting
	fileSearch        []mappedSetting
	fileRoots         []string
	fileIgnore        []string
	contentRoots      []string
	quicklinks        []mappedQuicklink
	snippets          []mappedSnippet
	notes             []mappedNote
	clipboard         []mappedClipboard
	clipboardSettings []mappedSetting
	summary           clipboardSummary
}

type mappedHotkey struct {
	id         string
	titleKey   string
	title      string
	detailKey  string
	detail     string
	hotkey     string
	query      string
	selectable bool
	status     string
	kind       string
}

type mappedShortcut struct {
	id                   string
	commandIndex         int
	title                string
	detail               string
	alias                string
	hotkey               string
	pluginID             string
	search               string
	command              string
	trigger              string
	identityKey          string
	path                 string
	matchTitle           string
	keyword              string
	requireSettings      bool
	selectable           bool
	status               string
	prerequisiteID       string
	prerequisiteFavorite bool
	prerequisiteName     string
	prerequisitePath     string
	prerequisiteKeyword  string
	prerequisiteURL      string
	prerequisiteBrowser  string
}

type mappedQuery struct {
	id         string
	title      string
	alias      string
	query      string
	detailKey  string
	detail     string
	selectable bool
	status     string
}

type mappedSetting struct {
	id         string
	titleKey   string
	title      string
	key        string
	value      string
	detailKey  string
	detail     string
	pluginID   string
	merge      string
	selectable bool
	status     string
}

type mappedQuicklink struct {
	id         string
	sourceID   string
	title      string
	keyword    string
	url        string
	path       string
	browser    string
	favorite   bool
	detailKey  string
	detail     string
	selectable bool
	status     string
}

type mappedNote struct {
	id         string
	sourceID   string
	title      string
	body       string
	detail     string
	document   common.NoteDocument
	selectable bool
	status     string
}

type mappedSnippet struct {
	id          string
	sourceID    string
	title       string
	text        string
	keyword     string
	keepKeyword bool
	detailKey   string
	detail      string
	selectable  bool
	status      string
}

type mappedClipboard struct {
	id         string
	text       string
	imagePath  string
	filePaths  []string
	ocrText    string
	imageHash  string
	createdAt  time.Time
	pinned     bool
	detailKey  string
	detail     string
	selectable bool
	status     string
}

type clipboardSummary struct {
	importable int
	duplicate  int
	expired    int
	capacity   int
	missing    int
}
