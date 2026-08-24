package common

// NotesPluginID is the stable identity of the built-in Notes plugin.
const NotesPluginID = "4d8b0a4e-1fd5-4d96-9c51-6d64793683fa"

// NoteBlockType identifies the visual and semantic role of one document block.
type NoteBlockType string

const (
	NoteBlockParagraph NoteBlockType = "paragraph"
	NoteBlockHeading1  NoteBlockType = "heading1"
	NoteBlockHeading2  NoteBlockType = "heading2"
	NoteBlockHeading3  NoteBlockType = "heading3"
	NoteBlockQuote     NoteBlockType = "quote"
	NoteBlockCode      NoteBlockType = "code"
	NoteBlockBullet    NoteBlockType = "bullet"
	NoteBlockOrdered   NoteBlockType = "ordered"
	NoteBlockTask      NoteBlockType = "task"
	NoteBlockDivider   NoteBlockType = "divider"
	NoteMaximumIndent                = 2
)

// NoteSpan stores one inline style range using rune offsets within its block.
type NoteSpan struct {
	Start     int    `json:"start"`
	End       int    `json:"end"`
	Bold      bool   `json:"bold,omitempty"`
	Italic    bool   `json:"italic,omitempty"`
	Underline bool   `json:"underline,omitempty"`
	Strike    bool   `json:"strike,omitempty"`
	Code      bool   `json:"code,omitempty"`
	Link      string `json:"link,omitempty"`
}

// NoteBlock is one editable block in a note document.
type NoteBlock struct {
	ID      string        `json:"id"`
	Type    NoteBlockType `json:"type"`
	Text    string        `json:"text,omitempty"`
	Checked bool          `json:"checked,omitempty"`
	Indent  int           `json:"indent,omitempty"`
	Spans   []NoteSpan    `json:"spans,omitempty"`
}

// NoteDocument is the versioned rich-text payload stored with a note.
type NoteDocument struct {
	Version int         `json:"version"`
	Blocks  []NoteBlock `json:"blocks"`
}

// NoteRecord is the durable unit independently synchronized by Cloud Sync.
type NoteRecord struct {
	SchemaVersion int          `json:"schemaVersion"`
	ID            string       `json:"id"`
	Document      NoteDocument `json:"document"`
	CreatedAt     int64        `json:"createdAt"`
	UpdatedAt     int64        `json:"updatedAt"`
	PinnedAt      int64        `json:"pinnedAt,omitempty"`
	DeletedAt     int64        `json:"deletedAt,omitempty"`
	Revision      string       `json:"revision"`
}

// NoteSummary carries list/search data without duplicating the complete document.
type NoteSummary struct {
	ID        string
	Title     string
	Preview   string
	UpdatedAt int64
	PinnedAt  int64
	DeletedAt int64
}

// NoteSaveResult reports an optimistic save and whether it became a conflict copy.
type NoteSaveResult struct {
	Record   NoteRecord
	Conflict bool
}

// NoteExport contains one encoded note and its file extension.
type NoteExport struct {
	Content   string
	Extension string
}

// NotesWindowAction describes how a plugin request should affect the Notes window.
type NotesWindowAction string

const (
	NotesWindowToggle NotesWindowAction = "toggle"
	NotesWindowOpen   NotesWindowAction = "open"
	NotesWindowNew    NotesWindowAction = "new"
)

// NotesWindowRequest opens, creates, or toggles a native Notes utility window.
type NotesWindowRequest struct {
	Action       NotesWindowAction
	NoteID       string
	ExportFormat string
}
