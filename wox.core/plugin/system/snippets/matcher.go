package snippets

import (
	"sort"
	"strings"
)

// matcher retains only enough recent input to match the longest active keyword.
// Callers serialize input and reset it on focus or caret changes.
type matcher struct {
	buffer   string
	records  []Record
	maxBytes int
}

// replace rebuilds the index after local edits or Cloud Sync; ambiguous keywords stay inactive.
func (m *matcher) replace(records []Record) {
	counts := make(map[string]int, len(records))
	for _, record := range records {
		if record.Keyword != "" {
			counts[record.Keyword]++
		}
	}
	m.records = nil
	m.maxBytes = 0
	m.buffer = ""
	for _, record := range records {
		if record.Keyword == "" || counts[record.Keyword] != 1 {
			continue
		}
		m.records = append(m.records, record)
		m.maxBytes = max(m.maxBytes, len(record.Keyword))
	}
	sort.Slice(m.records, func(i, j int) bool {
		if len(m.records[i].Keyword) != len(m.records[j].Keyword) {
			return len(m.records[i].Keyword) > len(m.records[j].Keyword)
		}
		return m.records[i].ID < m.records[j].ID
	})
}

// input matches literal printable ASCII, including trailing spaces and punctuation.
func (m *matcher) input(text string) (Record, bool) {
	for _, c := range text {
		if c < 0x20 || c > 0x7e {
			m.buffer = ""
			return Record{}, false
		}
	}
	m.buffer += text
	if len(m.buffer) > m.maxBytes {
		m.buffer = m.buffer[len(m.buffer)-m.maxBytes:]
	}
	for _, record := range m.records {
		if strings.HasSuffix(m.buffer, record.Keyword) {
			m.buffer = ""
			return record, true
		}
	}
	return Record{}, false
}

func (m *matcher) backspace() {
	if len(m.buffer) > 0 {
		m.buffer = m.buffer[:len(m.buffer)-1]
	}
}

func (m *matcher) reset() { m.buffer = "" }
