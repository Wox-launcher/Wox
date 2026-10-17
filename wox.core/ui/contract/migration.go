package contract

import (
	"context"
	"time"
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

// MigratedNote is one plain note produced by launcher migration.
type MigratedNote struct {
	ItemID      string
	Source      string
	SourceID    string
	Fingerprint string
	Title       string
	Body        string
}

// MigratedClipboard is clipboard content produced by launcher migration.
type MigratedClipboard struct {
	ItemID    string
	Text      string
	ImagePath string
	CreatedAt time.Time
	Pinned    bool
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
}
