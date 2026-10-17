package notes

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"wox/common"
	"wox/util"
)

const (
	// ImportStatusImported means a new note was stored.
	ImportStatusImported = "imported"
	// ImportStatusExists means the same snippet is already stored.
	ImportStatusExists = "exists"
	// ImportStatusConflict means the stored note was edited and is left unchanged.
	ImportStatusConflict = "conflict"
	// ImportStatusFailed means the note could not be stored.
	ImportStatusFailed = "failed"
)

// ImportedNote is one plain-text note created from a migrated snippet.
type ImportedNote struct {
	Source      string
	SourceID    string
	Fingerprint string
	Title       string
	Body        string
}

// ImportedNoteResult is the outcome for one ImportedNote.
type ImportedNoteResult struct {
	Status    string
	ErrorCode string
}

// ImportPlainNotes stores migrated snippets through the Notes repository.
func (p *Plugin) ImportPlainNotes(items []ImportedNote) []ImportedNoteResult {
	if p == nil || p.repository == nil {
		results := make([]ImportedNoteResult, len(items))
		for index := range results {
			results[index] = ImportedNoteResult{Status: ImportStatusFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	return p.repository.ImportPlain(items)
}

// ImportPlain stores notes without replacing an edited copy of the same snippet.
func (r *Repository) ImportPlain(items []ImportedNote) []ImportedNoteResult {
	results := make([]ImportedNoteResult, len(items))
	if r == nil {
		for index := range results {
			results[index] = ImportedNoteResult{Status: ImportStatusFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	records, err := r.List(true)
	if err != nil {
		for index := range results {
			results[index] = ImportedNoteResult{Status: ImportStatusFailed, ErrorCode: "failed"}
		}
		return results
	}
	titles := map[string]struct{}{}
	for _, record := range records {
		if record.DeletedAt != 0 {
			continue
		}
		if title := CustomNoteTitle(record.Document); title != "" {
			titles[title] = struct{}{}
		}
	}
	for index, item := range items {
		results[index] = r.importOnePlain(item, records, titles)
	}
	return results
}

func (r *Repository) importOnePlain(item ImportedNote, records []common.NoteRecord, titles map[string]struct{}) ImportedNoteResult {
	title := strings.TrimSpace(item.Title)
	body := strings.ReplaceAll(item.Body, "\r\n", "\n")
	if title == "" {
		return ImportedNoteResult{Status: ImportStatusFailed, ErrorCode: "failed"}
	}
	fingerprint := strings.TrimSpace(item.Fingerprint)
	for _, record := range records {
		if record.DeletedAt != 0 || fingerprint == "" || record.ImportFingerprint != fingerprint {
			continue
		}
		if noteBody(record.Document) == body {
			return ImportedNoteResult{Status: ImportStatusExists}
		}
		return ImportedNoteResult{Status: ImportStatusConflict, ErrorCode: "conflict"}
	}
	for _, record := range records {
		if record.DeletedAt != 0 {
			continue
		}
		if CustomNoteTitle(record.Document) == title && noteBody(record.Document) == body {
			return ImportedNoteResult{Status: ImportStatusExists}
		}
	}
	storedTitle := uniqueNoteTitle(title, titles)
	now := util.GetSystemTimestamp()
	record := common.NoteRecord{
		SchemaVersion:     noteSchemaVersion,
		ID:                uuid.NewString(),
		Document:          plainImportDocument(storedTitle, body),
		CreatedAt:         now,
		UpdatedAt:         now,
		Revision:          uuid.NewString(),
		ImportSource:      strings.TrimSpace(item.Source),
		ImportFingerprint: fingerprint,
	}
	if err := r.write(record); err != nil {
		return ImportedNoteResult{Status: ImportStatusFailed, ErrorCode: "failed"}
	}
	titles[storedTitle] = struct{}{}
	go r.notify(record.ID)
	return ImportedNoteResult{Status: ImportStatusImported}
}

func plainImportDocument(title, body string) common.NoteDocument {
	text := title
	if strings.TrimSpace(body) != "" {
		text += "\n\n" + body
	}
	return common.NoteDocument{Version: documentVersion, Blocks: []common.NoteBlock{{
		Type: common.NoteBlockParagraph, Text: text,
	}}}
}

func noteBody(document common.NoteDocument) string {
	plain := strings.ReplaceAll(ToPlainText(document), "\r\n", "\n")
	title := CustomNoteTitle(document)
	rest := strings.TrimPrefix(plain, title)
	return strings.Trim(rest, "\n")
}

func uniqueNoteTitle(title string, used map[string]struct{}) string {
	if _, exists := used[title]; !exists {
		return title
	}
	for number := 2; ; number++ {
		candidate := fmt.Sprintf("%s (%d)", title, number)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}
