package raycast

import "time"

const (
	statusExists      = "imported"
	statusConflict    = "conflict"
	statusUnsupported = "unsupported"
)

// mappedExport is the session data kept after the raw JSON is released.
type mappedExport struct {
	hotkeys    []mappedHotkey
	queries    []mappedQuery
	general    []mappedSetting
	quicklinks []mappedQuicklink
	snippets   []mappedSnippet
	clipboard  []mappedClipboard
	summary    clipboardSummary
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
	key        string
	value      string
	detailKey  string
	selectable bool
	status     string
}

type mappedQuicklink struct {
	id         string
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
