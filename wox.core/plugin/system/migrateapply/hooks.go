// Package migrateapply holds import callbacks registered by system plugins.
// The UI calls these hooks so it does not import plugins that already import the UI.
package migrateapply

import (
	"context"
	"time"
)

// Quicklink is one Web Search row produced by a migration source.
type Quicklink struct {
	Title   string
	Keyword string
	URL     string
	Browser string
}

// ClipboardItem is one clipboard row still pointing at the source image file.
type ClipboardItem struct {
	Text      string
	ImagePath string
	FilePaths []string
	OCRText   string
	CreatedAt time.Time
	Pinned    bool
}

// Result is the outcome for one migrated row.
type Result struct {
	Status    string
	ErrorCode string
}

// Favorite is one file or folder favorite produced by a migration source.
type Favorite struct {
	Name string
	Path string
}

// ImportQuicklinks is set by the Web Search plugin.
var ImportQuicklinks func(ctx context.Context, items []Quicklink) []Result

// ImportFavorites is set by the Folder plugin.
var ImportFavorites func(ctx context.Context, items []Favorite) []Result

// ImportClipboardItems is set by the Clipboard plugin.
var ImportClipboardItems func(ctx context.Context, items []ClipboardItem) []Result
