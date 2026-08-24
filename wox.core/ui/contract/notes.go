package contract

import (
	"context"

	"wox/common"
)

// NotesServices exposes the built-in Notes repository to the native utility window.
type NotesServices interface {
	NotesList(ctx context.Context, query string, includeDeleted bool) ([]common.NoteSummary, error)
	NotesGet(ctx context.Context, id string) (common.NoteRecord, error)
	NotesCreate(ctx context.Context) (common.NoteRecord, error)
	NotesSave(ctx context.Context, id, expectedRevision string, document common.NoteDocument) (common.NoteSaveResult, error)
	NotesSetPinned(ctx context.Context, id string, pinned bool) (common.NoteRecord, error)
	NotesDelete(ctx context.Context, id string) (common.NoteRecord, error)
	NotesDiscard(ctx context.Context, id string) error
	NotesRestore(ctx context.Context, id string) (common.NoteRecord, error)
	NotesExport(ctx context.Context, id, format string) (common.NoteExport, error)
	NotesGetLocal(ctx context.Context, key string) (string, error)
	NotesSetLocal(ctx context.Context, key, value string) error
}
