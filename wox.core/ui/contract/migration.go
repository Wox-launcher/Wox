package contract

import (
	"context"
	"time"

	"wox/common"
)

// MigratedQuicklink is one web search row produced by launcher migration.
type MigratedQuicklink struct {
	ItemID  string
	Title   string
	Keyword string
	URL     string
	Browser string
}

// MigratedFavorite is one file or folder favorite produced by launcher migration.
type MigratedFavorite struct {
	ItemID string
	Name   string
	Path   string
}

// MigratedNote is one note produced by launcher migration.
// Document is set when the source note has structure. An empty document uses Title and Body.
type MigratedNote struct {
	ItemID      string
	Source      string
	SourceID    string
	Fingerprint string
	Title       string
	Body        string
	Document    common.NoteDocument
}

// MigratedClipboard is clipboard content produced by launcher migration.
type MigratedClipboard struct {
	ItemID    string
	Text      string
	ImagePath string
	FilePaths []string
	OCRText   string
	CreatedAt time.Time
	Pinned    bool
}

// MigratedResultBinding is an alias and hotkey for one result found by a plugin query.
type MigratedResultBinding struct {
	ItemID              string
	PluginID            string
	Search              string
	Command             string
	TriggerKeyword      string
	IdentityKey         string
	Path                string
	Title               string
	Keyword             string
	RequireSettingsPath bool
	Alias               string
	Hotkey              string
}

// MigratedItemResult is one migration write outcome. Status and ErrorCode are stable codes.
type MigratedItemResult struct {
	ID        string
	Status    string
	ErrorCode string
}

// MigrationImportServices writes migrated rows through the owning plugins.
type MigrationImportServices interface {
	ImportMigratedQuicklinks(ctx context.Context, sessionID string, items []MigratedQuicklink) []MigratedItemResult
	ImportMigratedFavorites(ctx context.Context, sessionID string, items []MigratedFavorite) []MigratedItemResult
	ImportMigratedNotes(ctx context.Context, sessionID string, items []MigratedNote) []MigratedItemResult
	ImportMigratedClipboard(ctx context.Context, sessionID string, items []MigratedClipboard) []MigratedItemResult
	ImportMigratedResultBindings(ctx context.Context, sessionID string, items []MigratedResultBinding) []MigratedItemResult
}
