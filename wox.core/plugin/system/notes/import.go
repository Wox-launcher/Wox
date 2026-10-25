package notes

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

// ImportedNote is one note created from a migrated snippet or Raycast note.
// Document keeps the source structure. An empty document is stored as Title and Body.
type ImportedNote struct {
	Source      string
	SourceID    string
	Fingerprint string
	Title       string
	Body        string
	Document    common.NoteDocument
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
	document := item.Document
	title := strings.TrimSpace(item.Title)
	body := strings.ReplaceAll(item.Body, "\r\n", "\n")
	if len(document.Blocks) > 0 {
		if title == "" {
			title = CustomNoteTitle(document)
		}
		if body == "" {
			body = noteBody(document)
		}
	}
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
	if len(document.Blocks) == 0 {
		document = plainImportDocument(storedTitle, body)
	} else if storedTitle != title {
		document = retitleImportedNote(document, title, storedTitle)
	}
	now := util.GetSystemTimestamp()
	record := common.NoteRecord{
		SchemaVersion:     noteSchemaVersion,
		ID:                uuid.NewString(),
		Document:          document,
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

// retitleImportedNote renames the first visible line when another note already uses that title.
func retitleImportedNote(document common.NoteDocument, from, to string) common.NoteDocument {
	if from == "" || from == to {
		return document
	}
	for index, block := range document.Blocks {
		line, rest, hasRest := strings.Cut(block.Text, "\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) != from {
			return document
		}
		previous := utf8.RuneCountInString(line)
		if hasRest {
			block.Text = to + "\n" + rest
		} else {
			block.Text = to
		}
		delta := utf8.RuneCountInString(to) - previous
		if delta != 0 {
			for spanIndex, span := range block.Spans {
				if span.Start >= previous {
					span.Start += delta
					span.End += delta
					block.Spans[spanIndex] = span
				}
			}
		}
		document.Blocks[index] = block
		return document
	}
	return document
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
