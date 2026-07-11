package dictation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"wox/plugin"

	"github.com/google/uuid"
)

const (
	settingKeyDictionary = "dictionary"
	dictionaryMaxEntries = 100
)

// dictionaryEntry is a single phrase the user wants the AI refiner to know and
// spell correctly. Unlike the previous context/wrong/correct triple, this is a
// plain phrase list fed verbatim to the AI prompt when AI refinement is enabled.
type dictionaryEntry struct {
	ID        string `json:"id"`
	Phrase    string `json:"phrase"`
	CreatedAt int64  `json:"createdAt"`
}

type dictionaryStore struct {
	mu      sync.Mutex
	entries []dictionaryEntry
	api     dictationSettingAPI
}

func newDictionaryStore(api dictationSettingAPI) *dictionaryStore {
	return &dictionaryStore{api: api}
}

// load reads persisted dictionary entries. If the stored payload looks like the
// legacy three-column format (has a wrongPhrase field), it is discarded and
// cleared so old data does not interfere with the new phrase-only schema.
func (d *dictionaryStore) load(ctx context.Context) {
	raw := d.api.GetSetting(ctx, settingKeyDictionary)
	if strings.TrimSpace(raw) == "" {
		d.entries = nil
		return
	}

	// Detect legacy format: entries had a WrongPhrase field. When found, clear
	// the store so the user starts fresh with the new phrase-only schema.
	if strings.Contains(raw, "wrongPhrase") {
		d.api.Log(ctx, plugin.LogLevelInfo, "dictation dictionary: legacy format detected, clearing")
		d.entries = nil
		d.save(ctx)
		return
	}

	var entries []dictionaryEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		d.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to parse dictation dictionary: %s", err.Error()))
		d.entries = nil
		return
	}
	d.entries = entries
}

func (d *dictionaryStore) save(ctx context.Context) {
	data, err := json.Marshal(d.entries)
	if err != nil {
		d.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("failed to marshal dictation dictionary: %s", err.Error()))
		return
	}
	d.api.SaveSetting(ctx, settingKeyDictionary, string(data), false)
}

// addOrUpdate adds a phrase to the dictionary. Duplicate phrases (case-insensitive,
// whitespace-normalized) are ignored so the list stays clean.
func (d *dictionaryStore) addOrUpdate(ctx context.Context, phrase string, timestamp int64) error {
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return fmt.Errorf("phrase is required")
	}

	key := normalizeDictionaryText(phrase)
	d.mu.Lock()
	for _, entry := range d.entries {
		if normalizeDictionaryText(entry.Phrase) == key {
			d.mu.Unlock()
			return nil
		}
	}

	d.entries = append([]dictionaryEntry{{
		ID:        uuid.NewString(),
		Phrase:    phrase,
		CreatedAt: timestamp,
	}}, d.entries...)
	if len(d.entries) > dictionaryMaxEntries {
		d.entries = d.entries[:dictionaryMaxEntries]
	}
	d.mu.Unlock()

	d.save(ctx)
	return nil
}

// activePhrases returns the user's phrase list for the AI refiner prompt.
func (d *dictionaryStore) activePhrases() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]string, 0, len(d.entries))
	for _, entry := range d.entries {
		if phrase := strings.TrimSpace(entry.Phrase); phrase != "" {
			out = append(out, phrase)
		}
	}
	return out
}

func normalizeDictionaryText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
}
