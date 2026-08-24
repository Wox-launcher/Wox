package ui

import (
	"context"
	"errors"

	"wox/common"
	"wox/plugin"
	"wox/ui/contract"
)

func notesServices() (contract.NotesServices, error) {
	instance := plugin.GetPluginManager().GetSystemPlugin(common.NotesPluginID)
	services, ok := instance.(contract.NotesServices)
	if !ok || services == nil {
		return nil, errors.New("Notes plugin is unavailable")
	}
	return services, nil
}

func (s *CoreServices) NotesList(ctx context.Context, query string, includeDeleted bool) ([]common.NoteSummary, error) {
	services, err := notesServices()
	if err != nil {
		return nil, err
	}
	return services.NotesList(ctx, query, includeDeleted)
}

func (s *CoreServices) NotesGet(ctx context.Context, id string) (common.NoteRecord, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteRecord{}, err
	}
	return services.NotesGet(ctx, id)
}

func (s *CoreServices) NotesCreate(ctx context.Context) (common.NoteRecord, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteRecord{}, err
	}
	return services.NotesCreate(ctx)
}

func (s *CoreServices) NotesSave(ctx context.Context, id, expectedRevision string, document common.NoteDocument) (common.NoteSaveResult, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteSaveResult{}, err
	}
	return services.NotesSave(ctx, id, expectedRevision, document)
}

func (s *CoreServices) NotesSetPinned(ctx context.Context, id string, pinned bool) (common.NoteRecord, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteRecord{}, err
	}
	return services.NotesSetPinned(ctx, id, pinned)
}

func (s *CoreServices) NotesDelete(ctx context.Context, id string) (common.NoteRecord, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteRecord{}, err
	}
	return services.NotesDelete(ctx, id)
}

// NotesDiscard permanently removes a note, including empty drafts that were never saved.
func (s *CoreServices) NotesDiscard(ctx context.Context, id string) error {
	services, err := notesServices()
	if err != nil {
		return err
	}
	return services.NotesDiscard(ctx, id)
}

func (s *CoreServices) NotesRestore(ctx context.Context, id string) (common.NoteRecord, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteRecord{}, err
	}
	return services.NotesRestore(ctx, id)
}

func (s *CoreServices) NotesExport(ctx context.Context, id, format string) (common.NoteExport, error) {
	services, err := notesServices()
	if err != nil {
		return common.NoteExport{}, err
	}
	return services.NotesExport(ctx, id, format)
}

func (s *CoreServices) NotesGetLocal(ctx context.Context, key string) (string, error) {
	services, err := notesServices()
	if err != nil {
		return "", err
	}
	return services.NotesGetLocal(ctx, key)
}

func (s *CoreServices) NotesSetLocal(ctx context.Context, key, value string) error {
	services, err := notesServices()
	if err != nil {
		return err
	}
	return services.NotesSetLocal(ctx, key, value)
}
