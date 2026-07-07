package dictation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"wox/plugin"

	"github.com/google/uuid"
)

const (
	settingKeyDictionary       = "dictionary"
	dictionarySourceCorrection = "correction"
	dictionaryMaxEntries       = 500
)

type dictionaryEntry struct {
	ID            string `json:"id"`
	Context       string `json:"context"`
	WrongPhrase   string `json:"wrongPhrase"`
	CorrectPhrase string `json:"correctPhrase"`
	Source        string `json:"source"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	Count         int    `json:"count"`
}

type dictionaryStore struct {
	mu      sync.Mutex
	entries []dictionaryEntry
	api     dictationSettingAPI
}

func newDictionaryStore(api dictationSettingAPI) *dictionaryStore {
	return &dictionaryStore{api: api}
}

// load reads persisted dictionary entries and ignores corrupt payloads so
// dictation output never depends on optional personalization state.
func (d *dictionaryStore) load(ctx context.Context) {
	raw := d.api.GetSetting(ctx, settingKeyDictionary)
	if strings.TrimSpace(raw) == "" {
		d.entries = nil
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

// addOrUpdateCorrection records a user-approved correction with the sentence
// context that made the phrase incorrect.
func (d *dictionaryStore) addOrUpdateCorrection(ctx context.Context, contextText string, wrongPhrase string, correctPhrase string, timestamp int64) error {
	contextText = strings.TrimSpace(contextText)
	wrongPhrase = strings.TrimSpace(wrongPhrase)
	correctPhrase = strings.TrimSpace(correctPhrase)
	if contextText == "" {
		return fmt.Errorf("context is required")
	}
	if wrongPhrase == "" {
		return fmt.Errorf("wrongPhrase is required")
	}
	if correctPhrase == "" {
		return fmt.Errorf("correctPhrase is required")
	}
	if wrongPhrase == correctPhrase {
		return fmt.Errorf("wrongPhrase and correctPhrase are identical")
	}

	key := dictionaryKey(contextText, wrongPhrase)
	d.mu.Lock()
	found := -1
	for i := range d.entries {
		if dictionaryKey(d.entries[i].Context, d.entries[i].WrongPhrase) == key {
			found = i
			break
		}
	}

	if found >= 0 {
		entry := d.entries[found]
		entry.Context = contextText
		entry.WrongPhrase = wrongPhrase
		entry.CorrectPhrase = correctPhrase
		entry.Source = dictionarySourceCorrection
		entry.UpdatedAt = timestamp
		entry.Count++
		if entry.Count <= 0 {
			entry.Count = 1
		}
		d.entries[found] = entry
	} else {
		d.entries = append([]dictionaryEntry{{
			ID:            uuid.NewString(),
			Context:       contextText,
			WrongPhrase:   wrongPhrase,
			CorrectPhrase: correctPhrase,
			Source:        dictionarySourceCorrection,
			CreatedAt:     timestamp,
			UpdatedAt:     timestamp,
			Count:         1,
		}}, d.entries...)
		if len(d.entries) > dictionaryMaxEntries {
			d.entries = d.entries[:dictionaryMaxEntries]
		}
	}
	d.mu.Unlock()

	d.save(ctx)
	return nil
}

func (d *dictionaryStore) activeEntries() []dictionaryEntry {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]dictionaryEntry, 0, len(d.entries))
	for _, entry := range d.entries {
		if strings.TrimSpace(entry.Context) != "" && strings.TrimSpace(entry.WrongPhrase) != "" && strings.TrimSpace(entry.CorrectPhrase) != "" {
			out = append(out, entry)
		}
	}
	return out
}

// applyExact performs conservative post-processing when AI refinement is off:
// multi-word phrases use literal replacement, and single words require word
// boundaries so substrings inside larger words are left untouched.
func (d *dictionaryStore) applyExact(text string) string {
	if strings.TrimSpace(text) == "" {
		return text
	}
	for _, entry := range d.activeEntries() {
		if dictionaryContextMatches(text, entry.Context) {
			text = replaceExactPhrase(text, entry.WrongPhrase, entry.CorrectPhrase)
		}
	}
	return text
}

func dictionaryKey(contextText string, phrase string) string {
	return normalizeDictionaryText(contextText) + "\x00" + normalizeDictionaryText(phrase)
}

func normalizeDictionaryText(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
}

func dictionaryContextMatches(text string, contextText string) bool {
	normalizedText := normalizeDictionaryText(text)
	normalizedContext := normalizeDictionaryText(contextText)
	return normalizedContext != "" && strings.Contains(normalizedText, normalizedContext)
}

// extractCorrectionContext returns the sentence that contained the corrected
// selection. If no sentence boundary is available, it falls back to the full content.
func extractCorrectionContext(content string, selectedText string) string {
	content = strings.TrimSpace(content)
	selectedText = strings.TrimSpace(selectedText)
	if content == "" || selectedText == "" {
		return content
	}

	selectedIndex := strings.Index(content, selectedText)
	if selectedIndex < 0 {
		return content
	}

	start := 0
	for i := selectedIndex; i > 0; {
		r, size := utf8.DecodeLastRuneInString(content[:i])
		if isSentenceBoundary(r) {
			start = i
			break
		}
		i -= size
	}

	end := len(content)
	for i := selectedIndex + len(selectedText); i < len(content); {
		r, size := utf8.DecodeRuneInString(content[i:])
		i += size
		if isSentenceBoundary(r) {
			end = i
			break
		}
	}

	return strings.TrimSpace(content[start:end])
}

func isSentenceBoundary(r rune) bool {
	switch r {
	case '.', '!', '?', ';', '。', '！', '？', '；', '\n', '\r':
		return true
	default:
		return false
	}
}

func replaceExactPhrase(text string, wrongPhrase string, correctPhrase string) string {
	wrongPhrase = strings.TrimSpace(wrongPhrase)
	correctPhrase = strings.TrimSpace(correctPhrase)
	if wrongPhrase == "" || correctPhrase == "" || wrongPhrase == correctPhrase {
		return text
	}
	if strings.ContainsAny(wrongPhrase, " \t\r\n") {
		return strings.ReplaceAll(text, wrongPhrase, correctPhrase)
	}

	var b strings.Builder
	position := 0
	for {
		relativeIndex := strings.Index(text[position:], wrongPhrase)
		if relativeIndex == -1 {
			b.WriteString(text[position:])
			break
		}

		start := position + relativeIndex
		end := start + len(wrongPhrase)
		if hasWordBoundaries(text, start, end) {
			b.WriteString(text[position:start])
			b.WriteString(correctPhrase)
		} else {
			b.WriteString(text[position:end])
		}
		position = end
	}
	return b.String()
}

func hasWordBoundaries(text string, start int, end int) bool {
	if start > 0 {
		before, _ := utf8.DecodeLastRuneInString(text[:start])
		if isWordRune(before) {
			return false
		}
	}
	if end < len(text) {
		after, _ := utf8.DecodeRuneInString(text[end:])
		if isWordRune(after) {
			return false
		}
	}
	return true
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}
